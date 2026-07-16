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
	"github.com/agentdeck/agentdeck/internal/state"
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
// FS-03.A3: prompt → permission → approval → durable transcript.
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
		r, err := http.Get(ts.URL + "/api/sessions/" + agentID + "/transcript")
		if err != nil {
			t.Fatalf("get transcript: %v", err)
		}
		defer r.Body.Close()
		b := make([]byte, 8192)
		n, _ := r.Body.Read(b)
		return r, b[:n]
	}()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("transcript status = %d: %s", resp.StatusCode, body)
	}
	if !bytes.Contains(body, []byte(`"events"`)) || !bytes.Contains(body, []byte(`"permission_request"`)) || !bytes.Contains(body, []byte(`"permission_resolved"`)) {
		t.Fatalf("transcript body missing retained events: %s", body)
	}
	if _, err := os.Stat(filepath.Join(srv.configStore.Home(), "sessions", agentID, "transcript.ndjson")); err != nil {
		t.Fatalf("persisted transcript missing: %v", err)
	}

	// GET detail reflects the agent.
	resp, body = func() (*http.Response, []byte) {
		r, err := http.Get(ts.URL + "/api/sessions/" + agentID)
		if err != nil {
			t.Fatalf("get session detail: %v", err)
		}
		defer r.Body.Close()
		b := make([]byte, 4096)
		n, _ := r.Body.Read(b)
		return r, b[:n]
	}()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("detail status = %d: %s", resp.StatusCode, body)
	}
}

