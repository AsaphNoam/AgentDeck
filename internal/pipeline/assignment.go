package pipeline

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/agentdeck/agentdeck/internal/state"
)

const (
	assignmentVersion  = 2
	maxAssignmentRunes = 48000
)

func renderAssignment(run state.PipelineRunRecord, template Template, stage Stage, values []state.PipelineValueRecord, attempts []state.PipelineAttemptRecord, continuation string) (string, string) {
	valueMap := map[string]string{}
	for _, value := range values {
		valueMap[value.Name] = value.Value
	}
	outputs := append([]StageOutput{}, stage.Outputs...)
	sort.Slice(outputs, func(i, j int) bool { return outputs[i].Name < outputs[j].Name })

	// Keep the protocol and every declared output name outside the variable-text
	// budget. A single legal 64k input can exceed the whole assignment limit, so
	// clipping the fully rendered prompt would otherwise remove the one instruction
	// that lets the stage complete the run.
	var fixed strings.Builder
	fmt.Fprintf(&fixed, "# Pipeline stage assignment\n\nRun: %s (%s)\nStage: %s (%s)\n",
		clipText(run.DisplayName, MaxTitleRunes), run.RunID, stage.Title, stage.ID)
	fixed.WriteString("\nScope: perform only this stage's responsibility in the shared project workspace. Do not claim that runtime status alone completes the stage.\n")
	fixed.WriteString("\nBefore finishing, call report_pipeline_stage_result exactly once with outcome success, failure, or blocked, plus a bounded summary, details/checks, and declared outputs.\n")
	if len(outputs) > 0 {
		fixed.WriteString("Declared outputs (use these local names):\n")
		for _, output := range outputs {
			fmt.Fprintf(&fixed, "- %s\n", output.Name)
		}
	}

	var variable strings.Builder
	fmt.Fprintf(&variable, "\nGoal:\n%s\n\nResponsibility:\n%s\n",
		clipText(run.Goal, MaxGoalRunes), clipText(stage.Instruction, MaxInstructionRunes))
	if len(stage.Inputs) > 0 {
		variable.WriteString("\nDeclared inputs:\n")
		for _, input := range stage.Inputs {
			fmt.Fprintf(&variable, "- %s: %s\n", input.Name, clipText(valueMap[input.Value], MaxValueRunes))
		}
	}
	prior := make([]state.PipelineAttemptRecord, 0, len(attempts))
	for _, attempt := range attempts {
		if attempt.ReportOutcome != "" {
			prior = append(prior, attempt)
		}
	}
	if len(prior) > 0 {
		variable.WriteString("\nPrior structured results:\n")
		for _, attempt := range prior {
			fmt.Fprintf(&variable, "- %s attempt %d: %s — %s\n", attempt.StageID, attempt.AttemptNo, attempt.ReportOutcome, clipText(attempt.ReportSummary, MaxSummaryRunes))
		}
	}
	if strings.TrimSpace(continuation) != "" {
		fmt.Fprintf(&variable, "\nHuman continuation input:\n%s\n", clipText(continuation, MaxValueRunes))
	}
	if len(outputs) > 0 {
		variable.WriteString("\nOutput guidance:\n")
		for _, output := range outputs {
			fmt.Fprintf(&variable, "- %s: %s\n", output.Name, clipText(output.Description, MaxDescriptionRunes))
		}
	}
	fixedText := fixed.String()
	remaining := maxAssignmentRunes - utf8.RuneCountInString(fixedText)
	if remaining < 0 {
		remaining = 0
	}
	text := fixedText + clipText(variable.String(), remaining)
	sum := sha256.Sum256([]byte(text))
	return text, hex.EncodeToString(sum[:])
}

func clipText(value string, max int) string {
	if utf8.RuneCountInString(value) <= max {
		return value
	}
	runes := []rune(value)
	return string(runes[:max])
}
