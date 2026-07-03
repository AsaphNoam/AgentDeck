package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/agentdeck/agentdeck/internal/backend"
	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/runtime/terminal"
	"github.com/agentdeck/agentdeck/internal/state"
)

// switchCancelTimeout bounds the wait for an in-flight turn to settle before the
// switch stops the old runtime (techspec §9; config plumbing deferred).
const switchCancelTimeout = 5 * time.Second

// switchRuntimeRequest is the POST body (techspec §8.1); any subset, ≥1 field
// must differ from current.
type switchRuntimeRequest struct {
	Interface string `json:"interface"`
	Backend   string `json:"backend"`
	Model     string `json:"model"`
}

// switchRuntimeResponse is the 200 body (techspec §8.1).
type switchRuntimeResponse struct {
	AgentID        string              `json:"agent_id"`
	Interface      string              `json:"interface"`
	Backend        string              `json:"backend"`
	Model          string              `json:"model"`
	Running        *state.RunningEntry `json:"running,omitempty"`
	HistoryHandoff string              `json:"history_handoff"` // "native_resume" | "primer"
}

// handleSwitchRuntime implements POST /api/sessions/{id}/switch-runtime
// (techspec §5.1–5.4): stop the current runtime, persist the new identity, and
// resume on the same agent_id. Same-backend compatible switches use native
// resume; cross-backend/incompatible switches use a bounded AgentDeck transcript
// history primer.
func (s *Server) handleSwitchRuntime(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req switchRuntimeRequest
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAPIError(w, apiError(runtime.CodeValidation, "invalid JSON body"))
			return
		}
	}

	agent, err := s.stateStore.ReadAgent(id)
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeNotFound, "no such agent: "+id))
		return
	}

	// Compute the target identity (merge requested fields over current).
	target := agent
	if req.Interface != "" {
		target.Interface = req.Interface
	}
	if req.Backend != "" {
		target.Backend = req.Backend
	}
	if req.Model != "" {
		target.Model = req.Model
	}
	if target.Interface == agent.Interface && target.Backend == agent.Backend && target.Model == agent.Model {
		writeAPIError(w, apiError(runtime.CodeNoChange, "switch request equals current state"))
		return
	}
	if target.Interface != "chat" && target.Interface != "terminal" {
		writeAPIError(w, apiError(runtime.CodeInvalidField, "invalid interface: "+target.Interface))
		return
	}
	if target.Interface == "terminal" && !terminal.Probe().DriverAvailable("xterm") {
		writeAPIError(w, apiError(runtime.CodeTerminalUnavailable, "no terminal driver available on this host"))
		return
	}

	// Per-agent switch lock (§5.4).
	if !s.acquireSwitch(id) {
		writeAPIError(w, apiError(runtime.CodeSwitchInProgress, "a switch is already in progress for this agent"))
		return
	}
	defer s.releaseSwitch(id)

	// The switch operates on a live agent (cancel → stop → resume). No running row
	// → nothing to switch.
	prev, err := s.stateStore.ReadRunning(id)
	if errors.Is(err, state.ErrNotFound) {
		writeAPIError(w, apiError(runtime.CodeAgentNotRunning, "agent is not running"))
		return
	}
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}

	// Resolve native resume vs primer via the target adapter (§5.3): cross-backend
	// swaps never share native history; same-backend model swaps use native resume
	// only when the adapter says that is supported.
	ad, ok := backend.For(s.backendType(target.Backend))
	if !ok {
		writeAPIError(w, apiError(runtime.CodeInvalidField, "unknown backend: "+target.Backend))
		return
	}
	handoff := "native_resume"
	sameBackend := target.Backend == agent.Backend
	usePrimer := !sameBackend || (target.Model != agent.Model && !ad.CanSwitchModelOnResume())
	resumeID := ad.ResolveResumeID(prev.SessionID, sameBackend && !usePrimer)
	if usePrimer {
		resumeID = ""
		handoff = "primer"
	}

	// Validate the target backend/model/session snapshot exist before tearing
	// anything down — WITHOUT the registration side effects. composeSwitchSpec
	// (below) writes the per-agent hook settings file and registers a fresh MCP
	// token, both keyed by the unchanged agent_id; doing that here would let the
	// old-artifact cleanup (step 2) wipe the target's just-created registration.
	if ae := s.validateSwitchTarget(target); ae != nil {
		writeAPIError(w, ae)
		return
	}

	// 1. Cancel any in-flight turn and let it settle (streamed events already
	//    persisted), then stop the old runtime (removes running row, status done).
	s.cancelAndWait(r.Context(), id, switchCancelTimeout)
	if err := s.registry.Stop(r.Context(), id); err != nil && !errors.Is(err, runtime.ErrNoHandle) {
		writeAPIError(w, apiError(runtime.CodeInternal, "stop current runtime: "+err.Error()))
		return
	}

	// 2. Clean the OLD runtime's MCP token + hook settings (keyed by agent_id)
	//    BEFORE composing the target spec. Because the agent_id is unchanged,
	//    composeSwitchSpec re-registers a fresh MCP token and rewrites the
	//    per-agent hook settings file under the same id; cleaning afterward would
	//    revoke the new token and delete the settings file the resume needs (and
	//    orphan the old MCP token, whose cleanup closure the new register overwrote).
	s.cleanupMessagingMCP(id)
	s.cleanupHookSettings(id)

	// 3. Compose the target launch spec (registers a fresh MCP token + writes the
	//    per-agent hook settings file for the target identity).
	// Any failure from here on happens AFTER the old runtime was stopped + cleaned,
	// so it must roll back to the previous identity (re-register + re-resume) rather
	// than leave the agent dead with no running row — the same recovery step 5 uses.
	spec, ae := s.composeSwitchSpec(target, resumeID)
	if ae != nil {
		s.rollbackSwitch(r.Context(), w, agent, prev.SessionID, errors.New(ae.Message))
		return
	}
	if usePrimer {
		primer, err := s.buildHistoryPrimer(r.Context(), spec, s.switchPrimerTokenBudget())
		if err != nil {
			s.rollbackSwitch(r.Context(), w, agent, prev.SessionID, fmt.Errorf("build history primer: %w", err))
			return
		}
		// Feed the primer to the new backend for THIS resume only. spec.SystemPrompt
		// stays pristine (the frozen snapshot), so persistence (runtimeMeta /
		// UpsertSessionMeta / session_meta) keeps recording the pre-primer prompt and
		// a later switch primes from that clean base rather than stacking primer on
		// primer. RuntimeSystemPrompt is consumed only by the process-start params.
		spec.RuntimeSystemPrompt = joinSystemPrompt(spec.SystemPrompt, primer)
	}

	// 4. Persist the new identity (agent_id UNCHANGED) so the resume composes the
	//    new interface/backend/model and the card re-renders its badges.
	if err := s.stateStore.WriteAgent(target); err != nil {
		s.rollbackSwitch(r.Context(), w, agent, prev.SessionID, fmt.Errorf("persist identity: %w", err))
		return
	}

	// 5. Resume under the target runtime (registry dispatches by target interface).
	if _, err := s.registry.Resume(r.Context(), spec); err != nil {
		s.rollbackSwitch(r.Context(), w, agent, prev.SessionID, err)
		return
	}
	if usePrimer {
		if err := appendBackendSwitchMarker(s.configStore.Home(), id, agent.Backend+"/"+agent.Model, target.Backend+"/"+target.Model, time.Now().UTC()); err != nil {
			writeAPIError(w, apiError(runtime.CodeInternal, "append backend switch marker: "+err.Error()))
			return
		}
	}

	// The identity write + the resume's running/status writes already published
	// state_update via the runtime's state-touch, so the card re-renders.
	running, _ := s.stateStore.ReadRunning(id)
	writeJSON(w, http.StatusOK, switchRuntimeResponse{
		AgentID: id, Interface: target.Interface, Backend: target.Backend, Model: target.Model,
		Running: ptrRunning(running), HistoryHandoff: handoff,
	})
}

