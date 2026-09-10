package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/config"
)

// busyChatServer launches one chat agent on the hold_turn scenario and leaves a
// turn open, which is the state the queue and steer routes are defined against.
// It returns the agent id and a release func that ends the open turn.
func busyChatServer(t *testing.T, env map[string]string) (*Server, *httptest.Server, string, func()) {
	t.Helper()
	fake := buildFakeACP(t)
	holdFile := filepath.Join(t.TempDir(), "hold")
	t.Setenv("FAKEACP_SCENARIO", "hold_turn")
	t.Setenv("FAKEACP_HOLD_FILE", holdFile)
	for k, v := range env {
		t.Setenv(k, v)
	}

	srv := testServer(t, true)
	srv.registry.Chat().SetCommand(fake)
	if err := srv.configStore.WriteProject("tmpproj", config.Project{Title: "Tmp", Cwd: t.TempDir()}); err != nil {
		t.Fatalf("WriteProject: %v", err)
	}
	if err := srv.configStore.WriteRole("impl", config.Role{Title: "Impl", SystemPrompt: "be helpful"}); err != nil {
		t.Fatalf("WriteRole: %v", err)
	}
	ts := httptest.NewServer(srv.routes())
	release := func() { _ = os.WriteFile(holdFile, []byte("go"), 0o644) }
	t.Cleanup(ts.Close)
	t.Cleanup(func() {
		release()
		srv.registry.Shutdown(context.Background())
	})

	id := launchAndWaitIdle(t, ts, "impl", "tmpproj")
	resp, body := post(t, ts.URL+"/api/sessions/"+id+"/prompt", map[string]string{"text": "first"})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("first prompt status = %d: %s", resp.StatusCode, body)
	}
	waitStatus(t, srv, id, "busy")
	return srv, ts, id, release
}

// waitStatus polls the durable status row until the agent reaches want.
func waitStatus(t *testing.T, srv *Server, id, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		st, err := srv.stateStore.ReadStatus(id)
		if err == nil && st.State == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for status %q (last %+v err %v)", want, st, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// deleteJSON issues a DELETE and returns the response and body.
func deleteJSON(t *testing.T, url string) (*http.Response, []byte) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodDelete, url, bytes.NewReader(nil))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", url, err)
	}
	defer resp.Body.Close()
	body := make([]byte, 0)
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		body = append(body, buf[:n]...)
		if err != nil {
			break
		}
	}
	return resp, body
}

func fieldOf(t *testing.T, body []byte, key string) string {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	value, _ := out[key].(string)
	return value
}

// TestPromptRouteHoldsInsteadOfConflicting is TS-03.R38's contract: the person's
// prompt to a busy chat agent stops being a 409 and becomes the same 202 with a
// field naming what happened, so a client renders pending without inferring it
// from agent status.
func TestPromptRouteHoldsInsteadOfConflicting(t *testing.T) {
	srv, ts, id, release := busyChatServer(t, nil)

	resp, body := post(t, ts.URL+"/api/sessions/"+id+"/prompt", map[string]string{"text": "follow up"})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("prompt to a busy agent = %d: %s, want 202", resp.StatusCode, body)
	}
	if got := fieldOf(t, body, "delivery"); got != "held" {
		t.Fatalf("delivery = %q, want held", got)
	}
	var accepted map[string]any
	if err := json.Unmarshal(body, &accepted); err != nil {
		t.Fatalf("decode held response: %v", err)
	}
	boundary, ok := accepted["after_seq"].(float64)
	if !ok || boundary <= 0 {
		t.Fatalf("after_seq = %#v, want the acceptance boundary", accepted["after_seq"])
	}

	getResp, err := http.Get(ts.URL + "/api/sessions/" + id + "/prompt")
	if err != nil {
		t.Fatalf("GET held prompt: %v", err)
	}
	defer getResp.Body.Close()
	var snapshot struct {
		Text     string `json:"text"`
		AfterSeq int64  `json:"after_seq"`
	}
	if err := json.NewDecoder(getResp.Body).Decode(&snapshot); err != nil {
		t.Fatalf("decode held snapshot: %v", err)
	}
	if snapshot.Text != "follow up" || snapshot.AfterSeq != int64(boundary) {
		t.Fatalf("held snapshot = %+v, want text and boundary %v", snapshot, boundary)
	}

	// Withdraw before releasing so the open turn is the last one: a released hold
	// would start its own turn, and the brief idle between the two is not the
	// settled state this half of the contract is about.
	if resp, body := deleteJSON(t, ts.URL+"/api/sessions/"+id+"/prompt"); resp.StatusCode != http.StatusOK {
		t.Fatalf("withdraw status = %d: %s", resp.StatusCode, body)
	}
	release()
	waitStatus(t, srv, id, "idle")
	resp, body = post(t, ts.URL+"/api/sessions/"+id+"/prompt", map[string]string{"text": "now"})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("prompt to an idle agent = %d: %s", resp.StatusCode, body)
	}
	if got := fieldOf(t, body, "delivery"); got != "sent" {
		t.Fatalf("delivery to an idle agent = %q, want sent", got)
	}
}

