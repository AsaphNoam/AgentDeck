package state

import (
	"errors"
	"testing"
	"time"
)

// newTask creates a task with the given arms, failing the test if the store
// rejects it. Every task here is person-created and targets an existing agent
// unless the caller says otherwise.
func newTask(t *testing.T, st *Store, project, name string, arms ...TaskArm) Task {
	t.Helper()
	id, err := st.NewTaskID()
	if err != nil {
		t.Fatalf("NewTaskID: %v", err)
	}
	task, err := st.CreateTask(Task{
		TaskID: id, Project: project, DisplayName: name, Instruction: "do " + name,
		TargetKind: TargetLaunch, Role: "impl", Backend: "claude-acp", Model: "sonnet",
		CreatedByKind: "person", Arms: arms,
	})
	if err != nil {
		t.Fatalf("CreateTask %s: %v", name, err)
	}
	return task
}

func taskArm(sourceID string, outcomes ...string) TaskArm {
	return TaskArm{
		Kind: ArmWorkResult, SourceKind: SourceTask, SourceID: sourceID,
		SatisfyingOutcomes: outcomes,
	}
}

// FS-16.R5 / TS-10.R16 — a task with no arms is ready the moment it is created,
// and one with an unsatisfied arm is armed. The arms round-trip with their
// satisfying set intact.
func TestCreateTaskArmsDecideTheInitialState(t *testing.T) {
	st, _ := newTestStore(t)

	free := newTask(t, st, "my-app", "free")
	if free.State != TaskReady {
		t.Fatalf("task with no arms = %s, want %s", free.State, TaskReady)
	}
	if free.ReadyAt == nil {
		t.Fatal("a ready task has no admission order")
	}

	dependent := newTask(t, st, "my-app", "dependent", taskArm(free.TaskID, OutcomeSuccess, OutcomeBlocked))
	if dependent.State != TaskArmed {
		t.Fatalf("task with an unsatisfied arm = %s, want %s", dependent.State, TaskArmed)
	}
	if dependent.ReadyAt != nil {
		t.Fatal("an armed task already carries an admission order")
	}
	if len(dependent.Arms) != 1 {
		t.Fatalf("arms = %+v, want one", dependent.Arms)
	}
	arm := dependent.Arms[0]
	if arm.State != ArmUnsatisfied {
		t.Fatalf("arm state = %s, want %s", arm.State, ArmUnsatisfied)
	}
	if got := arm.SatisfyingOutcomes; len(got) != 2 || got[0] != OutcomeBlocked || got[1] != OutcomeSuccess {
		t.Fatalf("satisfying outcomes = %v, want the sorted set", got)
	}
}