// rollbackSwitch re-launches the previous identity after a failed Resume (§5.4).
// If rollback also fails, it leaves the status row at error and returns 500
// switch_failed.
func (s *Server) rollbackSwitch(ctx context.Context, w http.ResponseWriter, prevAgent state.Agent, prevSessionID string, cause error) {
	s.cleanupMessagingMCP(prevAgent.AgentID)
	s.cleanupHookSettings(prevAgent.AgentID)
	if err := s.stateStore.WriteAgent(prevAgent); err != nil {
		s.failSwitch(w, prevAgent.AgentID, "restore identity: "+err.Error())
		return
	}
	spec, ae := s.composeSwitchSpec(prevAgent, prevSessionID)
	if ae != nil {
		s.failSwitch(w, prevAgent.AgentID, "recompose previous: "+ae.Message)
		return
	}
	if _, err := s.registry.Resume(ctx, spec); err != nil {
		s.failSwitch(w, prevAgent.AgentID, err.Error())
		return
	}
	writeAPIError(w, apiError(runtime.CodeSwitchFailedRolledBack, "switch failed, rolled back to previous runtime: "+cause.Error()))
}

// failSwitch records the unrecoverable state (status error) and returns 500
// switch_failed; the agent is recoverable via archive resume (§5.4).
func (s *Server) failSwitch(w http.ResponseWriter, agentID, detail string) {
	if st, err := s.stateStore.ReadStatus(agentID); err == nil {
		st.State = "error"
		st.Detail = "switch failed: " + clipDetail(detail)
		st.BusySince = nil
		_ = s.stateStore.WriteStatus(st)
		if _, terr := s.stateMgr.Touch(agentID); terr != nil {
			s.log.Debug("touch after failed switch", "agent", agentID, "err", terr)
		}
	}
	writeAPIError(w, apiError(runtime.CodeSwitchFailed, detail))
}