func TestCrashMidTurnPersistsDeliveredTranscript(t *testing.T) {
	fake := buildFakeACP(t)
	t.Setenv("FAKEACP_SCENARIO", "crash_midturn")

	srv := testServer(t, true)
	srv.registry.Chat().SetCommand(fake)
	if err := srv.configStore.WriteProject("tmpproj", config.Project{Title: "Tmp", Cwd: t.TempDir()}); err != nil {
		t.Fatalf("WriteProject: %v", err)
	}
	if err := srv.configStore.WriteRole("impl", config.Role{Title: "Impl", SystemPrompt: "be helpful"}); err != nil {
		t.Fatalf("WriteRole: %v", err)
	}

	ts := httptest.NewServer(srv.routes())
	defer ts.Close()
	t.Cleanup(func() { srv.registry.Shutdown(context.Background()) })

	resp, body := post(t, ts.URL+"/api/sessions", map[string]string{"role": "impl", "project": "tmpproj"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("launch status = %d: %s", resp.StatusCode, body)
	}
	var launched sessionResponse
	json.Unmarshal(body, &launched)
	agentID := launched.Agent.AgentID

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	frames := streamSSE(t, ctx, ts.URL+"/api/events")
	<-frames // initial state_update replay

	resp, body = post(t, ts.URL+"/api/sessions/"+agentID+"/prompt", map[string]string{"text": "crash"})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("prompt status = %d: %s", resp.StatusCode, body)
	}
	waitForEventType(t, frames, "assistant_text")
	waitForEventType(t, frames, "turn_end")

	r, err := http.Get(ts.URL + "/api/sessions/" + agentID + "/transcript")
	if err != nil {
		t.Fatalf("GET transcript: %v", err)
	}
	defer r.Body.Close()
	data := make([]byte, 8192)
	n, _ := r.Body.Read(data)
	body = data[:n]
	if r.StatusCode != http.StatusOK {
		t.Fatalf("transcript status = %d: %s", r.StatusCode, body)
	}
	if !bytes.Contains(body, []byte("about to crash")) {
		t.Fatalf("transcript missing pre-crash delivered text: %s", body)
	}
	if !bytes.Contains(body, []byte(`"type":"user_text"`)) || !bytes.Contains(body, []byte(`"text":"crash"`)) {
		t.Fatalf("transcript missing durable user prompt: %s", body)
	}
	raw, err := os.ReadFile(filepath.Join(srv.configStore.Home(), "sessions", agentID, "transcript.ndjson"))
	if err != nil {
		t.Fatalf("read raw transcript: %v", err)
	}
	if !bytes.Contains(raw, []byte("about to crash")) {
		t.Fatalf("raw transcript missing pre-crash delivered text: %s", raw)
	}
	if !bytes.Contains(raw, []byte(`"type":"user_text"`)) {
		t.Fatalf("raw transcript missing durable user prompt: %s", raw)
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

// launchAndWaitIdle launches an agent and waits for the idle state_update before returning.
func launchAndWaitIdle(t *testing.T, ts *httptest.Server, role, project string) string {
	t.Helper()
	resp, body := post(t, ts.URL+"/api/sessions", map[string]string{"role": role, "project": project})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("launch status = %d: %s", resp.StatusCode, body)
	}
	var launched sessionResponse
	json.Unmarshal(body, &launched)
	if launched.Agent.AgentID == "" {
		t.Fatalf("bad launch response: %s", body)
	}
	return launched.Agent.AgentID
}

// TestResumeHappyPath exercises the full stop→resume lifecycle:
//   - agent_id unchanged after resume
//   - running.session_id is different (new ACP session)
//   - GET /transcript?include_meta=true contains prior events + session_meta with resumed_at
//   - resumed:true in resume response (techspec §11.3)
func TestResumeHappyPath(t *testing.T) {
	fake := buildFakeACP(t)
	t.Setenv("FAKEACP_SCENARIO", "stream_text")

	srv := testServer(t, true)
	srv.registry.Chat().SetCommand(fake)
	if err := srv.configStore.WriteProject("tmpproj", config.Project{Title: "Tmp", Cwd: t.TempDir()}); err != nil {
		t.Fatalf("WriteProject: %v", err)
	}
	if err := srv.configStore.WriteRole("impl", config.Role{Title: "Impl", SystemPrompt: "be helpful"}); err != nil {
		t.Fatalf("WriteRole: %v", err)
	}

	ts := httptest.NewServer(srv.routes())
	defer ts.Close()
	t.Cleanup(func() { srv.registry.Shutdown(context.Background()) })

	// 1. Launch and open SSE stream.
	agentID := launchAndWaitIdle(t, ts, "impl", "tmpproj")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	frames := streamSSE(t, ctx, ts.URL+"/api/events")
	select {
	case <-frames:
	case <-time.After(3 * time.Second):
		t.Fatal("no initial state_update")
	}

	// 2. Send a prompt and wait for turn_end so the transcript has events.
	resp, body := post(t, ts.URL+"/api/sessions/"+agentID+"/prompt", map[string]string{"text": "hello"})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("prompt status = %d: %s", resp.StatusCode, body)
	}
	waitForEventType(t, frames, "turn_end")

	// 3. Capture first running.session_id from GET /api/sessions.
	firstSessionID := func() string {
		r, err := http.Get(ts.URL + "/api/sessions")
		if err != nil {
			t.Fatalf("get sessions: %v", err)
		}
		defer r.Body.Close()
		var sessions []struct {
			AgentID string `json:"agent_id"`
			Running *struct {
				SessionID string `json:"session_id"`
			} `json:"running,omitempty"`
		}
		json.NewDecoder(r.Body).Decode(&sessions)
		for _, s := range sessions {
			if s.AgentID == agentID && s.Running != nil {
				return s.Running.SessionID
			}
		}
		return ""
	}()
	if firstSessionID == "" {
		t.Fatal("could not get first session_id from /api/sessions")
	}

	// 4. Stop the agent.
	resp, body = post(t, ts.URL+"/api/sessions/"+agentID+"/stop", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stop status = %d: %s", resp.StatusCode, body)
	}

	// 5. Resume.
	resp, body = post(t, ts.URL+"/api/sessions/"+agentID+"/resume", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("resume status = %d: %s", resp.StatusCode, body)
	}
	var resumed struct {
		Agent struct {
			AgentID string `json:"agent_id"`
		} `json:"agent"`
		Running *struct {
			SessionID string `json:"session_id"`
		} `json:"running"`
		Resumed bool `json:"resumed"`
	}
	if err := json.Unmarshal(body, &resumed); err != nil {
		t.Fatalf("resume body JSON: %v", err)
	}
	if resumed.Agent.AgentID != agentID {
		t.Fatalf("resume agent_id = %q, want %q", resumed.Agent.AgentID, agentID)
	}
	if !resumed.Resumed {
		t.Fatal("resume response: resumed = false, want true")
	}
	if resumed.Running == nil || resumed.Running.SessionID == "" {
		t.Fatalf("resume response missing running.session_id: %s", body)
	}
	if resumed.Running.SessionID == firstSessionID {
		t.Fatalf("resume running.session_id = %q, want different from first %q", resumed.Running.SessionID, firstSessionID)
	}

	// 6. GET /transcript?include_meta=true: prior events + new session_meta with resumed_at.
	r, err := http.Get(ts.URL + "/api/sessions/" + agentID + "/transcript?include_meta=true")
	if err != nil {
		t.Fatalf("get transcript with metadata: %v", err)
	}
	defer r.Body.Close()
	var txBody []byte
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Body.Read(buf)
		txBody = append(txBody, buf[:n]...)
		if err != nil {
			break
		}
	}
	if r.StatusCode != http.StatusOK {
		t.Fatalf("transcript status = %d: %s", r.StatusCode, txBody)
	}
	// Should contain two session_meta entries (launch + resume).
	if count := bytes.Count(txBody, []byte(`"session_meta"`)); count < 2 {
		t.Fatalf("transcript has %d session_meta events, want >=2: %s", count, txBody)
	}
	// The resume session_meta must carry resumed_at.
	if !bytes.Contains(txBody, []byte(`"resumed_at"`)) {
		t.Fatalf("transcript missing resumed_at field: %s", txBody)
	}
	// Prior events (assistant_text from the stream_text scenario) must be present.
	if !bytes.Contains(txBody, []byte("Sure")) {
		t.Fatalf("transcript missing pre-stop assistant_text content: %s", txBody)
	}

	// 7. After resume, send a prompt; seq must continue monotonically past the pre-resume max.
	frames2 := streamSSE(t, ctx, ts.URL+"/api/events")
	select {
	case <-frames2:
	case <-time.After(3 * time.Second):
		t.Fatal("no state_update after resume SSE open")
	}
	resp, body = post(t, ts.URL+"/api/sessions/"+agentID+"/prompt", map[string]string{"text": "hello again"})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("post-resume prompt status = %d: %s", resp.StatusCode, body)
	}
	waitForEventType(t, frames2, "turn_end")

	// Read transcript again and verify seq is monotonically increasing.
	r2, err := http.Get(ts.URL + "/api/sessions/" + agentID + "/transcript?include_meta=true")
	if err != nil {
		t.Fatalf("get resumed transcript with metadata: %v", err)
	}
	defer r2.Body.Close()
	var txBody2 []byte
	for {
		n, err := r2.Body.Read(buf)
		txBody2 = append(txBody2, buf[:n]...)
		if err != nil {
			break
		}
	}
	var txResp struct {
		Events []struct {
			Seq int64 `json:"seq"`
		} `json:"events"`
	}
	if err := json.Unmarshal(txBody2, &txResp); err != nil {
		t.Fatalf("transcript JSON: %v", err)
	}
	for i := 1; i < len(txResp.Events); i++ {
		if txResp.Events[i].Seq <= txResp.Events[i-1].Seq {
			t.Fatalf("seq not monotonic at index %d: seq[%d]=%d <= seq[%d]=%d",
				i, i, txResp.Events[i].Seq, i-1, txResp.Events[i-1].Seq)
		}
	}
}

