package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/hooks"
	"github.com/agentdeck/agentdeck/internal/messaging"
	"github.com/agentdeck/agentdeck/internal/state"
)

// wakeTestServer is a fake-ACP chat server with one project and role, ready to
// launch, stop, and wake a chat agent (FS-01.R33).
func wakeTestServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
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
	t.Cleanup(ts.Close)
	t.Cleanup(func() { srv.registry.Shutdown(context.Background()) })
	return srv, ts
}

// launchThenStop leaves one agent with a frozen session snapshot and no process —
// the state a message has to wake.
func launchThenStop(t *testing.T, srv *Server, ts *httptest.Server) string {
	t.Helper()
	id := launchAndWaitIdle(t, ts, "impl", "tmpproj")
	if resp, body := post(t, ts.URL+"/api/sessions/"+id+"/stop", nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("stop status = %d: %s", resp.StatusCode, body)
	}
	if _, err := srv.stateStore.ReadSession(id); err != nil {
		t.Fatalf("no frozen snapshot after stop: %v", err)
	}
	waitRunning(t, srv, id, false)
	return id
}

// waitRunning waits for the durable running row to appear or disappear.
func waitRunning(t *testing.T, srv *Server, id string, want bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		_, err := srv.stateStore.ReadRunning(id)
		if (err == nil) == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for running row present=%v (err=%v)", want, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// apiErrorCode extracts the §7.7 error envelope's code.
func apiErrorCode(t *testing.T, body []byte) string {
	t.Helper()
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("error envelope %q: %v", body, err)
	}
	return env.Error.Code
}

// associatePipeline gives an agent a pipeline attempt, the association that bars
// it from every wake path (FS-01.R33).
func associatePipeline(t *testing.T, srv *Server, agentID string) {
	t.Helper()
	if _, err := srv.stateStore.DB().Exec(`
INSERT INTO pipeline_runs(run_id, template_id, display_name, project, goal, state, created_at, updated_at)
VALUES ('pr_1','t_1','Build','tmpproj','ship','running','2026-08-16T10:00:00Z','2026-08-16T10:00:00Z')`); err != nil {
		t.Fatalf("insert pipeline run: %v", err)
	}
	if err := srv.stateStore.InsertPipelineAttempt(state.PipelineAttemptRecord{
		AttemptID: "pa_1", RunID: "pr_1", StageID: "build", AttemptNo: 1, VisitNo: 1,
		AgentID: agentID, Backend: "claude", Model: "sonnet", State: "done",
	}); err != nil {
		t.Fatalf("InsertPipelineAttempt: %v", err)
	}
}

// FS-01.A17 / TS-03.R25 — a prompt to a stopped chat agent resumes it inside the
// same request and then streams the turn, under the same agent_id.
func TestPromptWakesStoppedChatAgent(t *testing.T) {
	srv, ts := wakeTestServer(t)
	id := launchThenStop(t, srv, ts)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	frames := streamSSE(t, ctx, ts.URL+"/api/events")

	resp, body := post(t, ts.URL+"/api/sessions/"+id+"/prompt", map[string]string{"text": "are you there"})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("wake prompt status = %d: %s", resp.StatusCode, body)
	}
	waitRunning(t, srv, id, true)
	waitForEventType(t, frames, "turn_end")

	// The woken agent keeps its identity and its frozen snapshot values.
	agent, err := srv.stateStore.ReadAgent(id)
	if err != nil || agent.AgentID != id {
		t.Fatalf("agent after wake = %+v, err = %v", agent, err)
	}
}

