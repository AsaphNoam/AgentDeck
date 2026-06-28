package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/agentdeck/agentdeck/internal/runtime"
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
			writeAPIError(w, apiError(runtime.CodeNotFound, "no such agent: "+id))
			return
		}
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
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
	case errors.Is(err, runtime.ErrInvalidDecision):
		return apiError(runtime.CodeValidation, "decision must be approve or deny")
	default:
		return apiError(runtime.CodeInternal, err.Error())
	}
}
