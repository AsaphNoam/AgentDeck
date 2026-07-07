package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/transcript"
)

// handlePrompt implements POST /api/sessions/{id}/prompt (techspec §7.3).
func (s *Server) handlePrompt(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIError(w, apiError(runtime.CodeValidation, "invalid JSON body"))
		return
	}
	if strings.TrimSpace(body.Text) == "" {
		writeAPIError(w, apiError(runtime.CodeValidation, "text is required"))
		return
	}
	if err := s.registry.SendPrompt(r.Context(), id, body.Text); err != nil {
		writeAPIError(w, sessionOpError(err))
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"accepted": true, "agent_id": id})
}

func (s *Server) handleTranscript(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.stateStore.ReadAgent(id); err != nil {
		writeAPIError(w, apiError(runtime.CodeNotFound, "no such agent: "+id))
		return
	}
	sinceSeq, err := parseInt64Query(r, "since_seq")
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeValidation, "since_seq must be an integer"))
		return
	}
	events, err := transcript.ReadFile(s.configStore.Home(), id, transcript.ReadOptions{
		SinceSeq:    sinceSeq,
		IncludeMeta: r.URL.Query().Get("include_meta") == "true",
	})
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"agent_id": id, "events": events})
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.stateStore.ReadAgent(id); err != nil {
		writeAPIError(w, apiError(runtime.CodeNotFound, "no such agent: "+id))
		return
	}
	unreadOnly := r.URL.Query().Get("unread_only") == "true"
	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			writeAPIError(w, apiError(runtime.CodeValidation, "limit must be a positive integer"))
			return
		}
		limit = n
	}
	if limit > 200 {
		limit = 200
	}
	// ListMessages returns the newest N (created_at DESC); the inbox presents
	// newest-first directly, so no reversal is needed. (The prior ASC query +
	// reversal returned the oldest N and dropped new mail past the limit.)
	msgs, err := s.stateStore.ListMessages(id, unreadOnly, limit)
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	unread, err := s.stateStore.UnreadCount(id)
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"agent_id":     id,
		"unread_count": unread,
		"messages":     msgs,
	})
}

func parseInt64Query(r *http.Request, key string) (int64, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return 0, nil
	}
	return strconv.ParseInt(raw, 10, 64)
}

// handleCancel implements POST /api/sessions/{id}/cancel (techspec §7.4).
func (s *Server) handleCancel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cancelled, err := s.registry.Cancel(r.Context(), id)
	if err != nil {
		if errors.Is(err, runtime.ErrNoHandle) {
			writeAPIError(w, apiError(runtime.CodeNotFound, "no such agent: "+id))
			return
		}
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	// false = the agent was idle, so the cancel was a no-op (techspec §7.4).
	writeJSON(w, http.StatusAccepted, map[string]any{"cancelled": cancelled})
}

// handleStop implements POST /api/sessions/{id}/stop (techspec §7.5).
func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.registry.Stop(r.Context(), id); err != nil {
		if errors.Is(err, runtime.ErrNoHandle) {
			// Not currently running. If the identity still exists, the agent is
			// already stopped — make stop idempotent so a double-click or a
			// lost-response retry reads as success, not a phantom "unknown agent".
			// 404 is reserved for ids with no identity row at all.
			if _, rerr := s.stateStore.ReadAgent(id); rerr == nil {
				s.teardownAgentRegistration(id)
				writeJSON(w, http.StatusOK, map[string]any{"stopped": true})
				return
			}
			writeAPIError(w, apiError(runtime.CodeNotFound, "no such agent: "+id))
			return
		}
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	s.teardownAgentRegistration(id)
	writeJSON(w, http.StatusOK, map[string]any{"stopped": true})
}

// handlePermission implements POST /api/sessions/{id}/permission (techspec §7.6).
func (s *Server) handlePermission(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		ToolCallID string `json:"tool_call_id"`
		Decision   string `json:"decision"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIError(w, apiError(runtime.CodeValidation, "invalid JSON body"))
		return
	}
	if body.Decision != "approve" && body.Decision != "deny" {
		writeAPIError(w, apiError(runtime.CodeValidation, "decision must be approve or deny"))
		return
	}
	if err := s.registry.Permission(r.Context(), id, body.ToolCallID, body.Decision); err != nil {
		writeAPIError(w, permissionError(err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"resolved": true, "tool_call_id": body.ToolCallID, "decision": body.Decision,
	})
}

func (s *Server) handleRename(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIError(w, apiError(runtime.CodeValidation, "invalid JSON body"))
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		writeAPIError(w, apiError(runtime.CodeEmptyName, "name is required"))
		return
	}
	agent, err := s.stateStore.ReadAgent(id)
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeNotFound, "no such agent: "+id))
		return
	}
	agent.Name = strings.TrimSpace(body.Name)
	if err := s.stateStore.WriteAgent(agent); err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	if _, err := s.stateMgr.Touch(id); err != nil {
		s.log.Debug("rename state touch failed", "agent", id, "err", err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"agent_id": id, "name": agent.Name})
}

func (s *Server) handleIdentity(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Name  *string `json:"name"`
		Group *string `json:"group"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIError(w, apiError(runtime.CodeValidation, "invalid JSON body"))
		return
	}
	agent, err := s.stateStore.ReadAgent(id)
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeNotFound, "no such agent: "+id))
		return
	}
	if body.Name != nil {
		name := strings.TrimSpace(*body.Name)
		if name == "" {
			writeAPIError(w, apiError(runtime.CodeEmptyName, "name is required"))
			return
		}
		agent.Name = name
	}
	if body.Group != nil {
		group := strings.TrimSpace(*body.Group)
		if group == "_ungrouped" {
			writeAPIError(w, apiError(runtime.CodeInvalidGroupName, "_ungrouped is reserved"))
			return
		}
		agent.Group = group
	}
	if err := s.stateStore.WriteAgent(agent); err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	if _, err := s.stateMgr.Touch(id); err != nil {
		s.log.Debug("identity state touch failed", "agent", id, "err", err)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"agent_id":  agent.AgentID,
		"group":     agent.Group,
		"name":      agent.Name,
		"role":      agent.Role,
		"project":   agent.Project,
		"backend":   agent.Backend,
		"model":     agent.Model,
		"interface": agent.Interface,
	})
}

// sessionOpError maps a prompt/control error to an APIError (techspec §7.3).
func sessionOpError(err error) *runtime.APIError {
	switch {
	case errors.Is(err, runtime.ErrNoHandle):
		return apiError(runtime.CodeNotFound, "agent not started")
	case errors.Is(err, runtime.ErrTurnInFlight):
		return apiError(runtime.CodeConflict, "a turn is already in flight")
	default:
		return apiError(runtime.CodeInternal, err.Error())
	}
}

// permissionError maps a Permission relay error to an APIError (techspec §7.6).
func permissionError(err error) *runtime.APIError {
	switch {
	case errors.Is(err, runtime.ErrNoHandle):
		return apiError(runtime.CodeNotFound, "agent not started")
	case errors.Is(err, runtime.ErrNoPendingPermission):
		return apiError(runtime.CodeConflict, "no pending permission for that tool_call_id")
	case errors.Is(err, runtime.ErrPermissionAlreadyResolved):
		return apiError(runtime.CodeConflict, "permission already resolved for that tool_call_id")
	case errors.Is(err, runtime.ErrInvalidDecision):
		return apiError(runtime.CodeValidation, "decision must be approve or deny")
	default:
		return apiError(runtime.CodeInternal, err.Error())
	}
}