// FS-16.R15 / TS-10.R9 — every graph check runs inside the insert transaction,
// and a rejected create leaves nothing behind. A cycle is the case a check
// outside the transaction cannot make safe: two writers each see an acyclic
// graph and commit the edge that closes the loop.
func TestCreateTaskRejectsUnusableArmsAtomically(t *testing.T) {
	st, _ := newTestStore(t)

	first := newTask(t, st, "my-app", "first")
	second := newTask(t, st, "my-app", "second", taskArm(first.TaskID, OutcomeSuccess))
	elsewhere := newTask(t, st, "other-app", "elsewhere")

	selfID, err := st.NewTaskID()
	if err != nil {
		t.Fatalf("NewTaskID: %v", err)
	}
	cases := []struct {
		name string
		arm  TaskArm
		want error
	}{
		{"names itself", taskArm(selfID, OutcomeSuccess), ErrTaskCycle},
		{"closes a loop", taskArm(second.TaskID, OutcomeSuccess), ErrTaskCycle},
		{"unknown source", taskArm("tk_missing", OutcomeSuccess), ErrTaskArmSource},
		{"cross-project source", taskArm(elsewhere.TaskID, OutcomeSuccess), ErrTaskArmSource},
		{"empty satisfying set", taskArm(first.TaskID), ErrTaskArmSource},
		{"signal with no name", TaskArm{Kind: ArmSignal}, ErrTaskArmSource},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// "closes a loop" reuses first's id so the walk second→first→this
			// reaches back: first must be the task under test.
			id := selfID
			if tc.name == "closes a loop" {
				id = first.TaskID
			}
			_, err := st.CreateTask(Task{
				TaskID: id, Project: "my-app", DisplayName: "rejected", Instruction: "no",
				TargetKind: TargetLaunch, CreatedByKind: "person", Arms: []TaskArm{tc.arm},
			})
			if !errors.Is(err, tc.want) {
				t.Fatalf("CreateTask = %v, want %v", err, tc.want)
			}
		})
	}

	// Nothing was written: the two originals plus the one in the other project.
	mine, err := st.ListTasks("my-app")
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(mine) != 2 {
		t.Fatalf("tasks after six rejected creates = %d, want the 2 originals", len(mine))
	}
	var arms int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM task_arms`).Scan(&arms); err != nil {
		t.Fatalf("count arms: %v", err)
	}
	if arms != 1 {
		t.Fatalf("task_arms rows = %d, want only second's one arm", arms)
	}
}

// FS-16.R2 / TS-10.R18 — one active task per agent is a database guarantee. The
// exclusive lifecycle claim is released after each transition, so it cannot hold
// this on its own: two tasks admitted for one agent must leave exactly one
// active, and the agent frees up only when the first is no longer starting or
// running.
func TestActiveAssignmentIsExclusivePerAgent(t *testing.T) {
	st, _ := newTestStore(t)
	first := newTask(t, st, "my-app", "first")
	second := newTask(t, st, "my-app", "second")

	reserve := func(taskID, state string) error {
		_, err := st.DB().Exec(
			`UPDATE tasks SET assigned_agent_id = ?, state = ? WHERE task_id = ?`,
			"a_worker", state, taskID)
		return err
	}
	if err := reserve(first.TaskID, TaskStarting); err != nil {
		t.Fatalf("first reservation: %v", err)
	}
	if err := reserve(second.TaskID, TaskStarting); err == nil {
		t.Fatal("two tasks reserved the same agent; the exclusive index is missing")
	}

	// Confirming the winner into running keeps the claim.
	if err := reserve(first.TaskID, TaskRunning); err != nil {
		t.Fatalf("confirm first: %v", err)
	}
	if err := reserve(second.TaskID, TaskRunning); err == nil {
		t.Fatal("a second task became running for an agent that already has one")
	}

	// Finishing the first releases the agent without clearing its provenance:
	// the assignee is retained, and the index no longer covers the row.
	if _, err := st.DB().Exec(`UPDATE tasks SET state = ? WHERE task_id = ?`, TaskFinished, first.TaskID); err != nil {
		t.Fatalf("finish first: %v", err)
	}
	if err := reserve(second.TaskID, TaskStarting); err != nil {
		t.Fatalf("second reservation after the first finished: %v", err)
	}
	done, err := st.ReadTask(first.TaskID)
	if err != nil {
		t.Fatalf("ReadTask: %v", err)
	}
	if done.AssignedAgentID != "a_worker" {
		t.Fatalf("finished task assignee = %q, want the durable provenance", done.AssignedAgentID)
	}
}

// TS-10.R8 — the result layer is keyed per source and immutable once written, so
// an arm can never be re-decided by a second registration, and it holds no
// foreign key into the domain that produced it.
func TestWorkResultRegistrationIsUniqueAndImmutable(t *testing.T) {
	st, _ := newTestStore(t)

	if err := st.RegisterWorkResult(WorkResult{
		SourceKind: SourcePipelineRun, SourceID: "pr_1", Outcome: OutcomeSuccess,
		RawLabel: "shipped", Summary: "all stages passed",
	}); err != nil {
		t.Fatalf("RegisterWorkResult: %v", err)
	}
	err := st.RegisterWorkResult(WorkResult{
		SourceKind: SourcePipelineRun, SourceID: "pr_1", Outcome: OutcomeFailure,
	})
	if !errors.Is(err, ErrWorkResultRecorded) {
		t.Fatalf("second registration = %v, want %v", err, ErrWorkResultRecorded)
	}

	got, err := st.ReadWorkResult(SourcePipelineRun, "pr_1")
	if err != nil {
		t.Fatalf("ReadWorkResult: %v", err)
	}
	if got.Outcome != OutcomeSuccess || got.RawLabel != "shipped" {
		t.Fatalf("result after a rejected overwrite = %+v, want the original", got)
	}

	// The same id under the other source kind is a different result entirely.
	if err := st.RegisterWorkResult(WorkResult{
		SourceKind: SourceTask, SourceID: "pr_1", Outcome: OutcomeFailure,
	}); err != nil {
		t.Fatalf("RegisterWorkResult for the other source kind: %v", err)
	}
	if _, err := st.ReadWorkResult(SourceTask, "tk_absent"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ReadWorkResult for an unregistered source = %v, want %v", err, ErrNotFound)
	}
}

// TS-10.R10 — task state advances monotonically under compare-and-set. A writer
// holding a stale revision loses instead of overwriting a newer transition, and
// a finished task is terminal: recovery may resume a pending transition but
// never reopens a recorded outcome.
func TestUpdateTaskCASRejectsStaleWritersAndFinishedTasks(t *testing.T) {
	st, _ := newTestStore(t)
	task := newTask(t, st, "my-app", "cas")
	stale := task.Revision

	reason := "waiting for capacity"
	advanced, err := st.UpdateTaskCAS(task.TaskID, stale, TaskUpdate{
		State: TaskStarting, AttentionReason: &reason,
	})
	if err != nil {
		t.Fatalf("UpdateTaskCAS: %v", err)
	}
	if advanced.Revision != stale+1 {
		t.Fatalf("revision = %d, want %d", advanced.Revision, stale+1)
	}
	if advanced.AttentionReason != reason {
		t.Fatalf("attention reason = %q, want %q", advanced.AttentionReason, reason)
	}

	if _, err := st.UpdateTaskCAS(task.TaskID, stale, TaskUpdate{State: TaskReady}); !errors.Is(err, ErrTaskConflict) {
		t.Fatalf("stale writer = %v, want %v", err, ErrTaskConflict)
	}
	if current, err := st.ReadTask(task.TaskID); err != nil || current.State != TaskStarting {
		t.Fatalf("state after a lost race = %+v, %v; want %s untouched", current.State, err, TaskStarting)
	}

	outcome, source := OutcomeSuccess, "agent"
	now := time.Now().UTC()
	finished, err := st.UpdateTaskCAS(task.TaskID, advanced.Revision, TaskUpdate{
		State: TaskFinished, Outcome: &outcome, OutcomeSource: &source, FinishedAt: &now,
	})
	if err != nil {
		t.Fatalf("finish: %v", err)
	}
	if finished.Outcome != OutcomeSuccess || finished.FinishedAt == nil {
		t.Fatalf("finished task = %+v, want a recorded outcome", finished)
	}
	if _, err := st.UpdateTaskCAS(task.TaskID, finished.Revision, TaskUpdate{State: TaskReady}); !errors.Is(err, ErrTaskConflict) {
		t.Fatalf("reopening a finished task = %v, want %v", err, ErrTaskConflict)
	}

	if _, err := st.UpdateTaskCAS("tk_absent", 1, TaskUpdate{State: TaskReady}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("CAS on an unknown task = %v, want %v", err, ErrNotFound)
	}
}

// TS-10.R16 — arms and attachments cascade from their task, while agent,
// pipeline-run, and context-reference ids are logical references. Deleting a
// task must not need, or reach, any of those domains.
func TestDeletingATaskCascadesOnlyItsOwnRows(t *testing.T) {
	st, _ := newTestStore(t)
	task := newTask(t, st, "my-app", "attached")
	if _, err := st.DB().Exec(`
