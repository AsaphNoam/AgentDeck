package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/agentdeck/agentdeck/internal/backend"
	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/hooks"
	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
)

// launchRequest is the POST /api/sessions body (techspec §7.1). backend/model/
// interface are optional and default per §6.5.
type launchRequest struct {
	Role      string `json:"role"`
	Project   string `json:"project"`
	Backend   string `json:"backend"`
	Model     string `json:"model"`
	Interface string `json:"interface"`
	Driver    string `json:"driver"` // terminal driver: ""/"xterm" | "tmux" | "iterm2" (§3.5)
	Name      string `json:"name"`
	Group     string `json:"group"`
}

// sessionResponse is the {agent, running, status} envelope (techspec §7.1/§7.2).
type sessionResponse struct {
	Agent   state.Agent         `json:"agent"`
	Running *state.RunningEntry `json:"running,omitempty"`
	Status  *state.Status       `json:"status,omitempty"`
}

// handleLaunch implements POST /api/sessions (techspec §6.1, §7.1).
func (s *Server) handleLaunch(w http.ResponseWriter, r *http.Request) {
	var req launchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apiError(runtime.CodeValidation, "invalid JSON body"))
		return
	}

	// Resolve config + defaults; validate (§6.1 step 1, §6.5).
	spec, agent, ae := s.composeLaunch(r.Context(), req)
	if ae != nil {
		writeAPIError(w, ae)
		return
	}

	// Insert identity row before Start so a crash mid-handshake still has a
	// stable id; roll back if Start fails outright (§6.1 step 4, step 8).
	if err := s.stateStore.WriteAgent(agent); err != nil {
		// composeLaunch already remembered the hook token, registered the MCP
		// session, and wrote the hook-settings file — tear them all down so a
		// WriteAgent failure doesn't leak a spoofable messaging identity + files.
		s.teardownAgentRegistration(agent.AgentID)
		writeAPIError(w, apiError(runtime.CodeInternal, "write identity: "+err.Error()))
		return
	}

	if _, err := s.registry.Launch(r.Context(), spec); err != nil {
		// Roll back identity + any partial rows + all registration artifacts.
		_ = s.stateStore.DeleteRunning(agent.AgentID)
		_ = s.stateStore.DeleteStatus(agent.AgentID)
		_ = s.stateStore.DeleteAgent(agent.AgentID)
		s.teardownAgentRegistration(agent.AgentID)
		writeAPIError(w, launchStartError(err))
		return
	}

	// Runtime inserted running + status rows during Start.
	resp := s.readSession(agent.AgentID)
	writeJSON(w, http.StatusCreated, resp)
}

// handleSessionDetail implements GET /api/sessions/{id} (techspec §7.2).
func (s *Server) handleSessionDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.stateStore.ReadAgent(id); err != nil {
		writeAPIError(w, apiError(runtime.CodeNotFound, "no such agent: "+id))
		return
	}
	writeJSON(w, http.StatusOK, s.readSession(id))
}

// readSession assembles the {agent, running, status} envelope from state.db.
func (s *Server) readSession(id string) sessionResponse {
	resp := sessionResponse{}
	if a, err := s.stateStore.ReadAgent(id); err == nil {
		resp.Agent = a
	}
	if r, err := s.stateStore.ReadRunning(id); err == nil {
		resp.Running = &r
	}
	if st, err := s.stateStore.ReadStatus(id); err == nil {
		resp.Status = &st
	}
	return resp
}

