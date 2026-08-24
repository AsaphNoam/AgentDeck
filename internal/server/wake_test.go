package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/hooks"
	"github.com/agentdeck/agentdeck/internal/messaging"
	"github.com/agentdeck/agentdeck/internal/pipeline"
	"github.com/agentdeck/agentdeck/internal/runtime"
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

// FS-06.A15 — a stopped wakeable agent is addressable and a claimed mail
// activation resumes it and starts one payload-free turn.
func TestMailActivationWakesStoppedRecipient(t *testing.T) {
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

	srv.executePendingMailActivations(context.Background(), stopped)
	waitRunning(t, srv, stopped, true)

	deadline := time.Now().Add(5 * time.Second)
	for {
		pending, err := srv.stateStore.PendingMailActivations(stopped, messaging.ActivationBatch)
		if err != nil {
			t.Fatalf("PendingMailActivations: %v", err)
		}
		if len(pending) == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for activation after wake: %+v", pending)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// FS-06.A11 — a failed wake keeps the mail unread, leaves the agent stopped, and
// makes no further spawn attempt — including for mail inserted in the same second
// and after a dashboard restart — because the attempted activation is never
// replayed. Only new mail creates the next opportunity.
func TestFailedMailWakeRetainsMailAndStopsRetrying(t *testing.T) {
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

	srv.executePendingMailActivations(context.Background(), stopped)

	waitRunning(t, srv, stopped, false)

	// The failed provider start crossed the activation attempt boundary. Mail
	// remains unread, but reconciliation has no activation to replay.
	deadline := time.Now().Add(5 * time.Second)
	for {
		waiting, err := srv.stateStore.PendingMailActivations(stopped, messaging.ActivationBatch)
		if err != nil {
			t.Fatalf("PendingMailActivations: %v", err)
		}
		if len(waiting) == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("activations after failure = %v, want none", waiting)
		}
		time.Sleep(10 * time.Millisecond)
	}
	srv.executePendingMailActivations(context.Background(), "")
	time.Sleep(200 * time.Millisecond)
	waitRunning(t, srv, stopped, false)

	// New mail always inserts as pending, which re-arms the wake.
	if _, err := srv.stateStore.InsertMessage(state.Message{
		FromAgent: "a_sender", FromAddress: "impl@tmpproj", FromName: "Atlas",
		ToAgent: stopped, Body: "third",
	}); err != nil {
		t.Fatalf("InsertMessage third: %v", err)
	}
	waiting, err := srv.stateStore.PendingMailActivations(stopped, messaging.ActivationBatch)
	if err != nil {
		t.Fatalf("PendingMailActivations after new mail: %v", err)
	}
	if len(waiting) != 1 || waiting[0].AgentID != stopped {
		t.Fatalf("activations after new mail = %v, want one for %s", waiting, stopped)
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
	srv.executePendingMailActivations(context.Background(), stopped)
	time.Sleep(300 * time.Millisecond)
	waitRunning(t, srv, stopped, false)
}

// slowACP wraps the fake adapter in a shell launcher that stalls for the given
// duration before exec'ing it, so a test can land a second lifecycle action
// squarely inside a resume instead of racing for a microsecond-wide window.
func slowACP(t *testing.T, delay string, exitCode int) string {
	t.Helper()
	script := filepath.Join(t.TempDir(), "slow-acp")
	body := "#!/bin/sh\nsleep " + delay + "\n"
	if exitCode != 0 {
		body += "exit " + strconv.Itoa(exitCode) + "\n"
	} else {
		body += "exec " + buildFakeACP(t) + " \"$@\"\n"
	}
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatalf("write slow adapter: %v", err)
	}
	return script
}

// waitLifecycleClaim blocks until an exclusive lifecycle transition owns the agent.
func waitLifecycleClaim(t *testing.T, srv *Server, id string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !srv.lifecycleInFlight(id) {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for the lifecycle claim on %s", id)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// FS-01.A17 / TS-01.R16 (INV §4) — Stop joins the same exclusive lifecycle claim
// as resume, so a Stop arriving while a wake is inside Registry.Resume cannot
// tear down the wake's registration. Registry.Stop reads an in-progress resume's
// nil sentinel as "no handle", so before the claim Stop fell into the idempotent
// reap branch, deleted the winner's hook token/MCP session/hook-settings file, and
// answered "stopped" while the resume ran on to report success.
func TestStopDuringWakeConflictsAndKeepsRegistration(t *testing.T) {
	srv, ts := wakeTestServer(t)
	id := launchThenStop(t, srv, ts)
	srv.registry.Chat().SetCommand(slowACP(t, "1", 0))

	type result struct {
		status int
		body   string
	}
	resumed := make(chan result, 1)
	go func() {
		resp, body := post(t, ts.URL+"/api/sessions/"+id+"/resume", nil)
		resumed <- result{resp.StatusCode, string(body)}
	}()
	waitLifecycleClaim(t, srv, id)

	resp, body := post(t, ts.URL+"/api/sessions/"+id+"/stop", nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("stop during wake = %d, want 409: %s", resp.StatusCode, body)
	}
	if code := apiErrorCode(t, body); code != "conflict" {
		t.Fatalf("stop during wake code = %q, want conflict: %s", code, body)
	}

	got := <-resumed
	if got.status != http.StatusOK {
		t.Fatalf("resume status = %d, want 200: %s", got.status, got.body)
	}
	// One coherent final state: the agent is running, and the resume's own
	// registration artifacts are all intact and usable.
	waitRunning(t, srv, id, true)
	srv.hookMu.Lock()
	token := srv.hookTokens[id]
	_, mcpOK := srv.mcpCleanups[id]
	srv.hookMu.Unlock()
	if token == "" || !mcpOK {
		t.Fatalf("resume registration torn down by the racing stop: token=%q mcp=%v", token, mcpOK)
	}
	settingsPath := filepath.Join(hooks.Dir(srv.configStore.Home()), "agents", id+".json")
	if _, err := os.Stat(settingsPath); err != nil {
		t.Fatalf("hook-settings file removed by the racing stop: %v", err)
	}
	// Once the transition settles, the retried stop succeeds normally.
	if resp, body := post(t, ts.URL+"/api/sessions/"+id+"/stop", nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("retried stop = %d, want 200: %s", resp.StatusCode, body)
	}
	waitRunning(t, srv, id, false)
}

// FS-01.A18 / FS-02.R20 / TS-03.R27 (INV §2/§4/§5) — Release group reserves every
// member's lifecycle claim before stopping any, so a release landing inside a wake
// fails closed with a retryable 409 and stops no member, leaving the live wake's
// hook token, MCP session, and hook-settings file intact. Before the shared claim
// the release worker stopped without the lifecycle claim, read the in-progress
// resume's nil sentinel as "no handle", counted the member as released, and cleaned
// up the artifacts the wake had just minted — the FS-01.R34 defect reached through
// a second door.
func TestReleaseGroupDuringWakeKeepsRegistration(t *testing.T) {
	srv, ts := wakeTestServer(t)
	resp, body := post(t, ts.URL+"/api/sessions", map[string]string{"role": "impl", "project": "tmpproj", "group": "auth"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("launch status = %d: %s", resp.StatusCode, body)
	}
	var launched sessionResponse
	if err := json.Unmarshal(body, &launched); err != nil || launched.Agent.AgentID == "" {
		t.Fatalf("bad launch response (%v): %s", err, body)
	}
	id := launched.Agent.AgentID
	if resp, body := post(t, ts.URL+"/api/sessions/"+id+"/stop", nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("stop status = %d: %s", resp.StatusCode, body)
	}
	waitRunning(t, srv, id, false)

	srv.registry.Chat().SetCommand(slowACP(t, "1", 0))
	resumed := make(chan int, 1)
	go func() {
		resp, _ := post(t, ts.URL+"/api/sessions/"+id+"/resume", nil)
		resumed <- resp.StatusCode
	}()
	waitLifecycleClaim(t, srv, id)

	// The release reserves every member claim before stopping any, so a member
	// mid-wake makes the whole release fail closed with a retryable 409 and stops
	// nobody (FS-02.R20/TS-03.R27, all-or-none).
	resp, body = post(t, ts.URL+"/api/groups/auth/release", nil)
	if resp.StatusCode != http.StatusConflict || apiErrorCode(t, body) != runtime.CodeConflict {
		t.Fatalf("release during wake = %d %s, want 409 conflict", resp.StatusCode, body)
	}
	var released struct {
		Stopped []releaseGroupResult `json:"stopped"`
	}

	if got := <-resumed; got != http.StatusOK {
		t.Fatalf("resume status = %d, want 200", got)
	}
	waitRunning(t, srv, id, true)
	srv.hookMu.Lock()
	token := srv.hookTokens[id]
	_, mcpOK := srv.mcpCleanups[id]
	srv.hookMu.Unlock()
	if token == "" || !mcpOK {
		t.Fatalf("wake registration torn down by the racing release: token=%q mcp=%v", token, mcpOK)
	}
	settingsPath := filepath.Join(hooks.Dir(srv.configStore.Home()), "agents", id+".json")
	if _, err := os.Stat(settingsPath); err != nil {
		t.Fatalf("hook-settings file removed by the racing release: %v", err)
	}

	// Once the transition settles, the retried release stops the member normally.
	resp, body = post(t, ts.URL+"/api/groups/auth/release", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("retried release = %d: %s", resp.StatusCode, body)
	}
	released.Stopped = nil
	if err := json.Unmarshal(body, &released); err != nil {
		t.Fatalf("decode retried release (%v): %s", err, body)
	}
	if len(released.Stopped) != 1 || !released.Stopped[0].OK {
		t.Fatalf("retried release = %+v, want one OK result: %s", released.Stopped, body)
	}
	waitRunning(t, srv, id, false)
}

// FS-06.A11 / TS-04.R26 (INV §15) — a wake attempt claims the mail it wakes for
// before spawning anything, so an adapter that completes its handshake and then
// dies before the first check_messages nudge is not respawned by every sweep.
func TestSuccessfulWakeConsumesMailEvenIfAdapterDiesBeforeNudge(t *testing.T) {
	srv, ts := wakeTestServer(t)
	stopped := launchThenStop(t, srv, ts)

	if _, err := srv.stateStore.InsertMessage(state.Message{
		FromAgent: "a_sender", FromAddress: "impl@tmpproj", FromName: "Atlas",
		ToAgent: stopped, Body: "pick this up",
	}); err != nil {
		t.Fatalf("InsertMessage: %v", err)
	}

	srv.executePendingMailActivations(context.Background(), stopped)
	waitRunning(t, srv, stopped, true)

	// Mail remains independently unread; the attempted activation itself is
	// removed after the provider handoff and cannot replay after a crash.
	msgs, err := srv.stateStore.ListMessages(stopped, false, 0)
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(msgs) != 1 || msgs[0].Read || msgs[0].DeliveredVia != state.DeliveryPending {
		t.Fatalf("mail after activation = %+v, want unread pending mail", msgs)
	}

	// The adapter dies before it is ever nudged.
	row, err := srv.stateStore.ReadRunning(stopped)
	if err != nil {
		t.Fatalf("ReadRunning: %v", err)
	}
	if err := syscall.Kill(-row.PID, syscall.SIGKILL); err != nil {
		t.Fatalf("kill woken adapter: %v", err)
	}
	waitRunning(t, srv, stopped, false)

	// The mail is still unread, but it no longer makes the agent a wake candidate,
	// so no sweep — including one after a dashboard restart's empty nudge map —
	// respawns the broken adapter.
	waiting, err := srv.stateStore.PendingMailActivations(stopped, messaging.ActivationBatch)
	if err != nil {
		t.Fatalf("PendingMailActivations: %v", err)
	}
	if len(waiting) != 0 {
		t.Fatalf("activations after a dead adapter = %v, want none", waiting)
	}
	for i := 0; i < 3; i++ {
		srv.executePendingMailActivations(context.Background(), "")
	}
	time.Sleep(300 * time.Millisecond)
	waitRunning(t, srv, stopped, false)
}

// FS-06.A11 / TS-04.R26 (INV §5) — a failing wake records its outcome on exactly
// the rows it claimed. Mail that arrives while the wake is in flight stays
// pending and re-arms the wake, instead of being consumed by an attempt that
// never saw it.
func TestFailedWakeLeavesNewerMailPending(t *testing.T) {
	srv, ts := wakeTestServer(t)
	stopped := launchThenStop(t, srv, ts)
	// Fail the resume, but slowly enough that newer mail lands mid-attempt.
	srv.registry.Chat().SetCommand(slowACP(t, "2", 1))

	_, err := srv.stateStore.InsertMessage(state.Message{
		FromAgent: "a_sender", FromAddress: "impl@tmpproj", FromName: "Atlas",
		ToAgent: stopped, Body: "before the wake",
	})
	if err != nil {
		t.Fatalf("InsertMessage first: %v", err)
	}

	srv.executePendingMailActivations(context.Background(), stopped)
	// The attempt is now in flight and stalls in the adapter for two seconds. The
	// second message therefore arrives strictly after the attempt began and
	// strictly before it fails — the interleaving the attempt must not consume.
	waitLifecycleClaim(t, srv, stopped)
	_, err = srv.stateStore.InsertMessage(state.Message{
		FromAgent: "a_sender", FromAddress: "impl@tmpproj", FromName: "Atlas",
		ToAgent: stopped, Body: "arrived mid-wake",
	})
	if err != nil {
		t.Fatalf("InsertMessage second: %v", err)
	}

	// The first wake now fails after crossing mail's attempted boundary. The
	// later insert has its own pending activation.
	deadline := time.Now().Add(15 * time.Second)
	for {
		pending, err := srv.stateStore.PendingMailActivations(stopped, messaging.ActivationBatch)
		if err != nil {
			t.Fatalf("PendingMailActivations: %v", err)
		}
		if len(pending) == 1 && pending[0].AgentID == stopped {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for the later activation: %+v", pending)
		}
		time.Sleep(20 * time.Millisecond)
	}
	waitRunning(t, srv, stopped, false)

	// Because the newer mail is still pending, the recipient is a candidate again.
	waiting, err := srv.stateStore.PendingMailActivations(stopped, messaging.ActivationBatch)
	if err != nil {
		t.Fatalf("PendingMailActivations: %v", err)
	}
	if len(waiting) != 1 || waiting[0].AgentID != stopped {
		t.Fatalf("activations after the interleaving = %v, want one for %s", waiting, stopped)
	}
}

// FS-01.A17 / TS-03.R25 (INV §7/§8) — a wake gate that cannot be evaluated is a
// typed failure, not a decision that the agent is unwakeable. A corrupt project
// definition previously collapsed into "not a candidate", so the prompt route
// answered with the ordinary no-handle 404 and the messaging directory silently
// omitted the agent.
func TestWakeGateFailureSurfacesTypedError(t *testing.T) {
	srv, ts := wakeTestServer(t)
	id := launchThenStop(t, srv, ts)

	projectPath := filepath.Join(srv.configStore.Home(), "projects", "tmpproj.json")
	if err := os.WriteFile(projectPath, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("corrupt project definition: %v", err)
	}

	resp, body := post(t, ts.URL+"/api/sessions/"+id+"/prompt", map[string]string{"text": "hi"})
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("prompt with an unreadable project = %d, want 500: %s", resp.StatusCode, body)
	}
	if code := apiErrorCode(t, body); code != "internal" {
		t.Fatalf("prompt error code = %q, want internal: %s", code, body)
	}
	if _, err := srv.addressableAgents(); err == nil {
		t.Fatal("addressableAgents silently omitted the agent instead of reporting the gate failure")
	}
	waitRunning(t, srv, id, false)
}

// FS-06.A11 / TS-04.R26 (INV §1/§5) — the addressable set is read from one SQLite
// snapshot, so an agent whose running row disappears mid-read can never be listed
// twice (once running, once stopped-wakeable). Two separate queries produced that
// duplicate, which made list_agents show the agent twice and role/name resolution
// report a false ambiguity.
func TestAddressableAgentsNeverDuplicatesAcrossStop(t *testing.T) {
	srv, ts := wakeTestServer(t)
	id := launchAndWaitIdle(t, ts, "impl", "tmpproj")
	row, err := srv.stateStore.ReadRunning(id)
	if err != nil {
		t.Fatalf("ReadRunning: %v", err)
	}

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
			}
			_ = srv.stateStore.DeleteRunning(id)
			_ = srv.stateStore.WriteRunning(row)
		}
	}()

	for i := 0; i < 400; i++ {
		agents, err := srv.addressableAgents()
		if err != nil {
			close(stop)
			<-done
			t.Fatalf("addressableAgents: %v", err)
		}
		seen := map[string]string{}
		for _, a := range agents {
			if prev, dup := seen[a.AgentID]; dup {
				close(stop)
				<-done
				t.Fatalf("agent %s listed twice (%s and %s) across a stop", a.AgentID, prev, a.Availability)
			}
			seen[a.AgentID] = a.Availability
		}
		if _, _, err := state.ResolveRecipient(agents, "impl@tmpproj"); err != nil &&
			errors.Is(err, state.ErrAmbiguousRecipient) {
			close(stop)
			<-done
			t.Fatalf("role@project resolution reported a false ambiguity across a stop")
		}
	}
	close(stop)
	<-done
}

