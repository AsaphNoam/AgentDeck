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
	assignmentVersion  = 1
	maxAssignmentRunes = 48000
)

func renderAssignment(run state.PipelineRunRecord, template Template, stage Stage, values []state.PipelineValueRecord, attempts []state.PipelineAttemptRecord, continuation string) (string, string) {
	valueMap := map[string]string{}
	for _, value := range values {
		valueMap[value.Name] = value.Value
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Pipeline stage assignment\n\nRun: %s (%s)\nGoal:\n%s\n\nStage: %s (%s)\nResponsibility:\n%s\n",
		clipText(run.DisplayName, MaxTitleRunes), run.RunID, clipText(run.Goal, MaxGoalRunes), stage.Title, stage.ID, clipText(stage.Instruction, MaxInstructionRunes))
	if len(stage.Inputs) > 0 {
		b.WriteString("\nDeclared inputs:\n")
		for _, input := range stage.Inputs {
			fmt.Fprintf(&b, "- %s: %s\n", input.Name, clipText(valueMap[input.Value], MaxValueRunes))
		}
	}
	prior := make([]state.PipelineAttemptRecord, 0, len(attempts))
	for _, attempt := range attempts {
		if attempt.ReportOutcome != "" {
			prior = append(prior, attempt)
		}
	}
	if len(prior) > 0 {
		b.WriteString("\nPrior structured results:\n")
		for _, attempt := range prior {
			fmt.Fprintf(&b, "- %s attempt %d: %s — %s\n", attempt.StageID, attempt.AttemptNo, attempt.ReportOutcome, clipText(attempt.ReportSummary, MaxSummaryRunes))
		}
	}
	if strings.TrimSpace(continuation) != "" {
		fmt.Fprintf(&b, "\nHuman continuation input:\n%s\n", clipText(continuation, MaxValueRunes))
	}
	b.WriteString("\nScope: perform only this stage's responsibility in the shared project workspace. Do not claim that runtime status alone completes the stage.\n")
	b.WriteString("\nBefore finishing, call report_pipeline_stage_result exactly once with outcome success, failure, or blocked, plus a bounded summary, details/checks, and declared outputs.\n")
	if len(stage.Outputs) > 0 {
		b.WriteString("Declared outputs (use these local names):\n")
		outputs := append([]StageOutput{}, stage.Outputs...)
		sort.Slice(outputs, func(i, j int) bool { return outputs[i].Name < outputs[j].Name })
		for _, output := range outputs {
			fmt.Fprintf(&b, "- %s: %s\n", output.Name, clipText(output.Description, MaxDescriptionRunes))
		}
	}
	text := clipText(b.String(), maxAssignmentRunes)
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
