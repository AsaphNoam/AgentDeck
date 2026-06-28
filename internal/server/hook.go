package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/agentdeck/agentdeck/internal/state"
)

type hookRequest struct {
	state.HookPayload
	Token string `json:"token,omitempty"`
}

type hookErrorBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func (s *Server) handleHook(w http.ResponseWriter, r *http.Request) {
	var req hookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeHookError(w, http.StatusBadRequest, "bad_request", "malformed JSON")
		return
	}

	token := r.Header.Get("X-AgentDeck-Token")
	if token == "" {
		token = req.Token
	}
	if token == "" {
		writeHookError(w, http.StatusUnauthorized, "unauthorized", "missing token")
		return
	}

	if _, err := s.stateMgr.ApplyHook(token, req.HookPayload); err != nil {
		switch {
		case errors.Is(err, state.ErrInvalidHook):
			writeHookError(w, http.StatusBadRequest, "bad_request", err.Error())
		case errors.Is(err, state.ErrTokenMismatch):
			writeHookError(w, http.StatusForbidden, "forbidden", "token mismatch")
		case errors.Is(err, state.ErrNotFound):
			writeHookError(w, http.StatusNotFound, "not_found", "unknown agent")
		default:
			s.log.Error("hook: apply", "err", err)
			writeHookError(w, http.StatusInternalServerError, "internal", err.Error())
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func writeHookError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, hookErrorBody{Error: code, Message: message})
}
