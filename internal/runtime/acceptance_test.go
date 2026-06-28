//go:build acceptance

// Package runtime acceptance tests — gated behind the `acceptance` build tag so
// credential-less CI never compiles or runs them (techspec §10.1). Run against a
// logged-in real adapter with:
//
//	go test -tags acceptance ./internal/runtime -v
//
// Override the adapter binary with ACP_CMD (default: claude-code-acp on PATH).
// These verify the §12.1 assumed wire shapes AND the Appendix A acceptance
// checklist (stream, permission gate, cancel, stop) against reality. Any wire
// drift is fixed in acpmap.go alone (the §12.1 isolation rule keeps it localized).
package runtime

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/state"
)

func acceptanceCmd() string {
	if v := os.Getenv("ACP_CMD"); v != "" {
		return v
	}
	return "claude-code-acp"
}

// startRealAgent spins up a ChatRuntime against the real adapter, seeds an
// identity row, starts the agent in cwd, and returns the runtime + handle +
// subscription. It skips the test if the adapter is not on PATH.
func startRealAgent(t *testing.T, ctx context.Context, id, cwd string, skipPerms bool) (*ChatRuntime, *Handle, <-chan Event, func()) {
	t.Helper()
	bin := acceptanceCmd()
	if _, err := exec.LookPath(bin); err != nil {
		t.Skipf("acceptance adapter %q not on PATH (set ACP_CMD); skipping", bin)
	}

	st, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	agent := state.Agent{
		AgentID: id, Name: "Atlas", Role: "implementer", Project: "acc",
		Backend: "claude", Model: "sonnet-4-6", Interface: "chat", CreatedAt: time.Now().UTC(),
	}
	if err := st.WriteAgent(agent); err != nil {
		t.Fatalf("WriteAgent: %v", err)
	}

	c := NewChatRuntime(st)
	c.SetCommand(bin)

	h, err := c.Start(ctx, LaunchSpec{
		Agent: agent, Cwd: cwd, BackendType: "claude-acp",
		ModelID: "claude-sonnet-4-6", SkipPerms: skipPerms, Env: os.Environ(),
	})
	if err != nil {
		t.Fatalf("Start (real adapter handshake): %v", err)
	}
	t.Cleanup(func() { c.Stop(context.Background(), h.AgentID) })

	ch, unsub, err := c.Subscribe(h.AgentID)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	t.Cleanup(unsub)
	return c, h, ch, unsub
}

// TestRealCLIAcceptance: prompt → incremental assistant_text stream → turn_end →
// idle status against the real adapter (Appendix A: incremental streaming, status
// idle→busy→idle).
func TestRealCLIAcceptance(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	c, h, ch, _ := startRealAgent(t, ctx, "a_accept1", t.TempDir(), false)

	if err := c.SendPrompt(ctx, h.AgentID, "Reply with exactly one word: pong"); err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}

	var texts int
	td, fatal := drainRealTurn(t, ctx, c, h, ch, &texts, "approve")
	if fatal != "" {
		t.Fatalf("fatal error from real adapter: %s", fatal)
	}
	if texts == 0 {
		t.Error("expected at least one assistant_text delta from the real adapter")
	}
	if td == nil {
		t.Fatal("expected a turn_end from the real adapter")
	}
	if status, err := c.store.ReadStatus(h.AgentID); err != nil || status.State != "idle" {
		t.Errorf("post-turn status = %+v err=%v, want idle", status, err)
	}
}

// TestRealCLIPermissionDeny: a prompt that needs a tool gates on a real
// permission request; denying it prevents the tool's side effect (Appendix A:
// "Permission gates execution; deny prevents tool"). We assert the side effect
// directly (the sentinel never appears) rather than waiting for the model to end
// the turn — after a denial the real model may retry/reason for a long time.
func TestRealCLIPermissionDeny(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cwd := t.TempDir()
	sentinel := filepath.Join(cwd, "deny_sentinel")
	c, h, ch, _ := startRealAgent(t, ctx, "a_deny01", cwd, false)

	perms := relayDecisions(t, ctx, c, h, ch, "deny")
	prompt := "Use your Bash tool to run exactly this one command and nothing else, do not explain: echo ok > " + sentinel
	if err := c.SendPrompt(ctx, h.AgentID, prompt); err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}

	if !waitCount(perms, 1, 120*time.Second) {
		t.Fatal("expected a real permission_request for the Bash tool (default permission mode should gate)")
	}
	// Give the (denied) tool a generous window to NOT run, auto-denying any retries.
	time.Sleep(8 * time.Second)
	if fileExists(sentinel) {
		t.Fatal("sentinel exists after deny — the real adapter ran the tool despite denial")
	}
	t.Logf("deny: %d permission request(s) relayed; sentinel correctly absent", perms.Load())
}

// TestRealCLIPermissionApprove: approving the real permission request lets the
// tool run and produce its side effect (Appendix A: permission gate, tool runs).
func TestRealCLIPermissionApprove(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cwd := t.TempDir()
	sentinel := filepath.Join(cwd, "approve_sentinel")
	c, h, ch, _ := startRealAgent(t, ctx, "a_appr01", cwd, false)

	perms := relayDecisions(t, ctx, c, h, ch, "approve")
	prompt := "Use your Bash tool to run exactly this one command and nothing else, do not explain: echo ok > " + sentinel
	if err := c.SendPrompt(ctx, h.AgentID, prompt); err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}

	if !waitCount(perms, 1, 120*time.Second) {
		t.Fatal("expected a real permission_request for the Bash tool")
	}
	// After approval the tool runs; poll for its side effect.
	if !waitFile(sentinel, 60*time.Second) {
		t.Fatal("sentinel missing after approve — the real adapter's tool did not run")
	}
	t.Logf("approve: %d permission request(s) relayed; sentinel created", perms.Load())
}

