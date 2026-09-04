package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func listProposals(t *testing.T, srv *Server) pipeline.ProposalCollections {
	t.Helper()
	rec := doGET(t, srv.routes(), "/api/pipeline-proposals")
	if rec.Code != http.StatusOK {
		t.Fatalf("list proposals = %d %s", rec.Code, rec.Body.String())
	}
	var collections pipeline.ProposalCollections
	if err := json.Unmarshal(rec.Body.Bytes(), &collections); err != nil {
		t.Fatalf("decode proposals: %v", err)
	}
	return collections
}

// TS-03.R16 / TS-09.R15: pending proposals are a server-owned collection, so a
// fresh Pipelines page can recover them without an ACP tool-result transcript.
// TS-03.R36 adds the declined collection beside it; both are always present and
// never null even when empty (INV §11).
func TestPipelineProposalAPIListsDurableRecords(t *testing.T) {
	srv := testServer(t, true)
	proposal, err := srv.pipelineMgr.ProposeTemplate("one-stage", apiTemplate())
	if err != nil {
		t.Fatalf("ProposeTemplate: %v", err)
	}

	rec := doGET(t, srv.routes(), "/api/pipeline-proposals")
	if rec.Code != http.StatusOK {
		t.Fatalf("list proposals = %d %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, `"declined":[]`) {
		t.Fatalf("body = %s, want an empty declined array rather than null", body)
	}
	var collections pipeline.ProposalCollections
	if err := json.Unmarshal(rec.Body.Bytes(), &collections); err != nil {
		t.Fatalf("decode proposals: %v", err)
	}
	if len(collections.Pending) != 1 || collections.Pending[0].ProposalID != proposal.ProposalID || collections.Pending[0].Kind != "save_template" {
		t.Fatalf("pending = %+v, want the durable proposal", collections.Pending)
	}
	if collections.Pending[0].CreatedAt.IsZero() || collections.Pending[0].DeclinedAt != nil {
		t.Fatalf("pending entry = %+v, want a creation time and no decline time", collections.Pending[0])
	}
}

// FS-14.R49 / TS-03.R36: Reject and Delete are two conventional routes on the
// existing family. A decline returns the updated record and moves it into the
// declined collection; the delete that follows returns 204 and removes it.
func TestPipelineProposalDeclineAndDeleteRoutes(t *testing.T) {
	srv := testServer(t, true)
	proposal, err := srv.pipelineMgr.ProposeTemplate("one-stage", apiTemplate())
	if err != nil {
		t.Fatalf("ProposeTemplate: %v", err)
	}

	rec := doRequest(t, srv.routes(), http.MethodPost, "/api/pipeline-proposals/"+proposal.ProposalID+"/decline", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("decline = %d %s", rec.Code, rec.Body.String())
	}
	var declined pipeline.ListedProposal
	if err := json.Unmarshal(rec.Body.Bytes(), &declined); err != nil {
		t.Fatalf("decode declined: %v", err)
	}
	if declined.ProposalID != proposal.ProposalID || declined.DeclinedAt == nil {
		t.Fatalf("declined = %+v, want the record with its decline time", declined)
	}

	collections := listProposals(t, srv)
	if len(collections.Pending) != 0 || len(collections.Declined) != 1 {
		t.Fatalf("collections after decline = %+v", collections)
	}
	if collections.Declined[0].DeclinedAt == nil {
		t.Fatalf("declined entry = %+v, want its decline time on the list route too", collections.Declined[0])
	}

	rec = doRequest(t, srv.routes(), http.MethodDelete, "/api/pipeline-proposals/"+proposal.ProposalID, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete = %d %s", rec.Code, rec.Body.String())
	}
	if collections = listProposals(t, srv); len(collections.Pending) != 0 || len(collections.Declined) != 0 {
		t.Fatalf("collections after delete = %+v, want both empty", collections)
	}
}

// TS-03.R36 / FS-14.R57: every refusal is a typed error whose reason names the
// state the record is actually in, so the surface can explain what happened and
// refresh rather than retry blind.
func TestPipelineProposalRefusalsNameTheRealState(t *testing.T) {
	srv := testServer(t, true)
	pending, err := srv.pipelineMgr.ProposeTemplate("one-stage", apiTemplate())
	if err != nil {
		t.Fatalf("ProposeTemplate: %v", err)
	}
	rejected, err := srv.pipelineMgr.ProposeTemplate("other-stage", apiTemplate())
	if err != nil {
		t.Fatalf("ProposeTemplate: %v", err)
	}
	if _, err := srv.pipelineMgr.DeclineProposal(rejected.ProposalID); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name, method, path string
		status             int
		code               string
	}{
		{"decline an already rejected offer", http.MethodPost, "/api/pipeline-proposals/" + rejected.ProposalID + "/decline", http.StatusConflict, "already_declined"},
		{"delete a still pending offer", http.MethodDelete, "/api/pipeline-proposals/" + pending.ProposalID, http.StatusConflict, "not_declined"},
		{"decline an unknown offer", http.MethodPost, "/api/pipeline-proposals/pp_absent/decline", http.StatusNotFound, ""},
		{"delete an unknown offer", http.MethodDelete, "/api/pipeline-proposals/pp_absent", http.StatusNotFound, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := doRequest(t, srv.routes(), tc.method, tc.path, nil)
			if rec.Code != tc.status {
				t.Fatalf("status = %d %s, want %d", rec.Code, rec.Body.String(), tc.status)
			}
			if tc.code == "" {
				return
			}
			var body struct {
				Error struct {
					Details struct {
						PipelineCode string `json:"pipeline_code"`
					} `json:"details"`
				} `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode refusal: %v", err)
			}
			if body.Error.Details.PipelineCode != tc.code {
				t.Fatalf("pipeline_code = %q, want %q (%s)", body.Error.Details.PipelineCode, tc.code, rec.Body.String())
			}
		})
	}

	// A refused action leaves the entry visible with its action retryable.
	collections := listProposals(t, srv)
	if len(collections.Pending) != 1 || len(collections.Declined) != 1 {
		t.Fatalf("collections after refusals = %+v, want both entries still listed", collections)
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

func TestPipelineRunDetailProjectsOneHopAgentsAndFallbacks(t *testing.T) {
	srv := testServer(t, true)
	base := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	for _, agent := range []state.Agent{
		{AgentID: "a_stage", Name: "Stage", Role: "implementer", Project: "my-app", Interface: "chat", CreatedAt: base},
		{AgentID: "a_delegate", Name: "Delegate", Role: "implementer", Project: "my-app", Interface: "chat", CreatedAt: base},
	} {
		if err := srv.stateStore.WriteAgent(agent); err != nil {
			t.Fatal(err)
		}
	}
	if err := srv.stateStore.WriteStatus(state.Status{AgentID: "a_stage", State: "busy", Detail: "stage status"}); err != nil {
		t.Fatal(err)
	}
	if err := srv.stateStore.WriteStatus(state.Status{AgentID: "a_delegate", State: "idle"}); err != nil {
		t.Fatal(err)
	}
	if err := srv.stateStore.WriteRunning(state.RunningEntry{AgentID: "a_delegate", PID: 1, Interface: "chat", StartedAt: base}); err != nil {
		t.Fatal(err)
	}
	insertTask := func(id, assignee, creator string, at time.Time) {
		t.Helper()
		if _, err := srv.stateStore.DB().Exec(`
INSERT INTO tasks(task_id, project, display_name, instruction, target_kind, state,
  created_by_kind, created_by_agent_id, created_by_generation, assigned_agent_id,
  outcome, outcome_summary, created_at, updated_at)
VALUES (?, 'my-app', ?, '', 'agent', 'finished', 'agent', ?, 'g1', ?, 'success', ?, ?, ?)`,
			id, id, creator, assignee, id+" outcome", at.Format(time.RFC3339Nano), at.Format(time.RFC3339Nano)); err != nil {
			t.Fatal(err)
		}
	}
	insertTask("first", "a_delegate", "a_stage", base.Add(time.Second))
	insertTask("missing", "a_missing", "a_stage", base.Add(2*time.Second))
	insertTask("second", "a_delegate", "a_stage", base.Add(11*time.Second))
	insertTask("second-hop", "a_stage", "a_delegate", base.Add(12*time.Second))
	next := base.Add(10 * time.Second)
	detail := pipeline.RunDetail{
		Run: state.PipelineRunRecord{Project: "my-app"},
		Attempts: []state.PipelineAttemptRecord{
			{AttemptID: "pa_1", AgentID: "a_stage", AgentGeneration: "g1", CreatedAt: base, ReportSummary: "first report"},
			{AttemptID: "pa_2", AgentID: "a_stage", AgentGeneration: "g1", CreatedAt: next, ReportSummary: "second report"},
			{AttemptID: "pa_empty", CreatedAt: next.Add(time.Second)},
		},
	}
	agents, err := srv.pipelineAttemptAgents(detail)
	if err != nil {
		t.Fatal(err)
	}
	first := agents["pa_1"]
	if first.StageAgent == nil || !first.StageAgent.Available || first.StageAgent.Route != "archive" || first.StageAgent.Preview != "stage status" {
		t.Fatalf("stage agent = %+v", first.StageAgent)
	}
	if len(first.DelegatedAgents) != 2 || first.DelegatedTotal != 2 || first.DelegatedRunningCount != 1 {
		t.Fatalf("first delegated agents = %+v", first)
	}
	if first.DelegatedAgents[1].AgentID != "a_delegate" || first.DelegatedAgents[1].Route != "live" || first.DelegatedAgents[1].Preview != "first outcome" {
		t.Fatalf("newest direct delegate = %+v", first.DelegatedAgents[1])
	}
	if first.DelegatedAgents[0].AgentID != "a_missing" || first.DelegatedAgents[0].Available || first.DelegatedAgents[0].Route != "unavailable" || first.DelegatedAgents[0].State != "unknown" {
		t.Fatalf("missing delegate fallback = %+v", first.DelegatedAgents[0])
	}
	second := agents["pa_2"]
	if len(second.DelegatedAgents) != 1 || second.DelegatedAgents[0].TaskID != "second" {
		t.Fatalf("second delegated agents = %+v", second)
	}
	encoded, err := json.Marshal(pipelineRunDetailResponse{RunDetail: detail, AgentsByAttempt: agents})
	if err != nil || !bytes.Contains(encoded, []byte(`"delegated_agents":[]`)) {
		t.Fatalf("empty delegated collection JSON = %s err=%v", encoded, err)
	}
}

func TestPipelineRunsAddFrozenTitleAndExactTotalHeader(t *testing.T) {
	srv := testServer(t, true)
	now := time.Now().UTC()
	snapshot, err := json.Marshal(pipeline.Template{Version: 1, Stages: []pipeline.Stage{{ID: "work", Title: "Frozen work"}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"pr_old", "pr_new"} {
		if _, _, err := srv.stateStore.CreatePipelineRun(state.CreatePipelineRunParams{Run: state.PipelineRunRecord{
			RunID: id, TemplateID: "template", TemplateSnapshot: snapshot, DisplayName: id, Project: "my-app",
			Goal: "goal", State: "completed", CurrentStageID: "work", CreatedAt: now, UpdatedAt: now,
		}, RequestID: id}); err != nil {
			t.Fatal(err)
		}
		now = now.Add(time.Second)
	}
	rec := doGET(t, srv.routes(), "/api/pipeline-runs?limit=1&offset=0")
	if rec.Code != http.StatusOK || rec.Header().Get("X-Total-Count") != "2" {
		t.Fatalf("run list = %d total=%q body=%s", rec.Code, rec.Header().Get("X-Total-Count"), rec.Body.String())
	}
	var runs []pipeline.RunSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &runs); err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || runs[0].CurrentStageTitle != "Frozen work" {
		t.Fatalf("run summaries = %+v", runs)
	}
}

// FS-14.R58 / TS-09.R33: a stage launch stores the stage agent's own name as its
// ordinary group label, so the dashboard's existing label-keyed sectioning puts a
// stage's agents in one section with no pipeline awareness.
func TestLaunchStageStoresTheStageLabelAsTheAgentGroup(t *testing.T) {
	srv, _ := wakeTestServer(t)
	id := "a_stage_group"
	execution := pipeline.StageExecution{
		AgentID: id, Generation: "g_stage_group", Role: "impl", Project: "tmpproj",
		Backend: "claude", Model: "sonnet", AgentName: "Review — Ship", Assignment: "begin",
	}
	if err := srv.LaunchStage(context.Background(), execution); err != nil {
		t.Fatalf("LaunchStage: %v", err)
	}
	agent, err := srv.stateStore.ReadAgent(id)
	if err != nil {
		t.Fatal(err)
	}
	if agent.Group != execution.AgentName || agent.Name != execution.AgentName {
		t.Fatalf("stage agent name=%q group=%q, want both %q", agent.Name, agent.Group, execution.AgentName)
	}
}
