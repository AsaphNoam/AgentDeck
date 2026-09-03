package runtime

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/state"
)

// startPermAgent launches a fake agent running the permission scenario with a
// sentinel path wired in. skip toggles skip_permissions. Returns the runtime,
// handle, the sentinel path, and the event channel (subscribed before prompt).
func startPermAgent(t *testing.T, skip bool, timeout string) (*ChatRuntime, *Handle, string, <-chan Event) {
	t.Helper()
	bin := buildFakeACP(t)
	st, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	agent := state.Agent{
		AgentID: "a_perm01", Name: "Echo", Role: "implementer", Project: "my-app",
		Backend: "claude", Model: "sonnet-4-6", Interface: "chat", CreatedAt: time.Now().UTC(),
	}
	if err := st.WriteAgent(agent); err != nil {
		t.Fatalf("WriteAgent: %v", err)
	}

	sentinel := filepath.Join(t.TempDir(), "sentinel")
	env := []string{
		"FAKEACP_SCENARIO=permission",
		"FAKEACP_SENTINEL=" + sentinel,
		"HOME=" + os.Getenv("HOME"),
	}
	if title := os.Getenv("FAKEACP_PERMISSION_TITLE"); title != "" {
		env = append(env, "FAKEACP_PERMISSION_TITLE="+title)
	}
	if os.Getenv("FAKEACP_PERMISSION_ALLOW_ALWAYS_ONLY") == "1" {
		env = append(env, "FAKEACP_PERMISSION_ALLOW_ALWAYS_ONLY=1")
	}
	if timeout != "" {
		env = append(env, "PERMISSION_TIMEOUT="+timeout)
		t.Setenv("PERMISSION_TIMEOUT", timeout) // read by the runtime side
	}

	c := NewChatRuntime(st)
	c.command = bin
	spec := LaunchSpec{
		Agent: agent, Cwd: t.TempDir(), BackendType: "claude-acp",
		ModelID: "claude-sonnet-4-6", SkipPerms: skip, Env: env,
	}
	if tool := os.Getenv("TEST_AUTO_APPROVE_TOOL"); tool != "" {
		spec.AutoApproveTools = map[string]struct{}{tool: {}}
	}
	h, err := c.Start(context.Background(), spec)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { c.Stop(context.Background(), h.AgentID) })

	ch, unsub, err := c.Subscribe(h.AgentID)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	t.Cleanup(unsub)
	return c, h, sentinel, ch
}

// waitForEvent reads ch until an event of typ arrives (or timeout).
func waitForEvent(t *testing.T, ch <-chan Event, typ string) Event {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				t.Fatalf("channel closed before %q", typ)
			}
			if ev.Type == typ {
				return ev
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %q", typ)
		}
	}
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }

func TestPermissionApprove(t *testing.T) {
	c, h, sentinel, ch := startPermAgent(t, false, "")
	ctx := context.Background()

	if err := c.SendPrompt(ctx, h.AgentID, "run ls"); err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}
	pr := waitForEvent(t, ch, EvPermissionRequest)
	var prd PermissionRequestData
	json.Unmarshal(pr.Data, &prd)
	if prd.AutoApproved {
		t.Fatal("non-skip request should not be auto-approved")
	}

	// While withheld, status is waiting_input.
	if st, _ := c.store.ReadStatus(h.AgentID); st.State != "waiting_input" {
		t.Fatalf("status while pending = %q, want waiting_input", st.State)
	}
	if fileExists(sentinel) {
		t.Fatal("sentinel exists before approval — tool ran without permission")
	}

	if err := c.Permission(ctx, h.AgentID, prd.ToolCallID, "approve"); err != nil {
		t.Fatalf("Permission approve: %v", err)
	}
	waitForEvent(t, ch, EvTurnEnd)
	if !fileExists(sentinel) {
		t.Fatal("sentinel missing after approve — tool did not run")
	}
}

