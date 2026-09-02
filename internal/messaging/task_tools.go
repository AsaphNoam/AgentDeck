package messaging

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/agentdeck/agentdeck/internal/state"
)

// --- get_assigned_task (FS-16.R11, TS-10.R13) ---

// outAttachment is one attached context reference as the assignee sees it: the
// canonical id it can read plus this task's own presentation for it. It carries
// no content and no grant (FS-16.R10).
type outAttachment struct {
	ContextRefID string `json:"context_ref_id"`
	Label        string `json:"label,omitempty"`
	Description  string `json:"description,omitempty"`
}

// handleGetAssignedTask answers the caller's own assignment. The caller is the
// session token's agent, never a tool argument, and the query is by assignment,
// so there is no task id to name and no other agent's work to reach (TS-05.R14).
func (s *Server) handleGetAssignedTask(_ context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	identity, ok := s.caller(req)
	if !ok {
		return sessionUnknown()
	}
	task, err := s.store.AssignedTask(identity.AgentID)
	if errors.Is(err, state.ErrNotFound) {
		// Not an error: an agent may be asked and honestly have nothing assigned.
		return jsonResult(map[string]any{"ok": true, "assigned": false})
	}
	if err != nil {
		s.log.Debug("read assigned task failed", "agent", identity.AgentID, "err", err)
		return errResult(map[string]any{
			"ok": false, "error": "internal", "message": "Could not read your assignment.",
		})
	}
	attachments, err := s.store.ListTaskAttachments(task.TaskID)
	if err != nil {
		s.log.Debug("list task attachments failed", "task", task.TaskID, "err", err)
		return errResult(map[string]any{
			"ok": false, "error": "internal", "message": "Could not read your assignment.",
		})
	}
	out := make([]outAttachment, 0, len(attachments))
	for _, a := range attachments {
		out = append(out, outAttachment{
			ContextRefID: a.ContextRefID, Label: a.Label, Description: a.Description,
		})
	}
	return jsonResult(map[string]any{
		"ok":           true,
		"assigned":     true,
		"task_id":      task.TaskID,
		"display_name": task.DisplayName,
		"instruction":  task.Instruction,
		"state":        task.State,
		"attachments":  out,
	})
}

// --- report_task_result (FS-16.R3, R20, TS-04.R29) ---

type reportTaskArgs struct {
	Outcome string `json:"outcome" jsonschema:"success, failure, or blocked"`
	Summary string `json:"summary" jsonschema:"bounded human-readable result summary"`
	Details string `json:"details,omitempty" jsonschema:"optional bounded result details"`
}

// handleReportTaskResult records the caller's own result for its own assignment.
// There is no task id argument and no reporter argument: both are the session
// token's, so no caller can report for work it does not hold (TS-05.R14).
func (s *Server) handleReportTaskResult(_ context.Context, req *mcp.CallToolRequest, input reportTaskArgs) (*mcp.CallToolResult, any, error) {
	identity, ok := s.caller(req)
	if !ok {
		return sessionUnknown()
	}
	task, err := s.store.AssignedTask(identity.AgentID)
	if errors.Is(err, state.ErrNotFound) {
		return errResult(map[string]any{
			"ok": false, "error": "not_assigned", "message": "You have no assigned task to report on.",
		})
	}
	if err != nil {
		s.log.Debug("read assigned task failed", "agent", identity.AgentID, "err", err)
		return errResult(map[string]any{"ok": false, "error": "internal", "message": "Could not read your assignment."})
	}
	finished, err := s.store.RecordAgentTaskResult(task.TaskID, identity.AgentID, identity.Generation,
		state.TaskResult{Outcome: input.Outcome, Summary: input.Summary, Details: input.Details})
	switch {
	case errors.Is(err, state.ErrInvalidOutcome):
		return errResult(map[string]any{
			"ok": false, "error": "invalid_outcome",
			"message": "outcome must be success, failure, or blocked.",
		})
	case errors.Is(err, state.ErrInvalidReportFields):
		return errResult(map[string]any{
			"ok": false, "error": "validation",
			"message": "A summary is required and report fields must fit their documented limits.",
		})
	case errors.Is(err, state.ErrTaskNotAssigned):
		return errResult(map[string]any{
			"ok": false, "error": "not_assigned",
			"message": "This assignment belongs to another agent or to an earlier session.",
		})
	case errors.Is(err, state.ErrWorkResultRecorded), errors.Is(err, state.ErrTaskNotReportable):
		return errResult(map[string]any{
			"ok": false, "error": "already_reported",
			"message": "This task already has a recorded result.",
		})
	case errors.Is(err, state.ErrNotFound):
		return errResult(map[string]any{
			"ok": false, "error": "task_not_found", "message": "No such task.",
		})
	case err != nil:
		s.log.Debug("record task result failed", "task", task.TaskID, "err", err)
		return errResult(map[string]any{"ok": false, "error": "internal", "message": "Could not record your result."})
	}
	// The runtime claim is released and the agent stopped at this turn's end, not
	// here: stopping now would cut off the response this call is returning
	// (TS-10.R19).
	s.taskResultRecorded(finished.TaskID)
	return jsonResult(map[string]any{
		"ok": true, "task_id": finished.TaskID, "outcome": finished.Outcome, "state": finished.State,
	})
}