INSERT INTO task_attachments(task_id, context_ref_id, label, description, created_at)
VALUES(?, ?, ?, ?, ?)`, task.TaskID, "ctxref_1", "the plan", "", formatTime(time.Now().UTC())); err != nil {
		t.Fatalf("attach: %v", err)
	}
	dependent := newTask(t, st, "my-app", "dependent", taskArm(task.TaskID, OutcomeSuccess))

	if _, err := st.DB().Exec(`DELETE FROM tasks WHERE task_id = ?`, task.TaskID); err != nil {
		t.Fatalf("delete task: %v", err)
	}
	var attachments, orphanArms int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM task_attachments`).Scan(&attachments); err != nil {
		t.Fatalf("count attachments: %v", err)
	}
	if attachments != 0 {
		t.Fatalf("attachments after delete = %d, want cascaded away", attachments)
	}
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM task_arms WHERE task_id = ?`, dependent.TaskID).Scan(&orphanArms); err != nil {
		t.Fatalf("count dependent arms: %v", err)
	}
	if orphanArms != 1 {
		t.Fatalf("dependent's arm rows = %d, want its arm to survive its source's deletion", orphanArms)
	}
	if _, err := st.ReadTask(dependent.TaskID); err != nil {
		t.Fatalf("dependent after its prerequisite was deleted: %v", err)
	}
}
