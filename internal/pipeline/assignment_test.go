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

	text, _ := renderAssignment(run, Template{}, stage, values, nil, "")
	if utf8.RuneCountInString(text) > maxAssignmentRunes {
		t.Fatalf("assignment has %d runes, max %d", utf8.RuneCountInString(text), maxAssignmentRunes)
	}
	for _, required := range []string{
		"call report_pipeline_stage_result exactly once",
		"outcome success, failure, or blocked",
		"Declared outputs (use these local names):\n- implementation",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("assignment lost required protocol %q", required)
		}
	}
}
