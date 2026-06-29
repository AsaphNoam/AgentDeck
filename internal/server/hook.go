package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

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

	// file_edit and command events are routed to the indexer for file/command
	// tracking; they do not touch the state manager.
	if req.Event == "file_edit" || req.Event == "command" {
		if err := s.applyTrackingHook(token, req.HookPayload); err != nil {
			switch {
			case errors.Is(err, state.ErrInvalidHook):
				writeHookError(w, http.StatusBadRequest, "bad_request", err.Error())
			case errors.Is(err, state.ErrTokenMismatch):
				writeHookError(w, http.StatusForbidden, "forbidden", "token mismatch")
			case errors.Is(err, state.ErrNotFound):
				writeHookError(w, http.StatusNotFound, "not_found", "unknown agent")
			default:
				s.log.Error("hook: tracking", "err", err)
				writeHookError(w, http.StatusInternalServerError, "internal", err.Error())
			}
			return
		}
		w.WriteHeader(http.StatusNoContent)
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

// applyTrackingHook validates the token via the running row and writes
// file_edit / command rows into the index tables.
func (s *Server) applyTrackingHook(token string, payload state.HookPayload) error {
	if strings.TrimSpace(payload.AgentID) == "" {
		return fmt.Errorf("%w: agent_id is required", state.ErrInvalidHook)
	}
	// Validate the token against the running row (same guard as ApplyHook).
	if err := s.stateStore.ValidateHookToken(payload.AgentID, token); err != nil {
		return err
	}
	ts := payload.Timestamp
	if ts == "" {
		ts = time.Now().UTC().Format(time.RFC3339)
	}
	switch payload.Event {
	case "file_edit":
		if strings.TrimSpace(payload.Path) == "" {
			return fmt.Errorf("%w: path is required for file_edit", state.ErrInvalidHook)
		}
		return s.indexer.CaptureHookFile(payload.AgentID, payload.Path, ts, payload.Seq)
	case "command":
		if strings.TrimSpace(payload.Command) == "" {
			return fmt.Errorf("%w: command is required for command event", state.ErrInvalidHook)
		}
		return s.indexer.CaptureHookCommand(payload.AgentID, payload.Command, ts, payload.ToolCallID, payload.Seq)
	default:
		return fmt.Errorf("%w: invalid event", state.ErrInvalidHook)
	}
}

func writeHookError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, hookErrorBody{Error: code, Message: message})
}
