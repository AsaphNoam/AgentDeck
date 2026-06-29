package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/config"
)

// switchTestServer launches a server wired to the fake ACP CLI (chat) and the
// "cat" PTY command (terminal), with a real project/role, and returns the live
// httptest server.
func switchTestServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	fake := buildFakeACP(t)
	t.Setenv("FAKEACP_SCENARIO", "stream_text")

	srv := testServer(t, true)
	srv.registry.Chat().SetCommand(fake)
	srv.terminal.SetCommand("cat") // harmless PTY process
	srv.terminal.SetInitialIdleDelay(10 * time.Millisecond)
	if err := srv.configStore.WriteProject("tmpproj", config.Project{Title: "Tmp", Cwd: t.TempDir()}); err != nil {
		t.Fatalf("WriteProject: %v", err)
	}
	if err := srv.configStore.WriteRole("impl", config.Role{Title: "Impl", SystemPrompt: "be helpful"}); err != nil {
		t.Fatalf("WriteRole: %v", err)
	}
	ts := httptest.NewServer(srv.routes())
	t.Cleanup(ts.Close)
	t.Cleanup(func() { srv.registry.Shutdown(context.Background()) })
	return srv, ts
}

func runningSessionID(t *testing.T, srv *Server, id string) string {
	t.Helper()
	r, err := srv.stateStore.ReadRunning(id)
	if err != nil {
		t.Fatalf("ReadRunning(%s): %v", id, err)
	}
	return r.SessionID
}

// Same-backend model swap keeps the agent_id and the persisted transcript while
// the runtime continues under a fresh native session (techspec §5.1, F7).
func TestSwitchRuntimeModelSwapSameBackend(t *testing.T) {
	srv, ts := switchTestServer(t)
	id := launchAndWaitIdle(t, ts, "impl", "tmpproj")
	first := runningSessionID(t, srv, id)

	resp, body := post(t, ts.URL+"/api/sessions/"+id+"/switch-runtime", map[string]string{"model": "opus-4-7"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("switch status = %d: %s", resp.StatusCode, body)
	}
	var sr switchRuntimeResponse
	if err := json.Unmarshal(body, &sr); err != nil {
		t.Fatalf("switch body: %v", err)
	}
	if sr.AgentID != id {
		t.Fatalf("agent_id changed: %s != %s", sr.AgentID, id)
	}
	if sr.Model != "opus-4-7" || sr.Backend != "claude" || sr.Interface != "chat" {
		t.Fatalf("identity not as expected: %+v", sr)
	}
	if sr.HistoryHandoff != "native_resume" {
		t.Fatalf("history_handoff = %q, want native_resume", sr.HistoryHandoff)
	}
	// Identity row persisted the new model; a fresh native session is running.
	if a, _ := srv.stateStore.ReadAgent(id); a.Model != "opus-4-7" {
		t.Fatalf("identity model = %q, want opus-4-7", a.Model)
	}
	if second := runningSessionID(t, srv, id); second == "" || second == first {
		t.Fatalf("expected a new session_id (was %q, now %q)", first, second)
	}
}

// chat → terminal interface swap on the same agent: the terminal runtime takes
// over, records its tty/driver in the running row, and status goes hook-driven
// (techspec §5.2). The transcript/identity survive the swap.
func TestSwitchRuntimeChatToTerminal(t *testing.T) {
	srv, ts := switchTestServer(t)
	id := launchAndWaitIdle(t, ts, "impl", "tmpproj")

	resp, body := post(t, ts.URL+"/api/sessions/"+id+"/switch-runtime", map[string]string{"interface": "terminal"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("switch status = %d: %s", resp.StatusCode, body)
	}
	var sr switchRuntimeResponse
	json.Unmarshal(body, &sr)
	if sr.Interface != "terminal" {
		t.Fatalf("interface = %q, want terminal", sr.Interface)
	}
	run, err := srv.stateStore.ReadRunning(id)
	if err != nil {
		t.Fatalf("ReadRunning: %v", err)
	}
	if run.Interface != "terminal" || run.Driver != "xterm" || run.TTY == "" {
		t.Fatalf("running row not terminal/xterm/tty: %+v", run)
	}
	if a, _ := srv.stateStore.ReadAgent(id); a.Interface != "terminal" {
		t.Fatalf("identity interface = %q, want terminal", a.Interface)
	}
}

// no_change: a switch that equals the current state is rejected 400.
func TestSwitchRuntimeNoChange(t *testing.T) {
	_, ts := switchTestServer(t)
	id := launchAndWaitIdle(t, ts, "impl", "tmpproj")
	resp, body := post(t, ts.URL+"/api/sessions/"+id+"/switch-runtime", map[string]string{"interface": "chat"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("no_change status = %d: %s", resp.StatusCode, body)
	}
}

// Rollback: when the target runtime fails to start after the old one is stopped,
// the previous runtime is restored, the identity is reverted, and the response is
// 500 switch_failed_rolled_back (techspec §5.4).
func TestSwitchRuntimeRollbackOnResumeFailure(t *testing.T) {
	srv, ts := switchTestServer(t)
	id := launchAndWaitIdle(t, ts, "impl", "tmpproj")
	// Make the terminal target fail to launch: a non-existent PTY binary.
	srv.terminal.SetCommand("/nonexistent/agentdeck-no-such-binary")

	resp, body := post(t, ts.URL+"/api/sessions/"+id+"/switch-runtime", map[string]string{"interface": "terminal"})
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("rollback status = %d, want 500: %s", resp.StatusCode, body)
	}
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	json.Unmarshal(body, &env)
	if env.Error.Code != "switch_failed_rolled_back" {
		t.Fatalf("error code = %q, want switch_failed_rolled_back (%s)", env.Error.Code, body)
	}
	// Identity reverted to chat and the previous runtime is live again.
	if a, _ := srv.stateStore.ReadAgent(id); a.Interface != "chat" {
		t.Fatalf("identity not reverted: interface = %q", a.Interface)
	}
	run, err := srv.stateStore.ReadRunning(id)
	if err != nil {
		t.Fatalf("previous runtime not restored (no running row): %v", err)
	}
	if run.Interface != "chat" {
		t.Fatalf("restored running interface = %q, want chat", run.Interface)
	}
}
