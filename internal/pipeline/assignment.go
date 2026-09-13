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
	assignmentVersion  = 3
	maxAssignmentRunes = 48000
)

// coordinatorHandoff names the managed child a dedicated stage delegates
// through. The standing owner cannot delegate to, correct, or observe a child
// whose id it was never told (TS-09.R39/R49).
type coordinatorHandoff struct {
	TaskID    string
	Role      string
	Objective string
}

// stageResultSummary is one earlier accepted stage result, rendered so a
// continuation or replacement in a fresh conversation still receives the prior
// findings and the authoritative named sources (TS-09.R39/R41).
type stageResultSummary struct {
	StageID string
	Attempt int
	Outcome string
	Summary string
	TaskID  string
	Outputs map[string]string
}

// assignmentContext carries the durable facts a stage assignment needs beyond
// the frozen template and run values. Every field is bounded before rendering.
type assignmentContext struct {
	Coordinator  *coordinatorHandoff
	PriorResults []stageResultSummary
	Continuation string
}

func renderAssignment(run state.PipelineRunRecord, template Template, stage Stage, values []state.PipelineValueRecord, ctx assignmentContext) (string, string) {
	continuation := ctx.Continuation
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
	fixed.WriteString("\nBefore finishing, call report_task_result with outcome success, failure, or blocked, plus a bounded summary, details/checks, and declared outputs. This assigned task is the sole authority for the stage result. Your part ends only when AgentDeck accepts the result.\n")
	// The boundary the agent cannot otherwise see: reporting ends this attempt's
	// participation, and a blocked report leaves the agent live and idle beside an
	// Open agent action, so without this an operator's chat answer produces work
	// that the run can never accept (FS-14.R47).
	fixed.WriteString("\nAn accepted result closes this stage task before cleanup. If AgentDeck accepts a blocked result, the run pauses for a person; do not continue stage work until a new assigned task arrives.\n")
	if len(outputs) > 0 {
		fixed.WriteString("Declared outputs (use these local names):\n")
		for _, output := range outputs {
			fmt.Fprintf(&fixed, "- %s\n", output.Name)
		}
	}
	if ctx.Coordinator != nil {
		// Without the child's id the owner cannot read, correct, watch or cancel
		// the work this stage is configured to delegate (TS-09.R49).
		fmt.Fprintf(&fixed, "\nManaged coordinator: task %s (role %s).\nObjective delegated to it: %s\nDelegate this stage's work through that task: read its progress with get_task, watch it with wait_for_tasks, correct it with ordinary mail, and cancel it if it is no longer useful. It reports upward to you; it never reports this pipeline stage.\n",
			ctx.Coordinator.TaskID, ctx.Coordinator.Role, clipText(ctx.Coordinator.Objective, MaxInstructionRunes))
	}

	responsibility := stage.Objective
	if strings.TrimSpace(responsibility) == "" {
		// Retain readable diagnostics/fixtures for old documents while v2 templates
		// use objective as the only stage responsibility field.
		responsibility = stage.Instruction
	}
	var variable strings.Builder
	fmt.Fprintf(&variable, "\nGoal:\n%s\n\nResponsibility:\n%s\n",
		clipText(run.Goal, MaxGoalRunes), clipText(responsibility, MaxInstructionRunes))
	if len(stage.Inputs) > 0 {
		variable.WriteString("\nDeclared inputs:\n")
		for _, input := range stage.Inputs {
			fmt.Fprintf(&variable, "- %s: %s\n", input.Name, clipText(valueMap[input.Value], MaxValueRunes))
		}
	}
	if len(ctx.PriorResults) > 0 {
		// A continuation or replacement often starts a fresh conversation, so the
		// only durable record of what earlier attempts found is what this handoff
		// carries (TS-09.R39/R41).
		variable.WriteString("\nPrior accepted results:\n")
		for _, prior := range ctx.PriorResults {
			fmt.Fprintf(&variable, "- %s attempt %d (task %s): %s — %s\n",
				prior.StageID, prior.Attempt, prior.TaskID, prior.Outcome, clipText(prior.Summary, MaxSummaryRunes))
			for _, name := range sortedNames(prior.Outputs) {
				fmt.Fprintf(&variable, "  - %s: %s\n", name, clipText(prior.Outputs[name], MaxValueRunes))
			}
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

func sortedNames(values map[string]string) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func clipText(value string, max int) string {
	if utf8.RuneCountInString(value) <= max {
		return value
	}
	runes := []rune(value)
	return string(runes[:max])
}
