package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/agentdeck/agentdeck/internal/pipeline"
	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
)

func (s *Server) handlePipelineTemplates(w http.ResponseWriter, _ *http.Request) {
	records, err := s.pipelineTemplates.List()
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, "pipeline templates are unavailable"))
		return
	}
	writeJSON(w, http.StatusOK, records)
}

func (s *Server) handlePipelineTemplate(w http.ResponseWriter, r *http.Request) {
	record, err := s.pipelineTemplates.Read(r.PathValue("id"))
	if err != nil {
		writePipelineError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, record)
}

type pipelineTemplateRequest struct {
	ID       string            `json:"id"`
	Template pipeline.Template `json:"template"`
}

func (s *Server) handleValidatePipelineTemplate(w http.ResponseWriter, r *http.Request) {
	var request pipelineTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeAPIError(w, apiError(runtime.CodeValidation, "invalid JSON body"))
		return
	}
	record := s.pipelineTemplates.Validate(request.ID, request.Template)
	writeJSON(w, http.StatusOK, record)
}

func (s *Server) handleCreatePipelineTemplate(w http.ResponseWriter, r *http.Request) {
	var request pipelineTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeAPIError(w, apiError(runtime.CodeValidation, "invalid JSON body"))
		return
	}
	record, err := s.pipelineTemplates.Create(request.ID, request.Template)
	if err != nil {
		writePipelineError(w, err)
		return
	}
	if !record.Valid {
		writePipelineValidation(w, record.Diagnostics)
		return
	}
	writeJSON(w, http.StatusCreated, record)
}