// FS-01.A17 — agents the wake gates exclude keep today's non-running rejection.
func TestPromptDoesNotWakeExcludedAgents(t *testing.T) {
	t.Run("archived agent", func(t *testing.T) {
		srv, ts := wakeTestServer(t)
		id := launchThenStop(t, srv, ts)
		if err := srv.stateStore.SetAgentsArchived([]string{id}, true); err != nil {
			t.Fatalf("SetAgentsArchived: %v", err)
		}
		resp, body := post(t, ts.URL+"/api/sessions/"+id+"/prompt", map[string]string{"text": "hi"})
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("archived prompt status = %d: %s", resp.StatusCode, body)
		}
		waitRunning(t, srv, id, false)
	})

	t.Run("no persisted snapshot", func(t *testing.T) {
		srv, ts := wakeTestServer(t)
		if err := srv.stateStore.WriteAgent(state.Agent{
			AgentID: "a_bare", Name: "Bare", Role: "impl", Project: "tmpproj",
			Backend: "claude", Model: "sonnet", Interface: "chat", CreatedAt: time.Now().UTC(),
		}); err != nil {
			t.Fatalf("WriteAgent: %v", err)
		}
		resp, body := post(t, ts.URL+"/api/sessions/a_bare/prompt", map[string]string{"text": "hi"})
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("snapshot-less prompt status = %d: %s", resp.StatusCode, body)
		}
	})

	t.Run("pipeline-associated agent", func(t *testing.T) {
		srv, ts := wakeTestServer(t)
		id := launchThenStop(t, srv, ts)
		associatePipeline(t, srv, id)
		resp, body := post(t, ts.URL+"/api/sessions/"+id+"/prompt", map[string]string{"text": "hi"})
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("pipeline prompt status = %d: %s", resp.StatusCode, body)
		}
		waitRunning(t, srv, id, false)

		// Explicit Resume remains the human-driven revival path for it.
		if resp, body := post(t, ts.URL+"/api/sessions/"+id+"/resume", nil); resp.StatusCode != http.StatusOK {
			t.Fatalf("explicit resume of pipeline agent status = %d: %s", resp.StatusCode, body)
		}
	})
}

// FS-01.A17 — a wake whose resume stage fails returns the typed resume error and
// tears down that wake's own registration artifacts, exactly as a failed explicit
// resume does.
func TestPromptWakeFailureReturnsResumeErrorAndTearsDown(t *testing.T) {
	srv, ts := wakeTestServer(t)
	id := launchThenStop(t, srv, ts)
	srv.registry.Chat().SetCommand("/nonexistent/agentdeck-no-such-binary")

	resp, body := post(t, ts.URL+"/api/sessions/"+id+"/prompt", map[string]string{"text": "hi"})
	if resp.StatusCode == http.StatusAccepted {
		t.Fatalf("wake unexpectedly succeeded: %s", body)
	}
	if code := apiErrorCode(t, body); code != "runtime_start_failed" {
		t.Fatalf("wake failure code = %q, want runtime_start_failed: %s", code, body)
	}
	waitRunning(t, srv, id, false)

	settingsPath := filepath.Join(hooks.Dir(srv.configStore.Home()), "agents", id+".json")
	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Fatalf("hook-settings file leaked after failed wake: stat err = %v (%s)", err, settingsPath)
	}
	srv.hookMu.Lock()
	_, tokenLeft := srv.hookTokens[id]
	_, mcpLeft := srv.mcpCleanups[id]
	srv.hookMu.Unlock()
	if tokenLeft || mcpLeft {
		t.Fatalf("registration leaked after failed wake: token=%v mcp=%v", tokenLeft, mcpLeft)
	}
}

