package pipeline

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

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
	updates       []PipelineUpdate
	notifications []string
}

func (p *fakePublisher) PublishPipelineUpdate(update PipelineUpdate) {
	p.updates = append(p.updates, update)
}
func (p *fakePublisher) PublishPipelineNotification(_ PipelineUpdate, kind string) {
	p.notifications = append(p.notifications, kind)
}

func (p *fakePublisher) PublishPipelineProposalUpdate() {}

func pipelineManagerFixture(t *testing.T) (*Manager, *fakeLifecycle, *fakePublisher) {
	t.Helper()
	home := t.TempDir()
	configStore := config.NewWithHome(home)
	if err := configStore.EnsureLayout(); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"implementer", "reviewer"} {
		if err := configStore.WriteRole(role, config.Role{Title: role}); err != nil {
			t.Fatal(err)
		}
	}
	template := Template{
		Version: 1, Title: "Quality loop",
		Inputs: []ValueDecl{{Name: "spec", Description: "Specification", Required: true}},
		Stages: []Stage{
			{ID: "work", Title: "Work", Role: "implementer", Instruction: "Implement.", MaxVisits: 2,
				Inputs:      []StageInput{{Name: "specification", Value: "spec", Required: true}},
				Outputs:     []StageOutput{{Name: "implementation", Value: "implementation", Description: "What changed"}},
				Transitions: OutcomeTransitions{Success: Transition{Stage: "review", Approval: "automatic"}, Failure: Transition{Final: "failure", Approval: "required"}}},
			{ID: "review", Title: "Review", Role: "reviewer", Instruction: "Review.", MaxVisits: 2,
				Inputs: []StageInput{{Name: "implementation", Value: "implementation", Required: true}}, Outputs: []StageOutput{},
				Transitions: OutcomeTransitions{Success: Transition{Final: "success", Approval: "automatic"}, Failure: Transition{Stage: "work", Approval: "automatic"}}},
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

func startPipeline(t *testing.T, manager *Manager, requestID string) RunDetail {
	t.Helper()
	detail, replay, err := manager.Start(context.Background(), StartRequest{
		RequestID: requestID, TemplateID: "quality", DisplayName: "Ship", Project: "app", Goal: "Implement the spec",
		Inputs: map[string]string{"spec": "Requirements"},
		Assignments: map[string]RuntimeAssignment{
			"work": {Backend: "codex", Model: "gpt", Effort: "high"}, "review": {Backend: "claude", Model: "sonnet"},
		},
	})
	if err != nil || replay {
		t.Fatalf("Start = %+v replay=%v err=%v", detail, replay, err)
	}
	return detail
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
	if len(lifecycle.launches) != 1 || lifecycle.launches[0].Model != "gpt-5.6-sol" {
		t.Fatalf("launches = %+v", lifecycle.launches)
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
	template := Template{Version: 1, Title: "Oversized", Inputs: []ValueDecl{}, Stages: []Stage{}}
	for i := 0; i < MaxStages; i++ {
		stageID := fmt.Sprintf("stage-%d", i)
		success := Transition{Final: "success", Approval: "automatic"}
		if i+1 < MaxStages {
			success = Transition{Stage: fmt.Sprintf("stage-%d", i+1), Approval: "automatic"}
		}
		template.Stages = append(template.Stages, Stage{
			ID: stageID, Title: stageID, Role: "implementer", Instruction: strings.Repeat("x", 9000),
			Inputs: []StageInput{}, Outputs: []StageOutput{},
			Transitions: OutcomeTransitions{Success: success, Failure: Transition{Final: "failure", Approval: "required"}},
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
	manager, lifecycle, _ := pipelineManagerFixture(t)
	detail := startPipeline(t, manager, "request-corrupt-detail")
	if _, err := manager.store.DB().Exec(`UPDATE pipeline_runs SET template_snapshot_json = '{' WHERE run_id = ?`, detail.Run.RunID); err != nil {
		t.Fatal(err)
	}
	runs, err := manager.List(10, 0)
	if err != nil || len(runs) != 1 || !hasDiagnostic(runs[0].Diagnostics, "run_read_failed") {
		t.Fatalf("List = %+v err=%v", runs, err)
	}
	if err := manager.Startup(context.Background()); err != nil {
		t.Fatal(err)
	}
	run, err := manager.store.ReadPipelineRun(detail.Run.RunID)
	if err != nil || run.State != "paused" || run.AttentionReason != "restart_state_invalid" || lifecycle.IsRunning(run.CurrentAgentID) {
		t.Fatalf("recovered run = %+v running=%v err=%v", run, lifecycle.IsRunning(run.CurrentAgentID), err)
	}
}

// FS-14.A2/A3/A4: explicit results and quiescence drive one sequential route;
// blocked continuation reuses the agent while a repair-loop visit does not.
func TestManagerSequentialRoutingBlockedContinuationAndIdentity(t *testing.T) {
	manager, lifecycle, _ := pipelineManagerFixture(t)
	detail := startPipeline(t, manager, "request-routing")
	if detail.Run.PendingAction != "await_result" || len(lifecycle.launches) != 1 {
		t.Fatalf("started detail = %+v launches=%d", detail.Run, len(lifecycle.launches))
	}
	work := detail.Attempts[0]
	if err := manager.OnTurnEnd(work.AgentID, work.AgentGeneration); err != nil {
		t.Fatal(err)
	}
	unchanged, _ := manager.Detail(detail.Run.RunID)
	if unchanged.Run.CurrentAttemptID != work.AttemptID {
		t.Fatal("turn_end without result advanced the run")
	}
	if _, err := manager.Report(work.AgentID, "wrong-generation", StageReport{Outcome: "success", Summary: "done", Outputs: map[string]string{"implementation": "change"}}); err == nil {
		t.Fatal("spoofed generation report succeeded")
	}
	if _, err := manager.Report(work.AgentID, work.AgentGeneration, StageReport{Outcome: "success", Summary: "done", Outputs: map[string]string{"implementation": "change"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Report(work.AgentID, work.AgentGeneration, StageReport{Outcome: "success", Summary: "duplicate"}); err == nil {
		t.Fatal("duplicate report succeeded")
	}
	if err := manager.OnTurnEnd(work.AgentID, work.AgentGeneration); err != nil {
		t.Fatal(err)
	}
	detail, _ = manager.Detail(detail.Run.RunID)
	review := detail.Attempts[len(detail.Attempts)-1]
	if review.StageID != "review" || review.AgentID == work.AgentID || len(lifecycle.launches) != 2 {
		t.Fatalf("review attempt = %+v launches=%d", review, len(lifecycle.launches))
	}
	if _, err := manager.Report(review.AgentID, review.AgentGeneration, StageReport{Outcome: "failure", Summary: "needs repair"}); err != nil {
		t.Fatal(err)
	}
	if err := manager.OnTurnEnd(review.AgentID, review.AgentGeneration); err != nil {
		t.Fatal(err)
	}
	detail, _ = manager.Detail(detail.Run.RunID)
	repair := detail.Attempts[len(detail.Attempts)-1]
	if repair.StageID != "work" || repair.AgentID == work.AgentID || repair.VisitNo != 2 {
		t.Fatalf("repair attempt = %+v", repair)
	}
	if _, err := manager.Report(repair.AgentID, repair.AgentGeneration, StageReport{Outcome: "blocked", Summary: "need input"}); err != nil {
		t.Fatal(err)
	}
	if err := manager.OnTurnEnd(repair.AgentID, repair.AgentGeneration); err != nil {
		t.Fatal(err)
	}
	detail, _ = manager.Detail(detail.Run.RunID)
	if detail.Run.State != "paused" || detail.Run.AttentionReason != "blocked" {
		t.Fatalf("blocked run = %+v", detail.Run)
	}
	detail, err := manager.Continue(context.Background(), detail.Run.RunID, detail.Run.Revision, "Use the fallback")
	if err != nil {
		t.Fatal(err)
	}
	continued := detail.Attempts[len(detail.Attempts)-1]
	if continued.AgentID != repair.AgentID || continued.AttemptID == repair.AttemptID || continued.Effort != "high" || len(lifecycle.continuations) != 1 || lifecycle.continuations[0].Effort != "high" {
		t.Fatalf("continued attempt = %+v continuations=%d", continued, len(lifecycle.continuations))
	}
}

// TS-09.R24: a resumed stage executes the effort recorded with that attempt,
// never a value re-read from the run's assignment document.
func TestStageExecutionUsesAttemptEffort(t *testing.T) {
	detail := RunDetail{Run: state.PipelineRunRecord{RunID: "pr_1", DisplayName: "Run", Project: "app"}, Assignments: map[string]RuntimeAssignment{
		"work": {Backend: "codex", Model: "gpt", Effort: "low"},
	}}
	execution := stageExecution(detail, state.PipelineAttemptRecord{
		AttemptID: "pa_1", StageID: "work", Backend: "codex", Model: "gpt", Effort: "high",
	}, Stage{ID: "work", Title: "Work", Role: "implementer"})
	if execution.Effort != "high" {
		t.Fatalf("stage execution effort = %q, want frozen attempt effort high", execution.Effort)
	}
}

// FS-14.A4: launch failure pauses honestly and Retry creates a fresh identity.
func TestManagerLaunchFailureRetryAndStop(t *testing.T) {
	manager, lifecycle, _ := pipelineManagerFixture(t)
	lifecycle.failNextLaunch = true
	detail := startPipeline(t, manager, "request-retry")
	failed := detail.Attempts[0]
	if detail.Run.State != "paused" || detail.Run.AttentionReason != "launch_failed" {
		t.Fatalf("failed launch run = %+v", detail.Run)
	}
	detail, err := manager.Retry(context.Background(), detail.Run.RunID, detail.Run.Revision)
	if err != nil {
		t.Fatal(err)
	}
	retried := detail.Attempts[len(detail.Attempts)-1]
	if retried.AgentID == failed.AgentID || detail.Run.PendingAction != "await_result" {
		t.Fatalf("retried attempt = %+v run=%+v", retried, detail.Run)
	}
	detail, err = manager.Stop(context.Background(), detail.Run.RunID, detail.Run.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Run.State != "stopped" || detail.Run.PendingAction != "" || lifecycle.IsRunning(retried.AgentID) {
		t.Fatalf("stopped run = %+v running=%v", detail.Run, lifecycle.IsRunning(retried.AgentID))
	}
}

// FS-14.A9: destination inputs are verified before accepting any report/output.
func TestManagerRejectsMissingDestinationValueAtomically(t *testing.T) {
	manager, _, _ := pipelineManagerFixture(t)
	detail := startPipeline(t, manager, "request-values")
	attempt := detail.Attempts[0]
	if _, err := manager.Report(attempt.AgentID, attempt.AgentGeneration, StageReport{Outcome: "success", Summary: "done"}); err == nil {
		t.Fatal("report without required implementation output succeeded")
	}
	after, _ := manager.Detail(detail.Run.RunID)
	if after.Attempts[0].ReportOutcome != "" || len(after.Values) != 1 || after.Run.Revision != detail.Run.Revision {
		t.Fatalf("rejected report mutated state: %+v values=%+v", after.Run, after.Values)
	}
}

// TS-09.R12/R13: crash recovery is generation-scoped, so a stage agent that
// exits without carrying its concrete launch generation (the resume path once
// dropped it) is silently ignored and the run never receives its agent_crash
// pause. The recorded generation must pause the run with Retry available.
func TestManagerCrashRecoveryRequiresTheConcreteGeneration(t *testing.T) {
	manager, _, _ := pipelineManagerFixture(t)
	detail := startPipeline(t, manager, "request-crash")
	attempt := detail.Attempts[0]

	if err := manager.OnExit(attempt.AgentID, "", "process_exit"); err != nil {
		t.Fatal(err)
	}
	unchanged, _ := manager.Detail(detail.Run.RunID)
	if unchanged.Run.State != "running" || unchanged.Run.AttentionReason != "" {
		t.Fatalf("empty-generation exit changed the run: %+v", unchanged.Run)
	}

	if err := manager.OnExit(attempt.AgentID, attempt.AgentGeneration, "process_exit"); err != nil {
		t.Fatal(err)
	}
	crashed, _ := manager.Detail(detail.Run.RunID)
	if crashed.Run.State != "paused" || crashed.Run.AttentionReason != "agent_crash" {
		t.Fatalf("crashed run = %+v, want paused/agent_crash", crashed.Run)
	}
	if crashed.Attempts[0].State != "crashed" {
		t.Fatalf("crashed attempt = %+v", crashed.Attempts[0])
	}
	if _, err := manager.Retry(context.Background(), crashed.Run.RunID, crashed.Run.Revision); err != nil {
		t.Fatalf("Retry after crash: %v", err)
	}
}
