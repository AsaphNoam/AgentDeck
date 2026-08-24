package server

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/state"
)

// newLaunchTask creates a ready task that brings up its own agent.
func newLaunchTask(t *testing.T, srv *Server, name string) state.Task {
	t.Helper()
	id, err := srv.stateStore.NewTaskID()
	if err != nil {
		t.Fatalf("NewTaskID: %v", err)
	}
	task, err := srv.stateStore.CreateTask(state.Task{
		TaskID: id, Project: "tmpproj", DisplayName: name,
		Instruction:   "please do " + name,
		TargetKind:    state.TargetLaunch,
		Role:          "impl",
		CreatedByKind: "person",
	})
	if err != nil {
		t.Fatalf("CreateTask %s: %v", name, err)
	}
	return task
}

// waitTaskState blocks until the task reaches want, which the dispatcher's start
// goroutine reaches asynchronously.
func waitTaskState(t *testing.T, srv *Server, taskID, want string) state.Task {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		task, err := srv.stateStore.ReadTask(taskID)
		if err != nil {
			t.Fatalf("ReadTask: %v", err)
		}
		if task.State == want {
			return task
		}
		if time.Now().After(deadline) {
			t.Fatalf("task %s = %s, want %s", taskID, task.State, want)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// dispatchUntilTaskState runs dispatch passes until the task reaches want.
func dispatchUntilTaskState(t *testing.T, srv *Server, taskID, want string) state.Task {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		task, err := srv.stateStore.ReadTask(taskID)
		if err != nil {
			t.Fatalf("ReadTask: %v", err)
		}
		if task.State == want {
			return task
		}
		if time.Now().After(deadline) {
			t.Fatalf("task %s = %s after repeated dispatch, want %s", taskID, task.State, want)
		}
		srv.dispatchReadyTasks(context.Background())
		time.Sleep(20 * time.Millisecond)
	}
}

// stampReadyOrder gives the tasks a distinct, increasing admission order, which
// second-resolution creation timestamps cannot express on their own.
func stampReadyOrder(t *testing.T, srv *Server, tasks ...state.Task) {
	t.Helper()
	for i, task := range tasks {
		stamp := time.Date(2026, 8, 24, 10, i, 0, 0, time.UTC).Format(time.RFC3339)
		if _, err := srv.stateStore.DB().Exec(
			`UPDATE tasks SET ready_at = ? WHERE task_id = ?`, stamp, task.TaskID); err != nil {
			t.Fatalf("stamp ready_at: %v", err)
		}
	}
}

// FS-16.R6 / TS-10.R4 — ready work starts without a model in the loop: the host
// launches the agent, sends the task's instruction as its assignment, and only
// then calls the task running. Nothing polls, waits, or announces anything.
func TestALaunchSpecTaskStartsItselfAndBecomesRunning(t *testing.T) {
	srv, _, promptLog := activationTestServer(t)
	task := newLaunchTask(t, srv, "build the thing")

	srv.dispatchReadyTasks(context.Background())
	running := waitTaskState(t, srv, task.TaskID, state.TaskRunning)

	if running.RuntimeClaim != state.ClaimCreated {
		t.Fatalf("runtime claim = %q, want %q", running.RuntimeClaim, state.ClaimCreated)
	}
	if running.AssignedAgentID == "" || running.StartAttemptID == "" {
		t.Fatalf("running task has no confirmed assignee: %+v", running)
	}
	if running.StartAttemptCount != 0 {
		t.Fatalf("a start that worked spent %d attempts, want 0", running.StartAttemptCount)
	}
	if _, err := srv.stateStore.ReadRunning(running.AssignedAgentID); err != nil {
		t.Fatalf("task is running but its agent has no runtime: %v", err)
	}
	agent, err := srv.stateStore.ReadAgent(running.AssignedAgentID)
	if err != nil {
		t.Fatalf("ReadAgent: %v", err)
	}
	if agent.Interface != "chat" || agent.Role != "impl" {
		t.Fatalf("task agent = %s/%s, want a chat agent in the task's role", agent.Interface, agent.Role)
	}

	waitPrompts(t, promptLog, 1)
	raw, err := os.ReadFile(promptLog)
	if err != nil {
		t.Fatalf("read prompt log: %v", err)
	}
	if !strings.Contains(string(raw), "build the thing") {
		t.Fatalf("the assignment prompt did not carry the instruction: %s", raw)
	}
	// No mail row was created to carry the assignment: this plane is control
	// state, not conversation (FS-16.R6).
	unread, err := srv.stateStore.ListMessages(running.AssignedAgentID, true, 0)
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(unread) != 0 {
		t.Fatalf("starting a task produced %d mail rows, want none", len(unread))
	}
}

// FS-16.R7, R21 / TS-10.R17 — readiness is logical and runtimes are budgeted.
// Three ready tasks with a budget of one leave three ready tasks and one running
// runtime, and the next admission goes to the task that became ready first.
func TestTheBudgetLimitsRuntimesWithoutNarrowingReadiness(t *testing.T) {
	srv, _ := wakeTestServer(t)
	cfg, err := srv.configStore.ReadConfig()
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	cfg.TaskConcurrency = 1
	if err := srv.configStore.WriteConfig(cfg); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}

	first := newLaunchTask(t, srv, "first")
	second := newLaunchTask(t, srv, "second")
	third := newLaunchTask(t, srv, "third")
	stampReadyOrder(t, srv, first, second, third)

	srv.dispatchReadyTasks(context.Background())
	waitTaskState(t, srv, first.TaskID, state.TaskRunning)
	for _, waiting := range []state.Task{second, third} {
		task, err := srv.stateStore.ReadTask(waiting.TaskID)
		if err != nil {
			t.Fatalf("ReadTask: %v", err)
		}
		if task.State != state.TaskReady {
			t.Fatalf("%s = %s with the budget full, want ready and waiting", task.DisplayName, task.State)
		}
		if task.StartAttemptCount != 0 {
			t.Fatalf("%s spent %d attempts waiting for capacity, want 0", task.DisplayName, task.StartAttemptCount)
		}
	}

	// A finished task releases its slot, and the oldest ready task takes it.
	done, err := srv.stateStore.ReadTask(first.TaskID)
	if err != nil {
		t.Fatalf("ReadTask: %v", err)
	}
	outcome := state.OutcomeSuccess
	if _, err := srv.stateStore.UpdateTaskCAS(done.TaskID, done.Revision, state.TaskUpdate{
		State: state.TaskFinished, Outcome: &outcome,
	}); err != nil {
		t.Fatalf("finish first: %v", err)
	}
	srv.dispatchReadyTasks(context.Background())
	waitTaskState(t, srv, second.TaskID, state.TaskRunning)
	last, err := srv.stateStore.ReadTask(third.TaskID)
	if err != nil {
		t.Fatalf("ReadTask: %v", err)
	}
	if last.State != state.TaskReady {
		t.Fatalf("third = %s, want still waiting behind second", last.State)
	}
}

