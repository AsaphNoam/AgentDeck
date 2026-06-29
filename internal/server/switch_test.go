package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/messaging"
	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/transcript"
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

func waitForStatus(t *testing.T, srv *Server, id, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		st, err := srv.stateStore.ReadStatus(id)
		if err == nil && st.State == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	if st, err := srv.stateStore.ReadStatus(id); err == nil {
		t.Fatalf("status = %q, want %q", st.State, want)
	}
	t.Fatalf("status %q not reached", want)
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

// Backend swap starts a fresh native session, injects a primer, records a
// backend_switch marker, and keeps the same AgentDeck agent/archive log
// (techspec §5.3, §8.1).
func TestSwitchRuntimeBackendSwapUsesPrimer(t *testing.T) {
	srv, ts := switchTestServer(t)
	id := launchAndWaitIdle(t, ts, "impl", "tmpproj")
	resp, body := post(t, ts.URL+"/api/sessions/"+id+"/prompt", map[string]string{"text": "say hello"})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("prompt status = %d: %s", resp.StatusCode, body)
	}
	waitForStatus(t, srv, id, "idle")

	summarized := false
	srv.primerSummarizer = func(_ context.Context, req primerSummaryRequest) (string, error) {
		summarized = true
		if req.Target != "codex/gpt-5.5" || req.Backend != "codex-acp" || req.Model != "gpt-5.5" {
			t.Fatalf("summary target = %+v", req)
		}
		return "Earlier assistant helped with setup.", nil
	}

	resp, body = post(t, ts.URL+"/api/sessions/"+id+"/switch-runtime", map[string]string{"backend": "codex", "model": "gpt-5.5"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("switch status = %d: %s", resp.StatusCode, body)
	}
	var sr switchRuntimeResponse
	if err := json.Unmarshal(body, &sr); err != nil {
		t.Fatalf("switch body: %v", err)
	}
	if sr.AgentID != id || sr.Backend != "codex" || sr.Model != "gpt-5.5" || sr.HistoryHandoff != "primer" {
		t.Fatalf("unexpected switch response: %+v", sr)
	}
	if summarized {
		t.Fatal("summarizer should not be called when only the last tail turns exist")
	}
	if a, _ := srv.stateStore.ReadAgent(id); a.Backend != "codex" || a.Model != "gpt-5.5" {
		t.Fatalf("identity not switched: %+v", a)
	}
	events, err := transcript.ReadFile(srv.configStore.Home(), id, transcript.ReadOptions{IncludeMeta: true})
	if err != nil {
		t.Fatalf("read transcript: %v", err)
	}
	if !hasBackendSwitchMarker(events, "claude/sonnet-4-6", "codex/gpt-5.5") {
		t.Fatalf("missing backend_switch marker in transcript: %+v", events)
	}

	resp, body = post(t, ts.URL+"/api/sessions/"+id+"/prompt", map[string]string{"text": "continue"})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("post-switch prompt status = %d: %s", resp.StatusCode, body)
	}
}

// Regression (review fix): a chat→terminal switch must leave the TARGET's hook
// settings file present and the TARGET's MCP token usable after the 200 — the
// old-artifact cleanup must run before the target spec is composed, not after.
func TestSwitchRuntimeKeepsTargetRegistration(t *testing.T) {
	srv, ts := switchTestServer(t)
	id := launchAndWaitIdle(t, ts, "impl", "tmpproj")

	resp, body := post(t, ts.URL+"/api/sessions/"+id+"/switch-runtime", map[string]string{"interface": "terminal"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("switch status = %d: %s", resp.StatusCode, body)
	}

	// The per-agent hook settings file the terminal CLI is pointed at must exist
	// (terminal agents launch with `--settings <path>`; deleting it breaks hooks).
	settingsPath := filepath.Join(srv.configStore.Home(), "hooks", "agents", id+".json")
	if _, err := os.Stat(settingsPath); err != nil {
		t.Fatalf("target hook settings file missing after switch: %v", err)
	}

	// The MCP token written into the target's config file must still authenticate
	// (the cleanup must not have revoked the freshly-registered token).
	token := readMessagingToken(t, srv, id)
	if got, ok := srv.messaging.Lookup(token); !ok || got != id {
		t.Fatalf("target MCP token not usable: lookup(%q) = (%q, %v), want (%q, true)", token, got, ok, id)
	}
}

// readMessagingToken extracts the X-AgentDeck-Token from the agent's persisted
// MCP config file.
func readMessagingToken(t *testing.T, srv *Server, id string) string {
	t.Helper()
	path := filepath.Join(srv.configStore.Home(), "mcp", id+".mcp.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mcp config %q: %v", path, err)
	}
	var cfg struct {
		MCPServers map[string]struct {
			Headers map[string]string `json:"headers"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse mcp config: %v", err)
	}
	entry, ok := cfg.MCPServers[messagingMCPName]
	if !ok {
		t.Fatalf("mcp config missing %q entry: %s", messagingMCPName, data)
	}
	return entry.Headers[messaging.TokenHeader]
}

func hasBackendSwitchMarker(events []runtime.Event, from, to string) bool {
	for _, ev := range events {
		if ev.Type != runtime.EvBackendSwitch {
			continue
		}
		var d runtime.BackendSwitchData
		if json.Unmarshal(ev.Data, &d) == nil && d.From == from && d.To == to && d.At != "" {
			return true
		}
	}
	return false
}

// Regression (review fix): archive-resume of a terminal agent must work. A chat
// agent switched to terminal then stopped has a persisted snapshot whose frozen
// interface is still "chat"; resume must honor the LIVE identity (terminal) and
// relaunch under the terminal runtime, recording a terminal running row with
// tty/driver — not return the old 501 guard.
func TestResumeTerminalAgent(t *testing.T) {
	srv, ts := switchTestServer(t)
	id := launchAndWaitIdle(t, ts, "impl", "tmpproj")

	// Switch chat → terminal so the live identity row is terminal.
	resp, body := post(t, ts.URL+"/api/sessions/"+id+"/switch-runtime", map[string]string{"interface": "terminal"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("switch status = %d: %s", resp.StatusCode, body)
	}

	// Stop the terminal agent (archive resume is for inactive sessions).
	resp, body = post(t, ts.URL+"/api/sessions/"+id+"/stop", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stop status = %d: %s", resp.StatusCode, body)
	}

	// Resume must succeed (no longer 501) and relaunch under the terminal runtime.
	resp, body = post(t, ts.URL+"/api/sessions/"+id+"/resume", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("terminal resume status = %d, want 200: %s", resp.StatusCode, body)
	}
	run, err := srv.stateStore.ReadRunning(id)
	if err != nil {
		t.Fatalf("ReadRunning after resume: %v", err)
	}
	if run.Interface != "terminal" || run.Driver != "xterm" || run.TTY == "" {
		t.Fatalf("resumed running row not terminal/xterm/tty: %+v", run)
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