// composeLaunch resolves config + defaults, validates, and builds the LaunchSpec
// and identity Agent (techspec §6.2). On validation failure it returns an APIError.
func (s *Server) composeLaunch(ctx context.Context, req launchRequest) (runtime.LaunchSpec, state.Agent, *runtime.APIError) {
	if req.Role == "" || req.Project == "" {
		return runtime.LaunchSpec{}, state.Agent{}, apiError(runtime.CodeValidation, "role and project are required")
	}

	role, err := s.configStore.ReadRole(req.Role)
	if err != nil {
		return runtime.LaunchSpec{}, state.Agent{}, apiError(runtime.CodeValidation, "unknown role: "+req.Role)
	}
	project, err := s.configStore.ReadProject(req.Project)
	if err != nil {
		return runtime.LaunchSpec{}, state.Agent{}, apiError(runtime.CodeValidation, "unknown project: "+req.Project)
	}

	backends, err := s.configStore.ReadBackends()
	if err != nil {
		if errors.Is(err, config.ErrNotFound) || errors.Is(err, config.ErrCorrupt) {
			backends = config.DefaultBackends()
		} else {
			return runtime.LaunchSpec{}, state.Agent{}, apiError(runtime.CodeInternal, "read backends: "+err.Error())
		}
	}

	// Defaults (§6.5): backend → the default backend; model → that backend's
	// default_model; interface → chat.
	backendID := req.Backend
	if backendID == "" {
		backendID = defaultBackendID(backends)
	}
	backend, ok := backends.Backends[backendID]
	if !ok {
		return runtime.LaunchSpec{}, state.Agent{}, apiError(runtime.CodeValidation, "unknown backend: "+backendID)
	}
	modelID := req.Model
	if modelID == "" {
		modelID = backend.DefaultModel
	}
	model, ok := backend.Models[modelID]
	if !ok {
		return runtime.LaunchSpec{}, state.Agent{}, apiError(runtime.CodeValidation, "unknown model: "+modelID)
	}
	iface := req.Interface
	if iface == "" {
		iface = "chat"
	}
	if iface != "chat" && iface != "terminal" {
		return runtime.LaunchSpec{}, state.Agent{}, apiError(runtime.CodeValidation, "invalid interface: "+iface)
	}
	// Only claude-acp has a verified interactive-CLI hook path; codex/opencode/
	// openhands terminal launches would be statusless and silently drop the
	// composed spec, so reject them rather than land them (§6 capability honesty).
	// Claude terminal is honored in the terminal runtime.
	if iface == "terminal" && !terminalSupported(backend.Type) {
		return runtime.LaunchSpec{}, state.Agent{}, apiError(runtime.CodeTerminalUnavailable, terminalUnsupportedReason(backend.Type))
	}
	// An explicitly-requested terminal driver must be available on this host; an
	// unavailable optional driver (e.g. iterm2 off macOS) is a 422 with a reason so
	// the UI can disable it (§3.5). Chat launches ignore the driver field.
	driver := req.Driver
	if iface == "terminal" {
		if ae := validateTerminalDriver(driver); ae != nil {
			return runtime.LaunchSpec{}, state.Agent{}, ae
		}
	}

	cwd, err := config.ExpandTilde(project.Cwd)
	if err != nil {
		return runtime.LaunchSpec{}, state.Agent{}, apiError(runtime.CodeValidation, "bad project cwd: "+err.Error())
	}
	// Pre-check the resolved cwd exists. Without this the process launch fails
	// deep in the runtime with a fork/exec ENOENT that names the adapter binary,
	// not the missing directory, so the user cannot self-diagnose (e.g. the
	// shipped my-app project points at ~/Projects/my-app, absent on a fresh box).
	if info, statErr := os.Stat(cwd); statErr != nil || !info.IsDir() {
		return runtime.LaunchSpec{}, state.Agent{}, apiError(runtime.CodeValidation,
			fmt.Sprintf("project directory %q does not exist — set project %q to an existing path", cwd, req.Project))
	}
	addDirs := expandAddDirs(project.AddDirs)

	// Ensure the project's shared-resources directory before any registration side
	// effect so an unusable path fails the launch cleanly (FS-11.R6/R9). Its path is
	// folded into the frozen add_dirs, prompt, and env below so resume/switch
	// reproduce it from the snapshot (INV §2).
	resourceDir, ae := s.ensureProjectResources(req.Project)
	if ae != nil {
		return runtime.LaunchSpec{}, state.Agent{}, ae
	}
	addDirs = appendResourceDir(addDirs, resourceDir)

	// Configuration federation (Phase 7 §2.5): when this backend has an active
	// source binding, resolve it FRESH at launch — the correctness boundary. A
	// stale/invalid/unapproved source blocks the launch (never composed from
	// cache). The resolved high-level view is frozen into the session snapshot as
	// redacted provenance. Placed before any registration side effects so a source
	// error returns cleanly with nothing to unwind.
	launchConfig, fedModel, ae := s.composeFederation(ctx, backendID, req, backend, project, modelID)
	if ae != nil {
		return runtime.LaunchSpec{}, state.Agent{}, ae
	}
	// Compose the effective model sent over ACP (§2.4). An explicit launch model
	// always wins (keeps model.Model, below). For a bound source with no explicit
	// choice: a source override is applied verbatim; otherwise the model is left to
	// native inheritance ("" → omitted over ACP). agent.Model keeps the resolved
	// backend model id as the display/search projection (§2.5).
	acpModelID := model.Model
	if fedModel != nil && req.Model == "" {
		switch {
		case fedModel.override != nil:
			acpModelID = *fedModel.override
		case fedModel.inherit:
			acpModelID = ""
		}
	}

	agentID, err := s.stateStore.NewAgentID()
	if err != nil {
		return runtime.LaunchSpec{}, state.Agent{}, apiError(runtime.CodeInternal, "mint agent id: "+err.Error())
	}
	name := req.Name
	if name == "" {
		name = s.suggestName()
	}

	agent := state.Agent{
		AgentID: agentID, Name: name, Role: req.Role, Project: req.Project,
		Backend: backendID, Model: modelID, Interface: iface,
		CreatedAt: time.Now().UTC(), Group: req.Group,
	}

	token := mintHookToken()
	s.rememberHookToken(agentID, token)
	mcpSpec, err := s.registerMessagingMCP(agent)
	if err != nil {
		s.forgetHookToken(agentID)
		return runtime.LaunchSpec{}, state.Agent{}, apiError(runtime.CodeInternal, err.Error())
	}

	hookEnv := s.hookEnv(agent, token)
	extraArgs, err := s.composeHookRegistration(agent, backend.Type)
	if err != nil {
		s.forgetHookToken(agentID)
		s.cleanupMessagingMCP(agentID)
		return runtime.LaunchSpec{}, state.Agent{}, apiError(runtime.CodeInternal, err.Error())
	}

	spec := runtime.LaunchSpec{
		Agent:        agent,
		Cwd:          cwd,
		AddDirs:      addDirs,
		SystemPrompt: joinSystemPrompt(project.ContextPrompt, role.SystemPrompt, projectResourcesInstruction(resourceDir)),
		BackendType:  backend.Type,
		ModelID:      acpModelID,
		Driver:       driver,
		Env:          composeEnv(os.Environ(), backend.Env, model.Env, hookEnv, projectResourcesEnv(resourceDir)),
		SkipPerms:    resolveSkip(s.cfg.SkipPermissions, role.SkipPermissions),
		HookToken:    token,
		MCPServers:   []runtime.MCPServerSpec{mcpSpec},
		ExtraArgs:    extraArgs,
		LaunchConfig: launchConfig,
	}
	return spec, agent, nil
}

