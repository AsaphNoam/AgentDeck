package pipeline

import (
	"errors"
	"strings"
	"testing"

	"github.com/agentdeck/agentdeck/internal/state"
)

// Reproduction for the 2026-08-28 bug investigation finding "blocked stage agent
// answered in chat is refused with a false stale_assignment". Committed skipped;
// the /fix session un-skips it as the regression test.
//
// A blocked report pauses the run with its stage agent still live and idle
// (FS-14.R11), and the run detail offers Open agent beside Continue. When the
// person answers in that chat instead of the Continue box, the agent's next
// report is refused with `stale_assignment` / "caller is not the current stage
// attempt" — while the caller is exactly the current attempt under its current
// generation. Refusing is correct (FS-14.R19: an already accepted result must
// not change run state); the code and message are not (INV §8), and FS-17
// classifies `stale_assignment` as retry `never`, so the agent abandons the run
// on a false reason.
//
// `already_reported` is the shared vocabulary's name for this condition and the
// task path already returns it (internal/messaging/task_tools.go), so a second
// meaning for stale_assignment here is also INV §2 drift. The fix session owns
// the final code choice; adjust the expectation with it if it picks another.
func TestBlockedStageAgentAnsweredInChatIsNotCalledStale(t *testing.T) {
	t.Skip("reproduces an open bug-investigation finding; un-skip in the fix session")

	manager, _, _ := pipelineManagerFixture(t)
	detail := startPipeline(t, manager, "request-blocked-chat-answer")
	work := detail.Attempts[0]

	if _, err := manager.Report(work.AgentID, work.AgentGeneration, StageReport{
		Outcome: "blocked", Summary: "Which fallback should I use?",
	}); err != nil {
		t.Fatalf("blocked report: %v", err)
	}
	if err := manager.OnTurnEnd(work.AgentID, work.AgentGeneration); err != nil {
		t.Fatalf("turn end: %v", err)
	}
	paused, err := manager.Detail(detail.Run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if paused.Run.State != "paused" || paused.Run.AttentionReason != "blocked" {
		t.Fatalf("blocked pause = %+v", paused.Run)
	}
	// The agent the run is still pointing at, under the generation the run
	// itself recorded: it is the current stage attempt by every stored fact.
	current, ok := currentAttempt(paused)
	if !ok || current.AgentID != work.AgentID || !state.OwnsReportedWork(work.AgentID, work.AgentGeneration, current.AgentID, current.AgentGeneration) {
		t.Fatalf("current attempt = %+v, want the blocked agent's own live assignment", current)
	}

	// The person answers in the agent's chat, so the agent finishes the stage and
	// reports again without a Continue ever being pressed.
	_, err = manager.Report(work.AgentID, work.AgentGeneration, StageReport{
		Outcome: "success", Summary: "Used the fallback", Outputs: map[string]string{"implementation": "change"},
	})
	if err == nil {
		t.Fatal("second report accepted; FS-14.R19 requires an already accepted result to be refused")
	}
	var controlled *ControlError
	if !errors.As(err, &controlled) {
		t.Fatalf("refusal = %v, want a control error", err)
	}
	if controlled.Code == "stale_assignment" {
		t.Fatalf("refusal code = stale_assignment for the run's own current attempt: %q", controlled.Message)
	}
	if controlled.Code != "already_reported" {
		t.Fatalf("refusal code = %q, want already_reported", controlled.Code)
	}
	if strings.Contains(controlled.Message, "not the current stage attempt") {
		t.Fatalf("refusal message denies the caller's own assignment: %q", controlled.Message)
	}
}
