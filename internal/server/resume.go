package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
)

// resumeResponse extends the standard session envelope with a `resumed` flag.
type resumeResponse struct {
	sessionResponse
	Resumed bool `json:"resumed"`
}

// resumeOverride carries the optional resume body (Phase 4: only the empty body
// is exercised; override fields are validated so Phase 6 can reuse this endpoint
// without changes). A wake (FS-01.R33) always resumes with the zero value.
type resumeOverride struct {
	Interface string `json:"interface"`
	Backend   string `json:"backend"`
	Model     string `json:"model"`
	Effort    string `json:"effort"`
	// ConfigRefresh selects "Resume with latest setup" (Phase 7 §2.5): re-resolve
	// the source fresh and freeze the new high-level object instead of the frozen
	// snapshot. Absent/false preserves the snapshot's frozen federation config.
	ConfigRefresh bool `json:"config_refresh"`
}

// claimResume takes the exclusive per-agent resume claim (TS-01.R16, INV §5).
// It is taken before any registration side effect because the artifacts minted
// during composition are keyed by agent_id alone: two concurrent composers would
// leave the second one's token/MCP/hook-settings in place of the first's, and the
// loser's teardown would then revoke the winner's (INV §4). acquireAgentStart is
// a counting archive lease and cannot serve this purpose.
func (s *Server) claimResume(agentID string) bool {
	s.resumeMu.Lock()
	defer s.resumeMu.Unlock()
	if s.resuming[agentID] {
		return false
	}
	s.resuming[agentID] = true
	return true
}

// resumeInFlight reports whether a resume currently owns this agent. It is a
// hint for callers that must not block on one (the nudger), never a substitute
// for claimResume.
func (s *Server) resumeInFlight(agentID string) bool {
	s.resumeMu.Lock()
	defer s.resumeMu.Unlock()
	return s.resuming[agentID]
}

func (s *Server) releaseResume(agentID string) {
	s.resumeMu.Lock()
	delete(s.resuming, agentID)
	s.resumeMu.Unlock()
}

// handleResume implements POST /api/sessions/{id}/resume (techspec §7.2).
func (s *Server) handleResume(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var override resumeOverride
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&override); err != nil {
			writeAPIError(w, apiError(runtime.CodeValidation, "invalid JSON body"))
			return
		}
	}
	if ae := s.resumeSession(r.Context(), id, override); ae != nil {
		writeAPIError(w, ae)
		return
	}
	writeJSON(w, http.StatusOK, resumeResponse{sessionResponse: s.readSession(id), Resumed: true})
}

