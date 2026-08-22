package server

import (
	"context"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/hooks"
	"github.com/agentdeck/agentdeck/internal/state"
)

// activationTestServer is wakeTestServer plus a provider prompt log, so a test
// can count how many model turns actually crossed the wire. FAKEACP_PROMPT_DUMP
// overwrites and therefore cannot distinguish one turn from five.
func activationTestServer(t *testing.T) (*Server, *httptest.Server, string) {
	t.Helper()
	promptLog := filepath.Join(t.TempDir(), "prompts.log")
	t.Setenv("FAKEACP_PROMPT_LOG", promptLog)
	srv, ts := wakeTestServer(t)
	return srv, ts, promptLog
}

// promptCount is the number of session/prompt frames the fake adapter has seen.
func promptCount(t *testing.T, promptLog string) int {
	t.Helper()
	raw, err := os.ReadFile(promptLog)
	if errors.Is(err, os.ErrNotExist) {
		return 0
	}
	if err != nil {
		t.Fatalf("read prompt log: %v", err)
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return 0
	}
	// The fake adapter writes one newline-free line per session/prompt frame.
	return len(strings.Split(trimmed, "\n"))
}

// waitPrompts blocks until exactly want prompts have been seen, failing if more
// arrive or the count never reaches want.
func waitPrompts(t *testing.T, promptLog string, want int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		got := promptCount(t, promptLog)
		if got == want {
			return
		}
		if got > want {
			t.Fatalf("provider prompts = %d, want %d", got, want)
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %d provider prompts, saw %d", want, got)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// holdPrompts asserts the count stays at want across repeated executor sweeps and
// a simulated dashboard restart, which is what "never replayed" has to mean.
func holdPrompts(t *testing.T, srv *Server, promptLog string, want int) {
	t.Helper()
	for i := 0; i < 3; i++ {
		srv.nudgeOnce(context.Background(), "", map[string]nudgeState{})
		time.Sleep(150 * time.Millisecond)
	}
	if err := srv.stateStore.RecoverMailActivations(); err != nil {
		t.Fatalf("RecoverMailActivations (restart): %v", err)
	}
	srv.nudgeOnce(context.Background(), "", map[string]nudgeState{})
	time.Sleep(300 * time.Millisecond)
	if got := promptCount(t, promptLog); got != want {
		t.Fatalf("provider prompts after sweeps and restart = %d, want %d", got, want)
	}
}

func insertMail(t *testing.T, srv *Server, to string, bodies ...string) {
	t.Helper()
	for _, body := range bodies {
		if _, err := srv.stateStore.InsertMessage(state.Message{
			FromAgent: "a_sender", FromAddress: "impl@tmpproj", FromName: "Atlas",
			ToAgent: to, Body: body,
		}); err != nil {
			t.Fatalf("InsertMessage %q: %v", body, err)
		}
	}
}

func pendingActivations(t *testing.T, srv *Server, agentID string) []state.Activation {
	t.Helper()
	pending, err := srv.stateStore.PendingMailActivations(agentID)
	if err != nil {
		t.Fatalf("PendingMailActivations: %v", err)
	}
	return pending
}

// FS-06.A13/A14 (R24–R26) — several messages waiting before the opportunity is
// claimed produce exactly one payload-free provider turn. Leaving them unread
// does not re-arm that opportunity across sweeps, idle transitions, or a restart;
// mail inserted after the claim produces exactly one later turn.
func TestCoalescedMailProducesOnePromptAndIsNeverReplayed(t *testing.T) {
	srv, ts, promptLog := activationTestServer(t)
	id := launchAndWaitIdle(t, ts, "impl", "tmpproj")

	insertMail(t, srv, id, "first", "second", "third")
	if pending := pendingActivations(t, srv, id); len(pending) != 1 {
		t.Fatalf("activations for three messages = %+v, want one coalesced opportunity", pending)
	}

	srv.nudgeOnce(context.Background(), id, map[string]nudgeState{})
	waitPrompts(t, promptLog, 1)

	// A14: the instruction is code-owned and carries no message payload.
	raw, err := os.ReadFile(promptLog)
	if err != nil {
		t.Fatalf("read prompt log: %v", err)
	}
	if !strings.Contains(string(raw), "check_messages") {
		t.Fatalf("activation prompt = %s, want the check_messages instruction", raw)
	}
	for _, body := range []string{"first", "second", "third"} {
		if strings.Contains(string(raw), body) {
			t.Fatalf("activation prompt leaked message body %q: %s", body, raw)
		}
	}

	// The agent never called check_messages, so the mail is still unread. That is
	// explicitly not a reason to activate again (R25/R26).
	msgs, err := srv.stateStore.ListMessages(id, true, 0)
	if err != nil || len(msgs) != 3 {
		t.Fatalf("ListMessages unread = %d, %v; want the three durable rows", len(msgs), err)
	}
	holdPrompts(t, srv, promptLog, 1)

	// New mail after the claim is a new opportunity — exactly one more turn.
	insertMail(t, srv, id, "fourth")
	srv.nudgeOnce(context.Background(), id, map[string]nudgeState{})
	waitPrompts(t, promptLog, 2)
	holdPrompts(t, srv, promptLog, 2)
}

// FS-06.R25 / INV §5/§15 — a mailbox drained before the pending opportunity is
// claimed retires it deterministically instead of starting an empty model turn.
// The availability test and the claim used to be separate database operations, so
// a concurrent check_messages between them still crossed the non-replayable
// attempt boundary and prompted with nothing to read.
func TestDrainedMailboxRetiresActivationWithoutPrompting(t *testing.T) {
	srv, ts, promptLog := activationTestServer(t)
	id := launchAndWaitIdle(t, ts, "impl", "tmpproj")

	insertMail(t, srv, id, "read me before the executor gets here")
	pending := pendingActivations(t, srv, id)
	if len(pending) != 1 {
		t.Fatalf("activations = %+v, want one", pending)
	}
	// The drain a check_messages call or a dashboard mailbox read performs.
	if _, err := srv.stateStore.DB().Exec(`UPDATE messages SET read = 1 WHERE to_agent = ?`, id); err != nil {
		t.Fatalf("drain mailbox: %v", err)
	}

	srv.nudgeOnce(context.Background(), id, map[string]nudgeState{})
	time.Sleep(400 * time.Millisecond)

	if got := promptCount(t, promptLog); got != 0 {
		t.Fatalf("provider prompts = %d, want none for a drained mailbox", got)
	}
	if left := pendingActivations(t, srv, id); len(left) != 0 {
		t.Fatalf("activations after drain = %+v, want retired", left)
	}
}

// TS-01.R21 / INV §5 — an exclusive lifecycle transition already owns the agent
// while its runtime is registered and idle: a pipeline stage composes its first
// durable assignment inside that claim. A mail activation starting in that window
// won the runtime turn gate and made the stage's own prompt fail with
// ErrTurnInFlight. The opportunity must stay pending and send nothing.
func TestMailActivationDefersWhileLifecycleClaimIsHeld(t *testing.T) {
	srv, ts, promptLog := activationTestServer(t)
	id := launchAndWaitIdle(t, ts, "impl", "tmpproj")

	if !srv.claimLifecycle(id) {
		t.Fatal("claimLifecycle: could not take the transition claim")
	}
	insertMail(t, srv, id, "arrives mid-stage-launch")
	srv.nudgeOnce(context.Background(), id, map[string]nudgeState{})
	time.Sleep(400 * time.Millisecond)

	if got := promptCount(t, promptLog); got != 0 {
		t.Fatalf("provider prompts = %d, want none while a lifecycle claim is held", got)
	}
	pending := pendingActivations(t, srv, id)
	if len(pending) != 1 || pending[0].State != state.ActivationPending {
		t.Fatalf("activations under lifecycle claim = %+v, want one still pending", pending)
	}

	// Once the transition settles, the same durable opportunity activates.
	srv.releaseLifecycle(id)
	srv.nudgeOnce(context.Background(), id, map[string]nudgeState{})
	waitPrompts(t, promptLog, 1)
}

// INV §4 / TS-01.R16 — a post-resume activation hook that fails runs after the
// registry has already created the process, running row, hook token, MCP
// registration, and hook-settings file. Returning the error without teardown left
// a live agent nobody asked for, whose activation had already been retired as
// attempted. Every artifact must be gone.
func TestPostResumeActivationFailureLeavesNoRegistration(t *testing.T) {
	srv, ts := wakeTestServer(t)
	id := launchThenStop(t, srv, ts)

	ae := srv.resumeSessionWithHooks(context.Background(), id, resumeOverride{}, nil, func() error {
		return errors.New("injected post-resume activation failure")
	})
	if ae == nil {
		t.Fatal("resumeSessionWithHooks = nil, want the injected failure surfaced")
	}

	waitRunning(t, srv, id, false)
	if srv.registry.Owns(id) {
		t.Fatal("registry still owns a runtime after a failed post-resume activation")
	}
	srv.hookMu.Lock()
	_, tokenLeft := srv.hookTokens[id]
	_, mcpLeft := srv.mcpCleanups[id]
	srv.hookMu.Unlock()
	if tokenLeft || mcpLeft {
		t.Fatalf("registration leaked: token=%v mcp=%v", tokenLeft, mcpLeft)
	}
	settingsPath := filepath.Join(hooks.Dir(srv.configStore.Home()), "agents", id+".json")
	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Fatalf("hook-settings file leaked: stat err = %v (%s)", err, settingsPath)
	}
	if _, err := os.Stat(filepath.Join(srv.configStore.Home(), "mcp", id+".mcp.json")); !os.IsNotExist(err) {
		t.Fatalf("mcp config leaked: stat err = %v", err)
	}
}

// INV §4/§9 — a stopped activation resumes in a detached executor goroutine, and
// ChatRuntime.Resume writes the durable running/status rows before it inserts the
// runtime into its agent map. Shutdown that snapshotted the registry inside that
// window walked past the agent entirely and let the resume register a live orphan
// into an already-cleared registry. Shutdown must quiesce lifecycle work first.
func TestShutdownWaitsForActivationResumeAndLeavesNoOrphan(t *testing.T) {
	srv, ts := wakeTestServer(t)
	id := launchThenStop(t, srv, ts)
	// A launcher that stalls before exec'ing the adapter makes the window this
	// test needs wide and deterministic rather than timing-dependent.
	srv.registry.Chat().SetCommand(slowACP(t, "1", 0))

	insertMail(t, srv, id, "wake me")
	go srv.nudgeOnce(context.Background(), id, map[string]nudgeState{})

	// Wait until the executor actually owns the lifecycle claim, then shut down.
	waitLifecycleClaim(t, srv, id)

	shutCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	srv.quiesceLifecycle(shutCtx)
	if srv.lifecycleInFlight(id) {
		t.Fatal("quiesceLifecycle returned while a lifecycle transition was still in flight")
	}
	// A transition starting after the gate closes is refused, so nothing can
	// register behind Shutdown's back.
	if srv.claimLifecycle(id) {
		t.Fatal("claimLifecycle succeeded after quiesce; shutdown can be raced")
	}
	srv.registry.Shutdown(shutCtx)

	if srv.registry.Owns(id) {
		t.Fatal("registry owns a runtime after Shutdown")
	}
	if _, err := srv.stateStore.ReadRunning(id); err == nil {
		t.Fatal("running row survived Shutdown; an orphan process is registered")
	} else if !errors.Is(err, state.ErrNotFound) {
		t.Fatalf("ReadRunning after Shutdown: %v", err)
	}
}

// INV §9/§15 — the executor only ever lists `pending` rows, so a claimed
// pre-attempt row that startup recovery failed to release is invisible for the
// life of the process. Recovery failure must fail startup, not be logged past.
func TestStartMessagingLoopsFailsWhenRecoveryFails(t *testing.T) {
	srv := testServer(t, true)
	agent := state.Agent{
		AgentID: "a_stranded", Name: "Nova", Role: "reviewer", Project: "my-app",
		Backend: "claude", Model: "sonnet", Interface: "chat", CreatedAt: time.Now().UTC(),
	}
	if err := srv.stateStore.WriteAgent(agent); err != nil {
		t.Fatalf("WriteAgent: %v", err)
	}
	insertMail(t, srv, agent.AgentID, "claimed before the crash")
	pending := pendingActivations(t, srv, agent.AgentID)
	if len(pending) != 1 {
		t.Fatalf("activations = %+v, want one", pending)
	}
	if _, claimed, err := srv.stateStore.ClaimMailActivation(pending[0].ActivationID); err != nil || !claimed {
		t.Fatalf("ClaimMailActivation = %v, %v; want a claim", claimed, err)
	}
	// Recovery releases a pre-attempt claim by updating it back to pending; fail
	// that write the way a transient storage fault would.
	if _, err := srv.stateStore.DB().Exec(`
CREATE TRIGGER inject_recovery_failure BEFORE UPDATE ON activations
BEGIN SELECT RAISE(ABORT, 'injected recovery failure'); END`); err != nil {
		t.Fatalf("install recovery trigger: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := srv.startMessagingLoops(ctx); err == nil {
		t.Fatal("startMessagingLoops = nil; want startup to fail rather than strand the claimed row")
	}
	// The row really is stranded: nothing lists it, so silently continuing would
	// have lost this mail for the life of the process.
	if left := pendingActivations(t, srv, agent.AgentID); len(left) != 0 {
		t.Fatalf("pending activations = %+v, want none (the row is still claimed)", left)
	}
}

// FS-06.A15 (R26–R27) — a stopped wakeable recipient is resumed once for its
// claimed opportunity and receives exactly one activation turn. Repeated sweeps
// and a restart never repeat it, and new mail re-arms a later opportunity.
func TestStoppedMailActivationResumesOnceAndIsNeverReplayed(t *testing.T) {
	srv, ts, promptLog := activationTestServer(t)
	id := launchThenStop(t, srv, ts)

	insertMail(t, srv, id, "first", "second")
	srv.nudgeOnce(context.Background(), id, map[string]nudgeState{})
	waitRunning(t, srv, id, true)
	waitPrompts(t, promptLog, 1)
	holdPrompts(t, srv, promptLog, 1)

	insertMail(t, srv, id, "third")
	srv.nudgeOnce(context.Background(), id, map[string]nudgeState{})
	waitPrompts(t, promptLog, 2)
	holdPrompts(t, srv, promptLog, 2)
}

// FS-06.A15 — a launch that fails after the attempt boundary is not retried, and
// the durable mail survives for the human and for a later opportunity.
func TestFailedStoppedActivationIsNotRetriedAfterRestart(t *testing.T) {
	srv, ts, promptLog := activationTestServer(t)
	id := launchThenStop(t, srv, ts)
	srv.registry.Chat().SetCommand("/nonexistent/agentdeck-no-such-binary")

	insertMail(t, srv, id, "please pick this up")
	srv.nudgeOnce(context.Background(), id, map[string]nudgeState{})
	waitRunning(t, srv, id, false)

	holdPrompts(t, srv, promptLog, 0)
	if left := pendingActivations(t, srv, id); len(left) != 0 {
		t.Fatalf("activations after an attempted failure = %+v, want none re-armed", left)
	}
	msgs, err := srv.stateStore.ListMessages(id, true, 0)
	if err != nil || len(msgs) != 1 {
		t.Fatalf("ListMessages unread = %d, %v; want the mail retained", len(msgs), err)
	}
}