func TestPermissionDeny(t *testing.T) {
	c, h, sentinel, ch := startPermAgent(t, false, "")
	ctx := context.Background()

	if err := c.SendPrompt(ctx, h.AgentID, "run ls"); err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}
	pr := waitForEvent(t, ch, EvPermissionRequest)
	var prd PermissionRequestData
	json.Unmarshal(pr.Data, &prd)

	if err := c.Permission(ctx, h.AgentID, prd.ToolCallID, "deny"); err != nil {
		t.Fatalf("Permission deny: %v", err)
	}
	waitForEvent(t, ch, EvTurnEnd)
	if fileExists(sentinel) {
		t.Fatal("sentinel exists after deny — tool ran despite denial")
	}
}

func TestPermissionTimeout(t *testing.T) {
	c, h, sentinel, ch := startPermAgent(t, false, "150ms")
	ctx := context.Background()

	if err := c.SendPrompt(ctx, h.AgentID, "run ls"); err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}
	waitForEvent(t, ch, EvPermissionRequest)
	// Do NOT decide. The runtime must auto-deny after PERMISSION_TIMEOUT.
	errEv := waitForEvent(t, ch, EvError)
	var ed ErrorData
	json.Unmarshal(errEv.Data, &ed)
	if ed.Message != "permission timed out" {
		t.Fatalf("error message = %q, want 'permission timed out'", ed.Message)
	}
	waitForEvent(t, ch, EvTurnEnd)
	if fileExists(sentinel) {
		t.Fatal("sentinel exists after timeout — auto-deny failed")
	}
}

func TestPermissionSkip(t *testing.T) {
	c, h, sentinel, ch := startPermAgent(t, true, "")
	ctx := context.Background()

	if err := c.SendPrompt(ctx, h.AgentID, "run ls"); err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}
	pr := waitForEvent(t, ch, EvPermissionRequest)
	var prd PermissionRequestData
	json.Unmarshal(pr.Data, &prd)
	if !prd.AutoApproved {
		t.Fatal("skip_permissions request should be auto-approved")
	}
	waitForEvent(t, ch, EvTurnEnd)
	if !fileExists(sentinel) {
		t.Fatal("sentinel missing — skip_permissions did not auto-run the tool")
	}
	// Never entered waiting_input.
	if st, _ := c.store.ReadStatus(h.AgentID); st.State == "waiting_input" {
		t.Fatal("skip_permissions must not enter waiting_input")
	}
}

func TestAgentDeckToolPermissionAutoApprovesExactIdentity(t *testing.T) {
	const identity = "mcp__agentdeck-messaging__send_message"
	t.Setenv("FAKEACP_PERMISSION_TITLE", identity)
	t.Setenv("TEST_AUTO_APPROVE_TOOL", identity)
	c, h, sentinel, ch := startPermAgent(t, false, "")
	if err := c.SendPrompt(context.Background(), h.AgentID, "send"); err != nil {
		t.Fatal(err)
	}
	pr := waitForEvent(t, ch, EvPermissionRequest)
	var data PermissionRequestData
	_ = json.Unmarshal(pr.Data, &data)
	if !data.AutoApproved {
		t.Fatal("AgentDeck tool request was not recorded as auto-approved")
	}
	// FS-03.A24 / INV §15: local truth precedes releasing a fast provider.
	resolved := waitForEvent(t, ch, EvPermissionResolved)
	var resolution PermissionResolvedData
	_ = json.Unmarshal(resolved.Data, &resolution)
	if resolution.Decision != "auto_approve" {
		t.Fatalf("auto-approval resolution = %+v", resolution)
	}
	waitForEvent(t, ch, EvTurnEnd)
	if !fileExists(sentinel) {
		t.Fatal("AgentDeck tool did not execute")
	}
}

