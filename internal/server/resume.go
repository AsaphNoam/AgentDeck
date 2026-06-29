package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"

	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
)

// resumeResponse extends the standard session envelope with a `resumed` flag.
type resumeResponse struct {
	sessionResponse
	Resumed bool `json:"resumed"`
}

// handleResume implements POST /api/sessions/{id}/resume (techspec §7.2).
func (s *Server) handleResume(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Parse the optional override body (Phase 4: only empty body is exercised; override
	// fields are validated so Phase 6 can reuse this endpoint without changes).
	var override struct {
		Interface string `json:"interface"`
		Backend   string `json:"backend"`
		Model     string `json:"model"`
	}
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&override); err != nil {
			writeAPIError(w, apiError(runtime.CodeValidation, "invalid JSON body"))
			return
		}
	}

	// 1. Load identity row → 404 if not found.
	agent, err := s.stateStore.ReadAgent(id)
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeNotFound, "no such agent: "+id))
		return
	}

	// 2. Running row present → 409 conflict (resume is for inactive sessions).
	// A non-ErrNotFound error means the DB read itself failed; do NOT fall through
	// (that could resume an already-running agent) — surface it as a 500 instead.
	if _, err := s.stateStore.ReadRunning(id); err == nil {
		writeAPIError(w, apiError(runtime.CodeConflict, "agent is already running"))
		return
	} else if !errors.Is(err, state.ErrNotFound) {
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}

	// 3. Load frozen config snapshot → 422 if missing.
	snap, err := s.stateStore.ReadSession(id)
	if errors.Is(err, state.ErrNotFound) {
		writeAPIError(w, apiError(runtime.CodeValidation, "no persisted session to resume"))
		return
	}
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}

	// 4. Apply optional override fields (Phase 4: only no-override path is exercised).
	backendID := snap.Backend
	modelKey := snap.Model
	iface := snap.Interface
	if override.Backend != "" {
		backendID = override.Backend
	}
	if override.Model != "" {
		modelKey = override.Model
	}
	if override.Interface != "" {
		iface = override.Interface
	}
	if iface == "terminal" {
		writeAPIError(w, apiError(runtime.CodeNotImplemented, "terminal resume not implemented until Phase 6"))
		return
	}

	// 5. Re-resolve env secrets from backends.json (snapshot stores key names only).
	backends, err := s.configStore.ReadBackends()
	if err != nil {
		if errors.Is(err, config.ErrNotFound) || errors.Is(err, config.ErrCorrupt) {
			backends = config.DefaultBackends()
		} else {
			writeAPIError(w, apiError(runtime.CodeInternal, "read backends: "+err.Error()))
			return
		}
	}
	backend, ok := backends.Backends[backendID]
	if !ok {
		writeAPIError(w, apiError(runtime.CodeValidation, "unknown backend: "+backendID))
		return
	}
	model, ok := backend.Models[modelKey]
	if !ok {
		writeAPIError(w, apiError(runtime.CodeValidation, "unknown model: "+modelKey))
		return
	}

	// 6. Mint fresh hook token and build LaunchSpec from the frozen snapshot.
	token := mintHookToken()
	s.rememberHookToken(id, token)
	mcpSpec, err := s.registerMessagingMCP(agent, backend.Type)
	if err != nil {
		s.forgetHookToken(id)
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}

	resumeAgent := state.Agent{
		AgentID:   agent.AgentID,
		Name:      agent.Name,
		Role:      agent.Role,
		Project:   agent.Project,
		Backend:   backendID,
		Model:     modelKey,
		Interface: iface,
		CreatedAt: agent.CreatedAt,
		Group:     agent.Group,
	}
	extraArgs, err := s.composeHookRegistration(resumeAgent, backend.Type)
	if err != nil {
		s.forgetHookToken(id)
		s.cleanupMessagingMCP(id)
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}

	spec := runtime.LaunchSpec{
		Agent:          resumeAgent,
		Cwd:            snap.Cwd,
		SystemPrompt:   snap.SystemPrompt,
		BackendType:    backend.Type,
		ModelID:        model.Model,
		Env:            composeEnv(os.Environ(), backend.Env, model.Env, s.hookEnv(resumeAgent, token)),
		HookToken:      token,
		MCPServers:     []runtime.MCPServerSpec{mcpSpec},
		ExtraArgs:      extraArgs,
		LastSessionID:  snap.LastSessionID,
		LastContextPct: snap.LastContextPct,
	}

	// 7. Resume via the registry (double-resume is guarded by the registry sentinel).
	if _, err := s.registry.Resume(r.Context(), spec); err != nil {
		s.forgetHookToken(id)
		s.cleanupMessagingMCP(id)
		writeAPIError(w, resumeStartError(err))
		return
	}

	writeJSON(w, http.StatusOK, resumeResponse{sessionResponse: s.readSession(id), Resumed: true})
}

// resumeStartError maps a Resume failure to the right API error code.
func resumeStartError(err error) *runtime.APIError {
	switch {
	case errors.Is(err, runtime.ErrNotImplemented):
		return apiError(runtime.CodeNotImplemented, err.Error())
	case errors.Is(err, runtime.ErrAlreadyStarted):
		return apiError(runtime.CodeConflict, err.Error())
	default:
		return apiError(runtime.CodeRuntimeStartFailed, err.Error())
	}
}
