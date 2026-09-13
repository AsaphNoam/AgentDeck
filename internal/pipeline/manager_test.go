package pipeline

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/state"
)

type fakeLifecycle struct {
	mu             sync.Mutex
	running        map[string]bool
	launches       []StageExecution
	continuations  []StageExecution
	stops          []string
	failNextLaunch bool
	startErr       error
}

func (f *fakeLifecycle) AcquirePipelineStart(context.Context, string) (func(), error) {
	if f.startErr != nil {
		return nil, f.startErr
	}
	return func() {}, nil
}
func (f *fakeLifecycle) ValidateStage(context.Context, StageExecution) error { return nil }
func (f *fakeLifecycle) LaunchStage(_ context.Context, execution StageExecution) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failNextLaunch {
		f.failNextLaunch = false
		return errors.New("injected launch failure")
	}
	f.running[execution.AgentID] = true
	f.launches = append(f.launches, execution)
	return nil
}
func (f *fakeLifecycle) ContinueStage(_ context.Context, execution StageExecution) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.running[execution.AgentID] = true
	f.continuations = append(f.continuations, execution)
	return nil
}
func (f *fakeLifecycle) StopStage(_ context.Context, agentID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.running, agentID)
	f.stops = append(f.stops, agentID)
	return nil
}
func (f *fakeLifecycle) IsRunning(agentID string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.running[agentID]
}

type fakePublisher struct {
	updates         []PipelineUpdate
	notifications   []string
	proposalUpdates int
}

func (p *fakePublisher) PublishPipelineUpdate(update PipelineUpdate) {
	p.updates = append(p.updates, update)
}
func (p *fakePublisher) PublishPipelineNotification(_ PipelineUpdate, kind string) {
	p.notifications = append(p.notifications, kind)
}

func (p *fakePublisher) PublishPipelineProposalUpdate() { p.proposalUpdates++ }

func pipelineManagerFixture(t *testing.T) (*Manager, *fakeLifecycle, *fakePublisher) {
	t.Helper()
	home := t.TempDir()
	configStore := config.NewWithHome(home)
	if err := configStore.EnsureLayout(); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"implementer", "reviewer", "orchestrator"} {
		if err := configStore.WriteRole(role, config.Role{Title: role}); err != nil {
			t.Fatal(err)
		}
	}
	template := Template{
		Version: 2, Title: "Quality", OrchestratorRole: "orchestrator",
		Inputs: []ValueDecl{{Name: "spec", Description: "Specification", Required: true}},
		Stages: []Stage{
			{ID: "work", Title: "Work", Objective: "Implement.", Instruction: "Implement.",
				Inputs:  []StageInput{{Name: "specification", Value: "spec", Required: true}},
				Outputs: []StageOutput{{Name: "implementation", Value: "implementation", Description: "What changed"}},
			},
			{ID: "review", Title: "Review", Objective: "Review.", Instruction: "Review.",
				Inputs: []StageInput{{Name: "implementation", Value: "implementation", Required: true}}, Outputs: []StageOutput{}},
		},
	}
	templates := NewTemplateStore(configStore)
	if record, err := templates.Create("quality", template); err != nil || !record.Valid {
		t.Fatalf("create template = %+v err=%v", record, err)
	}
	store, err := state.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	lifecycle := &fakeLifecycle{running: map[string]bool{}}
	publisher := &fakePublisher{}
	return NewManager(store, templates, lifecycle, publisher), lifecycle, publisher
}

