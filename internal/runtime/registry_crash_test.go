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

// TestResumedAgentCrashCarriesItsGeneration pins INV §4 on the resume path: a
// resumed runtime must carry the concrete launch generation the Registry
// recorded, or handleAgentExit rejects the unsolicited exit as stale and skips
// ownership teardown, the server's registration teardown, and the pipeline's
// agent_crash recovery — leaving a later Resume to fail with ErrAlreadyStarted.
func TestResumedAgentCrashCarriesItsGeneration(t *testing.T) {
	bin := buildFakeACP(t)
	st, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	agent := state.Agent{
		AgentID: "a_reg_resume_crash", Name: "Nova", Role: "implementer", Project: "my-app",
		Backend: "claude", Model: "sonnet-4-6", Interface: "chat", CreatedAt: time.Now().UTC(),
	}
	st.WriteAgent(agent)

	r := NewRegistry(st)
	r.Chat().SetCommand(bin)

	type exitCall struct{ agentID, generation, cause string }
	exits := make(chan exitCall, 4)
	r.SetExitHook(func(agentID, generation, cause string) {
		exits <- exitCall{agentID, generation, cause}
	})

	spec := LaunchSpec{
		Agent: agent, Cwd: t.TempDir(), BackendType: "claude-acp", ModelID: "claude-sonnet-4-6",
		Generation: "gen-resume-1", LastSessionID: "prior-session-id",
		Env: []string{"FAKEACP_SCENARIO=crash_midturn", "HOME=" + os.Getenv("HOME")},
	}
	ctx := context.Background()
	h, err := r.Resume(ctx, spec)
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}

	ch, unsub, err := r.Subscribe(h.AgentID)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer unsub()

	if err := r.SendPrompt(ctx, h.AgentID, "go"); err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}
	waitForEvent(t, ch, EvError)
	waitForEvent(t, ch, EvTurnEnd)

	select {
	case got := <-exits:
		if got.agentID != h.AgentID || got.generation != "gen-resume-1" || got.cause != "process_exit" {
			t.Fatalf("exit hook = %+v, want agent %q generation gen-resume-1 cause process_exit", got, h.AgentID)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("resumed agent crashed without running the registration teardown hook")
	}

	if _, err := r.ownerFor(h.AgentID); !errors.Is(err, ErrNoHandle) {
		t.Fatalf("ownerFor after resumed crash = %v, want ErrNoHandle (ownership not dropped)", err)
	}

	// A second Resume on the same agent must not be blocked by stale ownership.
	spec.Generation = "gen-resume-2"
	spec.Env = []string{"FAKEACP_SCENARIO=stream_text", "HOME=" + os.Getenv("HOME")}
	if _, err := r.Resume(ctx, spec); err != nil {
		t.Fatalf("re-resume after crash: %v", err)
	}
	_ = r.Stop(ctx, h.AgentID)
}
