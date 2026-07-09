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

	// 4. Resolve the identity to resume. backend/model/interface come from the LIVE
	// identity row (which switch-runtime keeps current), NOT the frozen snapshot:
	// after a chat→terminal switch the snapshot's interface stays "chat" (no
	// terminal turn_end refreshes it), but the agents row correctly reads
	// "terminal" — so resuming from the snapshot would relaunch the wrong runtime.
	// cwd/system_prompt/last_session_id below still come from the frozen snapshot.
	// Optional override fields win (Phase 4 exercises only the no-override path).
	backendID := agent.Backend
	modelKey := agent.Model
	iface := agent.Interface
	if override.Backend != "" {
		backendID = override.Backend
	}
	if override.Model != "" {
		modelKey = override.Model
	}
	if override.Interface != "" {
		iface = override.Interface
	}
	if iface != "chat" && iface != "terminal" {
		writeAPIError(w, apiError(runtime.CodeValidation, "invalid interface: "+iface))
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

	// 6. Build the resume LaunchSpec entirely from the frozen snapshot — including
	//    skip_permissions and add_dirs, which are persisted at launch so a later
	//    role/project edit cannot change a resumed agent's permission policy or
	//    accessible directories (techspec §12.4 frozen-snapshot rule).
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
	spec, ae := s.composeResumeSpec(resumeAgent, snap, backend, model)
	if ae != nil {
		writeAPIError(w, ae)
		return
	}

	// 7. Resume via the registry (double-resume is guarded by the registry sentinel).
	if _, err := s.registry.Resume(r.Context(), spec); err != nil {
		// composeResumeSpec wrote the hook-settings file too (via
		// composeHookRegistration) — tear down all three artifacts, not just
		// token + MCP, so a failed resume leaves nothing behind (launch parity).
		s.teardownAgentRegistration(id)
		writeAPIError(w, resumeStartError(err))
		return
	}

	writeJSON(w, http.StatusOK, resumeResponse{sessionResponse: s.readSession(id), Resumed: true})
}

// composeResumeSpec mints a fresh hook token + MCP registration and builds the
// resume LaunchSpec entirely from the frozen snapshot (cwd/system_prompt/
// last_session_id plus the frozen skip_permissions/add_dirs) — mirroring
// composeSwitchSpec. On registration failure it rolls back its own side effects
// and returns an APIError.
func (s *Server) composeResumeSpec(agent state.Agent, snap state.SessionSnapshot, be config.Backend, model config.Model) (runtime.LaunchSpec, *runtime.APIError) {
	// Only claude-acp supports the terminal interface; a resume that lands (or is
	// overridden to) terminal on any other backend would produce a statusless
	// agent, so reject it here — the third of the three composers that gate on
	// terminalSupported (launch/switch/resume drift hot spot, §4).
	if agent.Interface == "terminal" && !terminalSupported(be.Type) {
		return runtime.LaunchSpec{}, apiError(runtime.CodeTerminalUnavailable, terminalUnsupportedReason(be.Type))
	}
	token := mintHookToken()
	s.rememberHookToken(agent.AgentID, token)
	mcpSpec, err := s.registerMessagingMCP(agent)
	if err != nil {
		s.forgetHookToken(agent.AgentID)
		return runtime.LaunchSpec{}, apiError(runtime.CodeInternal, err.Error())
	}
	extraArgs, err := s.composeHookRegistration(agent, be.Type)
	if err != nil {
		s.forgetHookToken(agent.AgentID)
		s.cleanupMessagingMCP(agent.AgentID)
		return runtime.LaunchSpec{}, apiError(runtime.CodeInternal, err.Error())
	}
	return runtime.LaunchSpec{
		Agent:          agent,
		Cwd:            snap.Cwd,
		AddDirs:        snap.AddDirs,
		SystemPrompt:   snap.SystemPrompt,
		BackendType:    be.Type,
		ModelID:        model.Model,
		Env:            composeEnv(os.Environ(), be.Env, model.Env, s.hookEnv(agent, token)),
		SkipPerms:      snap.SkipPermissions,
		HookToken:      token,
		MCPServers:     []runtime.MCPServerSpec{mcpSpec},
		ExtraArgs:      extraArgs,
		LastSessionID:  snap.LastSessionID,
		LastContextPct: snap.LastContextPct,
	}, nil
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
