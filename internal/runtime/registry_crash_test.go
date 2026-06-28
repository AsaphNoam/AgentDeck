package runtime

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/state"
)

// TestRegistryForgetsAgentAfterCrash asserts the crash-teardown fix: when the
// ACP process crashes mid-turn, the runtime tears its handle down AND the
// Registry drops ownership, so a relaunch on the same agent_id is no longer
// rejected with ErrAlreadyStarted (techspec §8.2).
func TestRegistryForgetsAgentAfterCrash(t *testing.T) {
	bin := buildFakeACP(t)
	st, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	agent := state.Agent{
		AgentID: "a_reg_crash1", Name: "Nova", Role: "implementer", Project: "my-app",
		Backend: "claude", Model: "sonnet-4-6", Interface: "chat", CreatedAt: time.Now().UTC(),
	}
	st.WriteAgent(agent)

	r := NewRegistry(st)
	r.Chat().SetCommand(bin)

	spec := LaunchSpec{
		Agent: agent, Cwd: t.TempDir(), BackendType: "claude-acp", ModelID: "claude-sonnet-4-6",
		Env: []string{"FAKEACP_SCENARIO=crash_midturn", "HOME=" + os.Getenv("HOME")},
	}
	ctx := context.Background()
	h, err := r.Launch(ctx, spec)
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}

	// Ownership is recorded while live.
	if _, err := r.ownerFor(h.AgentID); err != nil {
		t.Fatalf("ownerFor while live: %v, want owned", err)
	}

	ch, unsub, err := r.Subscribe(h.AgentID)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer unsub()

	if err := r.SendPrompt(ctx, h.AgentID, "go"); err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}
	// The crash surfaces as a fatal error then turn_end. onExit runs before the
	// turn_end emit, so by the time we observe it the registry has forgotten.
	waitForEvent(t, ch, EvError)
	waitForEvent(t, ch, EvTurnEnd)

	if _, err := r.ownerFor(h.AgentID); !errors.Is(err, ErrNoHandle) {
		t.Fatalf("ownerFor after crash = %v, want ErrNoHandle (ownership not dropped)", err)
	}

	// The decisive check: a relaunch on the same agent_id must not be blocked.
	if _, err := r.Launch(ctx, spec); errors.Is(err, ErrAlreadyStarted) {
		t.Fatal("relaunch after crash rejected with ErrAlreadyStarted — stale ownership")
	} else if err != nil {
		t.Fatalf("relaunch after crash: %v", err)
	}
	_ = r.Stop(ctx, h.AgentID)
}