// hookEnv builds the per-launch AGENTDECK_* env the hook scripts read (§2.3,
// §4.1): the POST endpoint, the rotated per-launch token, the agent id, and the
// interface (which drives the chat self-suppression gate in _post.sh).
func (s *Server) hookEnv(agent state.Agent, token string) map[string]string {
	return map[string]string{
		"AGENTDECK_HOOK_URL":   fmt.Sprintf("http://127.0.0.1:%d/api/hook", s.cfg.Port),
		"AGENTDECK_HOOK_TOKEN": token,
		"AGENTDECK_AGENT_ID":   agent.AgentID,
		"AGENTDECK_INTERFACE":  agent.Interface,
	}
}

// composeHookRegistration writes the per-agent CLI hook settings artifact that
// launch/stop/crash cleanup owns symmetrically. For terminal it also returns the
// direct interactive CLI's --settings args. Chat runs through claude-agent-acp,
// which does not accept a per-launch --settings file and already provides status
// through ACP, so chat keeps the lifecycle artifact but receives no hook args.
func (s *Server) composeHookRegistration(agent state.Agent, backendType string) ([]string, error) {
	ad, ok := backend.For(backendType)
	if !ok {
		return nil, nil // unknown backend fails later at the runtime gate
	}
	settings := hooks.ClaudeSettings(s.configStore.Home(), ad.HookMap())
	settingsPath, err := hooks.WriteAgentSettings(s.configStore.Home(), agent.AgentID, settings)
	if err != nil {
		return nil, err
	}
	if agent.Interface != "terminal" {
		return nil, nil
	}
	return ad.HookLaunchArgs(settingsPath), nil
}