// TS-01.R16 / FS-02.A8 (INV §4/§5) — a lifecycle claim on any group member
// rejects Release group before it stops anyone, so a 200 release never leaves a
// claimed member running after stopping its peers.
func TestReleaseGroupRespectsLifecycleClaim(t *testing.T) {
	srv, ts := wakeTestServer(t)
	id := launchAndWaitIdle(t, ts, "impl", "tmpproj")
	otherID := launchAndWaitIdle(t, ts, "impl", "tmpproj")
	for _, agentID := range []string{id, otherID} {
		agent, err := srv.stateStore.ReadAgent(agentID)
		if err != nil {
			t.Fatalf("ReadAgent %s: %v", agentID, err)
		}
		agent.Group = "release"
		if err := srv.stateStore.WriteAgent(agent); err != nil {
			t.Fatalf("WriteAgent %s: %v", agentID, err)
		}
	}

	// Hold the claim as another in-flight transition would.
	if !srv.claimLifecycle(id) {
		t.Fatalf("could not take the lifecycle claim")
	}
	defer srv.releaseLifecycle(id)

	resp, body := post(t, ts.URL+"/api/groups/release/release", nil)
	if resp.StatusCode != http.StatusConflict || apiErrorCode(t, body) != runtime.CodeConflict {
		t.Fatalf("release during lifecycle transition = %d %s, want 409 conflict", resp.StatusCode, body)
	}

	// Neither the claimed member nor its peer was stopped, and the registration the
	// held transition owns is intact.
	srv.hookMu.Lock()
	token := srv.hookTokens[id]
	_, mcpOK := srv.mcpCleanups[id]
	srv.hookMu.Unlock()
	if token == "" || !mcpOK {
		t.Fatalf("release tore down a claimed agent's registration: token=%q mcp=%v", token, mcpOK)
	}
	waitRunning(t, srv, id, true)
	waitRunning(t, srv, otherID, true)
}

