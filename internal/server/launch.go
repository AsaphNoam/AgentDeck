package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
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
	spec, agent, ae := s.composeLaunch(req)
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
func (s *Server) composeLaunch(req launchRequest) (runtime.LaunchSpec, state.Agent, *runtime.APIError) {
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
	addDirs := expandAddDirs(project.AddDirs)

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
		SystemPrompt: joinSystemPrompt(project.ContextPrompt, role.SystemPrompt),
		BackendType:  backend.Type,
		ModelID:      model.Model,
		Driver:       driver,
		Env:          composeEnv(os.Environ(), backend.Env, model.Env, hookEnv),
		SkipPerms:    resolveSkip(s.cfg.SkipPermissions, role.SkipPermissions),
		HookToken:    token,
		MCPServers:   []runtime.MCPServerSpec{mcpSpec},
		ExtraArgs:    extraArgs,
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

// composeHookRegistration writes the per-agent CLI hook settings file (mapping
// each lifecycle event to its installed script via the backend adapter's
// hookMap) and returns the adapter's launch args that point the CLI at it.
//
// The settings file + AGENTDECK_* env are always prepared; whether the CLI-flag
// passthrough (claude's `--settings`, codex TBD) is added depends on interface:
//
//   - terminal: ON by default. The terminal runtime (6.3) runs the *real*
//     interactive CLI directly under a PTY (not claude-code-acp), where
//     `--settings` is a known-good flag, and hooks are the ONLY status producer —
//     so registration must be active for terminal status to flow over /api/hook.
//   - chat: gated behind AGENTDECK_HOOK_REGISTRATION=1 (default off). Chat runs
//     through claude-code-acp (whose `--settings` forwarding is unverified) AND
//     does not need registration: the runtime owns chat status and the `_post.sh`
//     interface gate self-suppresses redundant POSTs. Keeping it off avoids
//     regressing the currently-green real chat-launch path with an unconfirmed flag.
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
	if agent.Interface != "terminal" && os.Getenv("AGENTDECK_HOOK_REGISTRATION") != "1" {
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

// joinSystemPrompt joins project context then role persona, skipping empties so
// there are no leading/trailing blank lines (techspec §6.2).
func joinSystemPrompt(contextPrompt, systemPrompt string) string {
	parts := make([]string, 0, 2)
	for _, p := range []string{contextPrompt, systemPrompt} {
		if strings.TrimSpace(p) != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, "\n\n")
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
