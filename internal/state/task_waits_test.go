package state

import (
	"errors"
	"testing"
	"time"
)

func TestTaskWaitYieldsAndWakesAfterWatchedResult(t *testing.T) {
	st, _ := newTestStore(t)
	owner := newTask(t, st, "proj", "owner")
	if _, err := st.DB().Exec(`UPDATE tasks SET state = ?, assigned_agent_id = ?, assigned_generation = ?, runtime_claim = ?, execution_handle = ? WHERE task_id = ?`, TaskRunning, "a_owner", "g1", ClaimCreated, "exec1", owner.TaskID); err != nil {
		t.Fatal(err)
	}
	childID, err := st.NewTaskID()
	if err != nil {
		t.Fatal(err)
	}
	child, err := st.CreateTask(Task{TaskID: childID, Project: "proj", DisplayName: "child", Instruction: "work", TargetKind: TargetLaunch, Role: "impl", CreatedByKind: "agent", CreatedByAgentID: "a_owner"})
	if err != nil {
		t.Fatal(err)
	}
	changes, waiting, err := st.WaitForTasks("a_owner", "g1", "exec1", []TaskWaitObservation{{TaskID: child.TaskID, AfterRevision: child.Revision}})
	if err != nil || len(changes) != 0 || !waiting.PendingYield {
		t.Fatalf("wait = changes %+v task %+v err %v", changes, waiting, err)
	}
	if _, err := st.NotifyTaskWaiters(child.TaskID); err != nil {
		t.Fatal(err)
	}
	settled, err := st.CompleteTaskYield(owner.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if settled.State != TaskReady || !settled.ContinuationPending || settled.AssignedAgentID != "a_owner" || settled.RuntimeClaim != "" {
		t.Fatalf("settled wait = %+v", settled)
	}
}

func TestTaskCleanupFailureBacksOffWithoutChangingTaskOutcome(t *testing.T) {
	st, _ := newTestStore(t)
	task := newTask(t, st, "proj", "cleanup")
	if _, err := st.DB().Exec(`UPDATE tasks SET state = ?, outcome = ?, pending_release = 1, runtime_claim = ? WHERE task_id = ?`, TaskFinished, OutcomeSuccess, ClaimCreated, task.TaskID); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 13, 9, 0, 0, 0, time.UTC)
	original := timeNow
	timeNow = func() time.Time { return now }
	t.Cleanup(func() { timeNow = original })
	first, err := st.RecordTaskCleanupFailure(task.TaskID, cleanupPhaseRelease, "release:"+task.TaskID, errors.New("stop timed out"))
	if err != nil {
		t.Fatal(err)
	}
	if first.State != TaskFinished || first.Outcome != OutcomeSuccess || first.CleanupFailureCount != 1 || first.CleanupNextRetryAt == nil || !first.CleanupNextRetryAt.Equal(now.Add(2*time.Second)) {
		t.Fatalf("first cleanup failure = %+v", first)
	}
	if due, err := st.DueTaskCleanup(8); err != nil || len(due) != 0 {
		t.Fatalf("due before retry = %+v, %v", due, err)
	}
	now = now.Add(2 * time.Second)
	second, err := st.RecordTaskCleanupFailure(task.TaskID, cleanupPhaseRelease, "release:"+task.TaskID, errors.New("stop timed out"))
	if err != nil {
		t.Fatal(err)
	}
	if second.CleanupFailureCount != 2 || second.CleanupNextRetryAt == nil || !second.CleanupNextRetryAt.Equal(now.Add(4*time.Second)) {
		t.Fatalf("second cleanup failure = %+v", second)
	}
	if err := st.CompleteTaskRelease(task.TaskID); err != nil {
		t.Fatal(err)
	}
	settled, err := st.ReadTask(task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if settled.CleanupFailureCount != 0 || settled.CleanupNextRetryAt != nil || settled.CleanupLastError != "" || settled.RuntimeClaim != "" {
		t.Fatalf("settled cleanup = %+v", settled)
	}
}

func TestTaskWaitReturnsAlreadyChangedTaskWithoutYield(t *testing.T) {
	st, _ := newTestStore(t)
	owner := newTask(t, st, "proj", "owner")
	if _, err := st.DB().Exec(`UPDATE tasks SET state = ?, assigned_agent_id = ?, assigned_generation = ?, execution_handle = ? WHERE task_id = ?`, TaskRunning, "a_owner", "g1", "exec1", owner.TaskID); err != nil {
		t.Fatal(err)
	}
	childID, _ := st.NewTaskID()
	child, err := st.CreateTask(Task{TaskID: childID, Project: "proj", DisplayName: "child", Instruction: "work", TargetKind: TargetLaunch, Role: "impl", CreatedByKind: "agent", CreatedByAgentID: "a_owner"})
	if err != nil {
		t.Fatal(err)
	}
	changes, current, err := st.WaitForTasks("a_owner", "g1", "exec1", []TaskWaitObservation{{TaskID: child.TaskID, AfterRevision: 0}})
	if err != nil || len(changes) != 1 || current.PendingYield {
		t.Fatalf("changed wait = %+v %+v %v", changes, current, err)
	}
}

func TestTaskWaitRefusesSameAssigneeAndCancellationWinsYield(t *testing.T) {
	st, _ := newTestStore(t)
	owner := newTask(t, st, "proj", "owner")
	if _, err := st.DB().Exec(`UPDATE tasks SET state = ?, assigned_agent_id = ?, assigned_generation = ?, runtime_claim = ?, execution_handle = ? WHERE task_id = ?`, TaskRunning, "a_owner", "g1", ClaimBorrowed, "exec1", owner.TaskID); err != nil {
		t.Fatal(err)
	}
	childID, _ := st.NewTaskID()
	child, err := st.CreateTask(Task{TaskID: childID, Project: "proj", DisplayName: "same agent", Instruction: "work", TargetKind: TargetAgent, TargetAgentID: "a_owner", CreatedByKind: "agent", CreatedByAgentID: "a_owner"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.WaitForTasks("a_owner", "g1", "exec1", []TaskWaitObservation{{TaskID: child.TaskID, AfterRevision: child.Revision}}); !errors.Is(err, ErrTaskWaitConflict) {
		t.Fatalf("same-assignee wait error = %v, want wait conflict", err)
	}

	child.TargetAgentID = ""
	if _, err := st.DB().Exec(`UPDATE tasks SET target_agent_id = '' WHERE task_id = ?`, child.TaskID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.WaitForTasks("a_owner", "g1", "exec1", []TaskWaitObservation{{TaskID: child.TaskID, AfterRevision: child.Revision}}); err != nil {
		t.Fatal(err)
	}
	finished, err := st.CancelTask(owner.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if finished.State != TaskFinished || finished.PendingYield {
		t.Fatalf("cancelled pending yield = %+v", finished)
	}
	if _, err := st.CompleteTaskYield(owner.TaskID); !errors.Is(err, ErrTaskWaitConflict) {
		t.Fatalf("yield after cancellation error = %v, want wait conflict", err)
	}
}

func TestConfirmTaskStartDiscardsPriorWaits(t *testing.T) {
	st, _ := newTestStore(t)
	owner := newTask(t, st, "proj", "owner")
	child := newTask(t, st, "proj", "child")
	if _, err := st.DB().Exec(`UPDATE tasks SET created_by_kind = 'agent', created_by_agent_id = 'a_owner' WHERE task_id = ?`, child.TaskID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`UPDATE tasks SET state = ?, assigned_agent_id = ?, assigned_generation = ?, runtime_claim = ?, execution_handle = ?, wait_version = 1 WHERE task_id = ?`, TaskRunning, "a_owner", "g1", ClaimBorrowed, "exec1", owner.TaskID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.WaitForTasks("a_owner", "g1", "exec1", []TaskWaitObservation{{TaskID: child.TaskID, AfterRevision: child.Revision}}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`UPDATE tasks SET state = ?, pending_yield = 0, continuation_pending = 1, resume_needed = 1, start_attempt_id = ?, assigned_generation = ? WHERE task_id = ?`, TaskStarting, "next", "g2", owner.TaskID); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := st.ConfirmTaskStart(owner.TaskID, "next"); err != nil || !ok {
		t.Fatalf("confirm = ok %v err %v", ok, err)
	}
	var count int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM task_waits WHERE waiting_task_id = ?`, owner.TaskID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("prior waits remain after confirmation: %d", count)
	}
}
