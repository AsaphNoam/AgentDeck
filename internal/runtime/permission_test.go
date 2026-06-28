package runtime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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
