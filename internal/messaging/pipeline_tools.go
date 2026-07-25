package messaging

import (
	"context"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/agentdeck/agentdeck/internal/pipeline"
)

type reportPipelineArgs struct {
	Outcome string            `json:"outcome" jsonschema:"success, failure, or blocked"`
	Summary string            `json:"summary" jsonschema:"bounded human-readable result summary"`
	Details string            `json:"details,omitempty" jsonschema:"optional bounded result details"`
	Checks  string            `json:"checks,omitempty" jsonschema:"optional bounded checks performed"`
	Outputs map[string]string `json:"outputs,omitempty" jsonschema:"declared local output names to opaque text values"`
}

func (s *Server) handleReportPipelineStageResult(_ context.Context, req *mcp.CallToolRequest, input reportPipelineArgs) (*mcp.CallToolResult, any, error) {
	identity, ok := s.caller(req)
	if !ok {
		return sessionUnknown()
	}
	s.mu.RLock()
	manager := s.pipelines
	s.mu.RUnlock()
	if manager == nil {
		return errResult(map[string]any{"ok": false, "error": "pipeline_unavailable", "message": "Pipeline control plane is unavailable."})
	}
	detail, err := manager.Report(identity.AgentID, identity.Generation, pipeline.StageReport{
		Outcome: input.Outcome, Summary: input.Summary, Details: input.Details, Checks: input.Checks, Outputs: input.Outputs,
	})
	if err != nil {
		return pipelineToolError(err)
	}
	return jsonResult(map[string]any{"ok": true, "run_id": detail.Run.RunID, "attempt_id": detail.Run.CurrentAttemptID, "revision": detail.Run.Revision, "awaiting": "quiescence"})
}

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
