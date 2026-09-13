package state

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestCreatePipelineStageTaskIsIdempotentAndRetainsLineage(t *testing.T) {
	st, _ := newTestStore(t)
	runID, err := st.NewPipelineRunID()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, _, err := st.CreatePipelineRun(CreatePipelineRunParams{Run: PipelineRunRecord{
		RunID: runID, TemplateID: "quality", TemplateSnapshot: json.RawMessage(`{"version":2}`),
		DisplayName: "Quality", Project: "proj", Goal: "ship", Inputs: json.RawMessage(`{}`), Assignments: json.RawMessage(`{}`),
		State: "queued", Revision: 1, CurrentStageID: "implement", CreatedAt: now, UpdatedAt: now,
	}, RequestID: "stage-task-test", RequestHash: "hash"}); err != nil {
		t.Fatalf("CreatePipelineRun: %v", err)
	}
	taskID, err := st.NewTaskID()
	if err != nil {
		t.Fatal(err)
	}
	p := CreatePipelineStageTaskParams{RunID: runID, ExpectedRevision: 1, StageIndex: 0, AttemptNumber: 1, StageID: "implement", AssignmentDigest: "digest", Task: Task{
		TaskID: taskID, Project: "proj", DisplayName: "Implement", Instruction: "implement", TargetKind: TargetLaunch,
		Role: "orchestrator", Backend: "claude", Model: "sonnet", CreatedByKind: "pipeline",
	}}
	stage, task, replay, err := st.CreatePipelineStageTask(p)
	if err != nil || replay {
		t.Fatalf("CreatePipelineStageTask = %#v %#v replay=%v err=%v", stage, task, replay, err)
	}
	if task.State != TaskReady || stage.TaskID != taskID || stage.State != "open" {
		t.Fatalf("stage/task = %#v %#v", stage, task)
	}
	lineage, err := st.ReadTaskLineage(taskID)
	if err != nil || lineage.PipelineRunID != runID || lineage.PipelineStageID != "implement" {
		t.Fatalf("lineage = %#v, %v", lineage, err)
	}
	// A control replay supplies the run's new revision but must still find the
	// unique task rather than inserting a second owner.
	p.ExpectedRevision = 2
	stage2, task2, replay, err := st.CreatePipelineStageTask(p)
	if err != nil || !replay || stage2.TaskID != taskID || task2.TaskID != taskID {
		t.Fatalf("replay = %#v %#v %v %v", stage2, task2, replay, err)
	}
	var count int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM pipeline_stage_tasks WHERE run_id = ?`, runID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("stage count = %d, %v", count, err)
	}
}

func TestAgentCreatedWorkInheritsStageLineageAndClosureFence(t *testing.T) {
	st, _ := newTestStore(t)
	runID, _ := st.NewPipelineRunID()
	now := time.Now().UTC()
	stageTaskID, _ := st.NewTaskID()
	_, _, err := st.CreatePipelineRun(CreatePipelineRunParams{Run: PipelineRunRecord{
		RunID: runID, TemplateID: "quality", TemplateSnapshot: json.RawMessage(`{"version":2}`),
		DisplayName: "Quality", Project: "proj", Goal: "ship", Inputs: json.RawMessage(`{}`), Assignments: json.RawMessage(`{}`),
		State: "queued", Revision: 1, PendingAction: "dispatch_stage_task", CurrentStageID: "implement", CreatedAt: now, UpdatedAt: now,
	}, RequestID: "lineage-test", RequestHash: "hash", InitialStageTask: &CreatePipelineStageTaskParams{
		RunID: runID, ExpectedRevision: 1, StageIndex: 0, AttemptNumber: 1, StageID: "implement", Task: Task{
			TaskID: stageTaskID, Project: "proj", DisplayName: "Implement", Instruction: "implement", TargetKind: TargetLaunch, Role: "orchestrator", CreatedByKind: "pipeline",
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`UPDATE tasks SET state = ?, assigned_agent_id = 'owner', assigned_generation = 'gen' WHERE task_id = ?`, TaskRunning, stageTaskID); err != nil {
		t.Fatal(err)
	}
	childID, _ := st.NewTaskID()
	child, err := st.CreateTask(Task{TaskID: childID, Project: "proj", DisplayName: "Child", Instruction: "work", TargetKind: TargetLaunch, Role: "worker", CreatedByKind: "agent", CreatedByAgentID: "owner", CreatedByGeneration: "gen"})
	if err != nil {
		t.Fatal(err)
	}
	lineage, err := st.ReadTaskLineage(child.TaskID)
	if err != nil || lineage.ParentTaskID != stageTaskID || lineage.PipelineRunID != runID || lineage.PipelineStageID != "implement" {
		t.Fatalf("lineage = %#v, err=%v", lineage, err)
	}
	if err := st.ClosePipelineStageTask(stageTaskID, 2); err != nil {
		t.Fatal(err)
	}
	blockedID, _ := st.NewTaskID()
	_, err = st.CreateTask(Task{TaskID: blockedID, Project: "proj", DisplayName: "Too late", Instruction: "work", TargetKind: TargetLaunch, Role: "worker", CreatedByKind: "agent", CreatedByAgentID: "owner", CreatedByGeneration: "gen"})
	if !errors.Is(err, ErrTaskRunClosed) {
		t.Fatalf("closed-stage create error = %v", err)
	}
}

func TestTaskResultOutputsAreImmutable(t *testing.T) {
	st, _ := newTestStore(t)
	task := newTask(t, st, "proj", "outputs")
	if _, err := st.DB().Exec(`UPDATE tasks SET state = ?, assigned_agent_id = ?, assigned_generation = ? WHERE task_id = ?`, TaskRunning, "a", "g", task.TaskID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.RecordAgentTaskResult(task.TaskID, "a", "g", TaskResult{Outcome: OutcomeSuccess, Summary: "done", Outputs: map[string]string{"artifact": "ok"}}); err != nil {
		t.Fatalf("RecordAgentTaskResult: %v", err)
	}
	outputs, err := st.ReadTaskResultOutputs(task.TaskID)
	if err != nil || outputs["artifact"] != "ok" {
		t.Fatalf("outputs = %#v, %v", outputs, err)
	}
}
