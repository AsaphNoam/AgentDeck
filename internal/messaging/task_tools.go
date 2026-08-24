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
