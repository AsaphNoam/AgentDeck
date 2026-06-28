//go:build acceptance

// Package runtime acceptance test — gated behind the `acceptance` build tag so
// credential-less CI never compiles or runs it (techspec §10.1). Run it against
// a logged-in real adapter with:
//
//	go test -tags acceptance ./internal/runtime -run TestRealCLIAcceptance -v
//
// Override the adapter binary with ACP_CMD (default: claude-code-acp on PATH).
// It verifies the §12.1 assumed wire shapes against reality; any drift is fixed
// in acpmap.go alone (the §12.1 isolation rule keeps the blast radius localized).
package runtime

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
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

// TestRealCLIAcceptance drives a real prompt → incremental stream → (optional)
// permission gate → turn end → stop against the real adapter (Appendix A).
func TestRealCLIAcceptance(t *testing.T) {
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
		AgentID: "a_accept1", Name: "Atlas", Role: "implementer", Project: "acc",
		Backend: "claude", Model: "sonnet-4-6", Interface: "chat", CreatedAt: time.Now().UTC(),
	}
	if err := st.WriteAgent(agent); err != nil {
		t.Fatalf("WriteAgent: %v", err)
	}

	c := NewChatRuntime(st)
	c.SetCommand(bin)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	h, err := c.Start(ctx, LaunchSpec{
		Agent: agent, Cwd: t.TempDir(), BackendType: "claude-acp",
		ModelID: "claude-sonnet-4-6", SkipPerms: false, Env: os.Environ(),
	})
	if err != nil {
		t.Fatalf("Start (real adapter handshake): %v", err)
	}
	t.Cleanup(func() { c.Stop(context.Background(), h.AgentID) })

	ch, unsub, err := c.Subscribe(h.AgentID)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer unsub()

	if err := c.SendPrompt(ctx, h.AgentID, "Reply with exactly one word: pong"); err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}

	var texts int
	var sawTurnEnd bool
	deadline := time.After(90 * time.Second)
loop:
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				break loop
			}
			switch ev.Type {
			case EvAssistantText:
				texts++
			case EvPermissionRequest:
				// If the adapter gates a tool, approve it so the turn completes.
				var prd PermissionRequestData
				json.Unmarshal(ev.Data, &prd)
				_ = c.Permission(ctx, h.AgentID, prd.ToolCallID, "approve")
			case EvTurnEnd:
				sawTurnEnd = true
				break loop
			case EvError:
				var ed ErrorData
				json.Unmarshal(ev.Data, &ed)
				if ed.Fatal {
					t.Fatalf("fatal error from real adapter: %s", ed.Message)
				}
			}
		case <-deadline:
			t.Fatalf("timed out; assistant_text=%d turn_end=%v", texts, sawTurnEnd)
		}
	}

	if texts == 0 {
		t.Error("expected at least one assistant_text delta from the real adapter")
	}
	if !sawTurnEnd {
		t.Error("expected a turn_end from the real adapter")
	}

	// Status row should be back to idle with the session reflected.
	if status, err := c.store.ReadStatus(h.AgentID); err != nil || status.State != "idle" {
		t.Errorf("post-turn status = %+v err=%v, want idle", status, err)
	}
}
