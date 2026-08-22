package messaging

import (
	"context"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/agentdeck/agentdeck/internal/contextref"
)

// SetContextService supplies the in-process context-plane service. Without it
// the five context tools report that the surface is unavailable rather than
// falling back to any looser authority.
func (s *Server) SetContextService(svc *contextref.Service) {
	s.mu.Lock()
	s.context = svc
	s.mu.Unlock()
}

func (s *Server) contextService() *contextref.Service {
	s.mu.RLock()
	svc := s.context
	s.mu.RUnlock()
	return svc
}

// contextCaller resolves the token-bound identity every context operation is
// scoped to. Identity is never taken from a tool argument (TS-05.R16).
func (s *Server) contextCaller(req *mcp.CallToolRequest) (contextref.Caller, *contextref.Service, bool, *mcp.CallToolResult, any, error) {
	identity, ok := s.caller(req)
	if !ok {
		result, extra, err := sessionUnknown()
		return contextref.Caller{}, nil, false, result, extra, err
	}
	svc := s.contextService()
	if svc == nil {
		result, extra, err := errResult(map[string]any{"ok": false, "error": "context_unavailable",
			"message": "Context links are unavailable."})
		return contextref.Caller{}, nil, false, result, extra, err
	}
	return contextref.Caller{AgentID: identity.AgentID, Generation: identity.Generation}, svc, true, nil, nil, nil
}

// contextToolError renders a typed context outcome as bounded structured JSON.
// Messages carry no source bytes (TS-05.R16).
func contextToolError(err error) (*mcp.CallToolResult, any, error) {
	var typed *contextref.Error
	if errors.As(err, &typed) {
		out := map[string]any{"ok": false, "error": typed.Code, "message": typed.Message}
		if typed.Candidates != nil {
			out["candidates"] = typed.Candidates
		}
		return errResult(out)
	}
	return errResult(map[string]any{"ok": false, "error": "context_unavailable",
		"message": "Context operation could not be completed."})
}

// --- share_context (TS-04.R28) ---

type shareContextArgs struct {
	To          string `json:"to" jsonschema:"recipient chat agent: role@project, agent name, or agent_id"`
	Source      string `json:"source" jsonschema:"current_turn, latest_completed_turn, or current_pipeline_report"`
	Label       string `json:"label,omitempty" jsonschema:"optional label for this share, <=200 characters"`
	Description string `json:"description,omitempty" jsonschema:"optional description for this share, <=1000 characters"`
}

func (s *Server) handleShareContext(_ context.Context, req *mcp.CallToolRequest, in shareContextArgs) (*mcp.CallToolResult, any, error) {
	caller, svc, ok, result, extra, err := s.contextCaller(req)
	if !ok {
		return result, extra, err
	}
	res, shareErr := svc.Share(caller, in.Source, in.To, in.Label, in.Description)
	if shareErr != nil {
		return contextToolError(shareErr)
	}
	return jsonResult(map[string]any{
		"ok":             true,
		"context_ref_id": res.ContextRefID,
		"grant_id":       res.GrantID,
		"to":             res.To,
		"to_address":     res.ToAddress,
		"source":         res.Source,
	})
}

// --- list_context_links (TS-04.R28) ---

type listContextLinksArgs struct {
	IncludeHidden bool   `json:"include_hidden,omitempty" jsonschema:"include entries you have hidden (default false)"`
	Limit         *int   `json:"limit,omitempty" jsonschema:"max entries, 1..50 (default 20)"`
	Cursor        string `json:"cursor,omitempty" jsonschema:"continuation cursor from a previous call"`
}

func (s *Server) handleListContextLinks(_ context.Context, req *mcp.CallToolRequest, in listContextLinksArgs) (*mcp.CallToolResult, any, error) {
	caller, svc, ok, result, extra, err := s.contextCaller(req)
	if !ok {
		return result, extra, err
	}
	res, listErr := svc.List(caller, in.IncludeHidden, intOr(in.Limit, 0), in.Cursor)
	if listErr != nil {
		return contextToolError(listErr)
	}
	out := map[string]any{"ok": true, "links": res.Links}
	if res.NextCursor != "" {
		out["next_cursor"] = res.NextCursor
	}
	return jsonResult(out)
}

// --- read_context_link (TS-04.R28) ---

type readContextLinkArgs struct {
	ContextRefID string `json:"context_ref_id" jsonschema:"the reference id to read"`
	Cursor       string `json:"cursor,omitempty" jsonschema:"continuation cursor from a previous page"`
}

func (s *Server) handleReadContextLink(_ context.Context, req *mcp.CallToolRequest, in readContextLinkArgs) (*mcp.CallToolResult, any, error) {
	caller, svc, ok, result, extra, err := s.contextCaller(req)
	if !ok {
		return result, extra, err
	}
	res, readErr := svc.Read(caller, in.ContextRefID, in.Cursor)
	if readErr != nil {
		return contextToolError(readErr)
	}
	out := map[string]any{
		"ok":             true,
		"context_ref_id": res.ContextRefID,
		"source":         res.Source,
		"text":           res.Text,
		"complete":       res.Complete,
	}
	if res.NextCursor != "" {
		out["next_cursor"] = res.NextCursor
	}
	return jsonResult(out)
}

// --- set_context_link_visibility (TS-04.R28) ---

type setContextVisibilityArgs struct {
	GrantID string `json:"grant_id" jsonschema:"the direct-share id to hide or unhide"`
	Hidden  bool   `json:"hidden" jsonschema:"true hides the entry from your normal list"`
}

func (s *Server) handleSetContextLinkVisibility(_ context.Context, req *mcp.CallToolRequest, in setContextVisibilityArgs) (*mcp.CallToolResult, any, error) {
	caller, svc, ok, result, extra, err := s.contextCaller(req)
	if !ok {
		return result, extra, err
	}
	if setErr := svc.SetHidden(caller, in.GrantID, in.Hidden); setErr != nil {
		return contextToolError(setErr)
	}
	return jsonResult(map[string]any{"ok": true, "grant_id": in.GrantID, "hidden": in.Hidden})
}

// --- revoke_context_grant (TS-04.R28) ---

type revokeContextGrantArgs struct {
	GrantID string `json:"grant_id" jsonschema:"the direct-share id you granted and want to withdraw"`
}

func (s *Server) handleRevokeContextGrant(_ context.Context, req *mcp.CallToolRequest, in revokeContextGrantArgs) (*mcp.CallToolResult, any, error) {
	caller, svc, ok, result, extra, err := s.contextCaller(req)
	if !ok {
		return result, extra, err
	}
	if revokeErr := svc.Revoke(caller, in.GrantID); revokeErr != nil {
		return contextToolError(revokeErr)
	}
	return jsonResult(map[string]any{"ok": true, "grant_id": in.GrantID, "revoked": true})
}
