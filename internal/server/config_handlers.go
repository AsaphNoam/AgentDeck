package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/agentdeck/agentdeck/internal/backend/credcheck"
	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/state"
)

// validationFailedBody is the §5.6 error envelope for 400 validation failures.
type validationFailedBody struct {
	Error  string              `json:"error"`
	Errors []config.FieldError `json:"errors"`
}

func writeValidationError(w http.ResponseWriter, ve *config.ValidationErrors) {
	writeJSON(w, http.StatusBadRequest, validationFailedBody{
		Error:  "validation_failed",
		Errors: ve.Errors,
	})
}

// inUseBody is the §5.1/§5.2 409 in-use envelope.
type inUseBody struct {
	Error   string   `json:"error"`
	Message string   `json:"message"`
	Agents  []string `json:"agents"`
	Hint    string   `json:"hint"`
}

// runningAgentsForRole returns agent IDs of running agents that reference the given role.
func (s *Server) runningAgentsForRole(role string) ([]string, error) {
	return s.runningAgentsMatching(func(a state.Agent) bool { return a.Role == role })
}

// runningAgentsForProject returns agent IDs of running agents that reference the given project.
func (s *Server) runningAgentsForProject(project string) ([]string, error) {
	return s.runningAgentsMatching(func(a state.Agent) bool { return a.Project == project })
}

func (s *Server) runningAgentsMatching(match func(state.Agent) bool) ([]string, error) {
	running, err := s.stateStore.ListRunning()
	if err != nil {
		return nil, err
	}
	if len(running) == 0 {
		return nil, nil
	}
	agents, err := s.stateStore.ListAgents()
	if err != nil {
		return nil, err
	}
	byID := make(map[string]state.Agent, len(agents))
	for _, a := range agents {
		byID[a.AgentID] = a
	}
	var ids []string
	for _, r := range running {
		if a, ok := byID[r.AgentID]; ok && match(a) {
			ids = append(ids, r.AgentID)
		}
	}
	return ids, nil
}

// ---- Role request/response bodies ----

type roleCreateBody struct {
	RoleID          string `json:"role"`
	Title           string `json:"title"`
	SystemPrompt    string `json:"system_prompt"`
	SkipPermissions *bool  `json:"skip_permissions"`
}

func (b roleCreateBody) toRole() config.Role {
	return config.Role{
		Title:           b.Title,
		SystemPrompt:    b.SystemPrompt,
		SkipPermissions: b.SkipPermissions,
	}
}

type rolePutBody struct {
	RoleID          *string `json:"role"` // optional; must match path if present
	Title           string  `json:"title"`
	SystemPrompt    string  `json:"system_prompt"`
	SkipPermissions *bool   `json:"skip_permissions"`
}

func (b rolePutBody) toRole() config.Role {
	return config.Role{
		Title:           b.Title,
		SystemPrompt:    b.SystemPrompt,
		SkipPermissions: b.SkipPermissions,
	}
}

type roleResponse struct {
	RoleID          string `json:"role"`
	Title           string `json:"title"`
	SystemPrompt    string `json:"system_prompt"`
	SkipPermissions *bool  `json:"skip_permissions"`
}

func toRoleResponse(id string, r config.Role) roleResponse {
	return roleResponse{
		RoleID:          id,
		Title:           r.Title,
		SystemPrompt:    r.SystemPrompt,
		SkipPermissions: r.SkipPermissions,
	}
}