// TestResumeAlreadyRunning returns 409 when the agent is still active.
func TestResumeAlreadyRunning(t *testing.T) {
	fake := buildFakeACP(t)
	t.Setenv("FAKEACP_SCENARIO", "stream_text")

	srv := testServer(t, true)
	srv.registry.Chat().SetCommand(fake)
	if err := srv.configStore.WriteProject("tmpproj", config.Project{Title: "Tmp", Cwd: t.TempDir()}); err != nil {
		t.Fatalf("WriteProject: %v", err)
	}
	if err := srv.configStore.WriteRole("impl", config.Role{Title: "Impl", SystemPrompt: "be helpful"}); err != nil {
		t.Fatalf("WriteRole: %v", err)
	}

	ts := httptest.NewServer(srv.routes())
	defer ts.Close()
	t.Cleanup(func() { srv.registry.Shutdown(context.Background()) })

	agentID := launchAndWaitIdle(t, ts, "impl", "tmpproj")

	// Agent is still running — resume must 409.
	resp, body := post(t, ts.URL+"/api/sessions/"+agentID+"/resume", nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("resume while running: status = %d, want 409: %s", resp.StatusCode, body)
	}
	var errEnv struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	json.Unmarshal(body, &errEnv)
	if errEnv.Error.Code != "conflict" {
		t.Fatalf("error code = %q, want conflict", errEnv.Error.Code)
	}
}

// TestResumeNoPersistedSession returns 422 when there is no sessions row.
func TestResumeNoPersistedSession(t *testing.T) {
	srv := testServer(t, true)

	ts := httptest.NewServer(srv.routes())
	defer ts.Close()

	// Write an agent row but no sessions row.
	if err := srv.stateStore.WriteAgent(mustAgent()); err != nil {
		t.Fatalf("WriteAgent: %v", err)
	}

	resp, body := post(t, ts.URL+"/api/sessions/a_nopersist/resume", nil)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("resume no session: status = %d, want 422: %s", resp.StatusCode, body)
	}
	var errEnv struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	json.Unmarshal(body, &errEnv)
	if errEnv.Error.Code != "validation" {
		t.Fatalf("error code = %q, want validation", errEnv.Error.Code)
	}
}

