package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/pipeline"
	"github.com/agentdeck/agentdeck/internal/state"
)

func apiTemplate() pipeline.Template {
	return pipeline.Template{
		Version: 1, Title: "One stage", Inputs: []pipeline.ValueDecl{},
		Stages: []pipeline.Stage{{
			ID: "work", Title: "Work", Role: "implementer", Instruction: "Do the work.",
			Inputs: []pipeline.StageInput{}, Outputs: []pipeline.StageOutput{},
			Transitions: pipeline.OutcomeTransitions{
				Success: pipeline.Transition{Final: "success", Approval: "automatic"},
				Failure: pipeline.Transition{Final: "failure", Approval: "required"},
			},
		}},
	}
}

// FS-14.A1: template REST CRUD and validation share the canonical model-neutral
// contract and preserve non-null collection shapes.
func TestPipelineTemplateAPICRUDAndValidation(t *testing.T) {
	srv := testServer(t, true)
	handler := srv.routes()
	rec := doRequest(t, handler, http.MethodPost, "/api/pipelines", pipelineTemplateRequest{ID: "one-stage", Template: apiTemplate()})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create template = %d %s", rec.Code, rec.Body.String())
	}
	rec = doGET(t, handler, "/api/pipelines")
	if rec.Code != http.StatusOK {
		t.Fatalf("list templates = %d %s", rec.Code, rec.Body.String())
	}
	var list []pipeline.TemplateRecord
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || !list[0].Valid || list[0].Template.Inputs == nil || list[0].Template.Stages[0].Inputs == nil {
		t.Fatalf("template list = %+v", list)
	}
	invalid := apiTemplate()
	invalid.Stages[0].Role = "missing-role"
	rec = doRequest(t, handler, http.MethodPut, "/api/pipelines/one-stage", invalid)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid update = %d %s", rec.Code, rec.Body.String())
	}
	stored, err := srv.pipelineTemplates.Read("one-stage")
	if err != nil || stored.Template.Stages[0].Role != "implementer" {
		t.Fatalf("invalid update changed template: %+v err=%v", stored, err)
	}
	rec = doRequest(t, handler, http.MethodDelete, "/api/pipelines/one-stage", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete template = %d %s", rec.Code, rec.Body.String())
	}
}

// FS-14.A8: a same-project conflict is advisory but must be acknowledged before
// the start endpoint is allowed to create or launch anything.
func TestPipelineStartRequiresSharedWorkspaceAcknowledgement(t *testing.T) {
	srv := testServer(t, true)
	if err := srv.configStore.WriteProject("shared", config.Project{Title: "Shared", Cwd: t.TempDir(), AddDirs: []string{}}); err != nil {
		t.Fatal(err)
	}
	agent := state.Agent{AgentID: "a_busy", Name: "Busy", Role: "implementer", Project: "shared", Backend: "claude", Model: "sonnet", Interface: "chat", CreatedAt: time.Now().UTC()}
	if err := srv.stateStore.WriteAgent(agent); err != nil {
		t.Fatal(err)
	}
	if err := srv.stateStore.WriteRunning(state.RunningEntry{AgentID: agent.AgentID, PID: 123, SessionID: "s", Interface: "chat", StartedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	record, err := srv.pipelineTemplates.Create("one-stage", apiTemplate())
	if err != nil || !record.Valid {
		t.Fatalf("template = %+v err=%v", record, err)
	}
	request := startPipelineRequest{StartRequest: pipeline.StartRequest{
		RequestID: "request-shared", TemplateID: "one-stage", Project: "shared", Goal: "Do it",
		Inputs: map[string]string{}, Assignments: map[string]pipeline.RuntimeAssignment{"work": {Backend: "claude", Model: "sonnet"}},
	}}
	rec := doRequest(t, srv.routes(), http.MethodPost, "/api/pipeline-runs", request)
	if rec.Code != http.StatusConflict {
		t.Fatalf("unacknowledged start = %d %s", rec.Code, rec.Body.String())
	}
	if runs, err := srv.stateStore.ListPipelineRuns(10, 0); err != nil || len(runs) != 0 {
		t.Fatalf("warning created a run: %+v err=%v", runs, err)
	}
}

// FS-14.A4 / TS-09.R13: stopping a stage agent through its ordinary card/API
// action is not the pipeline's own transition stop. It must pause the run with
// a retryable recovery state instead of leaving await_result wedged forever.
func TestOrdinaryStageAgentStopPausesPipelineRun(t *testing.T) {
	srv := testServer(t, true)
	t.Cleanup(func() { srv.registry.Shutdown(context.Background()) })
	srv.registry.Chat().SetCommand(buildFakeACP(t))
	if err := srv.configStore.WriteProject("pipeline-stop", config.Project{
		Title: "Pipeline stop", Cwd: t.TempDir(), AddDirs: []string{},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.pipelineTemplates.Create("one-stage", apiTemplate()); err != nil {
		t.Fatal(err)
	}

	handler := srv.routes()
	rec := doRequest(t, handler, http.MethodPost, "/api/pipeline-runs", startPipelineRequest{StartRequest: pipeline.StartRequest{
		RequestID: "request-stage-stop", TemplateID: "one-stage", Project: "pipeline-stop", Goal: "Do it",
		Inputs: map[string]string{}, Assignments: map[string]pipeline.RuntimeAssignment{"work": {Backend: "claude", Model: "sonnet"}},
	}})
	if rec.Code != http.StatusCreated {
		t.Fatalf("start run = %d %s", rec.Code, rec.Body.String())
	}
	var started struct {
		Run pipeline.RunDetail `json:"run"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &started); err != nil {
		t.Fatal(err)
	}
	agentID := started.Run.Run.CurrentAgentID
	if agentID == "" || !srv.registry.Owns(agentID) {
		t.Fatalf("started run has no live stage agent: %+v", started.Run.Run)
	}

	rec = doRequest(t, handler, http.MethodPost, "/api/sessions/"+agentID+"/stop", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("ordinary stop = %d %s", rec.Code, rec.Body.String())
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		detail, err := srv.pipelineMgr.Detail(started.Run.Run.RunID)
		if err != nil {
			t.Fatal(err)
		}
		if detail.Run.State == "paused" {
			if detail.Run.AttentionReason != "agent_stopped" || detail.Run.PendingAction != "" {
				t.Fatalf("paused run = %+v", detail.Run)
			}
			if _, err := srv.pipelineMgr.Retry(t.Context(), detail.Run.RunID, detail.Run.Revision); err != nil {
				t.Fatalf("paused run is not retryable: %v", err)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("run stayed wedged after ordinary stage stop: %+v", detail.Run)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TS-05.R14: pipeline routes inherit the whole-mux Origin guard.
func TestPipelineRoutesRejectCrossOrigin(t *testing.T) {
	srv := testServer(t, true)
	req := newLocalRequest(http.MethodGet, "/api/pipelines", nil)
	req.Header.Set("Origin", "https://attacker.example")
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross-origin pipeline request = %d, want 403", rec.Code)
	}
}
