package messaging

import (
	"context"
	"errors"

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
