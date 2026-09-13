package messaging

import (
	"context"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/agentdeck/agentdeck/internal/pipeline"
)

type proposeTemplateArgs struct {
	ID       string            `json:"id" jsonschema:"lowercase immutable template id"`
	Template pipeline.Template `json:"template"`
}

func (s *Server) handleProposePipelineTemplate(_ context.Context, req *mcp.CallToolRequest, input proposeTemplateArgs) (*mcp.CallToolResult, any, error) {
	identity, ok := s.caller(req)
	if !ok {
		return sessionUnknown()
	}
	if !s.isAgentDecker(identity.AgentID) {
		return errResult(map[string]any{"ok": false, "error": "proposal_forbidden", "message": "Only a token-bound AgentDecker role session may propose pipelines."})
	}
	s.mu.RLock()
	manager := s.pipelines
	s.mu.RUnlock()
	if manager == nil {
		return errResult(map[string]any{"ok": false, "error": "pipeline_unavailable", "message": "Pipeline control plane is unavailable."})
	}
	proposal, err := manager.ProposeTemplate(input.ID, input.Template)
	if err != nil {
		return pipelineToolError(err)
	}
	return jsonResult(map[string]any{"ok": true, "proposal": proposal})
}

type proposeRunArgs struct {
	Run pipeline.StartRequest `json:"run"`
}

func (s *Server) handleProposePipelineRun(ctx context.Context, req *mcp.CallToolRequest, input proposeRunArgs) (*mcp.CallToolResult, any, error) {
	identity, ok := s.caller(req)
	if !ok {
		return sessionUnknown()
	}
	if !s.isAgentDecker(identity.AgentID) {
		return errResult(map[string]any{"ok": false, "error": "proposal_forbidden", "message": "Only a token-bound AgentDecker role session may propose pipelines."})
	}
	s.mu.RLock()
	manager := s.pipelines
	s.mu.RUnlock()
	if manager == nil {
		return errResult(map[string]any{"ok": false, "error": "pipeline_unavailable", "message": "Pipeline control plane is unavailable."})
	}
	proposal, err := manager.ProposeRun(ctx, input.Run)
	if err != nil {
		return pipelineToolError(err)
	}
	return jsonResult(map[string]any{"ok": true, "proposal": proposal})
}

func (s *Server) isAgentDecker(agentID string) bool {
	agent, err := s.store.ReadAgent(agentID)
	return err == nil && agent.Role == "agentdecker" && agent.Interface == "chat"
}

func pipelineToolError(err error) (*mcp.CallToolResult, any, error) {
	var controlled *pipeline.ControlError
	if errors.As(err, &controlled) {
		return errResult(map[string]any{"ok": false, "error": controlled.Code, "message": controlled.Message, "diagnostics": controlled.Diagnostics})
	}
	return errResult(map[string]any{"ok": false, "error": "pipeline_unavailable", "message": "Pipeline operation could not be completed."})
}