// defaultBackendID returns the backend marked Default, else "claude", else any.
func defaultBackendID(b config.BackendsConfig) string {
	for id, be := range b.Backends {
		if be.Default {
			return id
		}
	}
	if _, ok := b.Backends["claude"]; ok {
		return "claude"
	}
	for id := range b.Backends {
		return id
	}
	return ""
}

// joinSystemPrompt joins the given prompt segments in order (project context,
// role persona, then the project-resources instruction), skipping empties so
// there are no leading/trailing blank lines (techspec §6.2).
func joinSystemPrompt(segments ...string) string {
	parts := make([]string, 0, len(segments))
	for _, p := range segments {
		if strings.TrimSpace(p) != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, "\n\n")
}

// envProjectResources is the env var carrying the project's shared-resources
// directory to every launched agent (FS-11.R3).
const envProjectResources = "AGENTDECK_PROJECT_RESOURCES"

// ensureProjectResources ensures the project's AgentDeck-owned shared-resources
// directory exists and is usable, returning its absolute path. Launch, resume,
// and switch all call it before any registration side effect so an unusable path
// fails the operation with nothing to unwind (FS-11.R6/R9, INV §2). The immutable
// project id keeps the path identical across the whole lifecycle, so resume/switch
// inherit the same add_dirs/prompt from the frozen snapshot and only re-add the
// env var (env is recomposed each launch).
func (s *Server) ensureProjectResources(projectID string) (string, *runtime.APIError) {
	path, err := s.configStore.EnsureProjectResources(projectID)
	if err != nil {
		return "", apiError(runtime.CodeValidation, "project resources unavailable: "+err.Error())
	}
	return path, nil
}

// projectResourceDir returns the canonical resource-directory path for read-only
// response metadata (TS-03.R12) without creating anything; a malformed id yields "".
func (s *Server) projectResourceDir(id string) string {
	p, err := s.configStore.ProjectResourcesPath(id)
	if err != nil {
		return ""
	}
	return p
}

// projectResourcesEnv is the single env layer carrying the resource path; shared
// by launch/resume/switch so they never rebuild it independently (INV §2).
func projectResourcesEnv(path string) map[string]string {
	return map[string]string{envProjectResources: path}
}

// projectResourcesInstruction is the composed launch instruction telling the agent
// the directory is the project's shared place for agent-created material, outside
// the repository, readable and writable by project agents (FS-11.R3).
func projectResourcesInstruction(path string) string {
	return "Shared project resources: " + path + "\n" +
		"This AgentDeck-owned directory is the project's shared place for agent-created " +
		"material (specs, guides, research, test harnesses, results). It lives outside the " +
		"project repository, so nothing written there can become an accidental commit. You " +
		"and other agents on this project may read and write it freely; your working " +
		"directory stays the project checkout."
}

// appendResourceDir appends the resource path to add_dirs exactly once (FS-11.R3).
func appendResourceDir(dirs []string, resourceDir string) []string {
	for _, d := range dirs {
		if d == resourceDir {
			return dirs
		}
	}
	return append(dirs, resourceDir)
}

// resolveSkip computes the effective skip_permissions: role override if set, else
// the global config value (techspec §12.2).
func resolveSkip(global bool, roleSkip *bool) bool {
	if roleSkip != nil {
		return *roleSkip
	}
	return global
}

// expandAddDirs ~-expands each add_dir, dropping any that fail to expand.
func expandAddDirs(raw []string) []string {
	dirs := make([]string, 0, len(raw))
	for _, d := range raw {
		if ex, err := config.ExpandTilde(d); err == nil {
			dirs = append(dirs, ex)
		}
	}
	return dirs
}

// composeEnv layers env: process env, then backend env, then per-model env (later
// wins on key collision), returning a deduped []string of "K=V" (techspec §6.2).
func composeEnv(base []string, layers ...map[string]string) []string {
	merged := map[string]string{}
	order := []string{}
	add := func(k, v string) {
		if _, seen := merged[k]; !seen {
			order = append(order, k)
		}
		merged[k] = v
	}
	for _, kv := range base {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			add(kv[:i], kv[i+1:])
		}
	}
	for _, layer := range layers {
		for k, v := range layer {
			add(k, v)
		}
	}
	out := make([]string, 0, len(order))
	for _, k := range order {
		out = append(out, k+"="+merged[k])
	}
	return out
}

