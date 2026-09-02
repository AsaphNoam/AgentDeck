package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/runtime"
)

// The worktree-project HTTP surface (TS-03.R33, TS-12 §3). Both routes inherit
// the whole-mux localOnly guard like every other API route (INV §14).

// handleWorktreeFork implements POST /api/projects/{project}/worktree-fork.
// Success is 201 with the created project; Git and validation failures are 422
// with the specific reason, and an archived source is rejected.
func (s *Server) handleWorktreeFork(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("project")
	if !config.ValidSlug(id) {
		writeAPIError(w, apiError(runtime.CodeValidation, "invalid project"))
		return
	}
	var body forkRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIError(w, apiError(runtime.CodeValidation, "invalid JSON body"))
		return
	}
	// The fork writes a project definition and creates a checkout, so it joins
	// the same project start boundary an agent launch takes: an archive claim in
	// progress must not race a new project into existence beside it.
	if ae := s.acquireProjectStart(id); ae != nil {
		writeAPIError(w, ae)
		return
	}
	defer s.releaseProjectStart(id)

	result, ae := s.worktreeFork(r.Context(), id, body)
	if ae != nil {
		writeAPIError(w, ae)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

// handleWorktreeStatus implements GET /api/projects/{project}/worktree. It is
// the on-demand endpoint that feeds the fork form and the archive dialog, and
// the only place the expensive checks (dirty state, base detection) run.
func (s *Server) handleWorktreeStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("project")
	if !config.ValidSlug(id) {
		writeAPIError(w, apiError(runtime.CodeValidation, "invalid project"))
		return
	}
	project, err := s.configStore.ReadProject(id)
	if err != nil {
		if errors.Is(err, config.ErrNotFound) {
			writeAPIError(w, apiError(runtime.CodeNotFound, "no such project: "+id))
			return
		}
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, s.worktreeStatus(r.Context(), id, project))
}