// TestWithdrawPromptIsIdempotent covers TS-03.R38's DELETE: it is the
// resource-shaped spelling of the one operation the hold adds, and withdrawing
// nothing returns the same body rather than an error.
func TestWithdrawPromptIsIdempotent(t *testing.T) {
	_, ts, id, _ := busyChatServer(t, nil)

	if resp, body := post(t, ts.URL+"/api/sessions/"+id+"/prompt", map[string]string{"text": "oops"}); resp.StatusCode != http.StatusAccepted {
		t.Fatalf("hold status = %d: %s", resp.StatusCode, body)
	}
	for attempt := 0; attempt < 2; attempt++ {
		resp, body := deleteJSON(t, ts.URL+"/api/sessions/"+id+"/prompt")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("withdraw attempt %d = %d: %s", attempt, resp.StatusCode, body)
		}
		if got := fieldOf(t, body, "delivery"); got != "withdrawn" {
			t.Fatalf("withdraw attempt %d delivery = %q, want withdrawn", attempt, got)
		}
	}
}

// TestSteerRouteReportsTheAdapterOutcome is TS-03.R39's happy path against an
// adapter that advertises steering, plus the availability flag clients gate the
// control on.
func TestSteerRouteReportsTheAdapterOutcome(t *testing.T) {
	srv, ts, id, _ := busyChatServer(t, map[string]string{"FAKEACP_STEERING": "1"})

	if r, err := srv.stateStore.ReadRunning(id); err != nil || !r.SteeringAvailable {
		t.Fatalf("running steering_available = %v err %v, want true", r.SteeringAvailable, err)
	}
	update, err := srv.stateMgr.Touch(id)
	if err != nil {
		t.Fatalf("Touch: %v", err)
	}
	if !update.SteeringAvailable {
		t.Fatal("agent payload steering_available = false, want the live advertisement")
	}

	resp, body := post(t, ts.URL+"/api/sessions/"+id+"/steer", map[string]string{"text": "other file"})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("steer status = %d: %s", resp.StatusCode, body)
	}
	if got := fieldOf(t, body, "outcome"); got != "steered" {
		t.Fatalf("outcome = %q, want steered", got)
	}
}

// TestSteerRouteConflictsWhereThereIsNothingToSteer covers TS-03.R39's two
// conflicts: an adapter that does not advertise the extension, and an empty steer
// with no held message.
func TestSteerRouteConflictsWhereThereIsNothingToSteer(t *testing.T) {
	t.Run("unadvertised adapter", func(t *testing.T) {
		srv, ts, id, _ := busyChatServer(t, nil)
		if r, err := srv.stateStore.ReadRunning(id); err != nil || r.SteeringAvailable {
			t.Fatalf("running steering_available = %v err %v, want false", r.SteeringAvailable, err)
		}
		resp, body := post(t, ts.URL+"/api/sessions/"+id+"/steer", map[string]string{"text": "hi"})
		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("steer status = %d: %s, want 409", resp.StatusCode, body)
		}
	})

	t.Run("empty steer with nothing held", func(t *testing.T) {
		_, ts, id, _ := busyChatServer(t, map[string]string{"FAKEACP_STEERING": "1"})
		resp, body := post(t, ts.URL+"/api/sessions/"+id+"/steer", map[string]string{"text": ""})
		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("empty steer status = %d: %s, want 409", resp.StatusCode, body)
		}
	})
}

// TestSteerRoutePromotesTheHeldMessage is TS-03.R39's promotion path end to end:
// one request takes the held message and delivers it, so nothing can be dropped
// between a withdraw and a steer.
func TestSteerRoutePromotesTheHeldMessage(t *testing.T) {
	_, ts, id, _ := busyChatServer(t, map[string]string{"FAKEACP_STEERING": "1"})

	if resp, body := post(t, ts.URL+"/api/sessions/"+id+"/prompt", map[string]string{"text": "changed my mind"}); resp.StatusCode != http.StatusAccepted {
		t.Fatalf("hold status = %d: %s", resp.StatusCode, body)
	}
	resp, body := post(t, ts.URL+"/api/sessions/"+id+"/steer", map[string]string{"text": ""})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("promoting steer status = %d: %s", resp.StatusCode, body)
	}
	if got := fieldOf(t, body, "outcome"); got != "steered" {
		t.Fatalf("outcome = %q, want steered", got)
	}
	// The hold is gone, so a second promotion has nothing to send.
	resp, body = post(t, ts.URL+"/api/sessions/"+id+"/steer", map[string]string{"text": ""})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("second promoting steer = %d: %s, want 409", resp.StatusCode, body)
	}
}
