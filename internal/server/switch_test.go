package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/hooks"
	"github.com/agentdeck/agentdeck/internal/messaging"
	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/runtime/terminal"
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

// Regression (review fix): the backend-swap history primer is a one-shot context
// injection for the new backend — it must NOT bake into the frozen
// sessions.system_prompt snapshot, and two successive primer switches must not
// stack primers (each primes from the clean pre-primer base). Before the fix,
// switch.go overwrote spec.SystemPrompt with the primer, which flowed to
// UpsertSessionMeta (system_prompt=excluded) and permanently reframed the frozen
// snapshot; the next switch then read the already-primed prompt as its base.
func TestSwitchRuntimePrimerKeepsFrozenSystemPrompt(t *testing.T) {
	srv, ts := switchTestServer(t)
	id := launchAndWaitIdle(t, ts, "impl", "tmpproj")

	// The frozen snapshot at launch: joinSystemPrompt(project.ContextPrompt="",
	// role "impl".SystemPrompt="be helpful") == "be helpful".
	original, err := srv.stateStore.ReadSession(id)
	if err != nil {
		t.Fatalf("ReadSession(original): %v", err)
	}
	if original.SystemPrompt != "be helpful" {
		t.Fatalf("unexpected original system_prompt: %q", original.SystemPrompt)
	}

	const primerMark = "AgentDeck backend-switch history primer."

	// Switch #1: claude → codex is a cross-backend swap → primer path.
	resp, body := post(t, ts.URL+"/api/sessions/"+id+"/switch-runtime", map[string]string{"backend": "codex", "model": "gpt-5.5"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("switch#1 status = %d: %s", resp.StatusCode, body)
	}
	var sr switchRuntimeResponse
	if err := json.Unmarshal(body, &sr); err != nil {
		t.Fatalf("switch#1 body: %v", err)
	}
	if sr.HistoryHandoff != "primer" {
		t.Fatalf("switch#1 history_handoff = %q, want primer", sr.HistoryHandoff)
	}

	// The persisted (frozen) snapshot must be unchanged — no primer text.
	afterFirst, err := srv.stateStore.ReadSession(id)
	if err != nil {
		t.Fatalf("ReadSession(after#1): %v", err)
	}
	if afterFirst.SystemPrompt != original.SystemPrompt {
		t.Fatalf("frozen system_prompt mutated after primer switch:\n got: %q\nwant: %q", afterFirst.SystemPrompt, original.SystemPrompt)
	}
	if strings.Contains(afterFirst.SystemPrompt, primerMark) {
		t.Fatalf("primer text leaked into frozen system_prompt: %q", afterFirst.SystemPrompt)
	}

	// Switch #2: codex → claude is again cross-backend → primer path. If the frozen
	// snapshot had absorbed the first primer, this switch would prime primer-on-
	// primer; with the fix it primes from the clean base and the snapshot stays clean.
	resp, body = post(t, ts.URL+"/api/sessions/"+id+"/switch-runtime", map[string]string{"backend": "claude", "model": "sonnet-4-6"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("switch#2 status = %d: %s", resp.StatusCode, body)
	}
	if err := json.Unmarshal(body, &sr); err != nil {
		t.Fatalf("switch#2 body: %v", err)
	}
	if sr.HistoryHandoff != "primer" {
		t.Fatalf("switch#2 history_handoff = %q, want primer", sr.HistoryHandoff)
	}

	afterSecond, err := srv.stateStore.ReadSession(id)
	if err != nil {
		t.Fatalf("ReadSession(after#2): %v", err)
	}
	if afterSecond.SystemPrompt != original.SystemPrompt {
		t.Fatalf("frozen system_prompt mutated after second primer switch:\n got: %q\nwant: %q", afterSecond.SystemPrompt, original.SystemPrompt)
	}
	if strings.Contains(afterSecond.SystemPrompt, primerMark) {
		t.Fatalf("primer text leaked into frozen system_prompt after two switches: %q", afterSecond.SystemPrompt)
	}
	// And the primer is never stacked: the base a switch primes from is the frozen
	// snapshot, so it can carry at most one primer marker — never two.
	if n := strings.Count(afterSecond.SystemPrompt, primerMark); n != 0 {
		t.Fatalf("primer marker count in frozen snapshot = %d, want 0", n)
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

// Regression (review fix): an agent CRASH (unsolicited process exit) must tear
// down its registration artifacts, not just registry ownership. Before the fix
// only registry.forget ran on the crash path, so the hook token + MCP session +
// on-disk mcp/hook files leaked — leaving a spoofable messaging identity a
// lingering child could still send/check as.
func TestCrashTearsDownAgentRegistration(t *testing.T) {
	srv, ts := switchTestServer(t)
	id := launchAndWaitIdle(t, ts, "impl", "tmpproj")

	token := readMessagingToken(t, srv, id)
	if got, ok := srv.messaging.Lookup(token); !ok || got != id {
		t.Fatalf("precondition: token should resolve to %s, got (%q,%v)", id, got, ok)
	}
	mcpPath := filepath.Join(srv.configStore.Home(), "mcp", id+".mcp.json")
	hookPath := filepath.Join(srv.configStore.Home(), "hooks", "agents", id+".json")
	if _, err := os.Stat(mcpPath); err != nil {
		t.Fatalf("precondition mcp file: %v", err)
	}
	if _, err := os.Stat(hookPath); err != nil {
		t.Fatalf("precondition hook file: %v", err)
	}

	run, err := srv.stateStore.ReadRunning(id)
	if err != nil || run.PID <= 0 {
		t.Fatalf("ReadRunning for pid: %+v err=%v", run, err)
	}

	// Simulate a crash: hard-kill the agent process outside stop/switch.
	if err := syscall.Kill(run.PID, syscall.SIGKILL); err != nil {
		t.Fatalf("kill agent pid %d: %v", run.PID, err)
	}

	// The crash exit path (onExit → teardownAgentRegistration) must revoke the
	// token and remove the on-disk artifacts.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		_, ok := srv.messaging.Lookup(token)
		_, mcpErr := os.Stat(mcpPath)
		_, hookErr := os.Stat(hookPath)
		if !ok && os.IsNotExist(mcpErr) && os.IsNotExist(hookErr) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	_, ok := srv.messaging.Lookup(token)
	_, mcpErr := os.Stat(mcpPath)
	_, hookErr := os.Stat(hookPath)
	t.Fatalf("registration not torn down after crash: tokenResolves=%v mcpStat=%v hookStat=%v", ok, mcpErr, hookErr)
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

// Regression (review fix): both the resume and switch-runtime LaunchSpec
// composers must re-resolve role/project-derived fields (skip_permissions,
// add_dirs) from current config — the frozen snapshot doesn't persist them, so
// without this a skip_permissions=true / multi-dir agent silently loses
// auto-approval and its extra directories after any stop→resume or switch.
func TestResumeAndSwitchCarryRoleAndProjectFields(t *testing.T) {
	srv, ts := switchTestServer(t)

	// Re-declare the role with skip_permissions=true and the project with an add_dir.
	skip := true
	cwd := t.TempDir()
	extra := t.TempDir()
	if err := srv.configStore.WriteRole("impl", config.Role{Title: "Impl", SystemPrompt: "be helpful", SkipPermissions: &skip}); err != nil {
		t.Fatalf("WriteRole: %v", err)
	}
	if err := srv.configStore.WriteProject("tmpproj", config.Project{Title: "Tmp", Cwd: cwd, AddDirs: []string{extra}}); err != nil {
		t.Fatalf("WriteProject: %v", err)
	}

	id := launchAndWaitIdle(t, ts, "impl", "tmpproj")
	agent, err := srv.stateStore.ReadAgent(id)
	if err != nil {
		t.Fatalf("ReadAgent: %v", err)
	}
	snap, err := srv.stateStore.ReadSession(id)
	if err != nil {
		t.Fatalf("ReadSession: %v", err)
	}
	backends, err := srv.configStore.ReadBackends()
	if err != nil {
		t.Fatalf("ReadBackends: %v", err)
	}
	be := backends.Backends[agent.Backend]
	model := be.Models[agent.Model]

	// Resume spec.
	rspec, ae := srv.composeResumeSpec(agent, snap, be, model)
	if ae != nil {
		t.Fatalf("composeResumeSpec: %s", ae.Message)
	}
	if !rspec.SkipPerms {
		t.Errorf("resume spec SkipPerms = false, want true (role skip_permissions=true)")
	}
	if !containsStr(rspec.AddDirs, extra) {
		t.Errorf("resume spec AddDirs = %v, missing %q", rspec.AddDirs, extra)
	}

	// Switch spec (target == current identity; only the carried fields matter here).
	sspec, ae := srv.composeSwitchSpec(agent, "")
	if ae != nil {
		t.Fatalf("composeSwitchSpec: %s", ae.Message)
	}
	if !sspec.SkipPerms {
		t.Errorf("switch spec SkipPerms = false, want true")
	}
	if !containsStr(sspec.AddDirs, extra) {
		t.Errorf("switch spec AddDirs = %v, missing %q", sspec.AddDirs, extra)
	}
}

// TestResumeAndSwitchUseFrozenSkipAndAddDirs guards the BLOCKING finding that
// resume/switch re-read skip_permissions and add_dirs from the LIVE role/project
// files instead of the frozen snapshot. Per techspec §12.4 (and the master-PRD
// invariant that a running agent's spec is frozen, edits affecting future
// launches only), a role/project edit AFTER launch must NOT change a later
// resume's permission policy or accessible directories.
func TestResumeAndSwitchUseFrozenSkipAndAddDirs(t *testing.T) {
	srv, ts := switchTestServer(t)

	// Launch with role skip_permissions=true and a project add_dir.
	skipTrue := true
	cwd := t.TempDir()
	extra := t.TempDir()
	if err := srv.configStore.WriteRole("impl", config.Role{Title: "Impl", SystemPrompt: "be helpful", SkipPermissions: &skipTrue}); err != nil {
		t.Fatalf("WriteRole: %v", err)
	}
	if err := srv.configStore.WriteProject("tmpproj", config.Project{Title: "Tmp", Cwd: cwd, AddDirs: []string{extra}}); err != nil {
		t.Fatalf("WriteProject: %v", err)
	}
	id := launchAndWaitIdle(t, ts, "impl", "tmpproj")

	// Confirm the frozen snapshot captured the composed values at launch.
	snap, err := srv.stateStore.ReadSession(id)
	if err != nil {
		t.Fatalf("ReadSession: %v", err)
	}
	if !snap.SkipPermissions {
		t.Fatalf("frozen snapshot SkipPermissions = false, want true")
	}
	if !containsStr(snap.AddDirs, extra) {
		t.Fatalf("frozen snapshot AddDirs = %v, missing %q", snap.AddDirs, extra)
	}

	// Now EDIT the config after launch: role flips to skip=false, project drops
	// the add_dir. A frozen resume/switch must ignore these edits.
	skipFalse := false
	if err := srv.configStore.WriteRole("impl", config.Role{Title: "Impl", SystemPrompt: "be helpful", SkipPermissions: &skipFalse}); err != nil {
		t.Fatalf("WriteRole edit: %v", err)
	}
	if err := srv.configStore.WriteProject("tmpproj", config.Project{Title: "Tmp", Cwd: cwd, AddDirs: nil}); err != nil {
		t.Fatalf("WriteProject edit: %v", err)
	}

	agent, err := srv.stateStore.ReadAgent(id)
	if err != nil {
		t.Fatalf("ReadAgent: %v", err)
	}
	backends, err := srv.configStore.ReadBackends()
	if err != nil {
		t.Fatalf("ReadBackends: %v", err)
	}
	be := backends.Backends[agent.Backend]
	model := be.Models[agent.Model]

	rspec, ae := srv.composeResumeSpec(agent, snap, be, model)
	if ae != nil {
		t.Fatalf("composeResumeSpec: %s", ae.Message)
	}
	if !rspec.SkipPerms {
		t.Errorf("resume spec SkipPerms = false after role edit; must stay frozen true")
	}
	if !containsStr(rspec.AddDirs, extra) {
		t.Errorf("resume spec AddDirs = %v after project edit; must stay frozen with %q", rspec.AddDirs, extra)
	}

	sspec, ae := srv.composeSwitchSpec(agent, "")
	if ae != nil {
		t.Fatalf("composeSwitchSpec: %s", ae.Message)
	}
	if !sspec.SkipPerms {
		t.Errorf("switch spec SkipPerms = false after role edit; must stay frozen true")
	}
	if !containsStr(sspec.AddDirs, extra) {
		t.Errorf("switch spec AddDirs = %v after project edit; must stay frozen with %q", sspec.AddDirs, extra)
	}
}

// TestCodexTerminalRejected guards the BLOCKING §6 finding: a codex terminal
// agent has no verified hook-registration path (its status never flows) and its
// CLI flags differ from claude's, so launching/switching to it silently produces
// a statusless agent that drops the composed spec. Both the launch and switch
// paths must reject it with 422 terminal_unavailable rather than land it.
func TestCodexTerminalRejected(t *testing.T) {
	_, ts := switchTestServer(t)

	// Launch: interface=terminal + backend=codex → 422 terminal_unavailable.
	resp, body := post(t, ts.URL+"/api/sessions", map[string]string{
		"role": "impl", "project": "tmpproj", "interface": "terminal", "backend": "codex", "model": "gpt-5.5",
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("launch codex terminal status = %d, want 422: %s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), "terminal_unavailable") {
		t.Fatalf("launch codex terminal error = %s, want terminal_unavailable", body)
	}

	// Switch: a live chat agent switched to codex terminal → same 422.
	id := launchAndWaitIdle(t, ts, "impl", "tmpproj")
	resp, body = post(t, ts.URL+"/api/sessions/"+id+"/switch-runtime", map[string]string{
		"interface": "terminal", "backend": "codex", "model": "gpt-5.5",
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("switch to codex terminal status = %d, want 422: %s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), "terminal_unavailable") {
		t.Fatalf("switch codex terminal error = %s, want terminal_unavailable", body)
	}
}

// TestTerminalDriverUnavailableRejected covers the 6.7 capability-probe wiring:
// an explicit terminal driver the host does not advertise (iterm2 off macOS) is
// rejected with 422 terminal_unavailable AND a non-empty reason for the UI, on
// both the launch and switch paths (§3.5). The default xterm driver is never
// affected (it is always available).
func TestTerminalDriverUnavailableRejected(t *testing.T) {
	_, ts := switchTestServer(t)

	// iTerm2 is the spec's example (unavailable off macOS). On the rare host where
	// it IS available, fall back to a name that is never advertised so the test
	// stays host-independent.
	driver := "iterm2"
	if terminal.Probe().DriverAvailable("iterm2") {
		driver = "definitely-not-a-real-driver"
	}

	assertTerminalUnavailable := func(t *testing.T, body []byte, where string) {
		t.Helper()
		var env struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(body, &env); err != nil {
			t.Fatalf("%s: bad error envelope %s: %v", where, body, err)
		}
		if env.Error.Code != "terminal_unavailable" {
			t.Fatalf("%s: error code = %q, want terminal_unavailable (%s)", where, env.Error.Code, body)
		}
		if strings.TrimSpace(env.Error.Message) == "" {
			t.Fatalf("%s: terminal_unavailable must carry a reason (%s)", where, body)
		}
	}

	// Launch: interface=terminal + unavailable driver → 422 + reason.
	resp, body := post(t, ts.URL+"/api/sessions", map[string]string{
		"role": "impl", "project": "tmpproj", "interface": "terminal", "driver": driver,
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("launch unavailable-driver status = %d, want 422: %s", resp.StatusCode, body)
	}
	assertTerminalUnavailable(t, body, "launch")

	// Switch: a live chat agent → terminal with the unavailable driver → same 422
	// (rejected before any teardown, so the agent stays a live chat agent).
	id := launchAndWaitIdle(t, ts, "impl", "tmpproj")
	resp, body = post(t, ts.URL+"/api/sessions/"+id+"/switch-runtime", map[string]string{
		"interface": "terminal", "driver": driver,
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("switch unavailable-driver status = %d, want 422: %s", resp.StatusCode, body)
	}
	assertTerminalUnavailable(t, body, "switch")
}

func containsStr(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
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

// Regression (review fix, ADVISORY): a failed resume must tear down ALL
// registration artifacts — including the hook-settings file that composeResumeSpec
// writes — not just the token + MCP session. Otherwise a resume that fails after
// composeResumeSpec leaves an orphaned {home}/hooks/agents/{id}.json behind.
func TestResumeFailureRemovesHookSettings(t *testing.T) {
	srv, ts := switchTestServer(t)
	id := launchAndWaitIdle(t, ts, "impl", "tmpproj")

	// Switch chat → terminal so resume relaunches under the terminal runtime,
	// then stop (archive resume is for inactive sessions).
	if resp, body := post(t, ts.URL+"/api/sessions/"+id+"/switch-runtime", map[string]string{"interface": "terminal"}); resp.StatusCode != http.StatusOK {
		t.Fatalf("switch status = %d: %s", resp.StatusCode, body)
	}
	if resp, body := post(t, ts.URL+"/api/sessions/"+id+"/stop", nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("stop status = %d: %s", resp.StatusCode, body)
	}

	// Make the terminal target fail to launch so the resume-failure path runs.
	srv.terminal.SetCommand("/nonexistent/agentdeck-no-such-binary")

	resp, body := post(t, ts.URL+"/api/sessions/"+id+"/resume", nil)
	if resp.StatusCode == http.StatusOK {
		t.Fatalf("resume unexpectedly succeeded: %s", body)
	}
	settingsPath := filepath.Join(hooks.Dir(srv.configStore.Home()), "agents", id+".json")
	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Fatalf("hook-settings file leaked after failed resume: stat err = %v (%s)", err, settingsPath)
	}
}