// startStagePipeline starts a v2 run and puts its standing stage task into the
// running state the shared dispatcher would leave behind, so control-plane tests
// exercise the task-backed cursor rather than the removed v1 executor.
func startStagePipeline(t *testing.T, manager *Manager, requestID, agentID, generation string) (RunDetail, state.PipelineStageTask) {
	t.Helper()
	detail, replay, err := manager.Start(context.Background(), StartRequest{
		RequestID: requestID, TemplateID: "quality", DisplayName: "Ship", Project: "app", Goal: "Implement the spec",
		Inputs:       map[string]string{"spec": "Requirements"},
		Orchestrator: RuntimeAssignment{Backend: "codex", Model: "gpt", Effort: "high"},
	})
	if err != nil || replay {
		t.Fatalf("Start = %+v replay=%v err=%v", detail, replay, err)
	}
	stages, err := manager.store.ListPipelineStageTasks(detail.Run.RunID)
	if err != nil || len(stages) != 1 {
		t.Fatalf("stage tasks = %#v, err=%v", stages, err)
	}
	if _, err := manager.store.DB().Exec(`UPDATE tasks SET state = ?, assigned_agent_id = ?, assigned_generation = ? WHERE task_id = ?`,
		state.TaskRunning, agentID, generation, stages[0].TaskID); err != nil {
		t.Fatal(err)
	}
	if err := manager.store.BindPipelineStageTaskStandingAgent(stages[0].TaskID, agentID); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.store.DB().Exec(`UPDATE pipeline_runs SET state = 'running', pending_action = '' WHERE run_id = ?`, detail.Run.RunID); err != nil {
		t.Fatal(err)
	}
	detail, err = manager.Detail(detail.Run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	stages, _ = manager.store.ListPipelineStageTasks(detail.Run.RunID)
	return detail, stages[0]
}

func TestStartCreatesOneStandingStageTaskWithoutDirectLaunch(t *testing.T) {
	manager, lifecycle, _ := pipelineManagerFixture(t)
	detail, replay, err := manager.Start(context.Background(), StartRequest{
		RequestID: "v2-start", TemplateID: "quality", Project: "proj", Goal: "ship",
		Inputs:      map[string]string{"spec": "implement it"},
		Assignments: map[string]RuntimeAssignment{"standing": {Backend: "claude", Model: "sonnet"}},
	})
	if err != nil || replay {
		t.Fatalf("Start = %#v, replay=%v, err=%v", detail, replay, err)
	}
	if lifecycle.launches != nil {
		t.Fatalf("v2 Start launched directly: %#v", lifecycle.launches)
	}
	var taskID string
	if err := manager.store.DB().QueryRow(`SELECT task_id FROM pipeline_stage_tasks WHERE run_id = ?`, detail.Run.RunID).Scan(&taskID); err != nil {
		t.Fatal(err)
	}
	task, err := manager.store.ReadTask(taskID)
	if err != nil || task.Role != "orchestrator" || task.State != state.TaskReady {
		t.Fatalf("stage task = %#v, %v", task, err)
	}
}

func TestDedicatedCoordinatorWaitsForStandingOwnerConfirmation(t *testing.T) {
	manager, _, _ := pipelineManagerFixture(t)
	record, err := manager.templates.Read("quality")
	if err != nil {
		t.Fatal(err)
	}
	record.Template.Stages[0].Coordination = "dedicated"
	record.Template.Stages[0].DedicatedRole = "implementer"
	if updated, err := manager.templates.Update("quality", record.Template); err != nil || !updated.Valid {
		t.Fatalf("update template = %#v, err=%v", updated, err)
	}
	detail, _, err := manager.Start(context.Background(), StartRequest{
		RequestID: "v2-coordinator", TemplateID: "quality", Project: "proj", Goal: "ship",
		Inputs:               map[string]string{"spec": "implement it"},
		Orchestrator:         RuntimeAssignment{Backend: "claude", Model: "sonnet"},
		DedicatedAssignments: map[string]RuntimeAssignment{"work": {Backend: "codex", Model: "gpt"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	stages, err := manager.store.ListPipelineStageTasks(detail.Run.RunID)
	if err != nil || len(stages) != 1 || stages[0].CoordinatorTaskID == "" {
		t.Fatalf("stage tasks = %#v, err=%v", stages, err)
	}
	coordinator, err := manager.store.ReadTask(stages[0].CoordinatorTaskID)
	if err != nil || coordinator.State != state.TaskArmed || coordinator.Role != "implementer" {
		t.Fatalf("coordinator before bind = %#v, err=%v", coordinator, err)
	}
	if err := manager.store.BindPipelineStageTaskStandingAgent(stages[0].TaskID, "standing-agent"); err != nil {
		t.Fatal(err)
	}
	coordinator, err = manager.store.ReadTask(stages[0].CoordinatorTaskID)
	if err != nil || coordinator.State != state.TaskReady {
		t.Fatalf("coordinator after bind = %#v, err=%v", coordinator, err)
	}
}

func TestContinueFailureCreatesAnotherDurableStageTaskForStandingOwner(t *testing.T) {
	manager, _, _ := pipelineManagerFixture(t)
	detail, _, err := manager.Start(context.Background(), StartRequest{
		RequestID: "v2-continue", TemplateID: "quality", Project: "proj", Goal: "ship",
		Inputs: map[string]string{"spec": "implement it"},
		Assignments: map[string]RuntimeAssignment{
			"standing": {Backend: "claude", Model: "sonnet"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	stages, err := manager.store.ListPipelineStageTasks(detail.Run.RunID)
	if err != nil || len(stages) != 1 {
		t.Fatalf("stage tasks = %#v, err=%v", stages, err)
	}
	if _, err := manager.store.DB().Exec(`UPDATE pipeline_stage_tasks SET state = 'closing', standing_agent_id = 'agent-standing' WHERE task_id = ?`, stages[0].TaskID); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.store.DB().Exec(`UPDATE tasks SET state = ?, outcome = ? WHERE task_id = ?`, state.TaskFinished, state.OutcomeFailure, stages[0].TaskID); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.store.DB().Exec(`UPDATE pipeline_runs SET state = 'paused', pending_action = '', attention_reason = 'failure' WHERE run_id = ?`, detail.Run.RunID); err != nil {
		t.Fatal(err)
	}
	detail, err = manager.Detail(detail.Run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	continued, err := manager.Continue(context.Background(), detail.Run.RunID, detail.Run.Revision, "Correct the failed check")
	if err != nil {
		t.Fatal(err)
	}
	stages, err = manager.store.ListPipelineStageTasks(detail.Run.RunID)
	if err != nil || len(stages) != 2 {
		t.Fatalf("stage tasks = %#v, err=%v", stages, err)
	}
	retry, err := manager.store.ReadTask(stages[1].TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if continued.Run.State != "queued" || continued.Run.PendingAction != "dispatch_stage_task" || stages[1].AttemptNumber != 2 || retry.TargetKind != state.TargetAgent || retry.TargetAgentID != "agent-standing" || !strings.Contains(retry.Instruction, "Correct the failed check") {
		t.Fatalf("continued=%#v stage=%#v task=%#v", continued.Run, stages[1], retry)
	}
}

func TestReplaceInterruptedStandingOwnerFencesOldTask(t *testing.T) {
	manager, _, _ := pipelineManagerFixture(t)
	detail, _, err := manager.Start(context.Background(), StartRequest{RequestID: "v2-replace", TemplateID: "quality", Project: "proj", Goal: "ship", Inputs: map[string]string{"spec": "implement it"}, Orchestrator: RuntimeAssignment{Backend: "claude", Model: "sonnet"}})
	if err != nil {
		t.Fatal(err)
	}
	stages, _ := manager.store.ListPipelineStageTasks(detail.Run.RunID)
	if _, err := manager.store.DB().Exec(`UPDATE tasks SET state = ?, assigned_agent_id = 'old-owner' WHERE task_id = ?`, state.TaskInterrupted, stages[0].TaskID); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.store.DB().Exec(`UPDATE pipeline_stage_tasks SET standing_agent_id = 'old-owner' WHERE task_id = ?`, stages[0].TaskID); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.store.DB().Exec(`UPDATE pipeline_runs SET state = 'paused', pending_action = '', attention_reason = 'interrupted' WHERE run_id = ?`, detail.Run.RunID); err != nil {
		t.Fatal(err)
	}
	detail, _ = manager.Detail(detail.Run.RunID)
	replaced, err := manager.Replace(context.Background(), detail.Run.RunID, detail.Run.Revision, RuntimeAssignment{Backend: "codex", Model: "gpt"})
	if err != nil {
		t.Fatal(err)
	}
	stages, _ = manager.store.ListPipelineStageTasks(detail.Run.RunID)
	oldTask, _ := manager.store.ReadTask(stages[0].TaskID)
	newTask, _ := manager.store.ReadTask(stages[1].TaskID)
	if replaced.Run.State != "queued" || stages[0].State != "replaced" || stages[1].AttemptNumber != 2 || oldTask.Outcome != state.OutcomeCancelled || newTask.State != state.TaskReady || newTask.TargetKind != state.TargetLaunch || newTask.Model != "gpt" {
		t.Fatalf("run=%#v stages=%#v old=%#v new=%#v", replaced.Run, stages, oldTask, newTask)
	}
	if _, err := manager.store.AcceptPipelineStageTaskResult(oldTask.TaskID, "old-owner", "", oldTask.ExecutionHandle, replaced.Run.Revision, state.TaskResult{Outcome: state.OutcomeSuccess, Summary: "stale"}); !errors.Is(err, state.ErrPipelineStageConflict) {
		t.Fatalf("stale result error = %v", err)
	}
}

// FS-14.A30: permission attention is derived, edge-triggered, and clears
// without mutating the durable run revision or transition state.
func TestPermissionAttentionIsDerivedAndIdempotent(t *testing.T) {
	manager, _, publisher := pipelineManagerFixture(t)
	detail, _ := startStagePipeline(t, manager, "permission-attention", "a_owner", "gen-1")
	agentID, generation := "a_owner", "gen-1"
	if err := manager.OnPermissionEvent(agentID, generation, "tc_1", true); err != nil {
		t.Fatal(err)
	}
	if err := manager.OnPermissionEvent(agentID, generation, "tc_1", true); err != nil {
		t.Fatal(err)
	}
	derived, err := manager.Detail(detail.Run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if derived.Run.AttentionReason != "awaiting permission approval" || derived.Run.Revision != detail.Run.Revision {
		t.Fatalf("derived run = %+v", derived.Run)
	}
	if len(publisher.notifications) != 1 || publisher.notifications[0] != "needs_attention" {
		t.Fatalf("notifications = %v", publisher.notifications)
	}
	if err := manager.OnPermissionEvent(agentID, generation, "tc_2", true); err != nil {
		t.Fatal(err)
	}
	if err := manager.OnPermissionEvent(agentID, generation, "tc_1", false); err != nil {
		t.Fatal(err)
	}
	stillWaiting, err := manager.Detail(detail.Run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if stillWaiting.Run.AttentionReason != "awaiting permission approval" {
		t.Fatalf("first resolution cleared concurrent permission: %+v", stillWaiting.Run)
	}
	if err := manager.OnPermissionEvent(agentID, generation, "tc_2", false); err != nil {
		t.Fatal(err)
	}
	cleared, err := manager.Detail(detail.Run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if cleared.Run.AttentionReason != "" || cleared.Run.PendingAction != "" {
		t.Fatalf("cleared run = %+v", cleared.Run)
	}
}

// TS-09.R40/R42/R43, INV §5/§15 — waiting only for the standing owner's own
// release advanced the run with the finished stage's descendants still alive,
// and a release that settled in the cleanup timer re-drove nothing, parking
// `finishing` forever. One convergence contract cancels the stage's unfinished
// members, retains the cursor until every effect settles, and only then writes
// the next stage.
func TestStageCleanupCancelsDescendantsBeforeAdvancing(t *testing.T) {
	manager, _, _ := pipelineManagerFixture(t)
	detail, stage := startStagePipeline(t, manager, "stage-cleanup", "a_owner", "gen-1")
	if _, err := manager.store.DB().Exec(`UPDATE tasks SET execution_handle = 'ta_1' WHERE task_id = ?`, stage.TaskID); err != nil {
		t.Fatal(err)
	}
	// Two descendants the owner created for this stage attempt: one running, one
	// still only ready.
	addChild := func(id, taskState string) {
		t.Helper()
		if _, err := manager.store.DB().Exec(`
INSERT INTO tasks(task_id, project, display_name, instruction, target_kind, state, created_by_kind, created_by_agent_id, created_by_generation, created_at, updated_at)
VALUES (?, 'app', ?, 'work', 'launch', ?, 'agent', 'a_owner', 'gen-1', ?, ?)`,
			id, id, taskState, detail.Run.CreatedAt.Format(time.RFC3339Nano), detail.Run.CreatedAt.Format(time.RFC3339Nano)); err != nil {
			t.Fatal(err)
		}
		if _, err := manager.store.DB().Exec(`
INSERT INTO task_lineage(task_id, parent_task_id, pipeline_run_id, pipeline_stage_id, creation_attempt_id, created_at)
VALUES (?, ?, ?, ?, '1', ?)`, id, stage.TaskID, detail.Run.RunID, stage.StageID,
			detail.Run.CreatedAt.Format(time.RFC3339Nano)); err != nil {
			t.Fatal(err)
		}
	}
	addChild("tk_running_child", state.TaskRunning)
	addChild("tk_ready_child", state.TaskReady)

	if _, err := manager.store.AcceptPipelineStageTaskResult(stage.TaskID, "a_owner", "gen-1", "ta_1", detail.Run.Revision,
		state.TaskResult{Outcome: state.OutcomeSuccess, Summary: "stage done", Outputs: map[string]string{"implementation": "done"}}); err != nil {
		t.Fatal(err)
	}

	// The owner's release has not settled yet: nothing advances, but both
	// descendants are cancelled now rather than left running beside a new stage.
	if err := manager.Reconcile(context.Background(), detail.Run.RunID); err != nil {
		t.Fatal(err)
	}
	stages, err := manager.store.ListPipelineStageTasks(detail.Run.RunID)
	if err != nil || len(stages) != 1 {
		t.Fatalf("stage tasks = %#v, %v; want the run held at the finishing stage", stages, err)
	}
	for _, id := range []string{"tk_running_child", "tk_ready_child"} {
		child, err := manager.store.ReadTask(id)
		if err != nil || child.State != state.TaskFinished || child.Outcome != state.OutcomeCancelled {
			t.Fatalf("%s = %#v, %v; want a host-cancelled result", id, child, err)
		}
	}
	held, err := manager.store.ReadPipelineRun(detail.Run.RunID)
	if err != nil || held.State != "finishing" || held.PendingAction != "release_stage_task" {
		t.Fatalf("held run = %#v, %v", held, err)
	}
	// Retained cleanup on a finishing run still has an operator repair route.
	if !CleanupRepairable(held.State, held.PendingAction) {
		t.Fatal("a finishing run with retained cleanup has no repair control")
	}

	// The dispatcher's cleanup pass finally completes the release; the same
	// reconcile now converges and writes the next stage.
	if err := manager.store.CompleteTaskRelease(stage.TaskID); err != nil {
		t.Fatal(err)
	}
	if err := manager.Reconcile(context.Background(), detail.Run.RunID); err != nil {
		t.Fatal(err)
	}
	stages, err = manager.store.ListPipelineStageTasks(detail.Run.RunID)
	if err != nil || len(stages) != 2 || stages[1].StageID != "review" {
		t.Fatalf("stage tasks after convergence = %#v, %v", stages, err)
	}
}

// INV §16 — the per-run control mutex was appended and never removed, so a
// long-lived server retained one for every run it had ever started or read a
// control for, including deleted ones. The lock still excludes concurrent
// control transitions on the same run, and the map is reclaimed once the last
// holder and waiter release.
func TestRunLockExcludesAndReclaims(t *testing.T) {
	manager, _, _ := pipelineManagerFixture(t)

	held := manager.lockRun("pr_lock")
	entered := make(chan struct{})
	released := make(chan struct{})
	go func() {
		unlock := manager.lockRun("pr_lock")
		close(entered)
		unlock()
		close(released)
	}()
	select {
	case <-entered:
		t.Fatal("a second holder entered the same run's critical section")
	case <-time.After(50 * time.Millisecond):
	}
	manager.locksMu.Lock()
	waiting := len(manager.locks)
	manager.locksMu.Unlock()
	if waiting != 1 {
		t.Fatalf("retained locks while held = %d, want the one in use", waiting)
	}
	held()
	<-released

	manager.locksMu.Lock()
	remaining := len(manager.locks)
	manager.locksMu.Unlock()
	if remaining != 0 {
		t.Fatalf("retained locks after release = %d, want the map reclaimed", remaining)
	}
}

// TS-09.R42/R43, INV §15 — a Stop has already committed its closure fence, so a
// failed startup reconciliation must not pause the run: pausing dropped the
// fence and re-enabled Retry on a run a person asked to stop. The stop and its
// pending cleanup stand until cleanup can finish.
func TestFailedStartupRecoveryRetainsTheStopFence(t *testing.T) {
	manager, _, _ := pipelineManagerFixture(t)
	detail, _ := startStagePipeline(t, manager, "startup-stop-fence", "a_owner", "gen-1")
	if _, err := manager.store.DB().Exec(`UPDATE pipeline_runs SET state = 'stopping', pending_action = 'cleanup_run' WHERE run_id = ?`, detail.Run.RunID); err != nil {
		t.Fatal(err)
	}
	if err := manager.pauseStartupRun(context.Background(), detail.Run.RunID, "restart_reconcile_failed"); err != nil {
		t.Fatal(err)
	}
	run, err := manager.store.ReadPipelineRun(detail.Run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if run.State != "stopping" || run.PendingAction != "cleanup_run" {
		t.Fatalf("recovered run = %#v, want the stop and its cleanup retained", run)
	}
	if _, err := manager.Retry(context.Background(), run.RunID, run.Revision); err == nil {
		t.Fatal("Retry became available on a stopping run")
	}
}

// FS-14.A1 / INV §2: run assignments use configured catalog ids rather than
// the role/project filename slug rule. The seeded Codex id contains dots.
func TestStartAcceptsSeededCodexModelID(t *testing.T) {
	manager, lifecycle, _ := pipelineManagerFixture(t)
	detail, replay, err := manager.Start(context.Background(), StartRequest{
		RequestID: "request-dotted-codex-model", TemplateID: "quality", DisplayName: "Ship", Project: "app", Goal: "Implement the spec",
		Inputs: map[string]string{"spec": "Requirements"},
		Assignments: map[string]RuntimeAssignment{
			"work": {Backend: "codex", Model: "gpt-5.6-sol"}, "review": {Backend: "claude", Model: "sonnet"},
		},
	})
	if err != nil || replay {
		t.Fatalf("Start = %+v replay=%v err=%v", detail, replay, err)
	}
	if len(lifecycle.launches) != 0 {
		t.Fatalf("v2 Start launched directly: %+v", lifecycle.launches)
	}
}

func TestStartRejectsProjectArchiveClaimBeforeDurableMutation(t *testing.T) {
	manager, lifecycle, _ := pipelineManagerFixture(t)
	lifecycle.startErr = &ProjectGateError{Code: "project_archiving", Message: "project archive is in progress"}

	_, _, err := manager.Start(context.Background(), StartRequest{
		RequestID: "request-project-archiving", TemplateID: "quality", DisplayName: "Ship", Project: "app", Goal: "Implement the spec",
		Inputs: map[string]string{"spec": "Requirements"},
		Assignments: map[string]RuntimeAssignment{
			"work": {Backend: "codex", Model: "gpt"}, "review": {Backend: "claude", Model: "sonnet"},
		},
	})
	var gateErr *ProjectGateError
	if !errors.As(err, &gateErr) || gateErr.Code != "project_archiving" {
		t.Fatalf("Start error = %#v", err)
	}
	runs, err := manager.store.ListPipelineRuns(10, 0)
	if err != nil || len(runs) != 0 {
		t.Fatalf("runs = %+v err=%v", runs, err)
	}
}

func TestRunProposalDerivesAStableRequestIDFromExactPayload(t *testing.T) {
	manager, _, _ := pipelineManagerFixture(t)
	proposal, err := manager.ProposeRun(context.Background(), StartRequest{
		TemplateID: "quality", DisplayName: "Ship", Project: "app", Goal: "Implement the spec",
		Inputs: map[string]string{"spec": "Requirements"},
		Assignments: map[string]RuntimeAssignment{
			"work": {Backend: "codex", Model: "gpt"}, "review": {Backend: "claude", Model: "sonnet"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	payload, ok := proposal.Payload.(StartRequest)
	if !ok || payload.RequestID != proposal.ProposalID || payload.RequestID == "proposal-validation" {
		t.Fatalf("proposal = %+v payload = %+v", proposal, payload)
	}
	repeated, err := manager.ProposeRun(context.Background(), StartRequest{
		TemplateID: "quality", DisplayName: "Ship", Project: "app", Goal: "Implement the spec",
		Inputs: map[string]string{"spec": "Requirements"},
		Assignments: map[string]RuntimeAssignment{
			"work": {Backend: "codex", Model: "gpt"}, "review": {Backend: "claude", Model: "sonnet"},
		},
	})
	if err != nil || repeated.ProposalID != proposal.ProposalID {
		t.Fatalf("repeated = %+v err = %v", repeated, err)
	}
}

func TestTemplateProposalEnforcesTheCentralPayloadBound(t *testing.T) {
	manager, _, _ := pipelineManagerFixture(t)
	template := Template{Version: 2, Title: "Oversized", OrchestratorRole: "orchestrator", Inputs: []ValueDecl{}, Stages: []Stage{}}
	for i := 0; i < MaxStages; i++ {
		stageID := fmt.Sprintf("stage-%d", i)
		template.Stages = append(template.Stages, Stage{
			ID: stageID, Title: stageID, Objective: strings.Repeat("x", 9000), Instruction: strings.Repeat("x", 9000),
			Inputs: []StageInput{}, Outputs: []StageOutput{},
		})
	}
	_, err := manager.ProposeTemplate("oversized", template)
	var controlled *ControlError
	if !errors.As(err, &controlled) || controlled.Code != "validation_failed" || !hasDiagnostic(controlled.Diagnostics, "too_large") {
		t.Fatalf("ProposeTemplate error = %#v", err)
	}
}

func TestStartRejectsMissingRequiredFirstStageValueBeforeSideEffects(t *testing.T) {
	manager, lifecycle, _ := pipelineManagerFixture(t)
	_, _, err := manager.Start(context.Background(), StartRequest{
		RequestID: "request-missing-first-input", TemplateID: "quality", DisplayName: "Ship", Project: "app", Goal: "Implement the spec",
		Inputs: map[string]string{"spec": ""},
		Assignments: map[string]RuntimeAssignment{
			"work": {Backend: "codex", Model: "gpt"}, "review": {Backend: "claude", Model: "sonnet"},
		},
	})
	var controlled *ControlError
	if !errors.As(err, &controlled) || controlled.Code != "validation_failed" || !hasDiagnostic(controlled.Diagnostics, "required_for_first_stage") {
		t.Fatalf("Start error = %#v", err)
	}
	if len(lifecycle.launches) != 0 {
		t.Fatalf("missing first-stage input launched %d agents", len(lifecycle.launches))
	}
}

func TestRunListAndStartupIsolateMalformedRunDetail(t *testing.T) {
	manager, _, _ := pipelineManagerFixture(t)
	detail, _ := startStagePipeline(t, manager, "request-corrupt-detail", "a_owner", "gen-1")
	if _, err := manager.store.DB().Exec(`UPDATE pipeline_runs SET template_snapshot_json = '{', pending_action = 'release_stage_task' WHERE run_id = ?`, detail.Run.RunID); err != nil {
		t.Fatal(err)
	}
	runs, err := manager.List(10, 0)
	// The list projection decodes only the frozen snapshot (TS-03.R30), so the
	// diagnostic it shows names that failure and never claims a run-detail read.
	if err != nil || len(runs) != 1 || len(runs[0].Diagnostics) != 1 || !hasDiagnostic(runs[0].Diagnostics, "frozen_stage_title_unavailable") {
		t.Fatalf("List = %+v err=%v", runs, err)
	}
	// Startup reconciliation cannot decode the snapshot either, so it must pause
	// the run for a person rather than advance a cursor it cannot read.
	if err := manager.Startup(context.Background()); err != nil {
		t.Fatal(err)
	}
	run, err := manager.store.ReadPipelineRun(detail.Run.RunID)
	if err != nil || run.State != "paused" || run.AttentionReason != "restart_reconcile_failed" {
		t.Fatalf("recovered run = %+v err=%v", run, err)
	}
}