// handlePostRole implements POST /api/roles (§5.1).
func (s *Server) handlePostRole(w http.ResponseWriter, r *http.Request) {
	var body roleCreateBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeValidationError(w, &config.ValidationErrors{Errors: []config.FieldError{
			{Field: "", Code: "bad_request", Message: "malformed JSON"},
		}})
		return
	}
	role := body.toRole()
	if ve := config.ValidateRole(body.RoleID, role, true); ve != nil {
		writeValidationError(w, ve)
		return
	}
	// 409 if already exists.
	if _, err := s.configStore.ReadRole(body.RoleID); err == nil {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":   "already_exists",
			"message": fmt.Sprintf("role '%s' exists", body.RoleID),
		})
		return
	}
	if err := s.configStore.WriteRole(body.RoleID, role); err != nil {
		s.log.Error("roles: write", "err", err)
		writeAPIError(w, apiError("internal", "internal error"))
		return
	}
	writeJSON(w, http.StatusCreated, toRoleResponse(body.RoleID, role))
}

// handlePutRole implements PUT /api/roles/{role} (§5.1).
func (s *Server) handlePutRole(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("role")
	if !config.ValidSlug(id) {
		writeValidationError(w, &config.ValidationErrors{Errors: []config.FieldError{
			{Field: "role", Code: "invalid_slug", Message: "must match ^[a-z0-9][a-z0-9-]{0,62}$"},
		}})
		return
	}
	var body rolePutBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeValidationError(w, &config.ValidationErrors{Errors: []config.FieldError{
			{Field: "", Code: "bad_request", Message: "malformed JSON"},
		}})
		return
	}
	if body.RoleID != nil && *body.RoleID != id {
		writeValidationError(w, &config.ValidationErrors{Errors: []config.FieldError{
			{Field: "role", Code: "mismatch", Message: "role in body must match path"},
		}})
		return
	}
	role := body.toRole()
	if ve := config.ValidateRole(id, role, false); ve != nil {
		writeValidationError(w, ve)
		return
	}
	// 404 if absent.
	if _, err := s.configStore.ReadRole(id); err != nil {
		if errors.Is(err, config.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
			return
		}
		s.log.Error("roles: read", "err", err)
		writeAPIError(w, apiError("internal", "internal error"))
		return
	}
	if err := s.configStore.WriteRole(id, role); err != nil {
		s.log.Error("roles: write", "err", err)
		writeAPIError(w, apiError("internal", "internal error"))
		return
	}
	writeJSON(w, http.StatusOK, toRoleResponse(id, role))
}

// handleDeleteRole implements DELETE /api/roles/{role} (§5.1).
func (s *Server) handleDeleteRole(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("role")
	if !config.ValidSlug(id) {
		writeValidationError(w, &config.ValidationErrors{Errors: []config.FieldError{
			{Field: "role", Code: "invalid_slug", Message: "must match ^[a-z0-9][a-z0-9-]{0,62}$"},
		}})
		return
	}
	force := r.URL.Query().Get("force") == "true"
	// 404 if absent.
	if _, err := s.configStore.ReadRole(id); err != nil {
		if errors.Is(err, config.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
			return
		}
		s.log.Error("roles: read", "err", err)
		writeAPIError(w, apiError("internal", "internal error"))
		return
	}
	if !force {
		agents, err := s.runningAgentsForRole(id)
		if err != nil {
			s.log.Error("roles: in-use check", "err", err)
			writeAPIError(w, apiError("internal", "internal error"))
			return
		}
		if len(agents) > 0 {
			writeJSON(w, http.StatusConflict, inUseBody{
				Error:   "in_use",
				Message: fmt.Sprintf("role '%s' is used by %d running agent(s)", id, len(agents)),
				Agents:  agents,
				Hint:    "retry with ?force=true to delete the definition; running agents are unaffected",
			})
			return
		}
	}
	if err := s.configStore.DeleteRole(id); err != nil {
		s.log.Error("roles: delete", "err", err)
		writeAPIError(w, apiError("internal", "internal error"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Project request/response bodies ----

type projectCreateBody struct {
	ProjectID     string   `json:"project"`
	Title         string   `json:"title"`
	Color         [3]int   `json:"color"`
	Cwd           string   `json:"cwd"`
	AddDirs       []string `json:"add_dirs"`
	ContextPrompt string   `json:"context_prompt"`
}

func (b projectCreateBody) toProject() config.Project {
	color := b.Color
	if color == [3]int{0, 0, 0} {
		// Spec: default [128,128,128] if omitted. A zero value may mean omitted.
		// We keep user-supplied zeros because the user may intentionally want black;
		// the UI should send [128,128,128] explicitly when omitted.
		color = b.Color
	}
	addDirs := b.AddDirs
	if addDirs == nil {
		addDirs = []string{}
	}
	return config.Project{
		Title:         b.Title,
		Color:         color,
		Cwd:           b.Cwd,
		AddDirs:       addDirs,
		ContextPrompt: b.ContextPrompt,
	}
}

type projectPutBody struct {
	ProjectID     *string  `json:"project"` // optional; must match path if present
	Title         string   `json:"title"`
	Color         [3]int   `json:"color"`
	Cwd           string   `json:"cwd"`
	AddDirs       []string `json:"add_dirs"`
	ContextPrompt string   `json:"context_prompt"`
}

func (b projectPutBody) toProject() config.Project {
	addDirs := b.AddDirs
	if addDirs == nil {
		addDirs = []string{}
	}
	return config.Project{
		Title:         b.Title,
		Color:         b.Color,
		Cwd:           b.Cwd,
		AddDirs:       addDirs,
		ContextPrompt: b.ContextPrompt,
	}
}

type projectResponse struct {
	ProjectID     string              `json:"project"`
	Title         string              `json:"title"`
	Color         [3]int              `json:"color"`
	Cwd           string              `json:"cwd"`
	AddDirs       []string            `json:"add_dirs"`
	ContextPrompt string              `json:"context_prompt"`
	Warnings      []config.FieldError `json:"warnings,omitempty"`
}

func toProjectResponse(id string, p config.Project, warnings []config.FieldError) projectResponse {
	addDirs := p.AddDirs
	if addDirs == nil {
		addDirs = []string{}
	}
	return projectResponse{
		ProjectID:     id,
		Title:         p.Title,
		Color:         p.Color,
		Cwd:           p.Cwd,
		AddDirs:       addDirs,
		ContextPrompt: p.ContextPrompt,
		Warnings:      warnings,
	}
}

// handlePostProject implements POST /api/projects (§5.2).
func (s *Server) handlePostProject(w http.ResponseWriter, r *http.Request) {
	var body projectCreateBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeValidationError(w, &config.ValidationErrors{Errors: []config.FieldError{
			{Field: "", Code: "bad_request", Message: "malformed JSON"},
		}})
		return
	}
	proj := body.toProject()
	ve, warnings := config.ValidateProject(body.ProjectID, proj, true)
	if ve != nil {
		writeValidationError(w, ve)
		return
	}
	// 409 if already exists.
	if _, err := s.configStore.ReadProject(body.ProjectID); err == nil {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":   "already_exists",
			"message": fmt.Sprintf("project '%s' exists", body.ProjectID),
		})
		return
	}
	if err := s.configStore.WriteProject(body.ProjectID, proj); err != nil {
		s.log.Error("projects: write", "err", err)
		writeAPIError(w, apiError("internal", "internal error"))
		return
	}
	writeJSON(w, http.StatusCreated, toProjectResponse(body.ProjectID, proj, warnings))
}

// handlePutProject implements PUT /api/projects/{p} (§5.2).
func (s *Server) handlePutProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("project")
	if !config.ValidSlug(id) {
		writeValidationError(w, &config.ValidationErrors{Errors: []config.FieldError{
			{Field: "project", Code: "invalid_slug", Message: "must match ^[a-z0-9][a-z0-9-]{0,62}$"},
		}})
		return
	}
	var body projectPutBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeValidationError(w, &config.ValidationErrors{Errors: []config.FieldError{
			{Field: "", Code: "bad_request", Message: "malformed JSON"},
		}})
		return
	}
	if body.ProjectID != nil && *body.ProjectID != id {
		writeValidationError(w, &config.ValidationErrors{Errors: []config.FieldError{
			{Field: "project", Code: "mismatch", Message: "project in body must match path"},
		}})
		return
	}
	proj := body.toProject()
	ve, warnings := config.ValidateProject(id, proj, false)
	if ve != nil {
		writeValidationError(w, ve)
		return
	}
	// 404 if absent.
	if _, err := s.configStore.ReadProject(id); err != nil {
		if errors.Is(err, config.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
			return
		}
		s.log.Error("projects: read", "err", err)
		writeAPIError(w, apiError("internal", "internal error"))
		return
	}
	if err := s.configStore.WriteProject(id, proj); err != nil {
		s.log.Error("projects: write", "err", err)
		writeAPIError(w, apiError("internal", "internal error"))
		return
	}
	writeJSON(w, http.StatusOK, toProjectResponse(id, proj, warnings))
}

// handleDeleteProject implements DELETE /api/projects/{p} (§5.2).
func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("project")
	if !config.ValidSlug(id) {
		writeValidationError(w, &config.ValidationErrors{Errors: []config.FieldError{
			{Field: "project", Code: "invalid_slug", Message: "must match ^[a-z0-9][a-z0-9-]{0,62}$"},
		}})
		return
	}
	force := r.URL.Query().Get("force") == "true"
	// 404 if absent.
	if _, err := s.configStore.ReadProject(id); err != nil {
		if errors.Is(err, config.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
			return
		}
		s.log.Error("projects: read", "err", err)
		writeAPIError(w, apiError("internal", "internal error"))
		return
	}
	if !force {
		agents, err := s.runningAgentsForProject(id)
		if err != nil {
			s.log.Error("projects: in-use check", "err", err)
			writeAPIError(w, apiError("internal", "internal error"))
			return
		}
		if len(agents) > 0 {
			writeJSON(w, http.StatusConflict, inUseBody{
				Error:   "in_use",
				Message: fmt.Sprintf("project '%s' is used by %d running agent(s)", id, len(agents)),
				Agents:  agents,
				Hint:    "retry with ?force=true to delete the definition; running agents are unaffected",
			})
			return
		}
	}
	if err := s.configStore.DeleteProject(id); err != nil {
		s.log.Error("projects: delete", "err", err)
		writeAPIError(w, apiError("internal", "internal error"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Backends PUT handler ----

// backendsResponse is the §5.3 200 body: normalized doc + cred results.
type backendsResponse struct {
	config.BackendsConfig
	Credentials map[string]credcheck.CredResult `json:"credentials"`
}

// handlePutBackends implements PUT /api/backends (§5.3).
func (s *Server) handlePutBackends(w http.ResponseWriter, r *http.Request) {
	var body config.BackendsConfig
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeValidationError(w, &config.ValidationErrors{Errors: []config.FieldError{
			{Field: "", Code: "bad_request", Message: "malformed JSON"},
		}})
		return
	}

	// Validate + auto-promote defaults (mutates body in place for normalized form).
	if ve := config.ValidateBackendsConfig(&body); ve != nil {
		writeValidationError(w, ve)
		return
	}

	// Persist the normalized document (save regardless of cred-check outcome).
	if err := s.configStore.WriteBackends(body); err != nil {
		s.log.Error("backends: write", "err", err)
		writeAPIError(w, apiError("internal", "internal error"))
		return
	}

	// Invalidate the onboarding cred-check cache: env/model contents may have changed.
	s.onboardingCacheMu.Lock()
	s.onboardingCache = nil
	s.onboardingCacheMu.Unlock()

	// Run cred checks for the default model of each backend (best-effort, bounded).
	credentials := make(map[string]credcheck.CredResult, len(body.Backends))
	for id, bk := range body.Backends {
		model, ok := bk.Models[bk.DefaultModel]
		if !ok {
			credentials[id] = credcheck.CredResult{Status: "skipped", Detail: "no_default_model"}
			continue
		}
		merged := credcheck.MergeEnv(bk.Env, model.Env)
		credentials[id] = s.credCheck(context.Background(), bk, model, merged)
	}

	writeJSON(w, http.StatusOK, backendsResponse{
		BackendsConfig: body,
		Credentials:    credentials,
	})
}

// ---- Config GET/PUT handlers + onboarding gate ----

// onboardingStep is the per-step status in the GET /api/config onboarding block.
type onboardingStep struct {
	Done   bool   `json:"done"`
	Detail string `json:"detail,omitempty"`
}

// onboardingBlock is the computed onboarding status returned by GET /api/config.
type onboardingBlock struct {
	Satisfied bool `json:"satisfied"`
	Steps     struct {
		Backend onboardingStep `json:"backend"`
		Project onboardingStep `json:"project"`
		Role    onboardingStep `json:"role"`
	} `json:"steps"`
}

// configResponse is the GET /api/config body (config fields + onboarding block).
type configResponse struct {
	config.Config
	Onboarding onboardingBlock `json:"onboarding"`
}

// computeOnboarding computes the min-viable-config onboarding status.
// The backend cred-check result is cached for ~60s per §3.6.
func (s *Server) computeOnboarding(ctx context.Context) onboardingBlock {
	var ob onboardingBlock

	// Role step: ≥1 role exists.
	roles, err := s.configStore.ListRoles()
	if err == nil && len(roles) > 0 {
		ob.Steps.Role = onboardingStep{Done: true, Detail: fmt.Sprintf("%d roles", len(roles))}
	} else {
		ob.Steps.Role = onboardingStep{Done: false, Detail: "no roles defined"}
	}

	// Project step: ≥1 project exists.
	projects, err := s.configStore.ListProjects()
	if err == nil && len(projects) > 0 {
		ob.Steps.Project = onboardingStep{Done: true, Detail: fmt.Sprintf("%d projects", len(projects))}
	} else {
		ob.Steps.Project = onboardingStep{Done: false, Detail: "no projects defined"}
	}

	// Backend step: backends.json parses, version==2, default backend + model with ok creds.
	ob.Steps.Backend = s.computeBackendStep(ctx)

	ob.Satisfied = ob.Steps.Backend.Done && ob.Steps.Project.Done && ob.Steps.Role.Done
	return ob
}

func (s *Server) computeBackendStep(ctx context.Context) onboardingStep {
	backends, err := s.configStore.ReadBackends()
	if err != nil || backends.Version != 2 || len(backends.Backends) == 0 {
		return onboardingStep{Done: false, Detail: "no valid backends configured"}
	}

	// Find the default backend.
	var defaultBK config.Backend
	var defaultBKID string
	for id, bk := range backends.Backends {
		if bk.Default {
			defaultBK = bk
			defaultBKID = id
			break
		}
	}
	if defaultBKID == "" {
		return onboardingStep{Done: false, Detail: "no default backend set"}
	}
	defaultModel, ok := defaultBK.Models[defaultBK.DefaultModel]
	if !ok {
		return onboardingStep{Done: false, Detail: "default model not found"}
	}

	// Check cache.
	result := s.cachedCredCheck(ctx, defaultBK, defaultModel, defaultBKID)
	if result.Status == "ok" {
		return onboardingStep{Done: true, Detail: fmt.Sprintf("%s default model creds ok", defaultBKID)}
	}
	detail := result.Detail
	if detail == "" {
		detail = result.Status
	}
	return onboardingStep{Done: false, Detail: fmt.Sprintf("cred check %s: %s", result.Status, detail)}
}

// cachedCredCheck returns the cached cred result if still valid, otherwise runs the probe.
// The credential probe runs outside the mutex to avoid blocking concurrent GET /api/config
// requests for the full probe duration (up to 6 s).
func (s *Server) cachedCredCheck(ctx context.Context, bk config.Backend, model config.Model, backendID string) credcheck.CredResult {
	s.onboardingCacheMu.Lock()
	if s.onboardingCache != nil &&
		s.onboardingCache.backend == backendID &&
		s.onboardingCache.model == bk.DefaultModel &&
		time.Now().Before(s.onboardingCache.expires) {
		result := s.onboardingCache.result
		s.onboardingCacheMu.Unlock()
		return result
	}
	s.onboardingCacheMu.Unlock()

	merged := credcheck.MergeEnv(bk.Env, model.Env)
	result := s.credCheck(ctx, bk, model, merged)

	s.onboardingCacheMu.Lock()
	s.onboardingCache = &onboardingCacheEntry{
		result:  result,
		backend: backendID,
		model:   bk.DefaultModel,
		expires: time.Now().Add(onboardingCacheTTL),
	}
	s.onboardingCacheMu.Unlock()
	return result
}

// handleGetConfig implements GET /api/config (§5.4).
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.configStore.ReadConfig()
	if err != nil {
		if errors.Is(err, config.ErrNotFound) || errors.Is(err, config.ErrCorrupt) {
			cfg = config.DefaultConfig()
		} else {
			s.log.Error("config: read", "err", err)
			writeAPIError(w, apiError("internal", "internal error"))
			return
		}
	}

	ob := s.computeOnboarding(r.Context())
	// If onboarding_complete is already set, gate is satisfied regardless.
	if cfg.OnboardingComplete {
		ob.Satisfied = true
	}

	writeJSON(w, http.StatusOK, configResponse{Config: cfg, Onboarding: ob})
}

// configPutBody is the request body for PUT /api/config (§5.5).
// Only the user-editable subset; version and port are rejected.
type configPutBody struct {
	OnboardingComplete *bool   `json:"onboarding_complete"`
	DefaultProject     *string `json:"default_project"`
	DefaultRole        *string `json:"default_role"`
	// Sentinel fields: reject if present.
	Version *int `json:"version"`
	Port    *int `json:"port"`
}

// handlePutConfig implements PUT /api/config partial merge (§5.5).
func (s *Server) handlePutConfig(w http.ResponseWriter, r *http.Request) {
	var body configPutBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeValidationError(w, &config.ValidationErrors{Errors: []config.FieldError{
			{Field: "", Code: "bad_request", Message: "malformed JSON"},
		}})
		return
	}
	// Reject attempts to change immutable fields.
	if body.Version != nil || body.Port != nil {
		writeValidationError(w, &config.ValidationErrors{Errors: []config.FieldError{
			{Field: "version/port", Code: "immutable", Message: "version and port are not user-editable via PUT /api/config"},
		}})
		return
	}

	cfg, err := s.configStore.ReadConfig()
	if err != nil {
		if errors.Is(err, config.ErrNotFound) || errors.Is(err, config.ErrCorrupt) {
			cfg = config.DefaultConfig()
		} else {
			s.log.Error("config: read", "err", err)
			writeAPIError(w, apiError("internal", "internal error"))
			return
		}
	}

	// Merge provided fields.
	if body.OnboardingComplete != nil {
		cfg.OnboardingComplete = *body.OnboardingComplete
		// Invalidate the onboarding cred-check cache so the next GET re-evaluates.
		s.onboardingCacheMu.Lock()
		s.onboardingCache = nil
		s.onboardingCacheMu.Unlock()
	}
	if body.DefaultProject != nil {
		cfg.DefaultProject = *body.DefaultProject
	}
	if body.DefaultRole != nil {
		cfg.DefaultRole = *body.DefaultRole
	}

	if err := s.configStore.WriteConfig(cfg); err != nil {
		s.log.Error("config: write", "err", err)
		writeAPIError(w, apiError("internal", "internal error"))
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}