// --- create_task and cancel_task (FS-16.R12, R24, TS-04.R29) ---

// ToolError is a refusal carrying one of the stable outcome codes the task tools
// return. The control plane raises it so the code is decided by whoever knows
// why the refusal happened, not re-derived here from a message.
type ToolError struct {
	Code    string
	Message string
}

func (e *ToolError) Error() string { return e.Message }

// AgentTaskRequest is one agent-created task after identity and target
// resolution. Every field the caller may not decide — the creator, its
// generation, the project — is filled here from the session, never from an
// argument (TS-05.R17, FS-16.R24).
type AgentTaskRequest struct {
	CreatorAgentID    string
	CreatorGeneration string
	Project           string
	DisplayName       string
	Instruction       string
	// TargetAgentID is the resolved existing agent, or empty for a launch target.
	TargetAgentID string
	Role          string
	Backend       string
	Model         string
	Arms          []state.TaskArm
	Attachments   []state.TaskAttachment
}

// TaskControl is the control plane behind the agent-facing task tools. This
// package owns identity, resolution, and tool shape; validating a target and
// committing the record stay with the plane that owns them.
type TaskControl interface {
	CreateAgentTask(req AgentTaskRequest) (state.Task, error)
	CancelAgentTask(taskID, creatorAgentID string) (state.Task, error)
}

// SetTaskControl wires the task tools to the control plane.
func (s *Server) SetTaskControl(control TaskControl) {
	s.mu.Lock()
	s.tasks = control
	s.mu.Unlock()
}

func (s *Server) taskControl() TaskControl {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tasks
}

type createTaskArgs struct {
	DisplayName string `json:"display_name" jsonschema:"short name for the work"`
	Instruction string `json:"instruction" jsonschema:"what the assigned agent should do"`
	// To is the same friendly recipient selector a message uses: role@project, an
	// agent name, or an agent_id. Omit it to have AgentDeck launch a new agent.
	To      string           `json:"to,omitempty" jsonschema:"existing agent to assign: role@project, name, or agent_id; omit to launch a new agent"`
	Role    string           `json:"role,omitempty" jsonschema:"role for a new agent when no target is named"`
	Backend string           `json:"backend,omitempty" jsonschema:"optional backend for a new agent"`
	Model   string           `json:"model,omitempty" jsonschema:"optional model for a new agent"`
	Arms    []createArmInput `json:"arms,omitempty" jsonschema:"prerequisites that must all be satisfied before this starts"`
	// Attachments are context_ref_ids you can already read; the assignee reads
	// them through its own assignment, not through a share to it.
	Attachments []createAttachmentInput `json:"attachments,omitempty" jsonschema:"context references to attach for the assignee"`
}

type createAttachmentInput struct {
	ContextRefID string `json:"context_ref_id" jsonschema:"a context_ref_id you can read"`
	Label        string `json:"label,omitempty" jsonschema:"short label for the assignee"`
	Description  string `json:"description,omitempty" jsonschema:"why this context matters to the work"`
}

type createArmInput struct {
	Kind               string   `json:"kind" jsonschema:"work_result or signal"`
	SourceKind         string   `json:"source_kind,omitempty" jsonschema:"task or pipeline_run"`
	SourceID           string   `json:"source_id,omitempty" jsonschema:"the prerequisite task or pipeline run id"`
	SatisfyingOutcomes []string `json:"satisfying_outcomes,omitempty" jsonschema:"outcomes of the prerequisite that satisfy this arm"`
	SignalName         string   `json:"signal_name,omitempty" jsonschema:"project-scoped signal name to wait for"`
}

