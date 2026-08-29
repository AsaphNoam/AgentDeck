package state

import (
	"errors"
	"fmt"
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

// FS-16.R10 / TS-10.R12 — creating follow-on work accepts every effective
// context authorization path, including the durable work-derived path rather
// than only a direct grant.
func TestCreateTaskAttachmentsAcceptsWorkDerivedAuthorization(t *testing.T) {
	st, _ := newTestStore(t)
	contextAgent(t, st, "a_src", "source")
	contextAgent(t, st, "a_creator", "creator")
	contextAgent(t, st, "a_other", "other")
	ref, _, err := st.ShareContext(transcriptSpan("a_src", 1, 2), "a_src", "a_other", "", "")
	if err != nil {
		t.Fatalf("ShareContext: %v", err)
	}

	owner := newTask(t, st, "proj", "owner")
	if err := st.AttachTaskContext(owner.TaskID, []TaskAttachment{{ContextRefID: ref.ContextRefID}}); err != nil {
		t.Fatalf("AttachTaskContext: %v", err)
	}
	if _, err := st.DB().Exec(`UPDATE tasks SET assigned_agent_id = ?, started_at = ? WHERE task_id = ?`, "a_creator", formatTime(timeNow()), owner.TaskID); err != nil {
		t.Fatalf("make confirmed assignee: %v", err)
	}
	if authorized, err := st.ContextReadAuthorized(ref.ContextRefID, "a_creator"); err != nil || !authorized {
		t.Fatalf("work-derived ContextReadAuthorized = %v, %v", authorized, err)
	}

	id, err := st.NewTaskID()
	if err != nil {
		t.Fatalf("NewTaskID: %v", err)
	}
	created, err := st.CreateTaskWithAttachments(Task{
		TaskID: id, Project: "proj", DisplayName: "follow-up", Instruction: "use the context",
		TargetKind: TargetLaunch, Role: "impl", CreatedByKind: "agent", CreatedByAgentID: "a_creator",
	}, []TaskAttachment{{ContextRefID: ref.ContextRefID, Label: "prior work"}}, "a_creator")
	if err != nil {
		t.Fatalf("CreateTaskWithAttachments: %v", err)
	}
	attachments, err := st.ListTaskAttachments(created.TaskID)
	if err != nil || len(attachments) != 1 || attachments[0].ContextRefID != ref.ContextRefID {
		t.Fatalf("attachments = %+v, %v", attachments, err)
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

// FS-16.R5 / TS-10.R3 — fan-in. A task waiting on three prerequisites becomes
// ready exactly once, when the last one registers a satisfying result, and never
// before. Each registration re-evaluates only the arms naming that source.
func TestFanInBecomesReadyOnlyOnTheLastArm(t *testing.T) {
	st, _ := newTestStore(t)
	a := newTask(t, st, "my-app", "a")
	b := newTask(t, st, "my-app", "b")
	c := newTask(t, st, "my-app", "c")
	join := newTask(t, st, "my-app", "join",
		taskArm(a.TaskID, OutcomeSuccess), taskArm(b.TaskID, OutcomeSuccess),
		taskArm(c.TaskID, OutcomeSuccess, OutcomeBlocked))

	for i, src := range []Task{a, b} {
		if err := st.RegisterWorkResult(WorkResult{
			SourceKind: SourceTask, SourceID: src.TaskID, Outcome: OutcomeSuccess,
		}); err != nil {
			t.Fatalf("register %d: %v", i, err)
		}
		changed, err := st.EvaluateSource(SourceTask, src.TaskID)
		if err != nil {
			t.Fatalf("EvaluateSource %d: %v", i, err)
		}
		if len(changed) != 0 {
			t.Fatalf("task advanced after %d of 3 arms: %+v", i+1, changed)
		}
		current, err := st.ReadTask(join.TaskID)
		if err != nil {
			t.Fatalf("ReadTask: %v", err)
		}
		if current.State != TaskArmed {
			t.Fatalf("state after %d of 3 arms = %s, want %s", i+1, current.State, TaskArmed)
		}
	}

	if err := st.RegisterWorkResult(WorkResult{
		SourceKind: SourceTask, SourceID: c.TaskID, Outcome: OutcomeBlocked,
	}); err != nil {
		t.Fatalf("register c: %v", err)
	}
	changed, err := st.EvaluateSource(SourceTask, c.TaskID)
	if err != nil {
		t.Fatalf("EvaluateSource c: %v", err)
	}
	if len(changed) != 1 || changed[0].TaskID != join.TaskID || changed[0].State != TaskReady {
		t.Fatalf("last arm produced %+v, want join ready", changed)
	}
	if changed[0].ReadyAt == nil {
		t.Fatal("a ready task carries no admission order")
	}

	// Re-running evaluation is the startup sweep's behaviour: it must not
	// advance the task a second time or move it off ready.
	again, err := st.EvaluateSource(SourceTask, c.TaskID)
	if err != nil {
		t.Fatalf("re-evaluate: %v", err)
	}
	if len(again) != 0 {
		t.Fatalf("re-evaluation advanced %+v, want nothing", again)
	}
	final, err := st.ReadTask(join.TaskID)
	if err != nil {
		t.Fatalf("ReadTask: %v", err)
	}
	if final.State != TaskReady || final.Revision != changed[0].Revision {
		t.Fatalf("task after a repeated evaluation = %s rev %d, want %s rev %d",
			final.State, final.Revision, TaskReady, changed[0].Revision)
	}
}

// FS-16.R8 — an outcome outside an arm's satisfying set parks the dependent
// instead of leaving it waiting forever, and parking wins over readiness: a task
// whose other arms are all satisfied still parks. Nothing is silently cancelled.
func TestUnsatisfiableArmParksTheDependent(t *testing.T) {
	st, _ := newTestStore(t)
	good := newTask(t, st, "my-app", "good")
	bad := newTask(t, st, "my-app", "bad")
	dependent := newTask(t, st, "my-app", "dependent",
		taskArm(good.TaskID, OutcomeSuccess), taskArm(bad.TaskID, OutcomeSuccess))

	if err := st.RegisterWorkResult(WorkResult{
		SourceKind: SourceTask, SourceID: good.TaskID, Outcome: OutcomeSuccess,
	}); err != nil {
		t.Fatalf("register good: %v", err)
	}
	if _, err := st.EvaluateSource(SourceTask, good.TaskID); err != nil {
		t.Fatalf("EvaluateSource good: %v", err)
	}
	if err := st.RegisterWorkResult(WorkResult{
		SourceKind: SourceTask, SourceID: bad.TaskID, Outcome: OutcomeFailure,
	}); err != nil {
		t.Fatalf("register bad: %v", err)
	}
	changed, err := st.EvaluateSource(SourceTask, bad.TaskID)
	if err != nil {
		t.Fatalf("EvaluateSource bad: %v", err)
	}
	if len(changed) != 1 || changed[0].TaskID != dependent.TaskID || changed[0].State != TaskDependencyFailed {
		t.Fatalf("failing prerequisite produced %+v, want the dependent parked", changed)
	}
	if changed[0].AttentionReason == "" {
		t.Fatal("a parked task surfaces no reason")
	}
	states := map[string]string{}
	for _, arm := range changed[0].Arms {
		states[arm.SourceID] = arm.State
	}
	if states[good.TaskID] != ArmSatisfied || states[bad.TaskID] != ArmUnsatisfiable {
		t.Fatalf("arm states = %v, want the satisfied one kept and the other unsatisfiable", states)
	}

	// A prerequisite that is itself parked, or deleted, makes the same arms
	// unsatisfiable without any registered result.
	other := newTask(t, st, "my-app", "other")
	waiting := newTask(t, st, "my-app", "waiting", taskArm(other.TaskID, OutcomeSuccess))
	parked, err := st.MarkSourceUnsatisfiable(SourceTask, other.TaskID)
	if err != nil {
		t.Fatalf("MarkSourceUnsatisfiable: %v", err)
	}
	if len(parked) != 1 || parked[0].TaskID != waiting.TaskID || parked[0].State != TaskDependencyFailed {
		t.Fatalf("parking a prerequisite produced %+v, want the dependent parked", parked)
	}
}

// FS-16.R9 — a signal is a named release, not a stored object. Firing satisfies
// every arm waiting on that name in that project at that moment; firing a name
// nothing waits on succeeds and changes nothing; and another project's arm on the
// same name is untouched.
func TestFiringASignalReleasesOnlyItsProjectsArms(t *testing.T) {
	st, _ := newTestStore(t)
	mine := newTask(t, st, "my-app", "mine", TaskArm{Kind: ArmSignal, SignalName: "ci-green"})
	theirs := newTask(t, st, "other-app", "theirs", TaskArm{Kind: ArmSignal, SignalName: "ci-green"})

	quiet, err := st.FireSignal("my-app", "never-awaited")
	if err != nil {
		t.Fatalf("FireSignal for an unawaited name: %v", err)
	}
	if len(quiet) != 0 {
		t.Fatalf("firing an unawaited name changed %+v, want nothing", quiet)
	}

	changed, err := st.FireSignal("my-app", "ci-green")
	if err != nil {
		t.Fatalf("FireSignal: %v", err)
	}
	if len(changed) != 1 || changed[0].TaskID != mine.TaskID || changed[0].State != TaskReady {
		t.Fatalf("fired signal produced %+v, want only this project's task ready", changed)
	}
	if changed[0].Arms[0].SatisfiedAt == nil {
		t.Fatal("firing was not recorded on the arm it satisfied")
	}

	elsewhere, err := st.ReadTask(theirs.TaskID)
	if err != nil {
		t.Fatalf("ReadTask: %v", err)
	}
	if elsewhere.State != TaskArmed {
		t.Fatalf("other project's task = %s, want %s", elsewhere.State, TaskArmed)
	}

	// Firing again is harmless: the arm is already satisfied and the task has
	// already advanced.
	repeat, err := st.FireSignal("my-app", "ci-green")
	if err != nil {
		t.Fatalf("re-fire: %v", err)
	}
	if len(repeat) != 0 {
		t.Fatalf("re-firing changed %+v, want nothing", repeat)
	}
}

// reserve builds a start reservation for one agent. Every field is the caller's:
// the state layer never invents an assignee or decides how a runtime is claimed.
func reservation(attemptID, agentID, claim string) TaskReservation {
	return TaskReservation{
		AttemptID: attemptID, AgentID: agentID, Generation: "g_" + attemptID, Claim: claim,
	}
}

// FS-16.R7 / TS-10.R4, R17 — admission decides capacity and takes the claim in
// one statement. Two dispatch passes reading a free slot and then claiming it
// would both commit, so the budget has to be part of the update itself. A start
// that borrows a runtime already up creates no process and consumes no slot.
func TestAdmissionTakesCapacityInTheStatementThatGrantsIt(t *testing.T) {
	st, _ := newTestStore(t)
	first := newTask(t, st, "my-app", "first")
	second := newTask(t, st, "my-app", "second")
	third := newTask(t, st, "my-app", "third")
	borrower := newTask(t, st, "my-app", "borrower")

	admit := func(task Task, agentID, claim string) bool {
		t.Helper()
		_, ok, err := st.AdmitReadyTask(task.TaskID, reservation("at_"+task.DisplayName, agentID, claim), 2)
		if err != nil {
			t.Fatalf("AdmitReadyTask %s: %v", task.DisplayName, err)
		}
		return ok
	}
	if !admit(first, "a_one", ClaimCreated) || !admit(second, "a_two", ClaimWoke) {
		t.Fatal("the first two starts did not take the budget's two slots")
	}
	if admit(third, "a_three", ClaimCreated) {
		t.Fatal("a third runtime started with the budget set to two")
	}
	if !admit(borrower, "a_four", ClaimBorrowed) {
		t.Fatal("a borrowed runtime was refused for capacity it does not consume")
	}

	// The refused task is ready and waiting, not failed, and spent no attempt.
	waiting, err := st.ReadTask(third.TaskID)
	if err != nil {
		t.Fatalf("ReadTask: %v", err)
	}
	if waiting.State != TaskReady || waiting.StartAttemptCount != 0 {
		t.Fatalf("task waiting for capacity = %s after %d attempts, want %s after 0",
			waiting.State, waiting.StartAttemptCount, TaskReady)
	}
	if waiting.AssignedAgentID != "" {
		t.Fatalf("a task that was not admitted reserved %q", waiting.AssignedAgentID)
	}

	// A slot frees only when its task stops holding a runtime claim.
	if _, ok, err := st.AbandonTaskStart(first.TaskID, "at_first"); err != nil || !ok {
		t.Fatalf("AbandonTaskStart = %v, %v", ok, err)
	}
	if !admit(third, "a_three", ClaimCreated) {
		t.Fatal("a freed slot did not admit the waiting task")
	}
}

// FS-16.R2 / TS-10.R18 — two tasks admitted for the same agent leave exactly
// one active. The loser is refused by the durable index rather than by ordering
// luck, stays ready, and spends no attempt.
func TestAdmissionRefusesASecondTaskForOneAgent(t *testing.T) {
	st, _ := newTestStore(t)
	first := newTask(t, st, "my-app", "first")
	second := newTask(t, st, "my-app", "second")

	if _, ok, err := st.AdmitReadyTask(first.TaskID, reservation("at_1", "a_worker", ClaimBorrowed), 10); err != nil || !ok {
		t.Fatalf("first admission = %v, %v", ok, err)
	}
	_, ok, err := st.AdmitReadyTask(second.TaskID, reservation("at_2", "a_worker", ClaimBorrowed), 10)
	if err != nil {
		t.Fatalf("second admission: %v", err)
	}
	if ok {
		t.Fatal("both tasks were admitted for the same agent")
	}
	loser, err := st.ReadTask(second.TaskID)
	if err != nil {
		t.Fatalf("ReadTask: %v", err)
	}
	if loser.State != TaskReady || loser.StartAttemptCount != 0 || loser.StartAttemptID != "" {
		t.Fatalf("loser = %s with attempt %q after %d attempts, want an untouched ready task",
			loser.State, loser.StartAttemptID, loser.StartAttemptCount)
	}
}

// TS-10.R4 — running means a confirmed runtime, and every settlement is guarded
// by the attempt id that reserved it, so a worker whose attempt was already
// abandoned cannot confirm, fail, or re-abandon a newer one.
func TestOnlyTheCurrentAttemptSettlesAStart(t *testing.T) {
	st, _ := newTestStore(t)
	task := newTask(t, st, "my-app", "work")

	starting, ok, err := st.AdmitReadyTask(task.TaskID, reservation("at_1", "a_worker", ClaimCreated), 10)
	if err != nil || !ok {
		t.Fatalf("AdmitReadyTask = %v, %v", ok, err)
	}
	if starting.State != TaskStarting || starting.RuntimeClaim != ClaimCreated {
		t.Fatalf("admitted task = %s claiming %q, want %s claiming %s",
			starting.State, starting.RuntimeClaim, TaskStarting, ClaimCreated)
	}
	if starting.AssignedAgentID != "a_worker" || starting.AssignedGeneration != "g_at_1" {
		t.Fatalf("reservation = %q/%q, want the agent and generation it will act on",
			starting.AssignedAgentID, starting.AssignedGeneration)
	}
	if starting.StartedAt != nil {
		t.Fatal("a task that has not confirmed a runtime already has a start time")
	}

	if _, ok, err := st.ConfirmTaskStart(task.TaskID, "at_stale"); err != nil || ok {
		t.Fatalf("ConfirmTaskStart(stale attempt) = %v, %v; want no settlement", ok, err)
	}
	// The first attempt is given up before any effect, then re-admitted.
	if _, ok, err := st.AbandonTaskStart(task.TaskID, "at_1"); err != nil || !ok {
		t.Fatalf("AbandonTaskStart = %v, %v", ok, err)
	}
	if _, ok, err := st.AdmitReadyTask(task.TaskID, reservation("at_2", "a_worker", ClaimWoke), 10); err != nil || !ok {
		t.Fatalf("second admission = %v, %v", ok, err)
	}
	if _, ok, err := st.ConfirmTaskStart(task.TaskID, "at_1"); err != nil || ok {
		t.Fatalf("the abandoned attempt confirmed the task = %v, %v", ok, err)
	}
	running, ok, err := st.ConfirmTaskStart(task.TaskID, "at_2")
	if err != nil || !ok {
		t.Fatalf("ConfirmTaskStart = %v, %v", ok, err)
	}
	if running.State != TaskRunning || running.StartedAt == nil {
		t.Fatalf("confirmed task = %s started at %v, want %s with a start time",
			running.State, running.StartedAt, TaskRunning)
	}
	if running.StartAttemptCount != 0 {
		t.Fatalf("a start that succeeded spent %d attempts, want 0", running.StartAttemptCount)
	}
}

// FS-16.R25 / TS-10.R17 — only real failures spend an attempt, and the third
// parks the task recording that failure. A deferral in between leaves the
// allowance intact, so a busy machine can never exhaust a task's attempts.
func TestOnlyRealStartFailuresSpendTheBoundedAttempts(t *testing.T) {
	st, _ := newTestStore(t)
	task := newTask(t, st, "my-app", "work")

	fail := func(attempt, reason string) Task {
		t.Helper()
		if _, ok, err := st.AdmitReadyTask(task.TaskID, reservation(attempt, "a_worker", ClaimCreated), 10); err != nil || !ok {
			t.Fatalf("AdmitReadyTask %s = %v, %v", attempt, ok, err)
		}
		after, ok, err := st.FailTaskStart(task.TaskID, attempt, reason)
		if err != nil || !ok {
			t.Fatalf("FailTaskStart %s = %v, %v", attempt, ok, err)
		}
		return after
	}

	if after := fail("at_1", "the launch did not start"); after.State != TaskReady || after.StartAttemptCount != 1 {
		t.Fatalf("after one failure = %s after %d attempts, want %s after 1",
			after.State, after.StartAttemptCount, TaskReady)
	}

	// A deferral between two failures: admitted, then given up with no effect.
	if _, ok, err := st.AdmitReadyTask(task.TaskID, reservation("at_deferred", "a_worker", ClaimCreated), 10); err != nil || !ok {
		t.Fatalf("deferred admission = %v, %v", ok, err)
	}
	deferred, ok, err := st.AbandonTaskStart(task.TaskID, "at_deferred")
	if err != nil || !ok {
		t.Fatalf("AbandonTaskStart = %v, %v", ok, err)
	}
	if deferred.State != TaskReady || deferred.StartAttemptCount != 1 {
		t.Fatalf("after a deferral = %s after %d attempts, want %s with the allowance intact",
			deferred.State, deferred.StartAttemptCount, TaskReady)
	}
	if deferred.RuntimeClaim != "" || deferred.AssignedAgentID != "" || deferred.StartClaimedAt != nil {
		t.Fatalf("a deferred task still holds a reservation: %+v", deferred)
	}

	if after := fail("at_2", "the resume did not complete"); after.State != TaskReady {
		t.Fatalf("after two failures = %s, want %s", after.State, TaskReady)
	}
	parked := fail("at_3", "the target is archived")
	if parked.State != TaskDependencyFailed || parked.StartAttemptCount != MaxTaskStartAttempts {
		t.Fatalf("after three failures = %s after %d attempts, want %s after %d",
			parked.State, parked.StartAttemptCount, TaskDependencyFailed, MaxTaskStartAttempts)
	}
	if parked.AttentionReason != "the target is archived" {
		t.Fatalf("parked reason = %q, want the last failure", parked.AttentionReason)
	}
	if parked.Outcome != "" {
		t.Fatalf("a parked task recorded outcome %q; parking is not a result", parked.Outcome)
	}
}

// FS-16.R7 — ready work is admitted in the order it became ready, and a task
// that is not ready is never offered to the dispatcher.
func TestReadyTasksAreListedInAdmissionOrder(t *testing.T) {
	st, _ := newTestStore(t)
	first := newTask(t, st, "my-app", "first")
	second := newTask(t, st, "other-app", "second")
	third := newTask(t, st, "my-app", "third")
	armed := newTask(t, st, "my-app", "armed", taskArm(first.TaskID, OutcomeSuccess))

	for i, id := range []string{third.TaskID, first.TaskID, second.TaskID} {
		stamp := time.Date(2026, 8, 24, 10, i, 0, 0, time.UTC)
		if _, err := st.DB().Exec(`UPDATE tasks SET ready_at = ? WHERE task_id = ?`,
			formatTime(stamp), id); err != nil {
			t.Fatalf("stamp ready_at: %v", err)
		}
	}
	ready, err := st.ReadyTasks(10)
	if err != nil {
		t.Fatalf("ReadyTasks: %v", err)
	}
	var names []string
	for _, task := range ready {
		names = append(names, task.DisplayName)
	}
	if len(names) != 3 || names[0] != "third" || names[1] != "first" || names[2] != "second" {
		t.Fatalf("admission order = %v, want third, first, second across every project", names)
	}
	if _, err := st.ReadTask(armed.TaskID); err != nil {
		t.Fatalf("ReadTask: %v", err)
	}

	bounded, err := st.ReadyTasks(2)
	if err != nil {
		t.Fatalf("ReadyTasks(2): %v", err)
	}
	if len(bounded) != 2 || bounded[0].DisplayName != "third" {
		t.Fatalf("bounded pass = %d rows starting at %q, want the 2 oldest", len(bounded), bounded[0].DisplayName)
	}
}

// FS-16.A13 — under real concurrency two tasks admitted for one agent still
// leave exactly one active. The single statement is what makes this true; a read
// followed by a write would let both pass the check. Each round borrows its
// runtime so the exclusive claim is the only thing under test: a created or woken
// one would also hold a budget slot across rounds.
func TestConcurrentAdmissionForOneAgentLeavesExactlyOneActive(t *testing.T) {
	st, _ := newTestStore(t)
	for round := 0; round < 20; round++ {
		first := newTask(t, st, "my-app", "first")
		second := newTask(t, st, "my-app", "second")
		agentID := fmt.Sprintf("a_worker%02d", round)

		results := make(chan bool, 2)
		start := make(chan struct{})
		for i, task := range []Task{first, second} {
			go func(i int, task Task) {
				<-start
				_, ok, err := st.AdmitReadyTask(task.TaskID,
					reservation(fmt.Sprintf("at_%d_%d", round, i), agentID, ClaimBorrowed), 10)
				if err != nil {
					t.Errorf("AdmitReadyTask: %v", err)
				}
				results <- ok
			}(i, task)
		}
		close(start)
		admitted := 0
		for i := 0; i < 2; i++ {
			if <-results {
				admitted++
			}
		}
		if admitted != 1 {
			t.Fatalf("round %d admitted %d tasks for one agent, want exactly 1", round, admitted)
		}
	}
}

// FS-16.R21 / A9 — lowering the budget below the number of runtimes already up
// stops nothing and admits nothing until the count falls, because capacity is
// only ever consulted when admitting.
func TestLoweringTheBudgetStopsNothingAndAdmitsNothing(t *testing.T) {
	st, _ := newTestStore(t)
	var running []Task
	for i := 0; i < 3; i++ {
		task := newTask(t, st, "my-app", fmt.Sprintf("running%d", i))
		admitted, ok, err := st.AdmitReadyTask(task.TaskID,
			reservation(fmt.Sprintf("at_%d", i), fmt.Sprintf("a_%d", i), ClaimCreated), 3)
		if err != nil || !ok {
			t.Fatalf("admit %d = %v, %v", i, ok, err)
		}
		running = append(running, admitted)
	}
	waiting := newTask(t, st, "my-app", "waiting")

	// The budget is now one, below the three runtimes already up.
	if _, ok, err := st.AdmitReadyTask(waiting.TaskID, reservation("at_w", "a_w", ClaimCreated), 1); err != nil || ok {
		t.Fatalf("admission under a lowered budget = %v, %v; want refused", ok, err)
	}
	for _, task := range running {
		still, err := st.ReadTask(task.TaskID)
		if err != nil {
			t.Fatalf("ReadTask: %v", err)
		}
		if still.State != TaskStarting || still.RuntimeClaim == "" {
			t.Fatalf("lowering the budget disturbed %s: %+v", still.DisplayName, still)
		}
	}

	// Only when the count falls under the budget does the next one in.
	for _, task := range running {
		if _, _, err := st.AbandonTaskStart(task.TaskID, task.StartAttemptID); err != nil {
			t.Fatalf("AbandonTaskStart: %v", err)
		}
	}
	if _, ok, err := st.AdmitReadyTask(waiting.TaskID, reservation("at_w", "a_w", ClaimCreated), 1); err != nil || !ok {
		t.Fatalf("admission once the count fell = %v, %v", ok, err)
	}
}

// FS-16.R18 / A12 — deleting a prerequisite parks only a dependent whose arm was
// still waiting. A dependent that has passed its arms — starting, running,
// interrupted, or finished — is never reopened, and one the result already
// satisfied is untouched whatever state it is in.
func TestDeletingAPrerequisiteParksOnlyWhatWasStillWaiting(t *testing.T) {
	st, _ := newTestStore(t)
	prerequisite := newTask(t, st, "my-app", "prerequisite")

	waiting := map[string]Task{}
	for i, taskState := range []string{TaskArmed, TaskStarting, TaskRunning, TaskInterrupted, TaskFinished} {
		dependent := newTask(t, st, "my-app", "dependent-"+taskState,
			taskArm(prerequisite.TaskID, OutcomeSuccess))
		// Put the dependent in the state under test directly: what matters here is
		// what deletion does to it, not how it got there.
		if _, err := st.DB().Exec(`UPDATE tasks SET state = ?, assigned_agent_id = ? WHERE task_id = ?`,
			taskState, fmt.Sprintf("a_dep%d", i), dependent.TaskID); err != nil {
			t.Fatalf("set %s: %v", taskState, err)
		}
		waiting[taskState] = dependent
	}
	// One more whose arm the prerequisite already satisfied.
	if err := st.RegisterWorkResult(WorkResult{
		SourceKind: SourceTask, SourceID: prerequisite.TaskID, Outcome: OutcomeSuccess,
	}); err != nil {
		t.Fatalf("RegisterWorkResult: %v", err)
	}
	satisfied := newTask(t, st, "my-app", "already satisfied", taskArm(prerequisite.TaskID, OutcomeSuccess))
	if satisfied.State != TaskReady {
		t.Fatalf("a dependent of finished work = %s, want ready", satisfied.State)
	}

	if _, err := st.DeleteTask(prerequisite.TaskID); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}
	for taskState, dependent := range waiting {
		after, err := st.ReadTask(dependent.TaskID)
		if err != nil {
			t.Fatalf("ReadTask: %v", err)
		}
		want := taskState
		if taskState == TaskArmed {
			want = TaskDependencyFailed
		}
		if after.State != want {
			t.Fatalf("dependent in %s became %s after the deletion, want %s", taskState, after.State, want)
		}
	}
	still, err := st.ReadTask(satisfied.TaskID)
	if err != nil {
		t.Fatalf("ReadTask: %v", err)
	}
	if still.State != TaskReady || still.Arms[0].State != ArmSatisfied {
		t.Fatalf("a satisfied dependent changed: %+v", still)
	}
	// The result outlives the task that produced it, which is why that is true.
	if _, err := st.ReadWorkResult(SourceTask, prerequisite.TaskID); err != nil {
		t.Fatalf("the deleted task's result was removed: %v", err)
	}
}

// FS-16.R13 / A7 — a task armed on a pipeline run is released by that run's
// registered outcome, with no cross-plane reach: evaluation reads the shared
// result layer, never pipeline internals.
func TestATaskArmedOnAPipelineRunIsReleasedByItsOutcome(t *testing.T) {
	st, _ := newTestStore(t)
	runID := "run_quality"
	if _, err := st.DB().Exec(`
INSERT INTO pipeline_runs(run_id, template_id, display_name, project, goal, state, revision,
  current_stage_id, created_at, updated_at)
VALUES(?, 'quality', 'Quality run', 'my-app', 'ship', 'running', 1, 'work', ?, ?)`,
		runID, formatTime(timeNow()), formatTime(timeNow())); err != nil {
		t.Fatalf("insert run: %v", err)
	}
	dependent := newTask(t, st, "my-app", "after the run", TaskArm{
		Kind: ArmWorkResult, SourceKind: SourcePipelineRun, SourceID: runID,
		SatisfyingOutcomes: []string{OutcomeSuccess},
	})
	if dependent.State != TaskArmed {
		t.Fatalf("dependent = %s, want armed", dependent.State)
	}

	if err := st.RegisterWorkResult(WorkResult{
		SourceKind: SourcePipelineRun, SourceID: runID, Outcome: OutcomeSuccess, RawLabel: "success",
	}); err != nil {
		t.Fatalf("RegisterWorkResult: %v", err)
	}
	changed, err := st.EvaluateSource(SourcePipelineRun, runID)
	if err != nil {
		t.Fatalf("EvaluateSource: %v", err)
	}
	if len(changed) != 1 || changed[0].State != TaskReady {
		t.Fatalf("evaluation released %+v, want the dependent ready", changed)
	}
}

// FS-16.R23/R25 — retry eligibility has exactly one authority. RetryTask decides
// it, ReadTask/ListTasks project that same decision as `retry_eligible`, and the
// Tasks view reads the projection instead of restating the condition. The UI had
// already drifted from this switch once, narrowing Retry to `interrupted` and
// silently dropping work parked by exhausted start attempts (INV §2), so what is
// pinned here is the agreement itself: across every task state, the projected
// field says yes exactly when RetryTask succeeds.
func TestRetryEligibleProjectionAgreesWithRetryTask(t *testing.T) {
	st, _ := newTestStore(t)
	cases := []struct {
		name      string
		taskState string
		armState  string
		want      bool
	}{
		{name: "armed", taskState: TaskArmed, want: false},
		{name: "ready", taskState: TaskReady, want: false},
		{name: "starting", taskState: TaskStarting, want: false},
		{name: "running", taskState: TaskRunning, want: false},
		{name: "finished", taskState: TaskFinished, want: false},
		{name: "interrupted", taskState: TaskInterrupted, want: true},
		// The two ways to reach dependency_failed need different repairs: an arm
		// that can never be satisfied needs re-arming, while spent start attempts
		// or an ineligible target are exactly what Retry restores.
		{name: "parked by an unsatisfiable arm", taskState: TaskDependencyFailed, armState: ArmUnsatisfiable, want: false},
		{name: "parked by exhausted start attempts", taskState: TaskDependencyFailed, armState: ArmSatisfied, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := newTask(t, st, "my-app", "source-"+tc.name)
			task := newTask(t, st, "my-app", "task-"+tc.name, taskArm(source.TaskID, OutcomeSuccess))
			// Put the task in the state under test directly: what matters is what
			// eligibility says about that state, not how the task reached it.
			if _, err := st.DB().Exec(`UPDATE tasks SET state = ? WHERE task_id = ?`, tc.taskState, task.TaskID); err != nil {
				t.Fatalf("set task state: %v", err)
			}
			if tc.armState != "" {
				if _, err := st.DB().Exec(`UPDATE task_arms SET state = ? WHERE task_id = ?`, tc.armState, task.TaskID); err != nil {
					t.Fatalf("set arm state: %v", err)
				}
			}

			read, err := st.ReadTask(task.TaskID)
			if err != nil {
				t.Fatalf("ReadTask: %v", err)
			}
			if read.RetryEligible != tc.want {
				t.Fatalf("ReadTask retry_eligible = %v, want %v", read.RetryEligible, tc.want)
			}
			listed, err := st.ListTasks("my-app")
			if err != nil {
				t.Fatalf("ListTasks: %v", err)
			}
			found := false
			for _, item := range listed {
				if item.TaskID != task.TaskID {
					continue
				}
				found = true
				if item.RetryEligible != tc.want {
					t.Fatalf("ListTasks retry_eligible = %v, want %v", item.RetryEligible, tc.want)
				}
			}
			if !found {
				t.Fatal("the task was not listed")
			}

			// The projection is only worth anything if it matches the verb.
			if _, err := st.RetryTask(task.TaskID); (err == nil) != tc.want {
				t.Fatalf("RetryTask err = %v, but retry_eligible said %v", err, tc.want)
			}
		})
	}
}
