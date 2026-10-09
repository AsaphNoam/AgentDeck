package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AsaphNoam/Chuck/internal/config"
	"github.com/AsaphNoam/Chuck/internal/pipeline"
	"github.com/AsaphNoam/Chuck/internal/state"
)

const sharedGuidance = "Name every branch after its stage id and group agents by stage."

// FS-14.A54/A55: a run's standing owner and dedicated coordinator each freeze
// the shared block once into their session prompt and Claude's native-preset
// addition; assignments carry no copy, and later template edits do not reach
// the run.
func TestPipelineOrchestratorInstructionsReachStandingAndDedicatedOnce(t *testing.T) {
	srv := testServer(t, true)
	t.Cleanup(func() { srv.registry.Shutdown(context.Background()) })
	srv.registry.Chat().SetCommand(buildFakeACP(t))
	newDump := filepath.Join(t.TempDir(), "new.json")
	t.Setenv("FAKEACP_NEW_DUMP", newDump)
	if err := srv.configStore.WriteProject("guided", config.Project{Title: "Guided", Cwd: t.TempDir(), AddDirs: []string{}}); err != nil {
		t.Fatal(err)
	}
	template := apiTemplate()
	template.OrchestratorInstructions = sharedGuidance
	template.Stages[0].Coordination, template.Stages[0].DedicatedRole = "dedicated", "implementer"
	if _, err := srv.pipelineTemplates.Create("guided", template); err != nil {
		t.Fatal(err)
	}
	runtimeAssignment := pipeline.RuntimeAssignment{Backend: "claude", Model: "sonnet"}
	rec := doRequest(t, srv.routes(), http.MethodPost, "/api/pipeline-runs", startPipelineRequest{StartRequest: pipeline.StartRequest{
		RequestID: "guided-run", TemplateID: "guided", Project: "guided", Goal: "Do it", Inputs: map[string]string{},
		Orchestrator: runtimeAssignment, DedicatedAssignments: map[string]pipeline.RuntimeAssignment{"work": runtimeAssignment},
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
	// Later edits are future-run semantics only (FS-14.R87).
	template.OrchestratorInstructions = "Replacement guidance B."
	if _, err := srv.pipelineTemplates.Update("guided", template); err != nil {
		t.Fatal(err)
	}

	var stage state.PipelineStageTask
	var coordinator state.Task
	deadline := time.Now().Add(5 * time.Second)
	for {
		if found, err := srv.stateStore.ListPipelineStageTasks(started.Run.Run.RunID); err == nil && len(found) == 1 {
			stage = found[0]
			coordinator, _ = srv.stateStore.ReadTask(stage.CoordinatorTaskID)
		}
		if stage.StandingAgentID != "" && coordinator.AssignedAgentID != "" && srv.registry.Owns(coordinator.AssignedAgentID) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("orchestrators did not start: stage=%+v coordinator=%+v", stage, coordinator)
		}
		srv.dispatchReadyTasks(context.Background())
		time.Sleep(10 * time.Millisecond)
	}

	for _, agentID := range []string{stage.StandingAgentID, coordinator.AssignedAgentID} {
		snap, err := srv.stateStore.ReadSession(agentID)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(snap.SystemPrompt, sharedGuidance) != 1 || strings.Count(snap.SystemPrompt, "Pipeline orchestration instructions") != 1 ||
			!strings.Contains(snap.SystemPrompt, "overrides them") || strings.Contains(snap.SystemPrompt, "guidance B") {
			t.Fatalf("frozen prompt for %s = %q", agentID, snap.SystemPrompt)
		}
	}
	// The coordinator is the last fresh session: its native-preset addition
	// carries the block exactly once.
	dump, err := os.ReadFile(newDump)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(dump), sharedGuidance) != 1 {
		t.Fatalf("session/new params = %s", dump)
	}
	// Resume reuses the frozen prompt: the load carries the block once more
	// without a template read or another append.
	loadDump := filepath.Join(t.TempDir(), "load.json")
	t.Setenv("FAKEACP_LOAD_DUMP", loadDump)
	handler := srv.routes()
	if rec := doRequest(t, handler, http.MethodPost, "/api/sessions/"+coordinator.AssignedAgentID+"/stop", nil); rec.Code != http.StatusOK {
		t.Fatalf("stop = %d %s", rec.Code, rec.Body.String())
	}
	if rec := doRequest(t, handler, http.MethodPost, "/api/sessions/"+coordinator.AssignedAgentID+"/resume", nil); rec.Code != http.StatusOK {
		t.Fatalf("resume = %d %s", rec.Code, rec.Body.String())
	}
	loaded, err := os.ReadFile(loadDump)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(loaded), sharedGuidance) != 1 {
		t.Fatalf("session/load params = %s", loaded)
	}
	standingTask, err := srv.stateStore.ReadTask(stage.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(standingTask.Instruction, sharedGuidance) || strings.Contains(coordinator.Instruction, sharedGuidance) {
		t.Fatal("an assignment copied the shared block")
	}
}

// FS-14.R89, TS-09.R58: ordinary work never reads a run; a known pipeline
// participant whose binding or snapshot is unavailable fails closed.
func TestPipelineOrchestratorInstructionsEligibility(t *testing.T) {
	srv := testServer(t, true)
	for _, kind := range []string{"person", "agent", ""} {
		if got, err := srv.pipelineOrchestratorInstructions(state.Task{TaskID: "t_missing", CreatedByKind: kind}); got != "" || err != nil {
			t.Fatalf("%q task = %q, %v", kind, got, err)
		}
	}
	for _, kind := range []string{"pipeline", "pipeline_coordinator"} {
		if _, err := srv.pipelineOrchestratorInstructions(state.Task{TaskID: "t_missing", CreatedByKind: kind}); !errors.Is(err, errPipelineOrchestratorContext) {
			t.Fatalf("%q without binding = %v", kind, err)
		}
	}
	if pipelineOrchestrationBlock(" \n\t") != "" {
		t.Fatal("whitespace-only instructions rendered a block")
	}
}

func createPipelineSnapshotTask(t *testing.T, srv *Server, runID, taskID string, snapshot string) state.Task {
	t.Helper()
	now := time.Now().UTC()
	_, _, err := srv.stateStore.CreatePipelineRun(state.CreatePipelineRunParams{
		Run: state.PipelineRunRecord{
			RunID: runID, TemplateID: "snapshot", TemplateSnapshot: json.RawMessage(snapshot),
			DisplayName: "Snapshot", Project: "tmpproj", Goal: "test", Inputs: json.RawMessage(`{}`),
			Assignments: json.RawMessage(`{}`), State: "queued", Revision: 1, CurrentStageID: "stage", CreatedAt: now, UpdatedAt: now,
		},
		RequestID: "request_" + runID, RequestHash: "hash_" + runID,
		InitialStageTask: &state.CreatePipelineStageTaskParams{
			RunID: runID, ExpectedRevision: 1, StageIndex: 0, AttemptNumber: 1, StageID: "stage",
			Task: state.Task{TaskID: taskID, Project: "tmpproj", DisplayName: "Stage", Instruction: "do work",
				TargetKind: state.TargetLaunch, Role: "impl", CreatedByKind: "pipeline"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	task, err := srv.stateStore.ReadTask(taskID)
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func TestPipelineOrchestratorInstructionsValidateFrozenSnapshot(t *testing.T) {
	for name, snapshot := range map[string]string{
		"malformed":           `{`,
		"null":                `null`,
		"empty object":        `{}`,
		"missing fields":      `{"version":2}`,
		"null arrays":         `{"version":2,"title":"x","orchestrator_role":"impl","inputs":null,"stages":[]}`,
		"wrong guidance type": `{"version":2,"title":"x","orchestrator_role":"impl","inputs":[],"stages":[],"orchestrator_instructions":[]}`,
	} {
		t.Run(name, func(t *testing.T) {
			srv := testServer(t, true)
			task := createPipelineSnapshotTask(t, srv, "run_"+strings.ReplaceAll(name, " ", "_"), "task_bad", snapshot)
			if _, err := srv.pipelineOrchestratorInstructions(task); !errors.Is(err, errPipelineOrchestratorContext) {
				t.Fatalf("corrupt snapshot error = %v", err)
			}
		})
	}

	for name, snapshot := range map[string]string{
		"omitted": `{"version":2,"title":"x","orchestrator_role":"impl","inputs":[],"stages":[{"id":"stage","title":"Stage","objective":"Do work","inputs":[],"outputs":[]}]}`,
		"empty":   `{"version":2,"title":"x","orchestrator_role":"impl","inputs":[],"stages":[{"id":"stage","title":"Stage","objective":"Do work","inputs":[],"outputs":[]}],"orchestrator_instructions":""}`,
	} {
		t.Run(name, func(t *testing.T) {
			srv := testServer(t, true)
			task := createPipelineSnapshotTask(t, srv, "run_"+name, "task_valid", snapshot)
			if got, err := srv.pipelineOrchestratorInstructions(task); got != "" || err != nil {
				t.Fatalf("valid snapshot = %q, %v", got, err)
			}
		})
	}

	t.Run("storage read failure", func(t *testing.T) {
		srv := testServer(t, true)
		task := createPipelineSnapshotTask(t, srv, "run_read_error", "task_read_error", `{"version":2,"title":"x","orchestrator_role":"impl","inputs":[],"stages":[{"id":"stage","title":"Stage","objective":"Do work","inputs":[],"outputs":[]}]}`)
		if _, err := srv.stateStore.DB().Exec(`DROP TABLE pipeline_runs`); err != nil {
			t.Fatal(err)
		}
		if _, err := srv.pipelineOrchestratorInstructions(task); err == nil {
			t.Fatal("storage failure returned no error")
		}
	})
}

func TestCorruptPipelineSnapshotFailsTaskStartWithoutLaunching(t *testing.T) {
	srv := testServer(t, true)
	srv.registry.Chat().SetCommand(buildFakeACP(t))
	task := createPipelineSnapshotTask(t, srv, "run_dispatch_bad", "task_dispatch_bad", `{"version":2}`)
	for attempt := 0; attempt < state.MaxTaskStartAttempts; attempt++ {
		admitted, ok, err := srv.stateStore.AdmitReadyTask(task.TaskID, state.TaskReservation{
			AttemptID: "attempt_bad_" + string(rune('1'+attempt)), AgentID: "a_corrupt", Generation: "g_corrupt", Claim: state.ClaimCreated,
		}, 1)
		if err != nil || !ok {
			t.Fatalf("admit attempt %d = %+v, %v", attempt, admitted, err)
		}
		srv.startLaunchedTask(context.Background(), admitted)
		task, err = srv.stateStore.ReadTask(task.TaskID)
		if err != nil {
			t.Fatal(err)
		}
	}
	if task.State != state.TaskDependencyFailed || !strings.Contains(task.AttentionReason, "pipeline orchestrator context is unavailable") {
		t.Fatalf("task after corrupt snapshot = %+v", task)
	}
	if _, err := srv.stateStore.ReadSession("a_corrupt"); !errors.Is(err, state.ErrNotFound) {
		t.Fatalf("corrupt snapshot started a process/session: %v", err)
	}
}
