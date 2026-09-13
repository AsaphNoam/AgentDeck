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

// TS-09.R40, TS-10.R28/R31 — agent plus generation does not identify one
// execution: a borrowed runtime can be re-borrowed under the same generation, so
// a stale handle from an earlier assignment could land on the current stage
// task. A standing yield intent is refused for the same transaction, because the
// owner already asked to release this execution to watch child work. Neither
// refusal may leave a partial task result or move the run.
func TestAcceptPipelineStageTaskResultFencesHandleAndYield(t *testing.T) {
	st, _ := newTestStore(t)
	runID, _ := st.NewPipelineRunID()
	now := time.Now().UTC()
	stageTaskID, _ := st.NewTaskID()
	if _, _, err := st.CreatePipelineRun(CreatePipelineRunParams{Run: PipelineRunRecord{
		RunID: runID, TemplateID: "quality", TemplateSnapshot: json.RawMessage(`{"version":2}`),
		DisplayName: "Quality", Project: "proj", Goal: "ship", Inputs: json.RawMessage(`{}`), Assignments: json.RawMessage(`{}`),
		State: "queued", Revision: 1, PendingAction: "dispatch_stage_task", CurrentStageID: "implement", CreatedAt: now, UpdatedAt: now,
	}, RequestID: "handle-fence", RequestHash: "hash", InitialStageTask: &CreatePipelineStageTaskParams{
		RunID: runID, ExpectedRevision: 1, StageIndex: 0, AttemptNumber: 1, StageID: "implement", Task: Task{
			TaskID: stageTaskID, Project: "proj", DisplayName: "Implement", Instruction: "implement", TargetKind: TargetLaunch, Role: "orchestrator", CreatedByKind: "pipeline",
		},
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`UPDATE tasks SET state = ?, assigned_agent_id = 'owner', assigned_generation = 'gen', execution_handle = 'ta_current' WHERE task_id = ?`, TaskRunning, stageTaskID); err != nil {
		t.Fatal(err)
	}
	run, err := st.ReadPipelineRun(runID)
	if err != nil {
		t.Fatal(err)
	}
	result := TaskResult{Outcome: OutcomeSuccess, Summary: "done"}

	assertUntouched := func(t *testing.T, what string) {
		t.Helper()
		task, err := st.ReadTask(stageTaskID)
		if err != nil {
			t.Fatal(err)
		}
		if task.State != TaskRunning || task.Outcome != "" || task.PendingRelease {
			t.Fatalf("%s mutated the task result: %#v", what, task)
		}
		after, err := st.ReadPipelineRun(runID)
		if err != nil {
			t.Fatal(err)
		}
		if after.Revision != run.Revision || after.State != run.State {
			t.Fatalf("%s moved the run: %#v", what, after)
		}
	}

	if _, err := st.AcceptPipelineStageTaskResult(stageTaskID, "owner", "gen", "", run.Revision, result); !errors.Is(err, ErrPipelineStageConflict) {
		t.Fatalf("missing handle error = %v, want ErrPipelineStageConflict", err)
	}
	assertUntouched(t, "a stage report without an execution handle")

	if _, err := st.AcceptPipelineStageTaskResult(stageTaskID, "owner", "gen", "ta_previous", run.Revision, result); !errors.Is(err, ErrPipelineStageConflict) {
		t.Fatalf("stale handle error = %v, want ErrPipelineStageConflict", err)
	}
	assertUntouched(t, "a stage report carrying a stale execution handle")

	// The owner registers a wait in the same turn, then reports: yield and
	// release intents must never both stand on one task.
	if _, _, err := st.WaitForTasks("owner", "gen", "ta_current", []TaskWaitObservation{{TaskID: "tk_missing"}}); !errors.Is(err, ErrTaskWaitScope) {
		t.Fatalf("wait on unreadable source = %v", err)
	}
	if _, err := st.DB().Exec(`UPDATE tasks SET pending_yield = 1 WHERE task_id = ?`, stageTaskID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AcceptPipelineStageTaskResult(stageTaskID, "owner", "gen", "ta_current", run.Revision, result); !errors.Is(err, ErrPipelineStageConflict) {
		t.Fatalf("wait-then-report error = %v, want ErrPipelineStageConflict", err)
	}
	assertUntouched(t, "a stage report from an execution that is yielding")

	// The matching handle on a settled execution is still accepted.
	if _, err := st.DB().Exec(`UPDATE tasks SET pending_yield = 0 WHERE task_id = ?`, stageTaskID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AcceptPipelineStageTaskResult(stageTaskID, "owner", "gen", "ta_current", run.Revision, result); err != nil {
		t.Fatalf("AcceptPipelineStageTaskResult with the current handle: %v", err)
	}
	accepted, err := st.ReadTask(stageTaskID)
	if err != nil || accepted.Outcome != OutcomeSuccess || !accepted.PendingRelease {
		t.Fatalf("accepted task = %#v, %v", accepted, err)
	}
}

// TS-09.R37/R39/R41/R49, FS-14.R73 — a standing stage owner must be able to
// read, watch and manage the work its stage owns, including a host-created
// coordinator it did not create and retained predecessor work it inherits on
// replacement. Creator-only authority refused exactly that workflow. Authority
// follows the live stage binding; creation history is never rewritten and an
// unrelated caller still matches nothing.
func TestStandingOwnerManagesRunWorkItDidNotCreate(t *testing.T) {
	st, _ := newTestStore(t)
	runID, _ := st.NewPipelineRunID()
	now := time.Now().UTC()
	stageTaskID, _ := st.NewTaskID()
	if _, _, err := st.CreatePipelineRun(CreatePipelineRunParams{Run: PipelineRunRecord{
		RunID: runID, TemplateID: "quality", TemplateSnapshot: json.RawMessage(`{"version":2}`),
		DisplayName: "Quality", Project: "proj", Goal: "ship", Inputs: json.RawMessage(`{}`), Assignments: json.RawMessage(`{}`),
		State: "queued", Revision: 1, PendingAction: "dispatch_stage_task", CurrentStageID: "implement", CreatedAt: now, UpdatedAt: now,
	}, RequestID: "managed-authority", RequestHash: "hash", InitialStageTask: &CreatePipelineStageTaskParams{
		RunID: runID, ExpectedRevision: 1, StageIndex: 0, AttemptNumber: 1, StageID: "implement", Task: Task{
			TaskID: stageTaskID, Project: "proj", DisplayName: "Implement", Instruction: "implement", TargetKind: TargetLaunch, Role: "orchestrator", CreatedByKind: "pipeline",
		},
		Coordinator: &Task{TaskID: "tk_coord", Project: "proj", DisplayName: "Implement coordinator", Instruction: "coordinate", TargetKind: TargetLaunch, Role: "implementer", CreatedByKind: "pipeline_coordinator"},
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`UPDATE tasks SET state = ?, assigned_agent_id = 'owner', assigned_generation = 'gen', execution_handle = 'ta_1' WHERE task_id = ?`, TaskRunning, stageTaskID); err != nil {
		t.Fatal(err)
	}
	if err := st.BindPipelineStageTaskStandingAgent(stageTaskID, "owner"); err != nil {
		t.Fatal(err)
	}

	mustManage := func(t *testing.T, agentID, taskID string, want bool) {
		t.Helper()
		got, err := st.AgentManagesPipelineWork(agentID, taskID)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("AgentManagesPipelineWork(%q, %q) = %v, want %v", agentID, taskID, got, want)
		}
	}
	mustManage(t, "owner", "tk_coord", true)
	mustManage(t, "stranger", "tk_coord", false)
	// The stage task itself mutates only through the pipeline controller.
	mustManage(t, "owner", stageTaskID, false)

	// The owner waits on the managed child it never created.
	child, err := st.ReadTask("tk_coord")
	if err != nil {
		t.Fatal(err)
	}
	if _, task, err := st.WaitForTasks("owner", "gen", "ta_1", []TaskWaitObservation{{TaskID: "tk_coord", AfterRevision: child.Revision}}); err != nil || !task.PendingYield {
		t.Fatalf("WaitForTasks on the managed child = %#v, %v", task, err)
	}
	// Work in the same project that this run does not own stays out of scope.
	outsideID, _ := st.NewTaskID()
	if _, err := st.CreateTask(Task{TaskID: outsideID, Project: "proj", DisplayName: "Outside", Instruction: "unrelated", TargetKind: TargetLaunch, Role: "worker", CreatedByKind: "agent", CreatedByAgentID: "stranger", CreatedByGeneration: "g"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.WaitForTasks("owner", "gen", "ta_1", []TaskWaitObservation{{TaskID: outsideID}}); !errors.Is(err, ErrTaskWaitScope) {
		t.Fatalf("wait on unrelated work = %v, want ErrTaskWaitScope", err)
	}

	// A replacement standing owner inherits that same authority, and the
	// predecessor loses it, without any creator column being rewritten.
	replacementID, _ := st.NewTaskID()
	if _, err := st.DB().Exec(`UPDATE pipeline_stage_tasks SET state = 'replaced' WHERE task_id = ?`, stageTaskID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`
INSERT INTO tasks(task_id, project, display_name, instruction, target_kind, state, created_by_kind, assigned_agent_id, created_at, updated_at)
VALUES (?, 'proj', 'Implement', 'implement', 'launch', ?, 'pipeline', 'new-owner', ?, ?)`,
		replacementID, TaskRunning, formatTime(now), formatTime(now)); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`
INSERT INTO pipeline_stage_tasks(run_id, stage_index, attempt_number, stage_id, task_id, standing_agent_id, state, created_at)
VALUES (?, 0, 2, 'implement', ?, 'new-owner', 'open', ?)`, runID, replacementID, formatTime(now)); err != nil {
		t.Fatal(err)
	}
	mustManage(t, "new-owner", "tk_coord", true)
	mustManage(t, "owner", "tk_coord", false)
	coordinator, err := st.ReadTask("tk_coord")
	if err != nil || coordinator.CreatedByKind != "pipeline_coordinator" || coordinator.CreatedByAgentID != "" {
		t.Fatalf("coordinator provenance = %#v, %v; want immutable creation history", coordinator, err)
	}
}

// TS-09.R42, TS-10.R29/R32, INV §2/§5/§15 — creation was the only writer that
// checked inherited closure, so work that was already ready, a watch that fired
// later, a retry, a re-arm, and a report that arrived after Stop all escaped it.
// A report was the worst: the caller reads the run revision itself, so a report
// after Stop carried the *new* revision and overwrote `stopping` with
// `finishing`, un-stopping the run before cancellation reached its task.
func TestClosedRunFencesEveryTaskWriter(t *testing.T) {
	st, _ := newTestStore(t)
	runID, _ := st.NewPipelineRunID()
	now := time.Now().UTC()
	stageTaskID, _ := st.NewTaskID()
	if _, _, err := st.CreatePipelineRun(CreatePipelineRunParams{Run: PipelineRunRecord{
		RunID: runID, TemplateID: "quality", TemplateSnapshot: json.RawMessage(`{"version":2}`),
		DisplayName: "Quality", Project: "proj", Goal: "ship", Inputs: json.RawMessage(`{}`), Assignments: json.RawMessage(`{}`),
		State: "queued", Revision: 1, PendingAction: "dispatch_stage_task", CurrentStageID: "implement", CreatedAt: now, UpdatedAt: now,
	}, RequestID: "closure-fence", RequestHash: "hash", InitialStageTask: &CreatePipelineStageTaskParams{
		RunID: runID, ExpectedRevision: 1, StageIndex: 0, AttemptNumber: 1, StageID: "implement", Task: Task{
			TaskID: stageTaskID, Project: "proj", DisplayName: "Implement", Instruction: "implement", TargetKind: TargetLaunch, Role: "orchestrator", CreatedByKind: "pipeline",
		},
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`UPDATE tasks SET state = ?, assigned_agent_id = 'owner', assigned_generation = 'gen', execution_handle = 'ta_1' WHERE task_id = ?`, TaskRunning, stageTaskID); err != nil {
		t.Fatal(err)
	}
	// One child already ready to start, one settled waiter watching it.
	readyID, _ := st.NewTaskID()
	if _, err := st.CreateTask(Task{TaskID: readyID, Project: "proj", DisplayName: "Ready child", Instruction: "work", TargetKind: TargetLaunch, Role: "worker", CreatedByKind: "agent", CreatedByAgentID: "owner", CreatedByGeneration: "gen"}); err != nil {
		t.Fatal(err)
	}
	waiterID, _ := st.NewTaskID()
	if _, err := st.CreateTask(Task{TaskID: waiterID, Project: "proj", DisplayName: "Waiter", Instruction: "watch", TargetKind: TargetLaunch, Role: "worker", CreatedByKind: "agent", CreatedByAgentID: "owner", CreatedByGeneration: "gen"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`UPDATE tasks SET state = ? WHERE task_id = ?`, TaskWaiting, waiterID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`INSERT INTO task_waits(waiting_task_id, wait_version, source_task_id, after_revision) VALUES (?, 1, ?, 0)`, waiterID, readyID); err != nil {
		t.Fatal(err)
	}

	// A person stops the run. Nothing below may escape that fence.
	run, err := st.ReadPipelineRun(runID)
	if err != nil {
		t.Fatal(err)
	}
	stopping, err := st.UpdatePipelineRunCAS(runID, run.Revision, PipelineRunUpdate{
		State: "stopping", PendingAction: "cleanup_run", CurrentStageID: run.CurrentStageID,
	})
	if err != nil {
		t.Fatal(err)
	}

	// The report race: the reporter reads the revision Stop just committed.
	if _, err := st.AcceptPipelineStageTaskResult(stageTaskID, "owner", "gen", "ta_1", stopping.Revision,
		TaskResult{Outcome: OutcomeSuccess, Summary: "done"}); !errors.Is(err, ErrPipelineStageConflict) {
		t.Fatalf("report after Stop = %v, want ErrPipelineStageConflict", err)
	}
	stillStopping, err := st.ReadPipelineRun(runID)
	if err != nil {
		t.Fatal(err)
	}
	if stillStopping.State != "stopping" || stillStopping.PendingAction != "cleanup_run" {
		t.Fatalf("a post-Stop report un-stopped the run: %#v", stillStopping)
	}

	// Admission of work that was already ready.
	if _, ok, err := st.AdmitReadyTask(readyID, TaskReservation{AttemptID: "ta_x", AgentID: "a_new", Generation: "ta_x", Claim: ClaimCreated}, 8); err != nil || ok {
		t.Fatalf("admitted a ready child of a stopped run: ok=%v err=%v", ok, err)
	}

	// A watched change firing after the stop.
	if changed, err := st.NotifyTaskWaiters(readyID); err != nil || len(changed) != 0 {
		t.Fatalf("woke a waiter under a stopped run: %+v, %v", changed, err)
	}
	waiter, err := st.ReadTask(waiterID)
	if err != nil || waiter.State != TaskWaiting {
		t.Fatalf("waiter = %#v, %v; want it left waiting", waiter, err)
	}

	// Retry and re-arm of parked work under the same closed run.
	if _, err := st.DB().Exec(`UPDATE tasks SET state = ?, attention_reason = 'stopped' WHERE task_id = ?`, TaskInterrupted, readyID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.RetryTask(readyID); !errors.Is(err, ErrTaskRunClosed) {
		t.Fatalf("retry under a stopped run = %v, want ErrTaskRunClosed", err)
	}
	if _, err := st.DB().Exec(`UPDATE tasks SET state = ? WHERE task_id = ?`, TaskArmed, readyID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.RearmTask(readyID, []TaskArm{{Kind: ArmSignal, SignalName: "ready"}}); !errors.Is(err, ErrTaskRunClosed) {
		t.Fatalf("rearm under a stopped run = %v, want ErrTaskRunClosed", err)
	}
}

// TS-09.R40, INV §2/§8 — two successive stage tasks can share an agent and
// generation on a borrowed runtime, so an unordered assignee lookup could return
// an already-closed predecessor and live attention would be dropped. The cursor
// is the newest attempt of the newest stage, and acceptance marks that stage
// `closing` while turn-end release still acts on it, so the cursor is not
// filtered to open stages.
func TestStageCursorLookupsAreOrderedAndNotOpenOnly(t *testing.T) {
	st, _ := newTestStore(t)
	runID, _ := st.NewPipelineRunID()
	now := time.Now().UTC()
	firstID, _ := st.NewTaskID()
	if _, _, err := st.CreatePipelineRun(CreatePipelineRunParams{Run: PipelineRunRecord{
		RunID: runID, TemplateID: "quality", TemplateSnapshot: json.RawMessage(`{"version":2}`),
		DisplayName: "Quality", Project: "proj", Goal: "ship", Inputs: json.RawMessage(`{}`), Assignments: json.RawMessage(`{}`),
		State: "queued", Revision: 1, PendingAction: "dispatch_stage_task", CurrentStageID: "implement", CreatedAt: now, UpdatedAt: now,
	}, RequestID: "cursor-order", RequestHash: "hash", InitialStageTask: &CreatePipelineStageTaskParams{
		RunID: runID, ExpectedRevision: 1, StageIndex: 0, AttemptNumber: 1, StageID: "implement", Task: Task{
			TaskID: firstID, Project: "proj", DisplayName: "Implement", Instruction: "implement", TargetKind: TargetLaunch, Role: "orchestrator", CreatedByKind: "pipeline",
		},
	}}); err != nil {
		t.Fatal(err)
	}
	// The first stage is accepted and closing; its successor borrows the same
	// runtime, so both rows carry the same agent and generation.
	if _, err := st.DB().Exec(`UPDATE pipeline_stage_tasks SET state = 'closing', standing_agent_id = 'owner' WHERE task_id = ?`, firstID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`UPDATE tasks SET state = ?, outcome = ?, assigned_agent_id = 'owner', assigned_generation = 'gen' WHERE task_id = ?`, TaskFinished, OutcomeSuccess, firstID); err != nil {
		t.Fatal(err)
	}
	secondID, _ := st.NewTaskID()
	if _, err := st.DB().Exec(`
INSERT INTO tasks(task_id, project, display_name, instruction, target_kind, state, created_by_kind, assigned_agent_id, assigned_generation, created_at, updated_at)
VALUES (?, 'proj', 'Review', 'review', 'agent', ?, 'pipeline', 'owner', 'gen', ?, ?)`,
		secondID, TaskRunning, formatTime(now), formatTime(now)); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`
INSERT INTO pipeline_stage_tasks(run_id, stage_index, attempt_number, stage_id, task_id, standing_agent_id, state, created_at)
VALUES (?, 1, 1, 'review', ?, 'owner', 'open', ?)`, runID, secondID, formatTime(now)); err != nil {
		t.Fatal(err)
	}

	stage, task, err := st.PipelineStageTaskForAssignee("owner", "gen")
	if err != nil || stage.TaskID != secondID || task.TaskID != secondID {
		t.Fatalf("assignee lookup = %#v / %#v, %v; want the current cursor", stage, task, err)
	}
	latest, err := st.LatestPipelineStageTask(runID)
	if err != nil || latest.TaskID != secondID {
		t.Fatalf("LatestPipelineStageTask = %#v, %v", latest, err)
	}

	// A run whose only stage is closing still has a cursor: turn-end release and
	// progression act on exactly that row.
	if _, err := st.DB().Exec(`DELETE FROM pipeline_stage_tasks WHERE task_id = ?`, secondID); err != nil {
		t.Fatal(err)
	}
	closing, err := st.LatestPipelineStageTask(runID)
	if err != nil || closing.TaskID != firstID || closing.State != "closing" {
		t.Fatalf("closing-stage cursor = %#v, %v; want the closing stage retained", closing, err)
	}
}

// INV §2, TS-09.R37 — five statements re-spelled the task persistence shape with
// differing creator and ready-time columns, so any schema or default change had
// to find all of them. One transaction-local insert now serves every pipeline
// construction path while each keeps its own explicit lifecycle and ownership.
func TestPipelineTaskConstructionSharesOnePersistenceShape(t *testing.T) {
	st, _ := newTestStore(t)
	runID, _ := st.NewPipelineRunID()
	now := time.Now().UTC()
	firstID, _ := st.NewTaskID()
	if _, _, err := st.CreatePipelineRun(CreatePipelineRunParams{Run: PipelineRunRecord{
		RunID: runID, TemplateID: "quality", TemplateSnapshot: json.RawMessage(`{"version":2}`),
		DisplayName: "Quality", Project: "proj", Goal: "ship", Inputs: json.RawMessage(`{}`), Assignments: json.RawMessage(`{}`),
		State: "queued", Revision: 1, PendingAction: "dispatch_stage_task", CurrentStageID: "implement", CreatedAt: now, UpdatedAt: now,
	}, RequestID: "construction", RequestHash: "hash", InitialStageTask: &CreatePipelineStageTaskParams{
		RunID: runID, ExpectedRevision: 1, StageIndex: 0, AttemptNumber: 1, StageID: "implement",
		Task: Task{TaskID: firstID, Project: "proj", DisplayName: "Implement", Instruction: "implement",
			TargetKind: TargetLaunch, Role: "orchestrator", Backend: "claude", Model: "sonnet", CreatedByKind: "pipeline"},
		Coordinator: &Task{TaskID: "tk_coord", Project: "proj", DisplayName: "Coordinator", Instruction: "coordinate",
			TargetKind: TargetLaunch, Role: "implementer", Backend: "codex", Model: "gpt", CreatedByKind: "pipeline_coordinator"},
	}}); err != nil {
		t.Fatal(err)
	}
	// A continuation attempt on the same stage, with its own coordinator. The
	// first attempt is closed first, exactly as acceptance closes it.
	if err := st.ClosePipelineStageTask(firstID, 2); err != nil {
		t.Fatal(err)
	}
	created, err := st.ReadPipelineRun(runID)
	if err != nil {
		t.Fatal(err)
	}
	continuationID, _ := st.NewTaskID()
	if _, _, _, err := st.CreatePipelineStageTask(CreatePipelineStageTaskParams{
		RunID: runID, ExpectedRevision: created.Revision, StageIndex: 0, AttemptNumber: 2, StageID: "implement",
		ParentTaskID: firstID,
		Task: Task{TaskID: continuationID, Project: "proj", DisplayName: "Implement", Instruction: "again",
			TargetKind: TargetLaunch, Role: "orchestrator", Backend: "claude", Model: "sonnet", CreatedByKind: "pipeline"},
		Coordinator: &Task{TaskID: "tk_coord2", Project: "proj", DisplayName: "Coordinator", Instruction: "coordinate",
			TargetKind: TargetLaunch, Role: "implementer", Backend: "codex", Model: "gpt", CreatedByKind: "pipeline_coordinator"},
	}); err != nil {
		t.Fatal(err)
	}
	// A replacement successor, armed until its activation intent is replayed.
	if _, err := st.DB().Exec(`UPDATE tasks SET state = ? WHERE task_id = ?`, TaskInterrupted, continuationID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`UPDATE pipeline_runs SET state = 'paused', pending_action = '' WHERE run_id = ?`, runID); err != nil {
		t.Fatal(err)
	}
	run, err := st.ReadPipelineRun(runID)
	if err != nil {
		t.Fatal(err)
	}
	replacementID, _ := st.NewTaskID()
	if _, err := st.PreparePipelineStageReplacement(CreatePipelineStageTaskParams{
		RunID: runID, ExpectedRevision: run.Revision, StageIndex: 0, AttemptNumber: 3, StageID: "implement",
		ParentTaskID: continuationID,
		Task: Task{TaskID: replacementID, Project: "proj", DisplayName: "Implement", Instruction: "replace",
			TargetKind: TargetLaunch, Role: "orchestrator", Backend: "codex", Model: "gpt", CreatedByKind: "pipeline"},
	}, continuationID); err != nil {
		t.Fatal(err)
	}

	for _, want := range []struct {
		taskID, state, createdBy string
		ready                    bool
	}{
		{firstID, TaskReady, "pipeline", true},
		{"tk_coord", TaskArmed, "pipeline_coordinator", false},
		// Replacement finished this attempt as host-cancelled; its ready time and
		// creation provenance are untouched.
		{continuationID, TaskFinished, "pipeline", true},
		{"tk_coord2", TaskArmed, "pipeline_coordinator", false},
		{replacementID, TaskArmed, "pipeline", false},
	} {
		task, err := st.ReadTask(want.taskID)
		if err != nil {
			t.Fatalf("ReadTask(%s): %v", want.taskID, err)
		}
		if task.State != want.state || task.CreatedByKind != want.createdBy || task.AttentionReason != "" {
			t.Fatalf("%s = %#v", want.taskID, task)
		}
		if (task.ReadyAt != nil) != want.ready {
			t.Fatalf("%s ready_at set = %v, want %v", want.taskID, task.ReadyAt != nil, want.ready)
		}
		if task.Project != "proj" || task.Role == "" || task.Backend == "" || task.Model == "" {
			t.Fatalf("%s lost a persisted column: %#v", want.taskID, task)
		}
	}
}

func TestTaskResultOutputsAreImmutable(t *testing.T) {
	st, _ := newTestStore(t)
	task := newTask(t, st, "proj", "outputs")
	if _, err := st.DB().Exec(`UPDATE tasks SET state = ?, assigned_agent_id = ?, assigned_generation = ? WHERE task_id = ?`, TaskRunning, "a", "g", task.TaskID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.RecordAgentTaskResult(task.TaskID, "a", "g", "", TaskResult{Outcome: OutcomeSuccess, Summary: "done", Outputs: map[string]string{"artifact": "ok"}}); err != nil {
		t.Fatalf("RecordAgentTaskResult: %v", err)
	}
	outputs, err := st.ReadTaskResultOutputs(task.TaskID)
	if err != nil || outputs["artifact"] != "ok" {
		t.Fatalf("outputs = %#v, %v", outputs, err)
	}
}
