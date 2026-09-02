package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
)

func (s *Server) stopForArchive(r *http.Request, id string) *runtime.APIError {
	if err := s.registry.Stop(r.Context(), id); err != nil {
		if !errors.Is(err, runtime.ErrNoHandle) {
			return apiError(runtime.CodeInternal, "stop agent: "+err.Error())
		}
		if err := s.reapOrphanRuntime(id); err != nil {
			return apiError(runtime.CodeInternal, "reap orphan agent: "+err.Error())
		}
	}
	s.teardownAgentRegistration(id)
	return nil
}

func (s *Server) handleArchiveAgentAction(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	agent, err := s.stateStore.ReadAgent(id)
	if errors.Is(err, state.ErrNotFound) {
		writeAPIError(w, apiError(runtime.CodeNotFound, "no such agent: "+id))
		return
	}
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	if ae := s.beginAgentArchive(r.Context(), agent.Project, id); ae != nil {
		writeAPIError(w, ae)
		return
	}
	defer s.endAgentArchive(agent.Project, id)
	agent, err = s.stateStore.ReadAgent(id)
	if errors.Is(err, state.ErrNotFound) {
		writeAPIError(w, apiError(runtime.CodeNotFound, "no such agent: "+id))
		return
	}
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	if agent.Archived {
		writeJSON(w, http.StatusOK, s.readSession(id))
		return
	}
	project, err := s.configStore.ReadProject(agent.Project)
	if err == nil && project.Archived {
		writeAPIError(w, apiError(runtime.CodeProjectArchived, "project is archived"))
		return
	}
	if ae := s.stopForArchive(r, id); ae != nil {
		writeAPIError(w, ae)
		return
	}
	if err := s.setAgentsArchived([]string{id}, true); err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	_, _ = s.stateMgr.Touch(id)
	writeJSON(w, http.StatusOK, s.readSession(id))
}

func (s *Server) handleRestoreAgentAction(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	agent, err := s.stateStore.ReadAgent(id)
	if errors.Is(err, state.ErrNotFound) {
		writeAPIError(w, apiError(runtime.CodeNotFound, "no such agent: "+id))
		return
	}
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	if ae := s.beginAgentArchive(r.Context(), agent.Project, id); ae != nil {
		writeAPIError(w, ae)
		return
	}
	defer s.endAgentArchive(agent.Project, id)
	if ae := s.projectArchiveGate(agent.Project, "project is archived; restore it first"); ae != nil {
		writeAPIError(w, ae)
		return
	}
	if err := s.setAgentsArchived([]string{id}, false); err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	_, _ = s.stateMgr.Touch(id)
	writeJSON(w, http.StatusOK, s.readSession(id))
}

func (s *Server) handleArchiveProjectAction(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("project")
	if !config.ValidSlug(id) {
		writeAPIError(w, apiError(runtime.CodeValidation, "invalid project"))
		return
	}
	// The body is optional: archiving has never required one, and its absence
	// never deletes a checkout (FS-19.R8, TS-03.R33).
	var body archiveProjectRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	if ae := s.beginProjectArchive(r.Context(), id); ae != nil {
		writeAPIError(w, ae)
		return
	}
	defer s.endProjectArchive(id)
	project, err := s.configStore.ReadProject(id)
	if errors.Is(err, config.ErrNotFound) {
		writeAPIError(w, apiError(runtime.CodeNotFound, "no such project: "+id))
		return
	}
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	if project.Archived {
		writeJSON(w, http.StatusOK, archiveProjectActionResponse{
			Project:          s.toProjectResponse(id, project, nil),
			StoppedAgentIDs:  []string{},
			ArchivedAgentIDs: []string{},
		})
		return
	}
	if s.pipelineMgr != nil {
		if err := s.pipelineMgr.StopProject(r.Context(), id); err != nil {
			writeAPIError(w, apiError(runtime.CodeInternal, "stop project pipelines: "+err.Error()))
			return
		}
	}
	agents, err := s.stateStore.ListAgents()
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	ids := make([]string, 0)
	priorArchived := make(map[string]bool)
	for _, agent := range agents {
		if agent.Project == id {
			ids = append(ids, agent.AgentID)
			priorArchived[agent.AgentID] = agent.Archived
		}
	}
	for _, agentID := range ids {
		if ae := s.stopForArchive(r, agentID); ae != nil {
			writeAPIError(w, ae)
			return
		}
	}
	if err := s.setAgentsArchived(ids, true); err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	project.Archived = true
	if err := s.writeProject(id, project); err != nil {
		if restoreErr := s.restoreAgentArchiveStates(priorArchived); restoreErr != nil {
			for _, agentID := range ids {
				_, _ = s.stateMgr.Touch(agentID)
			}
			writeAPIError(w, apiError(runtime.CodeInternal,
				"write project archive state: "+err.Error()+"; restore agent archive states: "+restoreErr.Error()))
			return
		}
		writeAPIError(w, apiError(runtime.CodeInternal, "write project archive state: "+err.Error()))
		return
	}
	for _, agentID := range ids {
		_, _ = s.stateMgr.Touch(agentID)
	}
	// Checkout deletion runs last, inside this same claim and after every process
	// is stopped and the archive is durably committed (TS-12.R7). A failure here
	// is reported beside a successful archive rather than undoing it: the archive
	// already happened, and the checkout is still there to try again.
	resp := archiveProjectActionResponse{
		Project:          s.toProjectResponse(id, project, nil),
		StoppedAgentIDs:  append([]string{}, ids...),
		ArchivedAgentIDs: append([]string{}, ids...),
	}
	if body.DeleteCheckout {
		deleted, err := s.deleteOwnedCheckout(r.Context(), id)
		if err != nil {
			s.log.Error("worktree: delete checkout on archive", "project", id, "err", err)
			resp.CheckoutWarning = "the checkout could not be deleted: " + err.Error()
		}
		resp.CheckoutDeleted = deleted
	}
	writeJSON(w, http.StatusOK, resp)
}

// archiveProjectRequest is the optional archive body. delete_checkout is
// honored only for an AgentDeck-owned checkout and is never defaulted on.
type archiveProjectRequest struct {
	DeleteCheckout bool `json:"delete_checkout"`
}

func (s *Server) handleRestoreProjectAction(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("project")
	if !config.ValidSlug(id) {
		writeAPIError(w, apiError(runtime.CodeValidation, "invalid project"))
		return
	}
	project, err := s.configStore.ReadProject(id)
	if errors.Is(err, config.ErrNotFound) {
		writeAPIError(w, apiError(runtime.CodeNotFound, "no such project: "+id))
		return
	}
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	if ae := s.beginProjectArchive(r.Context(), id); ae != nil {
		writeAPIError(w, ae)
		return
	}
	defer s.endProjectArchive(id)
	project.Archived = false
	if err := s.writeProject(id, project); err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, s.toProjectResponse(id, project, nil))
}

type archiveProjectActionResponse struct {
	Project          projectResponse `json:"project"`
	StoppedAgentIDs  []string        `json:"stopped_agent_ids"`
	ArchivedAgentIDs []string        `json:"archived_agent_ids"`
	// CheckoutDeleted reports that the consented deletion actually happened;
	// CheckoutWarning carries the reason when it was requested and refused.
	CheckoutDeleted bool   `json:"checkout_deleted,omitempty"`
	CheckoutWarning string `json:"checkout_warning,omitempty"`
}
