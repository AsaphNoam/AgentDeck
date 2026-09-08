package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
)

type sessionConfigRequest struct {
	Effort *string `json:"effort"`
	Fast   *bool   `json:"fast"`
}

func (s *Server) handleSessionConfig(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req sessionConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || (req.Effort == nil && req.Fast == nil) {
		writeAPIError(w, apiError(runtime.CodeValidation, "effort or fast is required"))
		return
	}
	agent, err := s.stateStore.ReadAgent(id)
	if errors.Is(err, state.ErrNotFound) {
		writeAPIError(w, apiError(runtime.CodeNotFound, "no such agent: "+id))
		return
	}
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	if agent.Interface != "chat" || !s.registry.Owns(id) {
		writeAPIError(w, apiError(runtime.CodeConflict, "agent is not a running chat session"))
		return
	}
	backends, err := s.readBackendsOrDefault()
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, "read backends: "+err.Error()))
		return
	}
	backend, ok := backends.Backends[agent.Backend]
	if !ok {
		writeAPIError(w, apiError(runtime.CodeInvalidField, "unknown backend: "+agent.Backend))
		return
	}
	model, ok := backend.Models[agent.Model]
	if !ok {
		writeAPIError(w, apiError(runtime.CodeInvalidField, "unknown model: "+agent.Model))
		return
	}
	if req.Effort != nil {
		if err := config.ValidateModelEffort(backend, model, *req.Effort); err != nil {
			writeAPIError(w, apiError(runtime.CodeInvalidField, err.Error()))
			return
		}
	}
	if req.Fast != nil {
		if err := config.ValidateModelFast(backend, model, *req.Fast); err != nil {
			writeAPIError(w, apiError(runtime.CodeInvalidField, err.Error()))
			return
		}
	}
	appliedEffort, appliedFast, err := s.registry.SetSessionConfig(r.Context(), id, req.Effort, req.Fast)
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeRuntimeStartFailed, err.Error()))
		return
	}
	if req.Effort == nil {
		appliedEffort = agent.Effort
	}
	if req.Fast == nil {
		appliedFast = agent.Fast
	}
	if err := s.stateStore.UpdateAgentSessionSettings(id, appliedEffort, appliedFast); err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	_, _ = s.stateMgr.Touch(id)
	agent.Effort, agent.Fast = appliedEffort, appliedFast
	writeJSON(w, http.StatusOK, agent)
}