// FS-03.A25: Stop resolves a held permission before teardown and cancels its
// optional timer, so the stopped status cannot be overwritten later.
func TestStopCancelsPendingPermissionAndTimer(t *testing.T) {
	for _, timeout := range []string{"", "150ms"} {
		t.Run("timeout="+timeout, func(t *testing.T) {
			c, h, sentinel, ch := startPermAgent(t, false, timeout)
			if err := c.SendPrompt(context.Background(), h.AgentID, "run ls"); err != nil {
				t.Fatal(err)
			}
			waitForEvent(t, ch, EvPermissionRequest)
			if err := c.Stop(context.Background(), h.AgentID); err != nil {
				t.Fatal(err)
			}
			resolved := waitForEvent(t, ch, EvPermissionResolved)
			var data PermissionResolvedData
			_ = json.Unmarshal(resolved.Data, &data)
			if data.Decision != "cancelled" || fileExists(sentinel) {
				t.Fatalf("stop resolution = %+v sentinel=%v", data, fileExists(sentinel))
			}
			time.Sleep(250 * time.Millisecond)
			status, err := c.store.ReadStatus(h.AgentID)
			if err != nil || status.State != "done" {
				t.Fatalf("status after old deadline = %+v err=%v", status, err)
			}
		})
	}
}

func TestAgentDeckToolPermissionFailsClosedOnDifferentServer(t *testing.T) {
	t.Setenv("FAKEACP_PERMISSION_TITLE", "mcp__other__send_message")
	t.Setenv("TEST_AUTO_APPROVE_TOOL", "mcp__agentdeck-messaging__send_message")
	c, h, sentinel, ch := startPermAgent(t, false, "")
	if err := c.SendPrompt(context.Background(), h.AgentID, "send"); err != nil {
		t.Fatal(err)
	}
	pr := waitForEvent(t, ch, EvPermissionRequest)
	var data PermissionRequestData
	_ = json.Unmarshal(pr.Data, &data)
	if data.AutoApproved || fileExists(sentinel) {
		t.Fatal("same-named tool from another server bypassed the gate")
	}
}

func TestAgentDeckToolDoesNotChooseAlwaysAllow(t *testing.T) {
	const identity = "mcp__agentdeck-messaging__send_message"
	t.Setenv("FAKEACP_PERMISSION_TITLE", identity)
	t.Setenv("FAKEACP_PERMISSION_ALLOW_ALWAYS_ONLY", "1")
	t.Setenv("TEST_AUTO_APPROVE_TOOL", identity)
	c, h, sentinel, ch := startPermAgent(t, false, "")
	if err := c.SendPrompt(context.Background(), h.AgentID, "send"); err != nil {
		t.Fatal(err)
	}
	pr := waitForEvent(t, ch, EvPermissionRequest)
	var data PermissionRequestData
	_ = json.Unmarshal(pr.Data, &data)
	if data.AutoApproved || fileExists(sentinel) {
		t.Fatal("AgentDeck selected an always-allow provider option")
	}
}

func TestPermissionDeadlineIsOffByDefault(t *testing.T) {
	t.Setenv("PERMISSION_TIMEOUT", "")
	if got := permissionTimeout(); got != 0 {
		t.Fatalf("default permission timeout = %s, want disabled", got)
	}
}

func TestPermissionUnknownToolCall(t *testing.T) {
	c, h, _, ch := startPermAgent(t, false, "")
	ctx := context.Background()
	if err := c.SendPrompt(ctx, h.AgentID, "run ls"); err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}
	waitForEvent(t, ch, EvPermissionRequest)
	if err := c.Permission(ctx, h.AgentID, "no_such_tc", "approve"); err != ErrNoPendingPermission {
		t.Fatalf("Permission unknown id err = %v, want ErrNoPendingPermission", err)
	}
}

