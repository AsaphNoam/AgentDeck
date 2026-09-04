package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/configsource"
	"github.com/agentdeck/agentdeck/internal/pipeline"
	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
)

// INV §5 / FS-16.R7 — cancellation serializes with only its own task's effect;
// an unrelated launch must retain the dispatcher's configured parallelism.
func TestTaskStartLocksArePerTask(t *testing.T) {
	srv := testServer(t, true)
	releaseFirst := srv.lockTaskStart("tk_first")
	defer releaseFirst()

	enteredSecond := make(chan func(), 1)
	go func() { enteredSecond <- srv.lockTaskStart("tk_second") }()
	select {
	case releaseSecond := <-enteredSecond:
		releaseSecond()
	case <-time.After(time.Second):
		t.Fatal("an unrelated task start waited behind the first task")
	}
}

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

func newDependentTask(t *testing.T, srv *Server, name, sourceKind, sourceID string) state.Task {
	t.Helper()
	id, err := srv.stateStore.NewTaskID()
	if err != nil {
		t.Fatal(err)
	}
	task, err := srv.stateStore.CreateTask(state.Task{
		TaskID: id, Project: "tmpproj", DisplayName: name, Instruction: name,
		TargetKind: state.TargetLaunch, Role: "impl", CreatedByKind: "person",
		Arms: []state.TaskArm{{Kind: state.ArmWorkResult, SourceKind: sourceKind, SourceID: sourceID,
			SatisfyingOutcomes: []string{state.OutcomeSuccess}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func seedPipelineRun(t *testing.T, srv *Server, runID string) {
	t.Helper()
	now := time.Now().UTC()
	_, _, err := srv.stateStore.CreatePipelineRun(state.CreatePipelineRunParams{
		Run: state.PipelineRunRecord{RunID: runID, TemplateID: "quality",
			TemplateSnapshot: json.RawMessage(`{"version":1,"inputs":[],"stages":[]}`),
			DisplayName:      "Quality", Project: "tmpproj", Goal: "test",
			Inputs: json.RawMessage(`{}`), Assignments: json.RawMessage(`{}`),
			State: "queued", Revision: 1, CreatedAt: now, UpdatedAt: now},
		RequestID: "request_" + runID, RequestHash: "hash_" + runID,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func assertDependencyChainParked(t *testing.T, srv *Server, tasks ...state.Task) {
	t.Helper()
	for _, original := range tasks {
		task, err := srv.stateStore.ReadTask(original.TaskID)
		if err != nil || task.State != state.TaskDependencyFailed || task.AttentionReason == "" {
			t.Fatalf("task %s = %+v, %v; want surfaced dependency failure", original.TaskID, task, err)
		}
	}
}

// FS-16.A4 / TS-10.R3,R11 — a terminal pipeline result publishes and recursively
// propagates a failure through a three-level dependency chain.
func TestTerminalPipelineFailurePublishesAndPropagatesDependencyFailure(t *testing.T) {
	srv := testServer(t, true)
	seedPipelineRun(t, srv, "pr_failed")
	first := newDependentTask(t, srv, "first", state.SourcePipelineRun, "pr_failed")
	second := newDependentTask(t, srv, "second", state.SourceTask, first.TaskID)
	if err := srv.stateStore.RegisterWorkResult(state.WorkResult{SourceKind: state.SourcePipelineRun,
		SourceID: "pr_failed", Outcome: state.OutcomeFailure}); err != nil {
		t.Fatal(err)
	}
	events, unsubscribe := srv.eventBus.Subscribe()
	defer unsubscribe()
	srv.PublishPipelineUpdate(pipeline.PipelineUpdate{RunID: "pr_failed", State: "completed"})
	assertDependencyChainParked(t, srv, first, second)
	seen := map[string]bool{}
	for len(seen) < 2 {
		select {
		case event := <-events:
			if event.Type == "task_update" {
				if data, ok := event.Data.(map[string]any); ok {
					seen[data["task_id"].(string)] = true
				}
			}
		case <-time.After(time.Second):
			t.Fatalf("task_update events = %v", seen)
		}
	}
}

// FS-16.A7 / TS-10.R15 — restart re-evaluation uses the same publication and
// recursive propagation loop as the live result path.
func TestRestartReevaluationPublishesAndPropagatesDependencyFailure(t *testing.T) {
	srv := testServer(t, true)
	seedPipelineRun(t, srv, "pr_failed")
	first := newDependentTask(t, srv, "first", state.SourcePipelineRun, "pr_failed")
	second := newDependentTask(t, srv, "second", state.SourceTask, first.TaskID)
	if err := srv.stateStore.RegisterWorkResult(state.WorkResult{SourceKind: state.SourcePipelineRun,
		SourceID: "pr_failed", Outcome: state.OutcomeFailure}); err != nil {
		t.Fatal(err)
	}
	if err := srv.recoverTasks(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertDependencyChainParked(t, srv, first, second)
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

// newAgentTask creates a ready task that crosses into an existing agent.
func newAgentTask(t *testing.T, srv *Server, agentID, name string) state.Task {
	t.Helper()
	id, err := srv.stateStore.NewTaskID()
	if err != nil {
		t.Fatalf("NewTaskID: %v", err)
	}
	task, err := srv.stateStore.CreateTask(state.Task{
		TaskID: id, Project: "tmpproj", DisplayName: name,
		Instruction:   "please do " + name,
		TargetKind:    state.TargetAgent,
		TargetAgentID: agentID,
		CreatedByKind: "person",
	})
	if err != nil {
		t.Fatalf("CreateTask %s: %v", name, err)
	}
	return task
}

// FS-16.R6, R26 / TS-10.R5 — a task targeting a running agent gives it one
// bounded turn through the dependency activation kind. The agent is told it has
// a task, not told to check its mail, and its runtime is borrowed: it was already
// up for its own reasons, so the task takes no budget slot.
func TestATaskForARunningAgentActivatesThatConversation(t *testing.T) {
	srv, ts, promptLog := activationTestServer(t)
	agentID := launchAndWaitIdle(t, ts, "impl", "tmpproj")
	task := newAgentTask(t, srv, agentID, "review the migration")

	srv.dispatchReadyTasks(context.Background())
	running := waitTaskState(t, srv, task.TaskID, state.TaskRunning)
	if running.AssignedAgentID != agentID || running.RuntimeClaim != state.ClaimBorrowed {
		t.Fatalf("task assigned %q claiming %q, want %q borrowed",
			running.AssignedAgentID, running.RuntimeClaim, agentID)
	}

	waitPrompts(t, promptLog, 1)
	raw, err := os.ReadFile(promptLog)
	if err != nil {
		t.Fatalf("read prompt log: %v", err)
	}
	dependency, ok := runtime.LookupActivationKind(state.ActivationKindDependency)
	if !ok {
		t.Fatal("the dependency activation kind has no registered contract")
	}
	if !strings.Contains(string(raw), dependency.Instruction) {
		t.Fatalf("prompt = %s, want the task instruction %q", raw, dependency.Instruction)
	}
	mail, _ := runtime.LookupActivationKind(state.ActivationKindMail)
	if strings.Contains(string(raw), mail.Instruction) {
		t.Fatalf("the activated agent was told to check its messages: %s", raw)
	}
	// The activation record is consumed rather than left to fire again.
	pending, err := srv.stateStore.PendingActivations(state.ActivationKindDependency, agentID, 10)
	if err != nil {
		t.Fatalf("PendingActivations: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("dependency activations still pending after the turn: %+v", pending)
	}
	if _, err := srv.stateStore.ReadRunning(agentID); err != nil {
		t.Fatalf("the borrowed runtime was disturbed: %v", err)
	}
}

// FS-16.R6, R19 — a task targeting a stopped agent wakes it on the same terms
// mail does, and records that it woke the runtime, which is what decides whether
// finishing the task stops it again.
func TestATaskForAStoppedAgentWakesIt(t *testing.T) {
	srv, ts, promptLog := activationTestServer(t)
	agentID := launchThenStop(t, srv, ts)
	task := newAgentTask(t, srv, agentID, "pick this back up")

	srv.dispatchReadyTasks(context.Background())
	running := waitTaskState(t, srv, task.TaskID, state.TaskRunning)
	if running.RuntimeClaim != state.ClaimWoke {
		t.Fatalf("runtime claim = %q, want %q", running.RuntimeClaim, state.ClaimWoke)
	}
	realGeneration := srv.registry.Generation(agentID)
	if realGeneration == "" || running.AssignedGeneration != realGeneration {
		t.Fatalf("woken assignment generation = %q, runtime generation = %q", running.AssignedGeneration, realGeneration)
	}
	waitRunning(t, srv, agentID, true)
	waitPrompts(t, promptLog, 1)
	raw, err := os.ReadFile(promptLog)
	if err != nil {
		t.Fatalf("read prompt log: %v", err)
	}
	dependency, _ := runtime.LookupActivationKind(state.ActivationKindDependency)
	if !strings.Contains(string(raw), dependency.Instruction) {
		t.Fatalf("woken prompt = %s, want the task instruction", raw)
	}
	if _, err := srv.stateStore.RecordAgentTaskResult(task.TaskID, agentID, realGeneration,
		state.TaskResult{Outcome: state.OutcomeSuccess, Summary: "done"}); err != nil {
		t.Fatalf("RecordAgentTaskResult with runtime token: %v", err)
	}
	srv.dispatchTurnEnd(agentID, realGeneration)
	waitRunning(t, srv, agentID, false)
}

// FS-16.R25 / TS-10.R4 — once a wake delivered the assignment turn, losing
// its generation before confirmation spends the attempt and never strands the
// task in starting.
func TestWokenTaskWithMissingGenerationSettlesItsAttempt(t *testing.T) {
	srv, ts := wakeTestServer(t)
	agentID := launchAndWaitIdle(t, ts, "impl", "tmpproj")
	if resp, body := post(t, ts.URL+"/api/sessions/"+agentID+"/stop", nil); resp.StatusCode != 200 {
		t.Fatalf("stop status = %d: %s", resp.StatusCode, body)
	}
	task := newAgentTask(t, srv, agentID, "wake then disappear")
	srv.taskGeneration = func(string) string { return "" }
	srv.dispatchReadyTasks(context.Background())
	ready := waitTaskState(t, srv, task.TaskID, state.TaskReady)
	if ready.StartAttemptCount != 1 {
		t.Fatalf("spent attempts = %d, want 1", ready.StartAttemptCount)
	}
	if ready.RuntimeClaim != "" || ready.StartAttemptID != "" {
		t.Fatalf("missing generation left a starting claim: %+v", ready)
	}
}

// FS-16.R2, R19 — a target that can never execute work parks its task instead of
// spending attempts on a start that cannot succeed.
func TestATaskForAnIneligibleTargetParksWithoutSpendingAttempts(t *testing.T) {
	srv, _ := wakeTestServer(t)
	if err := srv.stateStore.WriteAgent(state.Agent{
		AgentID: "a_term", Name: "Shell", Role: "impl", Project: "tmpproj",
		Backend: "claude-acp", Model: "sonnet", Interface: "terminal",
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("WriteAgent: %v", err)
	}
	task := newAgentTask(t, srv, "a_term", "cannot run here")

	srv.dispatchReadyTasks(context.Background())
	parked := waitTaskState(t, srv, task.TaskID, state.TaskDependencyFailed)
	if parked.StartAttemptCount != 0 {
		t.Fatalf("parking a terminal target spent %d attempts, want 0", parked.StartAttemptCount)
	}
	if !strings.Contains(parked.AttentionReason, "terminal") {
		t.Fatalf("attention reason = %q, want it to name the terminal interface", parked.AttentionReason)
	}
	if parked.AssignedAgentID != "" || parked.RuntimeClaim != "" {
		t.Fatalf("a parked task still holds a reservation: %+v", parked)
	}
}

// FS-16.R4 / TS-10.R19 — finishing releases the claim and stops the runtime the
// task itself brought up, at the reporting turn's end rather than inside the
// call, and leaves a non-archived, resumable agent behind.
func TestFinishingStopsTheRuntimeTheTaskCreated(t *testing.T) {
	srv, _ := wakeTestServer(t)
	task := newLaunchTask(t, srv, "build the thing")
	srv.dispatchReadyTasks(context.Background())
	running := waitTaskState(t, srv, task.TaskID, state.TaskRunning)

	agentID := running.AssignedAgentID
	generation := srv.registry.Generation(agentID)
	if generation != running.AssignedGeneration {
		t.Fatalf("live generation %q is not the one the task reserved (%q)",
			generation, running.AssignedGeneration)
	}
	finished, err := srv.stateStore.RecordAgentTaskResult(task.TaskID, agentID, generation,
		state.TaskResult{Outcome: state.OutcomeSuccess, Summary: "built it"})
	if err != nil {
		t.Fatalf("RecordAgentTaskResult: %v", err)
	}
	if !finished.PendingRelease {
		t.Fatal("a recorded result committed no release intent")
	}
	// Still up: the reporting agent is receiving its own tool response.
	if _, err := srv.stateStore.ReadRunning(agentID); err != nil {
		t.Fatalf("the reporter was stopped inside its own call: %v", err)
	}

	srv.dispatchTurnEnd(agentID, generation)
	waitRunning(t, srv, agentID, false)
	released, err := srv.stateStore.ReadTask(task.TaskID)
	if err != nil {
		t.Fatalf("ReadTask: %v", err)
	}
	if released.PendingRelease || released.RuntimeClaim != "" {
		t.Fatalf("the release never completed: %+v", released)
	}
	if released.AssignedAgentID != agentID {
		t.Fatalf("assignee provenance = %q, want it retained", released.AssignedAgentID)
	}
	agent, err := srv.stateStore.ReadAgent(agentID)
	if err != nil {
		t.Fatalf("ReadAgent: %v", err)
	}
	if agent.Archived {
		t.Fatal("finishing a task archived its agent")
	}
}

// FS-16.R4 — a borrowed runtime was already up for someone else's reasons, so
// finishing releases the claim and leaves the conversation running.
func TestFinishingLeavesABorrowedRuntimeAlone(t *testing.T) {
	srv, ts := wakeTestServer(t)
	agentID := launchAndWaitIdle(t, ts, "impl", "tmpproj")
	task := newAgentTask(t, srv, agentID, "review this")
	srv.dispatchReadyTasks(context.Background())
	running := waitTaskState(t, srv, task.TaskID, state.TaskRunning)
	realGeneration := srv.registry.Generation(agentID)
	if realGeneration == "" || running.AssignedGeneration != realGeneration {
		t.Fatalf("borrowed assignment generation = %q, runtime generation = %q", running.AssignedGeneration, realGeneration)
	}

	if _, err := srv.stateStore.RecordAgentTaskResult(task.TaskID, agentID, realGeneration,
		state.TaskResult{Outcome: state.OutcomeBlocked, Summary: "needs a decision"}); err != nil {
		t.Fatalf("RecordAgentTaskResult: %v", err)
	}
	srv.dispatchTurnEnd(agentID, realGeneration)

	if _, err := srv.stateStore.ReadRunning(agentID); err != nil {
		t.Fatalf("finishing a task killed a conversation it only borrowed: %v", err)
	}
	released, err := srv.stateStore.ReadTask(task.TaskID)
	if err != nil {
		t.Fatalf("ReadTask: %v", err)
	}
	if released.PendingRelease || released.RuntimeClaim != "" {
		t.Fatalf("the borrowed claim was not released: %+v", released)
	}
}

// FS-16.R4/R12 / TS-10.R4 — effect-time generation wins over the planning
// snapshot when a borrowed runtime was relaunched before its assignment turn.
func TestBorrowedTaskRefreshesGenerationBeforeAssignment(t *testing.T) {
	srv, ts := wakeTestServer(t)
	agentID := launchAndWaitIdle(t, ts, "impl", "tmpproj")
	task := newAgentTask(t, srv, agentID, "use the current runtime")
	reserved, ok, err := srv.stateStore.AdmitReadyTask(task.TaskID, state.TaskReservation{
		AttemptID: "ta_stale", AgentID: agentID, Generation: "g_stale", Claim: state.ClaimBorrowed,
	}, 10)
	if err != nil || !ok {
		t.Fatalf("AdmitReadyTask: %+v, %v, %v", reserved, ok, err)
	}
	srv.startAdmittedTask(context.Background(), reserved)
	running := waitTaskState(t, srv, task.TaskID, state.TaskRunning)
	current := srv.registry.Generation(agentID)
	if current == "" || running.AssignedGeneration != current || running.AssignedGeneration == "g_stale" {
		t.Fatalf("assigned generation = %q, current = %q", running.AssignedGeneration, current)
	}
}

// FS-16.R5, R6 / TS-10.R3 — the loop closes: a dependent stays armed until its
// prerequisite records a satisfying outcome, then becomes ready and is started,
// with no agent polling, waiting, or announcing anything.
func TestADependentStartsWhenItsPrerequisiteRecordsItsResult(t *testing.T) {
	srv, _ := wakeTestServer(t)
	first := newLaunchTask(t, srv, "first")
	id, err := srv.stateStore.NewTaskID()
	if err != nil {
		t.Fatalf("NewTaskID: %v", err)
	}
	second, err := srv.stateStore.CreateTask(state.Task{
		TaskID: id, Project: "tmpproj", DisplayName: "second", Instruction: "after the first",
		TargetKind: state.TargetLaunch, Role: "impl", CreatedByKind: "person",
		Arms: []state.TaskArm{{
			Kind: state.ArmWorkResult, SourceKind: state.SourceTask, SourceID: first.TaskID,
			SatisfyingOutcomes: []string{state.OutcomeSuccess},
		}},
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if second.State != state.TaskArmed {
		t.Fatalf("dependent = %s, want armed", second.State)
	}

	srv.dispatchReadyTasks(context.Background())
	running := waitTaskState(t, srv, first.TaskID, state.TaskRunning)
	// The dependent is untouched while its prerequisite runs: liveness is not a
	// result (FS-16.R3).
	if waiting, err := srv.stateStore.ReadTask(second.TaskID); err != nil || waiting.State != state.TaskArmed {
		t.Fatalf("dependent = %+v, %v; want still armed", waiting, err)
	}

	generation := running.AssignedGeneration
	if _, err := srv.stateStore.RecordAgentTaskResult(first.TaskID, running.AssignedAgentID, generation,
		state.TaskResult{Outcome: state.OutcomeSuccess, Summary: "done"}); err != nil {
		t.Fatalf("RecordAgentTaskResult: %v", err)
	}
	srv.evaluateTaskResult(first.TaskID)

	ready, err := srv.stateStore.ReadTask(second.TaskID)
	if err != nil {
		t.Fatalf("ReadTask: %v", err)
	}
	if ready.State != state.TaskReady {
		t.Fatalf("dependent = %s after its prerequisite succeeded, want ready", ready.State)
	}
	srv.dispatchTurnEnd(running.AssignedAgentID, generation)
	srv.dispatchReadyTasks(context.Background())
	waitTaskState(t, srv, second.TaskID, state.TaskRunning)
}

// FS-16.R8 — a prerequisite that finishes outside its arm's satisfying set parks
// the dependent instead of leaving it waiting on an outcome that can never come.
func TestAnUnsatisfyingResultParksTheDependent(t *testing.T) {
	srv, _ := wakeTestServer(t)
	first := newLaunchTask(t, srv, "first")
	id, err := srv.stateStore.NewTaskID()
	if err != nil {
		t.Fatalf("NewTaskID: %v", err)
	}
	second, err := srv.stateStore.CreateTask(state.Task{
		TaskID: id, Project: "tmpproj", DisplayName: "second", Instruction: "only after success",
		TargetKind: state.TargetLaunch, Role: "impl", CreatedByKind: "person",
		Arms: []state.TaskArm{{
			Kind: state.ArmWorkResult, SourceKind: state.SourceTask, SourceID: first.TaskID,
			SatisfyingOutcomes: []string{state.OutcomeSuccess},
		}},
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	srv.dispatchReadyTasks(context.Background())
	running := waitTaskState(t, srv, first.TaskID, state.TaskRunning)
	if _, err := srv.stateStore.RecordAgentTaskResult(first.TaskID, running.AssignedAgentID,
		running.AssignedGeneration,
		state.TaskResult{Outcome: state.OutcomeFailure, Summary: "could not"}); err != nil {
		t.Fatalf("RecordAgentTaskResult: %v", err)
	}
	srv.evaluateTaskResult(first.TaskID)

	parked, err := srv.stateStore.ReadTask(second.TaskID)
	if err != nil {
		t.Fatalf("ReadTask: %v", err)
	}
	if parked.State != state.TaskDependencyFailed || parked.AttentionReason == "" {
		t.Fatalf("dependent = %s (%q), want it parked and surfaced", parked.State, parked.AttentionReason)
	}
	srv.dispatchReadyTasks(context.Background())
	time.Sleep(100 * time.Millisecond)
	if again, err := srv.stateStore.ReadTask(second.TaskID); err != nil ||
		again.State != state.TaskDependencyFailed {
		t.Fatalf("a parked task started on its own: %+v, %v", again, err)
	}
}

// FS-16.R16 — an assignee that goes away before recording a result leaves its
// task interrupted and needing attention, with its claim released. AgentDeck
// never converts a process event into success or failure.
func TestAnAgentThatGoesAwayLeavesItsTaskInterrupted(t *testing.T) {
	srv, ts := wakeTestServer(t)
	task := newLaunchTask(t, srv, "long job")
	srv.dispatchReadyTasks(context.Background())
	running := waitTaskState(t, srv, task.TaskID, state.TaskRunning)

	if resp, body := post(t, ts.URL+"/api/sessions/"+running.AssignedAgentID+"/stop", nil); resp.StatusCode != 200 {
		t.Fatalf("stop status = %d: %s", resp.StatusCode, body)
	}
	interrupted := waitTaskState(t, srv, task.TaskID, state.TaskInterrupted)
	if interrupted.Outcome != "" {
		t.Fatalf("an agent going away recorded outcome %q", interrupted.Outcome)
	}
	if interrupted.AttentionReason == "" {
		t.Fatal("an interrupted task does not say why its agent went away")
	}
	if interrupted.RuntimeClaim != "" {
		t.Fatalf("an interrupted task still holds a runtime claim: %+v", interrupted)
	}
	if interrupted.AssignedAgentID != running.AssignedAgentID {
		t.Fatalf("assignee provenance = %q, want it retained", interrupted.AssignedAgentID)
	}
}

// FS-16.R17 / TS-10.R15 — a restart resolves every unfinished task from its own
// durable row, never by asking whether a pre-crash runtime survived. A start that
// would have created a runtime is reaped and retried within its limit; one that
// merely borrowed a live conversation never touches it and becomes interrupted;
// a running task becomes interrupted; and a release a reporting turn never
// finished is completed.
func TestRestartResolvesUnfinishedTasksFromTheirOwnRows(t *testing.T) {
	srv, ts := wakeTestServer(t)
	borrowedRunning := launchAndWaitIdle(t, ts, "impl", "tmpproj")
	borrowedStarting := launchAndWaitIdle(t, ts, "impl", "tmpproj")

	// A running task on a conversation it borrowed.
	runningTask := newAgentTask(t, srv, borrowedRunning, "running")
	srv.dispatchReadyTasks(context.Background())
	waitTaskState(t, srv, runningTask.TaskID, state.TaskRunning)

	// A start attempt that had reserved a borrowed runtime and crashed before any
	// effect, and one that would have created its own.
	startingBorrowed := newAgentTask(t, srv, borrowedStarting, "starting borrowed")
	if _, ok, err := srv.stateStore.AdmitReadyTask(startingBorrowed.TaskID,
		state.TaskReservation{AttemptID: "ta_borrow", AgentID: borrowedStarting,
			Generation: "g_borrow", Claim: state.ClaimBorrowed}, 10); err != nil || !ok {
		t.Fatalf("admit borrowed start: %v, %v", ok, err)
	}
	startingCreated := newLaunchTask(t, srv, "starting created")
	if _, ok, err := srv.stateStore.AdmitReadyTask(startingCreated.TaskID,
		state.TaskReservation{AttemptID: "ta_ghost", AgentID: "a_ghost",
			Generation: "g_ghost", Claim: state.ClaimCreated}, 10); err != nil || !ok {
		t.Fatalf("admit created start: %v, %v", ok, err)
	}

	// A recorded result whose stop never happened.
	reported := newLaunchTask(t, srv, "reported")
	srv.dispatchReadyTasks(context.Background())
	reportedRunning := waitTaskState(t, srv, reported.TaskID, state.TaskRunning)
	if _, err := srv.stateStore.RecordAgentTaskResult(reported.TaskID,
		reportedRunning.AssignedAgentID, reportedRunning.AssignedGeneration,
		state.TaskResult{Outcome: state.OutcomeSuccess, Summary: "done"}); err != nil {
		t.Fatalf("RecordAgentTaskResult: %v", err)
	}

	if err := srv.recoverTasks(context.Background()); err != nil {
		t.Fatalf("recoverTasks: %v", err)
	}

	check := func(taskID, wantState string) state.Task {
		t.Helper()
		task, err := srv.stateStore.ReadTask(taskID)
		if err != nil {
			t.Fatalf("ReadTask: %v", err)
		}
		if task.State != wantState {
			t.Fatalf("%s = %s after recovery, want %s", task.DisplayName, task.State, wantState)
		}
		return task
	}
	check(runningTask.TaskID, state.TaskInterrupted)
	check(startingBorrowed.TaskID, state.TaskInterrupted)
	retryable := check(startingCreated.TaskID, state.TaskReady)
	if retryable.StartAttemptCount != 1 {
		t.Fatalf("a restarted start attempt spent %d attempts, want 1", retryable.StartAttemptCount)
	}
	released := check(reported.TaskID, state.TaskFinished)
	if released.PendingRelease || released.RuntimeClaim != "" {
		t.Fatalf("recovery did not finish the promised release: %+v", released)
	}
	waitRunning(t, srv, reportedRunning.AssignedAgentID, false)

	// Neither borrowed conversation was touched: R4's promise does not lapse
	// because AgentDeck restarted.
	for _, agentID := range []string{borrowedRunning, borrowedStarting} {
		if _, err := srv.stateStore.ReadRunning(agentID); err != nil {
			t.Fatalf("recovery stopped a runtime a task only borrowed: %v", err)
		}
	}
	// The in-flight activation rows are gone; the next attempt makes its own.
	pending, err := srv.stateStore.PendingActivations(state.ActivationKindDependency, "", 10)
	if err != nil {
		t.Fatalf("PendingActivations: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("dependency activations survived a restart: %+v", pending)
	}
}

// newLaunchTaskWithEffort creates launch-spec work that names the reasoning
// level its agent should run at (FS-16.R27).
func newLaunchTaskWithEffort(t *testing.T, srv *Server, name, effort string) state.Task {
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
		Effort:        effort,
		CreatedByKind: "person",
	})
	if err != nil {
		t.Fatalf("CreateTask %s: %v", name, err)
	}
	return task
}

// FS-16.A18 (R27) / FS-09.R41 / TS-10.R23 — the level a task stored reaches the agent
// it launches as the explicitly requested effort, and a task that named none
// resolves to the model's default exactly as any other launch does.
func TestALaunchTaskStartsItsAgentAtTheStoredEffort(t *testing.T) {
	for _, tt := range []struct {
		name   string
		stored string
		want   string
	}{
		{name: "stored effort", stored: "high", want: "high"},
		{name: "no effort falls through to the model default", stored: "", want: "medium"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			srv, _, _ := activationTestServer(t)
			task := newLaunchTaskWithEffort(t, srv, "effort work", tt.stored)

			srv.dispatchReadyTasks(context.Background())
			running := waitTaskState(t, srv, task.TaskID, state.TaskRunning)

			agent, err := srv.stateStore.ReadAgent(running.AssignedAgentID)
			if err != nil {
				t.Fatalf("ReadAgent: %v", err)
			}
			if agent.Effort != tt.want {
				t.Fatalf("task agent effort = %q, want %q", agent.Effort, tt.want)
			}
		})
	}
}

// FS-16.A18 (R28) / FS-09.R49, A22 — creation-time acceptance is not a promise the
// launch will succeed. A task whose effort its model no longer declares spends a
// start attempt instead of launching at a substituted level.
func TestALaunchTaskWhoseEffortWasDroppedFailsItsStart(t *testing.T) {
	srv, _, _ := activationTestServer(t)
	task := newLaunchTaskWithEffort(t, srv, "stale effort", "high")

	// The catalog is editable while work sits armed: drop the level the task
	// already asked for, leaving the rest of the specification valid.
	backends, err := srv.configStore.ReadBackends()
	if err != nil {
		t.Fatalf("ReadBackends: %v", err)
	}
	claude := backends.Backends["claude"]
	sonnet := claude.Models["sonnet"]
	sonnet.Efforts = []string{"low", "medium"}
	sonnet.DefaultEffort = "medium"
	claude.Models["sonnet"] = sonnet
	backends.Backends["claude"] = claude
	if err := srv.configStore.WriteBackends(backends); err != nil {
		t.Fatalf("WriteBackends: %v", err)
	}

	// Each pass spends one of the bounded attempts rather than launching at a
	// substituted level; the third parks the task with the reason (FS-16.R25).
	for attempt := 1; attempt <= state.MaxTaskStartAttempts; attempt++ {
		srv.dispatchReadyTasks(context.Background())
		spent := waitTaskAttempts(t, srv, task.TaskID, attempt)
		if spent.State == state.TaskRunning {
			t.Fatalf("a task naming an undeclared level launched anyway: %+v", spent)
		}
	}
	parked := waitTaskState(t, srv, task.TaskID, state.TaskDependencyFailed)
	if !strings.Contains(parked.AttentionReason, "high") {
		t.Fatalf("parked reason did not name the refused level: %q", parked.AttentionReason)
	}
	if agents, err := srv.stateStore.ListAgents(); err != nil {
		t.Fatalf("ListAgents: %v", err)
	} else if len(agents) != 0 {
		t.Fatalf("a refused launch created %d agents, want none", len(agents))
	}
}

// waitTaskAttempts blocks until the task has spent want start attempts, which
// the dispatcher's start goroutine records asynchronously.
func waitTaskAttempts(t *testing.T, srv *Server, taskID string, want int) state.Task {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		task, err := srv.stateStore.ReadTask(taskID)
		if err != nil {
			t.Fatalf("ReadTask: %v", err)
		}
		if task.StartAttemptCount >= want {
			return task
		}
		if time.Now().After(deadline) {
			t.Fatalf("task %s spent %d attempts, want %d (state %s, reason %q)",
				taskID, task.StartAttemptCount, want, task.State, task.AttentionReason)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// bindSourceEffortOverride binds the claude backend to a fixture Claude tree
// whose AgentDeck-owned overrides name an effort, so a launch composed for the
// seeded project has a source-level effort to lose to an explicit one.
func bindSourceEffortOverride(t *testing.T, srv *Server, override string) {
	t.Helper()
	userHome := t.TempDir()
	root := filepath.Join(userHome, ".claude")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "settings.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	project, err := srv.configStore.ReadProject("tmpproj")
	if err != nil {
		t.Fatalf("ReadProject: %v", err)
	}
	srv.sourceMgr = configsource.NewManager(srv.configStore, map[string]configsource.Resolver{
		configsource.ProviderClaude: configsource.NewClaudeResolver(userHome),
	}, nil)
	sources, _ := srv.readConfigSources()
	canonicalRoot := canonicalPath(t, root)
	sources.Sources["claude"] = config.SourceBinding{
		Provider: configsource.ProviderClaude, Mode: configsource.ModeLinked, Root: canonicalRoot,
		Claims:    []string{"launch_defaults"},
		Overrides: config.SourceOverrides{Effort: &override},
		Approved:  []string{canonicalRoot, canonicalPath(t, project.Cwd)},
	}
	if err := srv.configStore.WriteConfigSources(sources); err != nil {
		t.Fatalf("WriteConfigSources: %v", err)
	}
}

// FS-16.A18 (R27) / FS-09.R41 / TS-10.R23 (INV §2) — the effort a task stored
// reaches the provider itself, read off the call the adapter actually receives
// rather than off the persisted agent projection, and an explicit task effort
// beats the effort override a bound configuration source carries. The
// no-effort case proves the override is live, so "beats it" means something.
func TestALaunchTaskEffortReachesTheProviderOverASourceOverride(t *testing.T) {
	for _, tt := range []struct {
		name   string
		stored string
		want   string
	}{
		{name: "explicit task effort beats the source override", stored: "high", want: "high"},
		{name: "no task effort leaves the source override in force", stored: "", want: "low"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dump := filepath.Join(t.TempDir(), "effort_params.json")
			t.Setenv("FAKEACP_EFFORT_DUMP", dump)

			srv, _, _ := activationTestServer(t)
			bindSourceEffortOverride(t, srv, "low")
			task := newLaunchTaskWithEffort(t, srv, "effort work", tt.stored)

			srv.dispatchReadyTasks(context.Background())
			waitTaskState(t, srv, task.TaskID, state.TaskRunning)

			raw, err := os.ReadFile(dump)
			if err != nil {
				t.Fatalf("read effort dump (the provider was never told an effort?): %v", err)
			}
			var params struct {
				ConfigID string `json:"configId"`
				Value    string `json:"value"`
			}
			if err := json.Unmarshal(raw, &params); err != nil {
				t.Fatalf("unmarshal effort params: %v\n%s", err, raw)
			}
			if params.ConfigID != "effort" || params.Value != tt.want {
				t.Fatalf("provider effort = %+v, want configId=effort value=%s", params, tt.want)
			}
		})
	}
}
