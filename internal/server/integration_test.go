package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/runtime"
)

var (
	fakeBuildOnce sync.Once
	fakeBuildPath string
)

// buildFakeACP compiles the fake ACP CLI (in the runtime package's testdata) for
// the server integration tests.
func buildFakeACP(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	out := filepath.Join(dir, "fakeacp")
	cmd := exec.Command("go", "build", "-o", out, "../runtime/testdata/fakeacp")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fakeacp: %v\n%s", err, b)
	}
	fakeBuildOnce.Do(func() { fakeBuildPath = out })
	return out
}

// sseFrame is one parsed SSE frame.
type sseFrame struct {
	event string
	data  []byte
}

// streamSSE opens the events endpoint and pushes parsed frames to a channel until
// the context is cancelled or the stream ends.
func streamSSE(t *testing.T, ctx context.Context, url string) <-chan sseFrame {
	t.Helper()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("open SSE: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("SSE status = %d", resp.StatusCode)
	}
	out := make(chan sseFrame, 64)
	go func() {
		defer resp.Body.Close()
		defer close(out)
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
		var event string
		var data []byte
		for sc.Scan() {
			line := sc.Text()
			switch {
			case strings.HasPrefix(line, "event: "):
				event = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				data = []byte(strings.TrimPrefix(line, "data: "))
			case line == "":
				if event != "" {
					select {
					case out <- sseFrame{event: event, data: data}:
					case <-ctx.Done():
						return
					}
				}
				event, data = "", nil
			}
		}
	}()
	return out
}

// waitForEventType reads message frames until one carries an Event of the given
// type, returning the decoded Event.
func waitForEventType(t *testing.T, frames <-chan sseFrame, typ string) runtime.Event {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case f, ok := <-frames:
			if !ok {
				t.Fatalf("SSE closed before %q", typ)
			}
			if f.event != "new_message" {
				continue
			}
			var env struct {
				Data json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(f.data, &env); err != nil {
				continue
			}
			var ev runtime.Event
			if err := json.Unmarshal(env.Data, &ev); err != nil {
				continue
			}
			if ev.Type == typ {
				return ev
			}
		case <-deadline:
			t.Fatalf("timed out waiting for SSE %q", typ)
		}
	}
}

func post(t *testing.T, url string, body any) (*http.Response, []byte) {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(http.MethodPost, url, rdr)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()
	data := make([]byte, 0)
	buf := bufio.NewReader(resp.Body)
	chunk := make([]byte, 4096)
	for {
		n, err := buf.Read(chunk)
		data = append(data, chunk[:n]...)
		if err != nil {
			break
		}
	}
	return resp, data
}

// TestLaunchPromptPermissionFlow drives the full HTTP surface against the fake
// CLI: POST /sessions → /api/events → prompt → permission_request → permission
// approve → sentinel created → turn_end (techspec §10.3, Appendix A).
func TestLaunchPromptPermissionFlow(t *testing.T) {
	fake := buildFakeACP(t)
	sentinel := filepath.Join(t.TempDir(), "sentinel")
	t.Setenv("FAKEACP_SCENARIO", "permission")
	t.Setenv("FAKEACP_SENTINEL", sentinel)

	srv := testServer(t, true)
	srv.registry.Chat().SetCommand(fake)
	// A project whose cwd actually exists (the fake CLI is spawned there).
	if err := srv.configStore.WriteProject("tmpproj", config.Project{Title: "Tmp", Cwd: t.TempDir()}); err != nil {
		t.Fatalf("WriteProject: %v", err)
	}
	if err := srv.configStore.WriteRole("impl", config.Role{Title: "Impl", SystemPrompt: "be helpful"}); err != nil {
		t.Fatalf("WriteRole: %v", err)
	}

	ts := httptest.NewServer(srv.routes())
	defer ts.Close()
	t.Cleanup(func() { srv.registry.Shutdown(context.Background()) })

	// Launch.
	resp, body := post(t, ts.URL+"/api/sessions", map[string]string{"role": "impl", "project": "tmpproj"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("launch status = %d: %s", resp.StatusCode, body)
	}
	var launched sessionResponse
	json.Unmarshal(body, &launched)
	agentID := launched.Agent.AgentID
	if agentID == "" || launched.Status == nil || launched.Status.State != "idle" {
		t.Fatalf("bad launch response: %s", body)
	}
	if launched.Agent.Name == "" {
		t.Fatal("expected auto-suggested name")
	}

	// Open SSE and wait for the synthetic state_update replay to confirm we are
	// subscribed before prompting.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	frames := streamSSE(t, ctx, ts.URL+"/api/events")
	select {
	case f := <-frames:
		if f.event != "state_update" {
			t.Fatalf("first SSE frame = %q, want state_update", f.event)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no state_update replay")
	}

	// Prompt → permission gate.
	resp, body = post(t, ts.URL+"/api/sessions/"+agentID+"/prompt", map[string]string{"text": "run ls"})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("prompt status = %d: %s", resp.StatusCode, body)
	}

	pr := waitForEventType(t, frames, "permission_request")
	var prd runtime.PermissionRequestData
	json.Unmarshal(pr.Data, &prd)
	if fileExists(sentinel) {
		t.Fatal("sentinel exists before approval")
	}

	// Approve.
	resp, body = post(t, ts.URL+"/api/sessions/"+agentID+"/permission",
		map[string]string{"tool_call_id": prd.ToolCallID, "decision": "approve"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("permission status = %d: %s", resp.StatusCode, body)
	}

	waitForEventType(t, frames, "turn_end")
	if !fileExists(sentinel) {
		t.Fatal("sentinel missing after approve — tool did not run")
	}

	resp, body = func() (*http.Response, []byte) {
		r, _ := http.Get(ts.URL + "/api/sessions/" + agentID + "/transcript")
		defer r.Body.Close()
		b := make([]byte, 8192)
		n, _ := r.Body.Read(b)
		return r, b[:n]
	}()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("transcript status = %d: %s", resp.StatusCode, body)
	}
	if !bytes.Contains(body, []byte(`"events"`)) || !bytes.Contains(body, []byte(`"permission_request"`)) {
		t.Fatalf("transcript body missing retained events: %s", body)
	}

	// GET detail reflects the agent.
	resp, body = func() (*http.Response, []byte) {
		r, _ := http.Get(ts.URL + "/api/sessions/" + agentID)
		defer r.Body.Close()
		b := make([]byte, 4096)
		n, _ := r.Body.Read(b)
		return r, b[:n]
	}()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("detail status = %d: %s", resp.StatusCode, body)
	}
}

// TestLaunchValidationErrors covers the §7.7 error envelope on bad input.
func TestLaunchValidationErrors(t *testing.T) {
	srv := testServer(t, true)
	ts := httptest.NewServer(srv.routes())
	defer ts.Close()

	resp, body := post(t, ts.URL+"/api/sessions", map[string]string{"role": "nope", "project": "nope"})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("unknown role status = %d, want 422: %s", resp.StatusCode, body)
	}
	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	json.Unmarshal(body, &env)
	if env.Error.Code != "validation" {
		t.Fatalf("error code = %q, want validation: %s", env.Error.Code, body)
	}

	// Unknown agent control → 404.
	resp, _ = post(t, ts.URL+"/api/sessions/a_missing/prompt", map[string]string{"text": "hi"})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("prompt unknown agent status = %d, want 404", resp.StatusCode)
	}
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }
