package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

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
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, toRoleResponse(body.RoleID, role))
}

// handlePutRole implements PUT /api/roles/{role} (§5.1).
func (s *Server) handlePutRole(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("role")
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
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if err := s.configStore.WriteRole(id, role); err != nil {
		s.log.Error("roles: write", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, toRoleResponse(id, role))
}

// handleDeleteRole implements DELETE /api/roles/{role} (§5.1).
func (s *Server) handleDeleteRole(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("role")
	force := r.URL.Query().Get("force") == "true"
	// 404 if absent.
	if _, err := s.configStore.ReadRole(id); err != nil {
		if errors.Is(err, config.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
			return
		}
		s.log.Error("roles: read", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !force {
		agents, err := s.runningAgentsForRole(id)
		if err != nil {
			s.log.Error("roles: in-use check", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
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
		writeError(w, http.StatusInternalServerError, "internal error")
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
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, toProjectResponse(body.ProjectID, proj, warnings))
}

// handlePutProject implements PUT /api/projects/{p} (§5.2).
func (s *Server) handlePutProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("project")
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
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if err := s.configStore.WriteProject(id, proj); err != nil {
		s.log.Error("projects: write", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, toProjectResponse(id, proj, warnings))
}

// handleDeleteProject implements DELETE /api/projects/{p} (§5.2).
func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("project")
	force := r.URL.Query().Get("force") == "true"
	// 404 if absent.
	if _, err := s.configStore.ReadProject(id); err != nil {
		if errors.Is(err, config.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
			return
		}
		s.log.Error("projects: read", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !force {
		agents, err := s.runningAgentsForProject(id)
		if err != nil {
			s.log.Error("projects: in-use check", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
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
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