// FS-04.R43 — the budget is a setting with a shipped default of ten, read fresh
// so a change takes effect without a restart.
func TestTaskConcurrencyBudgetDefaultsToTen(t *testing.T) {
	srv := testServer(t, true)
	if got := srv.taskConcurrencyBudget(); got != 10 {
		t.Fatalf("default budget = %d, want 10", got)
	}
	cfg, err := srv.configStore.ReadConfig()
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	cfg.TaskConcurrency = 4
	if err := srv.configStore.WriteConfig(cfg); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}
	if got := srv.taskConcurrencyBudget(); got != 4 {
		t.Fatalf("configured budget = %d, want 4", got)
	}
	cfg.TaskConcurrency = 0
	if err := srv.configStore.WriteConfig(cfg); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}
	if got := srv.taskConcurrencyBudget(); got != 10 {
		t.Fatalf("unset budget = %d, want the default", got)
	}
}

// FS-16.R25 — a start that genuinely fails spends one of three attempts and the
// third parks the task, so unstartable work surfaces instead of retrying forever.
func TestRepeatedStartFailuresParkTheTask(t *testing.T) {
	srv, _ := wakeTestServer(t)
	// A role the launch boundary rejects makes every attempt fail for a real
	// reason rather than a simulated one.
	id, err := srv.stateStore.NewTaskID()
	if err != nil {
		t.Fatalf("NewTaskID: %v", err)
	}
	task, err := srv.stateStore.CreateTask(state.Task{
		TaskID: id, Project: "tmpproj", DisplayName: "unstartable",
		Instruction: "no", TargetKind: state.TargetLaunch, Role: "no-such-role",
		CreatedByKind: "person",
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	// Keep dispatching rather than counting passes: a pass that lands while the
	// previous attempt is still in flight finds the task starting and admits
	// nothing, which is correct and would make a fixed count flaky.
	parked := dispatchUntilTaskState(t, srv, task.TaskID, state.TaskDependencyFailed)
	if parked.StartAttemptCount != state.MaxTaskStartAttempts {
		t.Fatalf("parked after %d attempts, want %d", parked.StartAttemptCount, state.MaxTaskStartAttempts)
	}
	if parked.AttentionReason == "" {
		t.Fatal("a parked task did not record why it could not start")
	}
	if parked.Outcome != "" {
		t.Fatalf("parking recorded outcome %q; a failed start is not a result", parked.Outcome)
	}
	// Nothing is left holding a runtime or a slot.
	if parked.RuntimeClaim != "" || parked.AssignedAgentID != "" {
		t.Fatalf("parked task still holds %+v", parked)
	}
}