// FS-01.A17 / TS-01.R16 — simultaneous prompt, mail, and explicit-resume wakes on
// one agent produce exactly one process; the losers get the conflict outcome
// without composing or tearing down anything, so the winner's hook token, MCP
// registration, and hook-settings file survive intact.
func TestConcurrentWakesResumeOnceAndKeepWinnerRegistration(t *testing.T) {
	srv, ts := wakeTestServer(t)
	id := launchThenStop(t, srv, ts)

	var (
		mu       sync.Mutex
		wins     int
		conflict int
		other    []string
		wg       sync.WaitGroup
	)
	record := func(ok bool, conflicted bool, detail string) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case ok:
			wins++
		case conflicted:
			conflict++
		default:
			other = append(other, detail)
		}
	}

	start := make(chan struct{})
	wg.Add(3)
	// 1. The prompt route's wake.
	go func() {
		defer wg.Done()
		<-start
		resp, body := post(t, ts.URL+"/api/sessions/"+id+"/prompt", map[string]string{"text": "hi"})
		record(resp.StatusCode == http.StatusAccepted, resp.StatusCode == http.StatusConflict, string(body))
	}()
	// 2. The explicit resume handler.
	go func() {
		defer wg.Done()
		<-start
		resp, body := post(t, ts.URL+"/api/sessions/"+id+"/resume", nil)
		record(resp.StatusCode == http.StatusOK, resp.StatusCode == http.StatusConflict, string(body))
	}()
	// 3. The messaging wake, through the same shared helper the nudger uses.
	go func() {
		defer wg.Done()
		<-start
		woken, ae := srv.wakeAgent(context.Background(), id)
		record(woken, ae != nil && ae.Code == "conflict", func() string {
			if ae != nil {
				return ae.Message
			}
			return "not wakeable"
		}())
	}()
	close(start)
	wg.Wait()

	if wins != 1 || conflict != 2 || len(other) != 0 {
		t.Fatalf("wakes: %d won, %d conflicted, other=%v; want exactly one winner and two conflicts", wins, conflict, other)
	}
	waitRunning(t, srv, id, true)

	// The winner's three registration artifacts must all still be in place.
	srv.hookMu.Lock()
	token := srv.hookTokens[id]
	_, mcpOK := srv.mcpCleanups[id]
	srv.hookMu.Unlock()
	if token == "" || !mcpOK {
		t.Fatalf("winner registration incomplete: hook token=%q mcp=%v", token, mcpOK)
	}
	// The MCP config the woken child reads must name a session token that is
	// still registered — a loser's teardown would have revoked it.
	raw, err := os.ReadFile(filepath.Join(srv.configStore.Home(), "mcp", id+".mcp.json"))
	if err != nil {
		t.Fatalf("read winner mcp config: %v", err)
	}
	var mcpConfig struct {
		MCPServers map[string]struct {
			Headers map[string]string `json:"headers"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &mcpConfig); err != nil {
		t.Fatalf("unmarshal winner mcp config: %v\n%s", err, raw)
	}
	sessionToken := mcpConfig.MCPServers[messagingMCPName].Headers[messaging.TokenHeader]
	if got, ok := srv.messaging.Lookup(sessionToken); !ok || got != id {
		t.Fatalf("winner messaging session = %q,%v want %s", got, ok, id)
	}
	settingsPath := filepath.Join(hooks.Dir(srv.configStore.Home()), "agents", id+".json")
	if _, err := os.Stat(settingsPath); err != nil {
		t.Fatalf("winner hook-settings file missing: %v", err)
	}
}

// FS-06.A11 — a stopped wakeable agent is addressable and marked as such; mail
// for it is inserted durably, wakes it, and is then nudged like any running
// recipient.
func TestMailWakesStoppedRecipientAndNudges(t *testing.T) {
	srv, ts := wakeTestServer(t)
	running := launchAndWaitIdle(t, ts, "impl", "tmpproj")
	stopped := launchThenStop(t, srv, ts)

	addressable, err := srv.addressableAgents()
	if err != nil {
		t.Fatalf("addressableAgents: %v", err)
	}
	availability := map[string]string{}
	for _, a := range addressable {
		availability[a.AgentID] = a.Availability
	}
	if availability[running] != state.AvailabilityRunning {
		t.Fatalf("running agent availability = %q, want %q", availability[running], state.AvailabilityRunning)
	}
	if availability[stopped] != state.AvailabilityStoppedWakeable {
		t.Fatalf("stopped agent availability = %q, want %q", availability[stopped], state.AvailabilityStoppedWakeable)
	}
	if id, _, err := state.ResolveRecipient(addressable, stopped); err != nil || id != stopped {
		t.Fatalf("ResolveRecipient(stopped) = %q,%v want %q", id, err, stopped)
	}

	if _, err := srv.stateStore.InsertMessage(state.Message{
		FromAgent: running, FromAddress: "impl@tmpproj", FromName: "Atlas",
		ToAgent: stopped, Body: "please pick this up",
	}); err != nil {
		t.Fatalf("InsertMessage: %v", err)
	}

	nudges := map[string]nudgeState{}
	srv.nudgeOnce(context.Background(), stopped, nudges)
	waitRunning(t, srv, stopped, true)

	// Once the woken agent reports idle, the ordinary running-recipient nudge
	// delivers check_messages and stamps the rows.
	deadline := time.Now().Add(5 * time.Second)
	for {
		srv.nudgeOnce(context.Background(), stopped, nudges)
		msgs, err := srv.stateStore.ListMessages(stopped, false, 0)
		if err != nil {
			t.Fatalf("ListMessages: %v", err)
		}
		if len(msgs) == 1 && msgs[0].DeliveredVia == "nudge" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for nudge after wake: %+v", msgs)
		}
		// The nudge cooldown is per agent, so let it lapse between attempts.
		time.Sleep(200 * time.Millisecond)
		nudges[stopped] = nudgeState{}
	}
}

// FS-06.A11 — a failed wake keeps the mail, leaves the agent stopped, marks the
// still-pending rows wake_failed, and makes no further spawn attempt — including
// for mail inserted in the same second and after a dashboard restart (a fresh
// nudge map) — until new pending mail arrives.
func TestFailedMailWakeMarksRowsAndStopsRetrying(t *testing.T) {
	srv, ts := wakeTestServer(t)
	stopped := launchThenStop(t, srv, ts)
	srv.registry.Chat().SetCommand("/nonexistent/agentdeck-no-such-binary")

	for _, body := range []string{"first", "second in the same second"} {
		if _, err := srv.stateStore.InsertMessage(state.Message{
			FromAgent: "a_sender", FromAddress: "impl@tmpproj", FromName: "Atlas",
			ToAgent: stopped, Body: body,
		}); err != nil {
			t.Fatalf("InsertMessage: %v", err)
		}
	}

	nudges := map[string]nudgeState{}
	srv.nudgeOnce(context.Background(), stopped, nudges)

	deadline := time.Now().Add(5 * time.Second)
	for {
		msgs, err := srv.stateStore.ListMessages(stopped, false, 0)
		if err != nil {
			t.Fatalf("ListMessages: %v", err)
		}
		marked := 0
		for _, m := range msgs {
			if m.DeliveredVia == "wake_failed" && !m.Read {
				marked++
			}
		}
		if marked == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for wake_failed marks: %+v", msgs)
		}
		time.Sleep(20 * time.Millisecond)
	}
	waitRunning(t, srv, stopped, false)

	// A restart loses the in-process nudge state, but candidacy is durable: the
	// rows are no longer pending, so no further wake is attempted.
	waiting, err := srv.stateStore.PendingWakeMailAgents()
	if err != nil {
		t.Fatalf("PendingWakeMailAgents: %v", err)
	}
	if len(waiting) != 0 {
		t.Fatalf("wake candidates after failure = %v, want none", waiting)
	}
	srv.nudgeOnce(context.Background(), "", map[string]nudgeState{})
	time.Sleep(200 * time.Millisecond)
	waitRunning(t, srv, stopped, false)

	// New mail always inserts as pending, which re-arms the wake.
	if _, err := srv.stateStore.InsertMessage(state.Message{
		FromAgent: "a_sender", FromAddress: "impl@tmpproj", FromName: "Atlas",
		ToAgent: stopped, Body: "third",
	}); err != nil {
		t.Fatalf("InsertMessage third: %v", err)
	}
	waiting, err = srv.stateStore.PendingWakeMailAgents()
	if err != nil {
		t.Fatalf("PendingWakeMailAgents after new mail: %v", err)
	}
	if len(waiting) != 1 || waiting[0] != stopped {
		t.Fatalf("wake candidates after new mail = %v, want [%s]", waiting, stopped)
	}
}

// FS-06.A11 — a stopped agent the pipeline state machine owns is unlisted,
// unresolvable, and never woken by mail.
func TestStoppedPipelineAgentIsNeverAddressableOrWoken(t *testing.T) {
	srv, ts := wakeTestServer(t)
	stopped := launchThenStop(t, srv, ts)
	associatePipeline(t, srv, stopped)

	addressable, err := srv.addressableAgents()
	if err != nil {
		t.Fatalf("addressableAgents: %v", err)
	}
	for _, a := range addressable {
		if a.AgentID == stopped {
			t.Fatalf("pipeline-associated agent is addressable: %+v", a)
		}
	}
	if id, _, err := state.ResolveRecipient(addressable, stopped); err == nil {
		t.Fatalf("ResolveRecipient resolved a pipeline-associated agent: %q", id)
	}

	if _, err := srv.stateStore.InsertMessage(state.Message{
		FromAgent: "a_sender", FromAddress: "impl@tmpproj", FromName: "Atlas",
		ToAgent: stopped, Body: "mail inserted directly",
	}); err != nil {
		t.Fatalf("InsertMessage: %v", err)
	}
	srv.nudgeOnce(context.Background(), stopped, map[string]nudgeState{})
	time.Sleep(300 * time.Millisecond)
	waitRunning(t, srv, stopped, false)
}