// mintHookToken mints a per-launch one-time token (techspec §6.4).
func mintHookToken() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// suggestName picks the first wordlist name not used by a live agent (techspec §6.3).
func (s *Server) suggestName() string {
	used := map[string]bool{}
	if running, err := s.stateStore.ListRunning(); err == nil {
		for _, r := range running {
			if a, err := s.stateStore.ReadAgent(r.AgentID); err == nil {
				used[a.Name] = true
			}
		}
	}
	for _, n := range nameWords {
		if !used[n] {
			return n
		}
	}
	// All used: append a numeric suffix to keep it deterministic.
	for i := 2; ; i++ {
		cand := nameWords[0] + "-" + itoa(i)
		if !used[cand] {
			return cand
		}
	}
}

// nameWords is the curated auto-suggest wordlist (techspec §6.3).
var nameWords = []string{
	"Atlas", "Nova", "Echo", "Orion", "Vega", "Lyra", "Sol", "Iris",
	"Juno", "Rhea", "Titan", "Ceres", "Pax", "Aria", "Onyx", "Flint",
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(b[pos:])
}

// launchStartError maps a runtime Start failure to the right APIError (techspec §7.1).
func launchStartError(err error) *runtime.APIError {
	switch {
	case errors.Is(err, runtime.ErrNotImplemented):
		return apiError(runtime.CodeNotImplemented, err.Error())
	case errors.Is(err, runtime.ErrAlreadyStarted):
		return apiError(runtime.CodeConflict, err.Error())
	default:
		return apiError(runtime.CodeRuntimeStartFailed, err.Error())
	}
}

func (s *Server) rememberHookToken(agentID, token string) {
	s.hookMu.Lock()
	s.hookTokens[agentID] = token
	s.hookMu.Unlock()
}

func (s *Server) forgetHookToken(agentID string) {
	s.hookMu.Lock()
	delete(s.hookTokens, agentID)
	s.hookMu.Unlock()
}

// teardownAgentRegistration removes every per-agent server-side registration
// artifact — the in-memory hook token, the messaging MCP session + on-disk
// mcp/{id}.mcp.json, and the per-agent hooks settings file. It is the single
// teardown unit invoked from every agent exit: solicited stop AND the runtime
// crash path (wired as the Registry exit hook in server.New). Without the crash
// path, a crashed agent left a live hook token + MCP session behind — a spoofable
// messaging identity that an orphaned child/hook could still send/check as — plus
// leaked files that grow per crash.
func (s *Server) teardownAgentRegistration(agentID string) {
	s.forgetHookToken(agentID)
	s.cleanupMessagingMCP(agentID)
	s.cleanupHookSettings(agentID)
}

// cleanupHookSettings deletes the per-agent hook settings file on agent
// teardown, mirroring cleanupMessagingMCP so the two registration artifacts
// share a lifecycle (review fix: the settings file is no longer orphaned).
func (s *Server) cleanupHookSettings(agentID string) {
	if err := hooks.RemoveAgentSettings(s.configStore.Home(), agentID); err != nil {
		s.log.Warn("cleanup hook settings", "agent_id", agentID, "err", err)
	}
}

// reapOrphanRuntime checks if there is a live orphan running row (a process that
// survived a dashboard crash), kills it if so, and deletes the running row.
// Called when registry.Stop returns ErrNoHandle (no handler in the registry),
// which means the runtime exited and deregistered, but the running row persists.
// If the PID is still alive, it's an orphan left by a prior crash.
func (s *Server) reapOrphanRuntime(agentID string) error {
	running, err := s.stateStore.ReadRunning(agentID)
	if err != nil {
		// No running row: already cleaned up or never existed.
		return nil
	}

	// Check if the PID is actually still alive.
	if runtime.PidAlive(running.PID) {
		// Kill it with SIGKILL to ensure it dies (SIGTERM can be ignored).
		s.log.Info("reaping orphan runtime", "agent_id", agentID, "pid", running.PID)
		_ = syscall.Kill(running.PID, syscall.SIGKILL)
	}

	// Delete the running row.
	if err := s.stateStore.DeleteRunning(agentID); err != nil {
		s.log.Warn("delete running row", "agent_id", agentID, "err", err)
		return err
	}
	return nil
}