// validateSwitchTarget checks the target identity is launchable — the frozen
// session snapshot exists and the target backend/model are known — without any
// registration side effects, so the switch can reject a bad request before it
// stops the live agent. composeSwitchSpec repeats these lookups (and does the
// registration) once the old artifacts are cleaned.
func (s *Server) validateSwitchTarget(target state.Agent) *runtime.APIError {
	if _, err := s.stateStore.ReadSession(target.AgentID); err != nil {
		if errors.Is(err, state.ErrNotFound) {
			return apiError(runtime.CodeValidation, "no persisted session to switch")
		}
		return apiError(runtime.CodeInternal, err.Error())
	}
	backends, err := s.configStore.ReadBackends()
	if err != nil {
		if errors.Is(err, config.ErrNotFound) || errors.Is(err, config.ErrCorrupt) {
			backends = config.DefaultBackends()
		} else {
			return apiError(runtime.CodeInternal, "read backends: "+err.Error())
		}
	}
	be, ok := backends.Backends[target.Backend]
	if !ok {
		return apiError(runtime.CodeInvalidField, "unknown backend: "+target.Backend)
	}
	if _, ok := be.Models[target.Model]; !ok {
		return apiError(runtime.CodeInvalidField, "unknown model: "+target.Model)
	}
	return nil
}

