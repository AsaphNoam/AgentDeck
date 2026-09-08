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
	// Take the shared exclusive per-agent claim (TS-01.R16, INV §5) across the
	// whole read → apply → persist window. The runtime call alone serializes only
	// while it holds the agent's own lock, so without this claim two concurrent
	// setting changes can apply in one order and commit in the other, and a change
	// overlapping Stop, Resume, or Switch can land its durable write after a new
	// runtime generation has replaced the session it was applied to — leaving the
	// provider running one value while the agent, session, archive, and next
	// resume record another (INV §1, INV §15).
	if !s.claimLifecycle(id) {
		writeAPIError(w, apiError(runtime.CodeConflict, "a lifecycle transition is already in progress"))
		return
	}
	defer s.releaseLifecycle(id)
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
	// The settings reach the provider one at a time in an adapter-imposed order, so
	// a failure partway through leaves the earlier ones live. Reconcile against
	// what actually applied and persist that before reporting the failure,
	// otherwise the response, the stored agent, the archive, and the next resume
	// all disagree with the provider the next turn will run on (INV §15).
	change, runErr := s.registry.SetSessionConfig(r.Context(), id, req.Effort, req.Fast)
	appliedEffort, appliedFast := agent.Effort, agent.Fast
	if change.EffortApplied {
		appliedEffort = change.Effort
	}
	if change.FastApplied {
		appliedFast = change.Fast
	}
	if err := s.stateStore.UpdateAgentSessionSettings(id, appliedEffort, appliedFast, change.FastAvailable); err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	_, _ = s.stateMgr.Touch(id)
	if runErr != nil {
		writeAPIError(w, sessionConfigError(runErr))
		return
	}
	agent.Effort, agent.Fast = appliedEffort, appliedFast
	writeJSON(w, http.StatusOK, agent)
}

// sessionConfigError maps a runtime session-setting failure to the distinct,
// actionable reason the route contract promises (TS-03.R37). Collapsing all of
// them into one upstream-failure status left API clients unable to tell an
// option this session does not offer from a level the provider refused, which is
// the difference between "pick another model" and "pick another level" (INV §8).
func sessionConfigError(err error) *runtime.APIError {
	switch {
	case errors.Is(err, runtime.ErrNoHandle), errors.Is(err, runtime.ErrNotImplemented):
		return apiError(runtime.CodeAgentNotRunning, "agent is not a running chat session")
	case errors.Is(err, runtime.ErrSettingUnsupported):
		return apiError(runtime.CodeInvalidField, err.Error())
	case errors.Is(err, runtime.ErrSettingUnavailable):
		return apiError(runtime.CodeConflict, err.Error())
	case errors.Is(err, runtime.ErrSettingRejected):
		return apiError(runtime.CodeInvalidField, err.Error())
	default:
		// Includes ErrSettingIgnored: the provider answered success and did
		// something else, which is an upstream fault rather than a bad request.
		return apiError(runtime.CodeRuntimeStartFailed, err.Error())
	}
}