// TestRealCLICancel: a Cancel during an in-flight real turn interrupts it; the
// agent returns to idle (Appendix A: "Cancel interrupts").
func TestRealCLICancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	c, h, ch, _ := startRealAgent(t, ctx, "a_cancel1", t.TempDir(), true)

	// A prompt that yields a longer multi-token answer so there's a turn to cancel.
	if err := c.SendPrompt(ctx, h.AgentID, "Write a detailed 10-paragraph essay about the history of computing."); err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}

	// Wait for the turn to actually start (first streamed event), then cancel.
	select {
	case <-ch:
	case <-time.After(60 * time.Second):
		t.Fatal("no events before cancel — turn never started")
	}
	if _, err := c.Cancel(ctx, h.AgentID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	// The turn must end (cancelled or otherwise) and the agent return to idle.
	sawEnd := false
	deadline := time.After(60 * time.Second)
	for !sawEnd {
		select {
		case ev, ok := <-ch:
			if !ok {
				sawEnd = true
				break
			}
			if ev.Type == EvTurnEnd {
				sawEnd = true
			}
		case <-deadline:
			t.Fatal("turn did not end within 60s of Cancel")
		}
	}
	// Give the status write a moment to settle.
	time.Sleep(200 * time.Millisecond)
	if status, _ := c.store.ReadStatus(h.AgentID); status.State != "idle" {
		t.Errorf("post-cancel status = %q, want idle", status.State)
	}
}

// TestRealCLIStop: Stop terminates the process group and removes the running row
// (Appendix A: "Stop kills group + removes running row").
func TestRealCLIStop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()
	c, h, _, _ := startRealAgent(t, ctx, "a_stop01", t.TempDir(), false)

	if _, err := c.store.ReadRunning(h.AgentID); err != nil {
		t.Fatalf("running row missing before stop: %v", err)
	}
	if !pidAlive(h.Pid) {
		t.Fatalf("process group %d not alive before stop", h.Pid)
	}

	if err := c.Stop(ctx, h.AgentID); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	if _, err := c.store.ReadRunning(h.AgentID); err == nil {
		t.Fatal("running row still present after Stop")
	}
	// The process group should be gone (give SIGTERM a brief moment).
	time.Sleep(300 * time.Millisecond)
	if pidAlive(h.Pid) {
		t.Errorf("process group %d still alive after Stop", h.Pid)
	}
	if status, _ := c.store.ReadStatus(h.AgentID); status.State != "done" {
		t.Errorf("post-stop status = %q, want done", status.State)
	}
}

// relayDecisions drains the agent's event channel in the background, relaying the
// given approve/deny decision for every permission_request, and returns a counter
// of how many it has relayed. It stops when the channel closes or ctx is done.
func relayDecisions(t *testing.T, ctx context.Context, c *ChatRuntime, h *Handle, ch <-chan Event, decision string) *atomic.Int64 {
	t.Helper()
	var n atomic.Int64
	go func() {
		for {
			select {
			case ev, ok := <-ch:
				if !ok {
					return
				}
				if ev.Type == EvPermissionRequest {
					var prd PermissionRequestData
					json.Unmarshal(ev.Data, &prd)
					if n.Load() == 0 {
						t.Logf("real permission_request: name=%q options=%+v", prd.Name, prd.Options)
					}
					if err := c.Permission(ctx, h.AgentID, prd.ToolCallID, decision); err != nil {
						t.Errorf("Permission(%s): %v (option kinds may differ from §5.3 — fix selectOption/acpmap)", decision, err)
					}
					n.Add(1)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return &n
}

// waitCount polls until the counter reaches want (or timeout).
func waitCount(n *atomic.Int64, want int64, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if n.Load() >= want {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return n.Load() >= want
}

// waitFile polls until the path exists (or timeout).
func waitFile(path string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fileExists(path) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fileExists(path)
}

// drainRealTurn consumes events until turn_end, relaying any further permission
// requests with the given decision. Returns the turn_end payload (or nil) and a
// fatal error message (or "").
func drainRealTurn(t *testing.T, ctx context.Context, c *ChatRuntime, h *Handle, ch <-chan Event, texts *int, decision string) (*TurnEndData, string) {
	t.Helper()
	deadline := time.After(120 * time.Second)
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return nil, ""
			}
			switch ev.Type {
			case EvAssistantText:
				*texts++
			case EvPermissionRequest:
				var prd PermissionRequestData
				json.Unmarshal(ev.Data, &prd)
				_ = c.Permission(ctx, h.AgentID, prd.ToolCallID, decision)
			case EvTurnEnd:
				var td TurnEndData
				json.Unmarshal(ev.Data, &td)
				return &td, ""
			case EvError:
				var ed ErrorData
				json.Unmarshal(ev.Data, &ed)
				if ed.Fatal {
					return nil, ed.Message
				}
			}
		case <-deadline:
			t.Fatalf("timed out draining real turn; assistant_text=%d", *texts)
		}
	}
}