// TS-01.R16 (INV §4/§5) — LaunchStage holds the shared lifecycle claim from
// registration through its initial prompt, so a concurrent ordinary Stop returns
// 409 and cannot tear down the fresh hook/MCP/settings registration.
func TestPipelineLaunchStageRespectsLifecycleClaim(t *testing.T) {
	srv, ts := wakeTestServer(t)
	id := "a_pipeline_launch"
	srv.registry.Chat().SetCommand(slowACP(t, "1", 0))

	done := make(chan error, 1)
	go func() {
		done <- srv.LaunchStage(context.Background(), pipeline.StageExecution{
			AgentID: id, Generation: "g_pipeline_launch", Role: "impl", Project: "tmpproj",
			Backend: "claude", Model: "sonnet", AgentName: "Pipeline launch", Assignment: "begin",
		})
	}()
	waitLifecycleClaim(t, srv, id)

	resp, body := post(t, ts.URL+"/api/sessions/"+id+"/stop", nil)
	if resp.StatusCode != http.StatusConflict || apiErrorCode(t, body) != runtime.CodeConflict {
		t.Fatalf("stop during pipeline launch = %d %s, want 409 conflict", resp.StatusCode, body)
	}
	if err := <-done; err != nil {
		t.Fatalf("LaunchStage: %v", err)
	}
	waitRunning(t, srv, id, true)

	srv.hookMu.Lock()
	token := srv.hookTokens[id]
	_, mcpOK := srv.mcpCleanups[id]
	srv.hookMu.Unlock()
	if token == "" || !mcpOK {
		t.Fatalf("pipeline launch registration was torn down: token=%q mcp=%v", token, mcpOK)
	}
	settingsPath := filepath.Join(hooks.Dir(srv.configStore.Home()), "agents", id+".json")
	if _, err := os.Stat(settingsPath); err != nil {
		t.Fatalf("pipeline launch hook settings missing: %v", err)
	}
}

// TS-01.R16 (INV §4/§5) — the pipeline stage stop/continue seams take the shared
// exclusive lifecycle claim, so they refuse an agent already inside another
// lifecycle transition instead of racing its registration.
func TestPipelineStageRespectsLifecycleClaim(t *testing.T) {
	srv, ts := wakeTestServer(t)
	id := launchAndWaitIdle(t, ts, "impl", "tmpproj")

	if !srv.claimLifecycle(id) {
		t.Fatalf("could not take the lifecycle claim")
	}
	defer srv.releaseLifecycle(id)

	if err := srv.StopStage(context.Background(), id); err == nil {
		t.Fatalf("StopStage under a held claim = nil, want a conflict error")
	}
	if err := srv.ContinueStage(context.Background(), pipeline.StageExecution{AgentID: id, Project: "tmpproj"}); err == nil {
		t.Fatalf("ContinueStage under a held claim = nil, want a conflict error")
	}

	// The claimed registration is intact and the agent is still running.
	srv.hookMu.Lock()
	token := srv.hookTokens[id]
	srv.hookMu.Unlock()
	if token == "" {
		t.Fatalf("pipeline stage stop/continue tore down a claimed agent's registration")
	}
	waitRunning(t, srv, id, true)
}