// resumeSession is the one resume seam (TS-01.R16): the explicit resume route,
// the prompt route's wake, and the messaging wake all run exactly this flow —
// same gates, same composition, same failure teardown — under one exclusive
// per-agent claim. It returns nil once the agent is running again.
func (s *Server) resumeSession(ctx context.Context, id string, override resumeOverride) *runtime.APIError {
	if !s.claimResume(id) {
		return apiError(runtime.CodeConflict, "a resume is already in progress")
	}
	defer s.releaseResume(id)

	// 1. Load identity row → 404 if not found.
	agent, err := s.stateStore.ReadAgent(id)
	if err != nil {
		return apiError(runtime.CodeNotFound, "no such agent: "+id)
	}
	if ae := s.projectArchiveGate(agent.Project, "project is archived; restore it before resuming"); ae != nil {
		return ae
	}
	if agent.Archived {
		return apiError(runtime.CodeAgentArchived, "agent is archived; restore it before resuming")
	}
	if ae := s.acquireAgentStart(agent.Project, id); ae != nil {
		return ae
	}
	defer s.releaseAgentStart(agent.Project, id)

	// 2. Running row present → 409 conflict (resume is for inactive sessions).
	// A non-ErrNotFound error means the DB read itself failed; do NOT fall through
	// (that could resume an already-running agent) — surface it as a 500 instead.
	if _, err := s.stateStore.ReadRunning(id); err == nil {
		return apiError(runtime.CodeConflict, "agent is already running")
	} else if !errors.Is(err, state.ErrNotFound) {
		return apiError(runtime.CodeInternal, err.Error())
	}

	// 3. Load frozen config snapshot → 422 if missing.
	snap, err := s.stateStore.ReadSession(id)
	if errors.Is(err, state.ErrNotFound) {
		return apiError(runtime.CodeValidation, "no persisted session to resume")
	}
	if err != nil {
		return apiError(runtime.CodeInternal, err.Error())
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
	effort := agent.Effort
	iface := agent.Interface
	if override.Backend != "" {
		backendID = override.Backend
	}
	if override.Model != "" {
		modelKey = override.Model
	}
	if override.Effort != "" {
		effort = override.Effort
	}
	if override.Interface != "" {
		iface = override.Interface
	}
	if iface != "chat" && iface != "terminal" {
		return apiError(runtime.CodeValidation, "invalid interface: "+iface)
	}

	// 5. Re-resolve env secrets from backends.json (snapshot stores key names only).
	backends, err := s.configStore.ReadBackends()
	if err != nil {
		if errors.Is(err, config.ErrNotFound) || errors.Is(err, config.ErrCorrupt) {
			backends = config.DefaultBackends()
		} else {
			return apiError(runtime.CodeInternal, "read backends: "+err.Error())
		}
	}
	backend, ok := backends.Backends[backendID]
	if !ok {
		return apiError(runtime.CodeValidation, "unknown backend: "+backendID)
	}
	model, ok := backend.Models[modelKey]
	if !ok {
		return apiError(runtime.CodeValidation, "unknown model: "+modelKey)
	}
	// 5b. "Resume with latest setup" (§2.5): re-resolve the source fresh and freeze
	// the new object. Done before composeResumeSpec so a source error blocks the
	// resume with nothing registered to unwind. Absent/false keeps the frozen
	// snapshot; a backend without an active binding leaves the snapshot untouched.
	var refreshedConfig json.RawMessage
	if override.ConfigRefresh {
		project, perr := s.configStore.ReadProject(agent.Project)
		if perr != nil {
			return apiError(runtime.CodeValidation, "unknown project: "+agent.Project)
		}
		lc, fed, ae := s.composeFederation(ctx, backendID,
			launchRequest{Project: agent.Project, Model: override.Model, Effort: override.Effort}, backend, project, modelKey)
		if ae != nil {
			return ae
		}
		refreshedConfig = lc
		effort, ae = resolveEffort(override.Effort, model, fed)
		if ae != nil {
			return ae
		}
	}
	// An ordinary resume re-applies the effort frozen with the agent. Only a
	// caller selecting a new effort or fresh configuration asks the current
	// catalog to validate a newly resolved value.
	if override.Effort != "" || override.ConfigRefresh {
		if err := config.ValidateModelEffort(backend, model, effort); err != nil {
			return apiError(runtime.CodeInvalidField, err.Error())
		}
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
		Effort:    effort,
		Interface: iface,
		CreatedAt: agent.CreatedAt,
		Group:     agent.Group,
	}
	spec, ae := s.composeResumeSpec(resumeAgent, snap, backend, model)
	if ae != nil {
		return ae
	}
	// Override the frozen federation object with the freshly-resolved one when the
	// caller asked to resume with the latest setup.
	if refreshedConfig != nil {
		spec.LaunchConfig = refreshedConfig
	}
	// Honor native model inheritance across resume (§2.4): if the effective launch
	// object left the model to native resolution and the caller supplied no explicit
	// model override, omit the model over ACP so the CLI keeps resolving its own —
	// otherwise a resume would silently reinstate the backend default. config_refresh
	// uses the freshly-resolved doc (set above); the default path uses the frozen one.
	if override.Model == "" && frozenModelInherited(spec.LaunchConfig) {
		spec.ModelID = ""
	}

	// 7. Resume via the registry (double-resume is guarded by the registry sentinel
	//    inside the claim taken above).
	if _, err := s.registry.Resume(ctx, spec); err != nil {
		// composeResumeSpec wrote the hook-settings file too (via
		// composeHookRegistration) — tear down all three artifacts, not just
		// token + MCP, so a failed resume leaves nothing behind (launch parity).
		s.teardownAgentRegistration(id)
		return resumeStartError(err)
	}
	return nil
}

// wakeAgent is the shared wake seam for an incoming message (FS-01.R33): it
// applies the wake gates and then runs the same resumeSession the explicit
// resume route uses. It reports woken=false with no error when a wake gate
// excludes the agent, so the caller keeps its existing non-running behavior; a
// non-nil error means the wake was attempted and failed.
func (s *Server) wakeAgent(ctx context.Context, agentID string) (bool, *runtime.APIError) {
	if _, ok := s.wakeCandidate(agentID); !ok {
		return false, nil
	}
	if ae := s.resumeSession(ctx, agentID, resumeOverride{}); ae != nil {
		return false, ae
	}
	return true, nil
}

// wakeCandidate reports whether this stopped agent may be woken by a message.
// The database-checkable gates live in one query shared with the addressable set
// (state.StoppedWakeCandidates); the project-archive gate is applied here because
// it reads configuration, and a missing project definition passes exactly as it
// does for an explicit resume (FS-05.R34).
func (s *Server) wakeCandidate(agentID string) (state.LiveAgent, bool) {
	candidates, err := s.stateStore.StoppedWakeCandidates(agentID)
	if err != nil {
		s.log.Debug("wake candidate lookup failed", "agent", agentID, "err", err)
		return state.LiveAgent{}, false
	}
	if len(candidates) == 0 {
		return state.LiveAgent{}, false
	}
	if ae := s.projectArchiveGate(candidates[0].Project, "project is archived"); ae != nil {
		return state.LiveAgent{}, false
	}
	return candidates[0], true
}

// addressableAgents is the one addressable set (TS-04.R26) behind list_agents and
// send_message resolution: every running agent plus every stopped chat agent a
// message wakes.
func (s *Server) addressableAgents() ([]state.LiveAgent, error) {
	running, err := s.stateStore.LiveAgents()
	if err != nil {
		return nil, err
	}
	stopped, err := s.stateStore.StoppedWakeCandidates("")
	if err != nil {
		return nil, err
	}
	for _, a := range stopped {
		if ae := s.projectArchiveGate(a.Project, "project is archived"); ae != nil {
			continue
		}
		running = append(running, a)
	}
	return running, nil
}

// composeResumeSpec mints a fresh hook token + MCP registration and builds the
// resume LaunchSpec entirely from the frozen snapshot (cwd/system_prompt/
// last_session_id plus the frozen skip_permissions/add_dirs) — mirroring
// composeSwitchSpec. On registration failure it rolls back its own side effects
// and returns an APIError.
func (s *Server) composeResumeSpec(agent state.Agent, snap state.SessionSnapshot, be config.Backend, model config.Model) (runtime.LaunchSpec, *runtime.APIError) {
	return s.composeResumeSpecWithGeneration(agent, snap, be, model, "")
}

func (s *Server) composeResumeSpecWithGeneration(agent state.Agent, snap state.SessionSnapshot, be config.Backend, model config.Model, generation string) (runtime.LaunchSpec, *runtime.APIError) {
	// Only claude-acp supports the terminal interface; a resume that lands (or is
	// overridden to) terminal on any other backend would produce a statusless
	// agent, so reject it here — the third of the three composers that gate on
	// terminalSupported (launch/switch/resume drift hot spot, §4).
	if agent.Interface == "terminal" && !terminalSupported(be.Type) {
		return runtime.LaunchSpec{}, apiError(runtime.CodeTerminalUnavailable, terminalUnsupportedReason(be.Type))
	}
	// Ensure the shared-resources directory before any registration side effect
	// (FS-11.R6/R9). The frozen snapshot already carries the path in add_dirs and
	// the prompt for agents launched after the feature shipped; the env var is
	// recomposed here every launch (INV §2).
	resourceDir, ae := s.ensureProjectResources(agent.Project)
	if ae != nil {
		return runtime.LaunchSpec{}, ae
	}
	token := mintHookToken()
	if generation == "" {
		generation = token
	}
	s.rememberHookToken(agent.AgentID, token)
	mcpSpec, err := s.registerMessagingMCP(agent, generation)
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
		Generation:     generation,
		Cwd:            snap.Cwd,
		AddDirs:        snap.AddDirs,
		SystemPrompt:   snap.SystemPrompt,
		BackendType:    be.Type,
		ModelID:        model.Model,
		Effort:         agent.Effort,
		Env:            composeChildEnv(be.Type, s.configStore.Home(), be.Env, model.Env, s.hookEnv(agent, token), projectResourcesEnv(resourceDir)),
		SkipPerms:      snap.SkipPermissions,
		HookToken:      token,
		MCPServers:     []runtime.MCPServerSpec{mcpSpec},
		ExtraArgs:      extraArgs,
		LastSessionID:  snap.LastSessionID,
		LastContextPct: snap.LastContextPct,
		// Resume reproduces the frozen federation launch object by default
		// (techspec §2.5); "Resume with latest setup" re-resolves it (handleResume).
		LaunchConfig: snap.LaunchConfig,
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