// handleCreateTask lets an agent cause work to start without a person in the
// loop, which is the point of expressing orchestration as control state rather
// than prose. The creator, its generation, and the project come from the session
// token; the target is a friendly selector resolved server-side against durable
// identities, exactly as a message recipient is (FS-16.R12, R24).
func (s *Server) handleCreateTask(_ context.Context, req *mcp.CallToolRequest, input createTaskArgs) (*mcp.CallToolResult, any, error) {
	identity, ok := s.caller(req)
	if !ok {
		return sessionUnknown()
	}
	control := s.taskControl()
	if control == nil {
		return errResult(map[string]any{"ok": false, "error": "internal", "message": "Task control plane is unavailable."})
	}
	creator, err := s.store.ReadAgent(identity.AgentID)
	if err != nil {
		return storeUnavailable(err)
	}
	request := AgentTaskRequest{
		CreatorAgentID: creator.AgentID, CreatorGeneration: identity.Generation,
		Project: creator.Project, DisplayName: input.DisplayName, Instruction: input.Instruction,
		Role: input.Role, Backend: input.Backend, Model: input.Model,
	}
	if strings.TrimSpace(input.To) != "" {
		addressable, err := s.addressableAgents()
		if err != nil {
			return storeUnavailable(err)
		}
		toID, candidates, err := state.ResolveRecipient(addressable, input.To)
		if err != nil {
			var ambiguous *state.AmbiguousError
			switch {
			case errors.As(err, &ambiguous):
				return errResult(map[string]any{"ok": false, "error": "ambiguous_recipient",
					"message":    fmt.Sprintf("Multiple agents match %q; name an agent_id.", input.To),
					"candidates": ambiguous.Candidates})
			case errors.Is(err, state.ErrRecipientNotFound):
				if message, diagnosed := s.pipelineRecipientRefusal(input.To); diagnosed {
					return errResult(map[string]any{"ok": false, "error": "recipient_not_found", "message": message, "candidates": candidates})
				}
				return errResult(map[string]any{"ok": false, "error": "recipient_not_found",
					"message":    fmt.Sprintf("No agent matches %q.", input.To),
					"candidates": candidates})
			default:
				return storeUnavailable(err)
			}
		}
		request.TargetAgentID = toID
	}
	for _, arm := range input.Arms {
		request.Arms = append(request.Arms, state.TaskArm{
			Kind: arm.Kind, SourceKind: arm.SourceKind, SourceID: arm.SourceID,
			SatisfyingOutcomes: arm.SatisfyingOutcomes, SignalName: arm.SignalName,
		})
	}
	for _, attachment := range input.Attachments {
		request.Attachments = append(request.Attachments, state.TaskAttachment{
			ContextRefID: attachment.ContextRefID, Label: attachment.Label,
			Description: attachment.Description,
		})
	}
	task, err := control.CreateAgentTask(request)
	if err != nil {
		return taskToolError(err)
	}
	return jsonResult(map[string]any{
		"ok": true, "task_id": task.TaskID, "state": task.State,
	})
}

type cancelTaskArgs struct {
	TaskID string `json:"task_id" jsonschema:"the id of a task you created"`
}

// handleCancelTask cancels a task the caller created. Authority is the durably
// recorded creator id, so a stopped-and-resumed agent keeps it and a new
// generation is not a new principal; a task a person created can never be
// cancelled here (FS-16.R24, TS-10.R20).
func (s *Server) handleCancelTask(_ context.Context, req *mcp.CallToolRequest, input cancelTaskArgs) (*mcp.CallToolResult, any, error) {
	identity, ok := s.caller(req)
	if !ok {
		return sessionUnknown()
	}
	control := s.taskControl()
	if control == nil {
		return errResult(map[string]any{"ok": false, "error": "internal", "message": "Task control plane is unavailable."})
	}
	task, err := control.CancelAgentTask(input.TaskID, identity.AgentID)
	if err != nil {
		return taskToolError(err)
	}
	return jsonResult(map[string]any{
		"ok": true, "task_id": task.TaskID, "state": task.State, "outcome": task.Outcome,
	})
}

// taskToolError maps the control plane's refusals onto stable outcome codes. An
// unauthorized task and an unknown one are deliberately indistinguishable, so a
// caller cannot probe for tasks it does not own (TS-05.R14).
func taskToolError(err error) (*mcp.CallToolResult, any, error) {
	var toolErr *ToolError
	if errors.As(err, &toolErr) {
		return errResult(map[string]any{"ok": false, "error": toolErr.Code, "message": toolErr.Message})
	}
	switch {
	case errors.Is(err, state.ErrTaskCycle):
		return errResult(map[string]any{"ok": false, "error": "dependency_cycle",
			"message": "These prerequisites would make the task graph cyclic."})
	case errors.Is(err, state.ErrTaskArmSource):
		return errResult(map[string]any{"ok": false, "error": "validation", "message": err.Error()})
	case errors.Is(err, state.ErrTaskNotAssigned):
		return errResult(map[string]any{"ok": false, "error": "validation", "message": "You cannot read that context reference."})
	case errors.Is(err, state.ErrNotFound):
		return errResult(map[string]any{"ok": false, "error": "task_not_found", "message": "No such task."})
	case errors.Is(err, state.ErrTaskNotReportable):
		return errResult(map[string]any{"ok": false, "error": "invalid_state",
			"message": "This task cannot be cancelled in its current state."})
	}
	return errResult(map[string]any{"ok": false, "error": "internal", "message": "The task could not be recorded."})
}