// TestResumeUnknownAgent returns 404 for an unknown agent_id.
func TestResumeUnknownAgent(t *testing.T) {
	srv := testServer(t, true)
	ts := httptest.NewServer(srv.routes())
	defer ts.Close()

	resp, body := post(t, ts.URL+"/api/sessions/a_ghost/resume", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("resume unknown: status = %d, want 404: %s", resp.StatusCode, body)
	}
}

// TestStopIdempotent verifies that stopping an already-stopped (but known)
// agent reads as success, while an unknown id still 404s.
func TestStopIdempotent(t *testing.T) {
	fake := buildFakeACP(t)
	t.Setenv("FAKEACP_SCENARIO", "stream_text")

	srv := testServer(t, true)
	srv.registry.Chat().SetCommand(fake)
	if err := srv.configStore.WriteProject("tmpproj", config.Project{Title: "Tmp", Cwd: t.TempDir()}); err != nil {
		t.Fatalf("WriteProject: %v", err)
	}
	if err := srv.configStore.WriteRole("impl", config.Role{Title: "Impl", SystemPrompt: "be helpful"}); err != nil {
		t.Fatalf("WriteRole: %v", err)
	}

	ts := httptest.NewServer(srv.routes())
	defer ts.Close()
	t.Cleanup(func() { srv.registry.Shutdown(context.Background()) })

	agentID := launchAndWaitIdle(t, ts, "impl", "tmpproj")

	// First stop succeeds.
	resp, body := post(t, ts.URL+"/api/sessions/"+agentID+"/stop", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("first stop status = %d, want 200: %s", resp.StatusCode, body)
	}

	// Repeated stop on the known-but-stopped agent is idempotent success.
	resp, body = post(t, ts.URL+"/api/sessions/"+agentID+"/stop", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("second stop status = %d, want 200 (idempotent): %s", resp.StatusCode, body)
	}
	var stopped struct {
		Stopped bool `json:"stopped"`
	}
	json.Unmarshal(body, &stopped)
	if !stopped.Stopped {
		t.Fatalf("second stop body = %s, want stopped:true", body)
	}

	// Unknown id still 404s.
	resp, _ = post(t, ts.URL+"/api/sessions/a_ghost/stop", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("stop unknown agent status = %d, want 404", resp.StatusCode)
	}
}

// TestStopRemovesHookSettings verifies the per-agent hook settings file written
// at launch is deleted on stop, so the registration artifact is not orphaned
// (review fix: shared lifecycle with the messaging MCP registration).
func TestStopRemovesHookSettings(t *testing.T) {
	fake := buildFakeACP(t)
	t.Setenv("FAKEACP_SCENARIO", "stream_text")

	srv := testServer(t, true)
	srv.registry.Chat().SetCommand(fake)
	if err := srv.configStore.WriteProject("tmpproj", config.Project{Title: "Tmp", Cwd: t.TempDir()}); err != nil {
		t.Fatalf("WriteProject: %v", err)
	}
	if err := srv.configStore.WriteRole("impl", config.Role{Title: "Impl", SystemPrompt: "be helpful"}); err != nil {
		t.Fatalf("WriteRole: %v", err)
	}

	ts := httptest.NewServer(srv.routes())
	defer ts.Close()
	t.Cleanup(func() { srv.registry.Shutdown(context.Background()) })

	agentID := launchAndWaitIdle(t, ts, "impl", "tmpproj")

	// The settings file is written at launch regardless of interface.
	settingsPath := filepath.Join(srv.configStore.Home(), "hooks", "agents", agentID+".json")
	if _, err := os.Stat(settingsPath); err != nil {
		t.Fatalf("settings file not written at launch: %v", err)
	}

	resp, body := post(t, ts.URL+"/api/sessions/"+agentID+"/stop", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stop status = %d, want 200: %s", resp.StatusCode, body)
	}
	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Fatalf("settings file still present after stop (err=%v); want removed", err)
	}
}

func mustAgent() state.Agent {
	return state.Agent{
		AgentID: "a_nopersist", Name: "Ghost", Role: "impl", Project: "tmpproj",
		Backend: "claude", Model: "sonnet", Interface: "chat",
		CreatedAt: time.Now(),
	}
}