func TestTakePendingSingleWinner(t *testing.T) {
	st, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	c := NewChatRuntime(st)
	as := &agentState{
		agentID: "a_perm_race",
		hub:     NewHub(),
		pending: map[string]*pendingPerm{
			"tc_p": {
				req:       &IncomingRequest{ID: 1, t: NewTransport(io.Discard, nil, nil)},
				optByKind: map[string]string{"allow_once": "opt_allow"},
			},
		},
		resolved: map[string]struct{}{},
	}

	var wg sync.WaitGroup
	wg.Add(2)
	wins := make(chan bool, 2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			_, err := c.takePending(as, "tc_p")
			wins <- err == nil
		}()
	}
	wg.Wait()
	close(wins)

	got := 0
	for ok := range wins {
		if ok {
			got++
		}
	}
	if got != 1 {
		t.Fatalf("takePending winners = %d, want 1", got)
	}
}

func TestTakePendingReportsAlreadyResolved(t *testing.T) {
	st, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	c := NewChatRuntime(st)
	as := &agentState{
		agentID:  "a_perm_done",
		hub:      NewHub(),
		pending:  map[string]*pendingPerm{},
		resolved: map[string]struct{}{"tc_p": {}},
	}

	if _, err := c.takePending(as, "tc_p"); err != ErrPermissionAlreadyResolved {
		t.Fatalf("takePending err = %v, want ErrPermissionAlreadyResolved", err)
	}
}

func TestCancelDuringPendingPermission(t *testing.T) {
	c, h, sentinel, ch := startPermAgent(t, false, "")
	ctx := context.Background()

	if err := c.SendPrompt(ctx, h.AgentID, "run ls"); err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}
	waitForEvent(t, ch, EvPermissionRequest)

	if cancelled, err := c.Cancel(ctx, h.AgentID); err != nil {
		t.Fatalf("Cancel: %v", err)
	} else if !cancelled {
		t.Fatal("Cancel reported no-op, want cancelled=true (pending permission was in flight)")
	}
	// Cancelling a pending permission must emit/persist permission_resolved so the
	// live UI and durable transcript render a resolved chip instead of leaving the
	// prompt actionable forever (FS-03.R9, R14–R16).
	prRes := waitForEvent(t, ch, EvPermissionResolved)
	var prd PermissionResolvedData
	json.Unmarshal(prRes.Data, &prd)
	if prd.Decision != "cancelled" {
		t.Fatalf("permission_resolved decision = %q, want cancelled", prd.Decision)
	}
	te := waitForEvent(t, ch, EvTurnEnd)
	var td TurnEndData
	json.Unmarshal(te.Data, &td)
	if td.StopReason != "cancelled" {
		t.Fatalf("stop_reason = %q, want cancelled", td.StopReason)
	}
	if fileExists(sentinel) {
		t.Fatal("sentinel exists after cancel — tool ran")
	}

	// Cancelling again now that the agent is idle is a no-op: reports false.
	if cancelled, err := c.Cancel(ctx, h.AgentID); err != nil {
		t.Fatalf("idle Cancel: %v", err)
	} else if cancelled {
		t.Fatal("idle Cancel reported cancelled=true, want no-op false")
	}
}

