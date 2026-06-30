package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/agentdeck/agentdeck/internal/state"
)

// caller resolves the calling agent_id from the per-request session token
// (techspec §3.1). Identity is bound to the registered session, never to a tool
// argument. Returns ok=false when the token is absent or unknown/revoked.
func (s *Server) caller(req *mcp.CallToolRequest) (string, bool) {
	if req == nil || req.Extra == nil || req.Extra.Header == nil {
		return "", false
	}
	return s.Lookup(req.Extra.Header.Get(TokenHeader))
}

// jsonResult marshals v into a single text-content tool result.
func jsonResult(v any) (*mcp.CallToolResult, any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}, nil, nil
}

// errResult marshals v into a tool result flagged IsError.
func errResult(v any) (*mcp.CallToolResult, any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}, nil, nil
}

// sessionUnknown is the structured error for a call on an unknown/revoked
// session token (techspec §3.1, §9).
func sessionUnknown() (*mcp.CallToolResult, any, error) {
	return errResult(map[string]any{
		"ok":      false,
		"error":   "session_unknown",
		"message": "Calling MCP session is not registered (agent stopped or token revoked).",
	})
}

// --- list_agents (techspec §3.3) ---

type listAgentsArgs struct {
	IncludeSelf bool   `json:"include_self,omitempty" jsonschema:"include the caller in results"`
	State       string `json:"state,omitempty" jsonschema:"filter by latest status state"`
}

func (s *Server) handleListAgents(_ context.Context, req *mcp.CallToolRequest, in listAgentsArgs) (*mcp.CallToolResult, any, error) {
	self, ok := s.caller(req)
	if !ok {
		return sessionUnknown()
	}
	live, err := s.store.LiveAgents()
	if err != nil {
		return storeUnavailable(err)
	}
	agents := make([]state.LiveAgent, 0, len(live))
	for _, a := range live {
		if !in.IncludeSelf && a.AgentID == self {
			continue
		}
		if in.State != "" && a.State != in.State {
			continue
		}
		agents = append(agents, a)
	}
	return jsonResult(map[string]any{"agents": agents})
}

// --- send_message (techspec §3.4) ---

type sendMessageArgs struct {
	To        string `json:"to" jsonschema:"recipient: role@project, agent name, or agent_id"`
	Body      string `json:"body" jsonschema:"message body, 1..8000 chars"`
	Subject   string `json:"subject,omitempty" jsonschema:"optional subject, <=200 chars"`
	InReplyTo string `json:"in_reply_to,omitempty" jsonschema:"optional message_id being replied to"`
}

func (s *Server) handleSendMessage(_ context.Context, req *mcp.CallToolRequest, in sendMessageArgs) (*mcp.CallToolResult, any, error) {
	self, ok := s.caller(req)
	if !ok {
		return sessionUnknown()
	}
	if l := len(in.Body); l < 1 || l > maxBodyLen {
		return errResult(map[string]any{"ok": false, "error": "invalid_body",
			"message": fmt.Sprintf("body must be 1..%d chars (got %d).", maxBodyLen, l)})
	}
	if len(in.Subject) > maxSubjectLen {
		return errResult(map[string]any{"ok": false, "error": "invalid_subject",
			"message": fmt.Sprintf("subject must be <=%d chars.", maxSubjectLen)})
	}

	toID, candidates, err := s.store.ResolveRecipient(in.To)
	if err != nil {
		var amb *state.AmbiguousError
		switch {
		case errors.As(err, &amb):
			return errResult(map[string]any{"ok": false, "error": "ambiguous_recipient",
				"message":    fmt.Sprintf("Multiple live agents match %q; address by agent_id.", in.To),
				"candidates": amb.Candidates})
		case errors.Is(err, state.ErrRecipientNotFound):
			return errResult(map[string]any{"ok": false, "error": "recipient_not_found",
				"message":    fmt.Sprintf("No live agent matches %q.", in.To),
				"candidates": candidates})
		default:
			return storeUnavailable(err)
		}
	}

	sender, err := s.store.ReadAgent(self)
	if err != nil {
		return storeUnavailable(err)
	}
	recipient, err := s.store.ReadAgent(toID)
	if err != nil {
		return storeUnavailable(err)
	}

	msgID, budget, breached, err := s.store.InsertMessageWithBudget(state.Message{
		FromAgent:   self,
		FromAddress: sender.Role + "@" + sender.Project,
		FromName:    sender.Name,
		ToAgent:     toID,
		Subject:     in.Subject,
		Body:        in.Body,
		InReplyTo:   in.InReplyTo,
	}, MessageBudgetPerTurn)
	if err != nil {
		return storeUnavailable(err)
	}
	if breached {
		s.budgetExceeded(self, budget.TurnID, MessageBudgetPerTurn-budget.Remaining)
		return errResult(map[string]any{
			"ok":      false,
			"error":   "message_budget_exceeded",
			"message": fmt.Sprintf("Per-turn message budget (%d) reached. This message was not sent.", MessageBudgetPerTurn),
			"budget":  MessageBudgetPerTurn,
			"used":    MessageBudgetPerTurn,
		})
	}
	s.messageInserted(self, toID)
	return jsonResult(map[string]any{
		"ok":         true,
		"message_id": msgID,
		"to":         toID,
		"to_address": recipient.Role + "@" + recipient.Project,
	})
}