// composeSwitchSpec builds the resume LaunchSpec for the target identity from the
// frozen session snapshot (cwd/system_prompt) + re-resolved backend/model env,
// minting a fresh hook token and MCP registration (mirrors handleResume).
func (s *Server) composeSwitchSpec(target state.Agent, resumeID string) (runtime.LaunchSpec, *runtime.APIError) {
	snap, err := s.stateStore.ReadSession(target.AgentID)
	if errors.Is(err, state.ErrNotFound) {
		return runtime.LaunchSpec{}, apiError(runtime.CodeValidation, "no persisted session to switch")
	}
	if err != nil {
		return runtime.LaunchSpec{}, apiError(runtime.CodeInternal, err.Error())
	}

	backends, err := s.configStore.ReadBackends()
	if err != nil {
		if errors.Is(err, config.ErrNotFound) || errors.Is(err, config.ErrCorrupt) {
			backends = config.DefaultBackends()
		} else {
			return runtime.LaunchSpec{}, apiError(runtime.CodeInternal, "read backends: "+err.Error())
		}
	}
	be, ok := backends.Backends[target.Backend]
	if !ok {
		return runtime.LaunchSpec{}, apiError(runtime.CodeInvalidField, "unknown backend: "+target.Backend)
	}
	model, ok := be.Models[target.Model]
	if !ok {
		return runtime.LaunchSpec{}, apiError(runtime.CodeInvalidField, "unknown model: "+target.Model)
	}

	token := mintHookToken()
	s.rememberHookToken(target.AgentID, token)
	mcpSpec, err := s.registerMessagingMCP(target)
	if err != nil {
		s.forgetHookToken(target.AgentID)
		return runtime.LaunchSpec{}, apiError(runtime.CodeInternal, err.Error())
	}
	extraArgs, err := s.composeHookRegistration(target, be.Type)
	if err != nil {
		s.forgetHookToken(target.AgentID)
		s.cleanupMessagingMCP(target.AgentID)
		return runtime.LaunchSpec{}, apiError(runtime.CodeInternal, err.Error())
	}

	return runtime.LaunchSpec{
		Agent:          target,
		Cwd:            snap.Cwd,
		AddDirs:        s.resolveAddDirs(target.Project),
		SystemPrompt:   snap.SystemPrompt,
		BackendType:    be.Type,
		ModelID:        model.Model,
		Env:            composeEnv(os.Environ(), be.Env, model.Env, s.hookEnv(target, token)),
		SkipPerms:      s.resolveSkipForRole(target.Role),
		HookToken:      token,
		MCPServers:     []runtime.MCPServerSpec{mcpSpec},
		ExtraArgs:      extraArgs,
		LastSessionID:  resumeID,
		LastContextPct: snap.LastContextPct,
	}, nil
}

// cancelAndWait cancels the in-flight turn and waits up to timeout for the agent
// to leave the busy state (best-effort; the streamed events are already persisted).
func (s *Server) cancelAndWait(ctx context.Context, id string, timeout time.Duration) {
	_, _ = s.registry.Cancel(ctx, id)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		st, err := s.stateStore.ReadStatus(id)
		if err != nil || st.State != "busy" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func (s *Server) backendType(backendID string) string {
	backends, err := s.configStore.ReadBackends()
	if err != nil {
		backends = config.DefaultBackends()
	}
	if be, ok := backends.Backends[backendID]; ok {
		return be.Type
	}
	return ""
}

func (s *Server) switchPrimerTokenBudget() int {
	cfg := s.cfg
	if fromDisk, err := s.configStore.ReadConfig(); err == nil {
		cfg = fromDisk
	}
	if cfg.Switch.PrimerTokenBudget <= 0 {
		return defaultPrimerTokenBudget
	}
	return cfg.Switch.PrimerTokenBudget
}

func (s *Server) acquireSwitch(id string) bool {
	s.switchMu.Lock()
	defer s.switchMu.Unlock()
	if s.switching[id] {
		return false
	}
	s.switching[id] = true
	return true
}

func (s *Server) releaseSwitch(id string) {
	s.switchMu.Lock()
	delete(s.switching, id)
	s.switchMu.Unlock()
}

func ptrRunning(r state.RunningEntry) *state.RunningEntry {
	if r.AgentID == "" {
		return nil
	}
	return &r
}

func clipDetail(s string) string {
	if len(s) <= 120 {
		return s
	}
	return s[:120]
}