func TestCrashMidTurn(t *testing.T) {
	bin := buildFakeACP(t)
	st, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	agent := state.Agent{
		AgentID: "a_crash1", Name: "Nova", Role: "implementer", Project: "my-app",
		Backend: "claude", Model: "sonnet-4-6", Interface: "chat", CreatedAt: time.Now().UTC(),
	}
	st.WriteAgent(agent)

	c := NewChatRuntime(st)
	c.command = bin
	h, err := c.Start(context.Background(), LaunchSpec{
		Agent: agent, Cwd: t.TempDir(), BackendType: "claude-acp", ModelID: "claude-sonnet-4-6",
		Env: []string{"FAKEACP_SCENARIO=crash_midturn", "HOME=" + os.Getenv("HOME")},
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	ch, unsub, _ := c.Subscribe(h.AgentID)
	defer unsub()

	c.SendPrompt(context.Background(), h.AgentID, "go")
	errEv := waitForEvent(t, ch, EvError)
	var ed ErrorData
	json.Unmarshal(errEv.Data, &ed)
	if !ed.Fatal {
		t.Fatal("crash error should be fatal")
	}
	waitForEvent(t, ch, EvTurnEnd)

	// Running row deleted; status row error.
	if _, err := c.store.ReadRunning(h.AgentID); err == nil {
		t.Fatal("running row should be deleted after crash")
	}
	if status, _ := c.store.ReadStatus(h.AgentID); status.State != "error" {
		t.Fatalf("status after crash = %q, want error", status.State)
	}
}

// Regression (review fix): a peer that ignores session/cancel must not stay busy
// forever — Cancel escalates to SIGINT on the process group after the grace window
// (techspec §8.4), which reaps the hung agent (running row removed).
func TestCancelEscalatesToSIGINT(t *testing.T) {
	c, spec := newChatTest(t, "ignore_cancel")
	c.SetCancelGrace(100 * time.Millisecond)

	h, err := c.Start(context.Background(), spec)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	ch, unsub, err := c.Subscribe(h.AgentID)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer unsub()
	if err := c.SendPrompt(context.Background(), h.AgentID, "go"); err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}

	// Wait until the turn is active.
	busy := false
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
		if st, _ := c.store.ReadStatus(h.AgentID); st.State == "busy" {
			busy = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !busy {
		t.Fatal("turn never became busy")
	}

	// The peer ignores session/cancel → Cancel must escalate to SIGINT.
	if _, err := c.Cancel(context.Background(), h.AgentID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	reaped := false
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
		if _, err := c.store.ReadRunning(h.AgentID); err != nil {
			reaped = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !reaped {
		t.Fatal("agent still running after cancel; SIGINT escalation did not reach the hung peer")
	}
	if status, _ := c.store.ReadStatus(h.AgentID); status.State != "error" || status.Detail != "cancelled — process exited" {
		t.Fatalf("status after escalation = %+v, want error tied to cancellation", status)
	}
	errorEvent := waitForEvent(t, ch, EvError)
	var errorData ErrorData
	if err := json.Unmarshal(errorEvent.Data, &errorData); err != nil {
		t.Fatalf("decode error event: %v", err)
	}
	if !errorData.Fatal || !strings.Contains(errorData.Message, "after ignoring cancellation") {
		t.Fatalf("error event = %+v, want fatal cancellation-caused exit", errorData)
	}
	turnEnd := waitForEvent(t, ch, EvTurnEnd)
	var turnEndData TurnEndData
	if err := json.Unmarshal(turnEnd.Data, &turnEndData); err != nil {
		t.Fatalf("decode turn end: %v", err)
	}
	if turnEndData.StopReason != "cancelled" {
		t.Fatalf("stop reason = %q, want cancelled", turnEndData.StopReason)
	}
}

func TestReconcileStale(t *testing.T) {
	st, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	agent := state.Agent{
		AgentID: "a_stale1", Name: "Ghost", Role: "implementer", Project: "my-app",
		Backend: "claude", Model: "sonnet-4-6", Interface: "chat", CreatedAt: time.Now().UTC(),
	}
	st.WriteAgent(agent)
	// A running row with a pid that cannot be alive.
	st.WriteRunning(state.RunningEntry{
		AgentID: agent.AgentID, PID: 2147483600, SessionID: "s", Interface: "chat", StartedAt: time.Now().UTC(),
	})
	st.WriteStatus(state.Status{AgentID: agent.AgentID, State: "busy"})

	if err := ReconcileStale(st); err != nil {
		t.Fatalf("ReconcileStale: %v", err)
	}
	if _, err := st.ReadRunning(agent.AgentID); err == nil {
		t.Fatal("stale running row should be deleted")
	}
	if status, _ := st.ReadStatus(agent.AgentID); status.State != "error" {
		t.Fatalf("reconciled status = %q, want error", status.State)
	}
}