// --- check_messages (techspec §3.5) ---

type checkMessagesArgs struct {
	MarkRead    *bool `json:"mark_read,omitempty" jsonschema:"flag returned messages read (default true)"`
	DeleteAfter *bool `json:"delete_after,omitempty" jsonschema:"delete returned messages after reading (default false)"`
	UnreadOnly  *bool `json:"unread_only,omitempty" jsonschema:"only unread messages (default true)"`
	Limit       *int  `json:"limit,omitempty" jsonschema:"max messages, 1..50 (default 15)"`
}

// outMessage is the per-message shape check_messages returns (techspec §3.5).
type outMessage struct {
	MessageID   string `json:"message_id"`
	From        string `json:"from"`
	FromAddress string `json:"from_address"`
	FromName    string `json:"from_name"`
	Subject     string `json:"subject"`
	Body        string `json:"body"`
	CreatedAt   string `json:"created_at"`
	InReplyTo   string `json:"in_reply_to,omitempty"`
}

func (s *Server) handleCheckMessages(_ context.Context, req *mcp.CallToolRequest, in checkMessagesArgs) (*mcp.CallToolResult, any, error) {
	self, ok := s.caller(req)
	if !ok {
		return sessionUnknown()
	}
	markRead := boolOr(in.MarkRead, true)
	deleteAfter := boolOr(in.DeleteAfter, false)
	unreadOnly := boolOr(in.UnreadOnly, true)
	limit := intOr(in.Limit, defaultCheckLimit)
	if limit < 1 {
		limit = 1
	}
	if limit > maxCheckLimit {
		limit = maxCheckLimit
	}
	msgs, budget, breached, err := s.store.TakeMessagesWithBudget(self, unreadOnly, limit, MessageBudgetPerTurn, markRead, deleteAfter)
	if err != nil {
		return storeUnavailable(err)
	}
	if breached {
		s.budgetExceeded(self, budget.TurnID, MessageBudgetPerTurn-budget.Remaining)
	}

	out := make([]outMessage, len(msgs))
	for i, m := range msgs {
		out[i] = outMessage{
			MessageID:   m.MessageID,
			From:        m.FromAgent,
			FromAddress: m.FromAddress,
			FromName:    m.FromName,
			Subject:     m.Subject,
			Body:        m.Body,
			CreatedAt:   m.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
			InReplyTo:   m.InReplyTo,
		}
	}

	remaining, err := s.store.UnreadCount(self)
	if err != nil {
		return storeUnavailable(err)
	}
	return jsonResult(map[string]any{
		"messages":           out,
		"remaining":          remaining,
		"budget_remaining":   budget.Remaining,
		"budget_exhausted":   budget.Remaining == 0,
		"budget_exceeded":    breached,
		"budget_explanation": budgetExplanation(budget.Remaining),
	})
}

func budgetExplanation(remaining int) string {
	if remaining > 0 {
		return ""
	}
	return fmt.Sprintf("Per-turn message budget (%d) reached; no more messages can be processed this turn.", MessageBudgetPerTurn)
}

func storeUnavailable(err error) (*mcp.CallToolResult, any, error) {
	return errResult(map[string]any{
		"ok":      false,
		"error":   "store_unavailable",
		"message": err.Error(),
	})
}

func boolOr(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

func intOr(p *int, def int) int {
	if p == nil {
		return def
	}
	return *p
}
