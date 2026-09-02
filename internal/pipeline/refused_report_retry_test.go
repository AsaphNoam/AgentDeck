package pipeline

import (
	"errors"
	"strings"
	"testing"
)

// Regression for the 2026-09-02 bug investigation finding "a refused stage
// report is never retried because the assignment says to report exactly once".
//
// Two stalls in that run — about nine hours and about ten hours — happened after
// all the work was done: the stage agent called report_pipeline_stage_result,
// AgentDeck did not accept the call, and the agent read its own assignment's
// "exactly once" instruction as forbidding a second attempt. Nothing else in
// the run could advance it, so it sat until a person went into the agent's chat
// and told it to try again, at which point the same preserved result was
// accepted.
//
// The assignment text is unconditional (assignment.go:35-40: "call
// report_pipeline_stage_result exactly once", "That one call ends your part in
// this assignment", "only then is another report accepted"), while FS-17's
// shipped retry vocabulary classifies several report refusals as retryable —
// `validation_failed` as `after_change` and `pipeline_unavailable` as
// `transient` (internal/messaging/tools.go:50-61). Nothing reconciles the two,
// and nothing in the tool description or the refusal itself explains what a
// retry class means, so the instruction the agent reads every turn wins.
//
// This test pins the product fact the instruction contradicts: a refused report
// leaves the attempt exactly where it was (FS-14.R19), so retrying the
// corrected call is both permitted and the only way to finish the stage.
// The fix session owns the final wording; adjust the expectation with it.
func TestRefusedStageReportMustBeRetryable(t *testing.T) {
	manager, _, _ := pipelineManagerFixture(t)
	detail := startPipeline(t, manager, "request-refused-report-retry")
	work := detail.Attempts[0]

	// A refusal FS-17 classifies `after_change`: the work is done and the result
	// is right, but one output name is not declared by the stage.
	_, err := manager.Report(work.AgentID, work.AgentGeneration, StageReport{
		Outcome: "success", Summary: "Implemented every unit",
		Outputs: map[string]string{"implementation": "change", "notes": "undeclared"},
	})
	var controlled *ControlError
	if !errors.As(err, &controlled) || controlled.Code != "validation_failed" {
		t.Fatalf("refusal = %v, want validation_failed", err)
	}
	if !strings.Contains(controlled.Message, "still owes a result") || !strings.Contains(controlled.Message, "call report_pipeline_stage_result again") {
		t.Fatalf("refusal does not explain the retryable boundary: %q", controlled.Message)
	}

	// FS-14.R19: the refusal changed nothing, so the attempt is still the run's
	// current one and still owes a result. The agent is the only thing that can
	// supply it.
	stalled, err := manager.Detail(detail.Run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if stalled.Run.State != "running" || stalled.Run.PendingAction != "await_result" {
		t.Fatalf("run after a refused report = %+v, want it still awaiting the result", stalled.Run)
	}

	// The corrected retry is accepted, which is exactly the second call the
	// assignment told the agent not to make.
	if _, err := manager.Report(work.AgentID, work.AgentGeneration, StageReport{
		Outcome: "success", Summary: "Implemented every unit",
		Outputs: map[string]string{"implementation": "change"},
	}); err != nil {
		t.Fatalf("corrected retry refused: %v", err)
	}

	// So the assignment must not present the one call as unconditional. It has
	// to separate "AgentDeck accepted your result" from "you invoked the tool".
	lowered := strings.ToLower(work.AssignmentText)
	for _, required := range []string{"agentdeck accepts the result", "refused call records nothing", "still owing a result", "send it again"} {
		if !strings.Contains(lowered, required) {
			t.Fatalf("assignment omits %q from the accepted-result boundary:\n%s", required, work.AssignmentText)
		}
	}
}
