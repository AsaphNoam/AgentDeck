package pipeline

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/agentdeck/agentdeck/internal/state"
)

// FS-14.R5 / TS-09.R7: legal named values may be larger than the assignment
// budget, but they may never truncate away the stage-result protocol or output
// names that let the run advance.
func TestRenderAssignmentPreservesReportingProtocolAtMaximumInput(t *testing.T) {
	run := state.PipelineRunRecord{RunID: "pr_large", DisplayName: "Large input", Goal: "Process the specification"}
	stage := Stage{
		ID: "work", Title: "Work", Instruction: "Implement it.",
		Inputs:  []StageInput{{Name: "specification", Value: "spec", Required: true}},
		Outputs: []StageOutput{{Name: "implementation", Value: "implementation", Description: "What changed"}},
	}
	values := []state.PipelineValueRecord{{Name: "spec", Value: strings.Repeat("界", MaxValueRunes)}}

	text, _ := renderAssignment(run, Template{}, stage, values, assignmentContext{})
	if utf8.RuneCountInString(text) > maxAssignmentRunes {
		t.Fatalf("assignment has %d runes, max %d", utf8.RuneCountInString(text), maxAssignmentRunes)
	}
	for _, required := range []string{
		"call report_task_result",
		"Your part ends only when AgentDeck accepts the result",
		"sole authority for the stage result",
		"outcome success, failure, or blocked",
		"Declared outputs (use these local names):\n- implementation",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("assignment lost required protocol %q", required)
		}
	}
}

func TestRenderAssignmentUsesVersionTwoObjective(t *testing.T) {
	run := state.PipelineRunRecord{RunID: "pr_1", DisplayName: "Release", Goal: "ship it"}
	stage := Stage{ID: "build", Title: "Build", Objective: "Implement the accepted design", Instruction: "obsolete legacy instruction"}

	prompt, _ := renderAssignment(run, Template{Version: 2}, stage, nil, assignmentContext{})
	if !strings.Contains(prompt, "Responsibility:\nImplement the accepted design") {
		t.Fatalf("assignment does not contain v2 objective: %s", prompt)
	}
	if strings.Contains(prompt, "obsolete legacy instruction") {
		t.Fatalf("assignment contains legacy instruction: %s", prompt)
	}
}

// TS-09.R39/R41/R49 — a persisted handoff is all a continuation or replacement
// in a fresh conversation receives. Without the managed child's id the owner
// cannot delegate through it, and without prior accepted results it re-derives
// findings the run already paid for.
func TestRenderAssignmentCarriesManagedChildAndPriorResults(t *testing.T) {
	run := state.PipelineRunRecord{RunID: "pr_2", DisplayName: "Release", Goal: "ship it"}
	stage := Stage{ID: "review", Title: "Review", Objective: "Review the change"}

	prompt, _ := renderAssignment(run, Template{Version: 2}, stage, nil, assignmentContext{
		Coordinator: &coordinatorHandoff{TaskID: "tk_coord", Role: "implementer", Objective: "Implement the change"},
		PriorResults: []stageResultSummary{{
			StageID: "work", Attempt: 2, Outcome: state.OutcomeSuccess, Summary: "implemented",
			TaskID: "tk_work2", Outputs: map[string]string{"implementation": "added the endpoint"},
		}},
	})
	for _, required := range []string{
		"Managed coordinator: task tk_coord (role implementer)",
		"Objective delegated to it: Implement the change",
		"It reports upward to you",
		"Prior accepted results:",
		"- work attempt 2 (task tk_work2): success — implemented",
		"  - implementation: added the endpoint",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("assignment is missing %q:\n%s", required, prompt)
		}
	}
}