func (s *Server) handleUpdatePipelineTemplate(w http.ResponseWriter, r *http.Request) {
	var template pipeline.Template
	if err := json.NewDecoder(r.Body).Decode(&template); err != nil {
		writeAPIError(w, apiError(runtime.CodeValidation, "invalid JSON body"))
		return
	}
	record, err := s.pipelineTemplates.Update(r.PathValue("id"), template)
	if err != nil {
		writePipelineError(w, err)
		return
	}
	if !record.Valid {
		writePipelineValidation(w, record.Diagnostics)
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (s *Server) handleDeletePipelineTemplate(w http.ResponseWriter, r *http.Request) {
	if err := s.pipelineTemplates.Delete(r.PathValue("id")); err != nil {
		writePipelineError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handlePipelineRuns(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	runs, err := s.pipelineMgr.List(limit, offset)
	if err != nil {
		writePipelineError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, runs)
}

func (s *Server) handlePipelineRun(w http.ResponseWriter, r *http.Request) {
	detail, err := s.pipelineMgr.Detail(r.PathValue("id"))
	if err != nil {
		writePipelineError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

type startPipelineRequest struct {
	pipeline.StartRequest
	AcknowledgeSharedWorkspace bool `json:"acknowledge_shared_workspace"`
}

func (s *Server) handleStartPipelineRun(w http.ResponseWriter, r *http.Request) {
	var request startPipelineRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeAPIError(w, apiError(runtime.CodeValidation, "invalid JSON body"))
		return
	}
	if detail, replay, err := s.pipelineMgr.LookupStart(request.StartRequest); err != nil {
		writePipelineError(w, err)
		return
	} else if replay {
		writeJSON(w, http.StatusOK, map[string]any{"run": detail, "replay": true, "workspace_conflicts": []workspaceConflict{}})
		return
	}
	conflicts, err := s.pipelineWorkspaceConflicts(request.Project)
	if err != nil {
		writePipelineError(w, err)
		return
	}
	if len(conflicts) > 0 && !request.AcknowledgeSharedWorkspace {
		ae := apiError(runtime.CodeConflict, "another active agent or pipeline run shares this project")
		ae.Details = map[string]any{"code": "shared_workspace_confirmation_required", "conflicts": conflicts}
		writeAPIError(w, ae)
		return
	}
	detail, replay, err := s.pipelineMgr.Start(r.Context(), request.StartRequest)
	if err != nil {
		writePipelineError(w, err)
		return
	}
	status := http.StatusCreated
	if replay {
		status = http.StatusOK
	}
	writeJSON(w, status, map[string]any{"run": detail, "replay": replay, "workspace_conflicts": conflicts})
}

type pipelineControlRequest struct {
	Revision int64  `json:"revision"`
	Input    string `json:"input,omitempty"`
}

func (s *Server) handleContinuePipelineRun(w http.ResponseWriter, r *http.Request) {
	var request pipelineControlRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeAPIError(w, apiError(runtime.CodeValidation, "invalid JSON body"))
		return
	}
	detail, err := s.pipelineMgr.Continue(r.Context(), r.PathValue("id"), request.Revision, request.Input)
	if err != nil {
		writePipelineError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleRetryPipelineRun(w http.ResponseWriter, r *http.Request) {
	var request pipelineControlRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeAPIError(w, apiError(runtime.CodeValidation, "invalid JSON body"))
		return
	}
	detail, err := s.pipelineMgr.Retry(r.Context(), r.PathValue("id"), request.Revision)
	if err != nil {
		writePipelineError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleStopPipelineRun(w http.ResponseWriter, r *http.Request) {
	var request pipelineControlRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeAPIError(w, apiError(runtime.CodeValidation, "invalid JSON body"))
		return
	}
	detail, err := s.pipelineMgr.Stop(r.Context(), r.PathValue("id"), request.Revision)
	if err != nil {
		writePipelineError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleDeletePipelineRun(w http.ResponseWriter, r *http.Request) {
	if err := s.pipelineMgr.Delete(r.PathValue("id")); err != nil {
		writePipelineError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type workspaceConflict struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (s *Server) pipelineWorkspaceConflicts(project string) ([]workspaceConflict, error) {
	out := []workspaceConflict{}
	running, err := s.stateStore.ListRunning()
	if err != nil {
		return nil, err
	}
	for _, entry := range running {
		agent, err := s.stateStore.ReadAgent(entry.AgentID)
		if err == nil && agent.Project == project {
			out = append(out, workspaceConflict{Kind: "agent", ID: agent.AgentID, Name: agent.Name})
		}
	}
	runs, err := s.stateStore.ListActivePipelineRuns()
	if err != nil {
		return nil, err
	}
	for _, run := range runs {
		if run.Project == project {
			out = append(out, workspaceConflict{Kind: "run", ID: run.RunID, Name: run.DisplayName})
		}
	}
	return out, nil
}

func writePipelineValidation(w http.ResponseWriter, diagnostics []pipeline.Diagnostic) {
	ae := apiError(runtime.CodeValidation, "pipeline validation failed")
	ae.Details = map[string]any{"diagnostics": diagnostics}
	writeAPIError(w, ae)
}

func writePipelineError(w http.ResponseWriter, err error) {
	var controlled *pipeline.ControlError
	switch {
	case errors.As(err, &controlled):
		code := runtime.CodeValidation
		if controlled.Code == "revision_conflict" || controlled.Code == "request_conflict" || controlled.Code == "stale_assignment" || controlled.Code == "invalid_state" {
			code = runtime.CodeConflict
		}
		ae := apiError(code, controlled.Message)
		ae.Details = map[string]any{"pipeline_code": controlled.Code, "diagnostics": controlled.Diagnostics}
		writeAPIError(w, ae)
	case errors.Is(err, pipeline.ErrTemplateNotFound), errors.Is(err, state.ErrNotFound):
		writeAPIError(w, apiError(runtime.CodeNotFound, "pipeline resource not found"))
	case errors.Is(err, pipeline.ErrTemplateExists), errors.Is(err, state.ErrPipelineActive), errors.Is(err, state.ErrPipelineConflict), errors.Is(err, state.ErrPipelineRequestConflict):
		writeAPIError(w, apiError(runtime.CodeConflict, "pipeline operation conflicts with current state"))
	default:
		writeAPIError(w, apiError(runtime.CodeInternal, "pipeline operation failed"))
	}
}
