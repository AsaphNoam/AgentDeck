package pipeline

import (
	"context"
	"fmt"

	"github.com/AsaphNoam/Chuck/internal/state"
)

type RuntimeAssignment struct {
	Backend string `json:"backend"`
	Model   string `json:"model"`
	Effort  string `json:"effort,omitempty"`
	Fast    bool   `json:"fast,omitempty"`
}

type StartRequest struct {
	RequestID            string                       `json:"request_id"`
	TemplateID           string                       `json:"template_id"`
	DisplayName          string                       `json:"display_name"`
	Project              string                       `json:"project"`
	Goal                 string                       `json:"goal"`
	Inputs               map[string]string            `json:"inputs"`
	Assignments          map[string]RuntimeAssignment `json:"assignments,omitempty"`
	Orchestrator         RuntimeAssignment            `json:"orchestrator"`
	DedicatedAssignments map[string]RuntimeAssignment `json:"dedicated_assignments"`
	// ThinkTankAssignments supplies every room slot's runtime, keyed by stage id
	// (TS-09.R51).
	ThinkTankAssignments map[string]ThinkTankAssignment `json:"think_tank_assignments,omitempty"`
}

// ThinkTankAssignment is one think_tank stage's run-time slots: participants
// keyed by template participant id, plus the judge.
type ThinkTankAssignment struct {
	Participants map[string]RuntimeAssignment `json:"participants"`
	Judge        RuntimeAssignment            `json:"judge"`
}

// Think Tank slots share the run's one frozen assignment map under keys that
// cannot collide with a stage-id slug or the standing key.
func thinkTankParticipantKey(stageID, participantID string) string {
	return "think_tank:" + stageID + ":participant:" + participantID
}

func thinkTankJudgeKey(stageID string) string { return "think_tank:" + stageID + ":judge" }

// ThinkTankAssignmentsFrom rebuilds the public per-stage shape from a frozen
// assignment map.
func ThinkTankAssignmentsFrom(template Template, assignments map[string]RuntimeAssignment) map[string]ThinkTankAssignment {
	out := map[string]ThinkTankAssignment{}
	for _, stage := range template.Stages {
		if StageCoordination(stage) != CoordinationThinkTank || stage.ThinkTank == nil {
			continue
		}
		slot := ThinkTankAssignment{Participants: map[string]RuntimeAssignment{}, Judge: assignments[thinkTankJudgeKey(stage.ID)]}
		for _, p := range stage.ThinkTank.Participants {
			slot.Participants[p.ID] = assignments[thinkTankParticipantKey(stage.ID, p.ID)]
		}
		out[stage.ID] = slot
	}
	return out
}

type StageExecution struct {
	RunID      string
	RunName    string
	AttemptID  string
	StageID    string
	StageTitle string
	Role       string
	Project    string
	Backend    string
	Model      string
	Effort     string
	Fast       bool
	AgentID    string
	Generation string
	AgentName  string
	Assignment string
}

// Lifecycle is the server-owned seam shared by manual and pipeline controls.
// INV §6 contract checklist for the chat agents it creates:
//   - persistence: the ordinary launch/resume service writes agent/session rows;
//   - LaunchSpec: the shared composers retain model/prompt/add_dirs/MCP fields;
//   - fan-out/drain: the existing chat runtime hub owns output;
//   - messaging: the ordinary scoped MCP registration is installed;
//   - turn boundaries: normalized turn_end is fanned back to Manager;
//   - reconcile/hooks/status: existing chat paths remain authoritative;
//   - capabilities: ValidateStage rejects non-chat/unknown selections;
//   - teardown: StopStage uses generation-scoped ordinary cleanup.
type Lifecycle interface {
	AcquirePipelineStart(context.Context, string) (func(), error)
	ValidateStage(context.Context, StageExecution) error
	// RoomLaunchConfig composes a fresh Think Tank slot's opaque launch
	// request (Role/Project/runtime/AgentName) without launching anything.
	RoomLaunchConfig(context.Context, StageExecution) (string, error)
	// StopRoom closes a stopped run's stage room and cancels only that room's
	// in-flight turn (TS-09.R54).
	StopRoom(context.Context, string) error
	LaunchStage(context.Context, StageExecution) error
	ContinueStage(context.Context, StageExecution) error
	StopStage(context.Context, string) error
	IsRunning(string) bool
}

// ProjectGateError preserves the lifecycle gate's public conflict vocabulary
// without teaching the durable pipeline package about HTTP or server internals.
type ProjectGateError struct {
	Code    string
	Message string
}

func (e *ProjectGateError) Error() string { return e.Code + ": " + e.Message }

type Publisher interface {
	PublishPipelineUpdate(PipelineUpdate)
	PublishPipelineNotification(PipelineUpdate, string)
	PublishPipelineProposalUpdate()
}

type PipelineUpdate struct {
	RunID           string `json:"run_id"`
	DisplayName     string `json:"display_name"`
	Revision        int64  `json:"revision"`
	State           string `json:"state"`
	CurrentStageID  string `json:"current_stage_id"`
	CurrentAgentID  string `json:"current_agent_id"`
	AttentionReason string `json:"attention_reason"`
	FinalOutcome    string `json:"final_outcome"`
}

type RunDetail struct {
	Run         state.PipelineRunRecord      `json:"run"`
	Template    Template                     `json:"template"`
	Inputs      map[string]string            `json:"inputs"`
	Assignments map[string]RuntimeAssignment `json:"assignments"`
	// ThinkTankAssignments is the frozen per-stage room slot view.
	ThinkTankAssignments map[string]ThinkTankAssignment `json:"think_tank_assignments"`
	// Attempts is retained only while decoding/resetting historical v1 state.
	// Live API responses project stage tasks instead.
	Attempts    []state.PipelineAttemptRecord `json:"-"`
	Values      []state.PipelineValueRecord   `json:"values"`
	Diagnostics []Diagnostic                  `json:"diagnostics"`
}

// RunSummary keeps list responses readable even when one run's nested JSON or
// attempt history is malformed. Detail reads remain strict so controls cannot
// operate on a partially decoded state machine.
type RunSummary struct {
	RunID             string `json:"run_id"`
	TemplateID        string `json:"template_id"`
	DisplayName       string `json:"display_name"`
	Project           string `json:"project"`
	State             string `json:"state"`
	Revision          int64  `json:"revision"`
	PendingAction     string `json:"pending_action"`
	CurrentStageID    string `json:"current_stage_id"`
	CurrentStageTitle string `json:"current_stage_title"`
	// StageNumber (1-based) and StageCount give the phone's "Stage 2 of 4"
	// (FS-20.R11); zero when the frozen snapshot cannot say.
	StageNumber     int          `json:"stage_number"`
	StageCount      int          `json:"stage_count"`
	CurrentAgentID  string       `json:"current_agent_id"`
	AttentionReason string       `json:"attention_reason"`
	FinalOutcome    string       `json:"final_outcome"`
	UpdatedAt       string       `json:"updated_at"`
	Diagnostics     []Diagnostic `json:"diagnostics"`
}

type ControlError struct {
	Code        string       `json:"code"`
	Message     string       `json:"message"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
}

func (e *ControlError) Error() string { return fmt.Sprintf("pipeline %s: %s", e.Code, e.Message) }

func controlError(code, message string) *ControlError {
	return &ControlError{Code: code, Message: message, Diagnostics: []Diagnostic{}}
}

func validationError(message string, diagnostics []Diagnostic) *ControlError {
	if diagnostics == nil {
		diagnostics = []Diagnostic{}
	}
	return &ControlError{Code: "validation_failed", Message: message, Diagnostics: diagnostics}
}
