package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/state"
)

const maxReconcileSteps = 16

type Manager struct {
	store     *state.Store
	templates *TemplateStore
	lifecycle Lifecycle
	publisher Publisher

	locksMu            sync.Mutex
	locks              map[string]*sync.Mutex
	attentionMu        sync.Mutex
	pendingPermissions map[string]map[string]pendingPermission
}

type pendingPermission struct {
	agentID, generation, toolCallID string
}

func NewManager(store *state.Store, templates *TemplateStore, lifecycle Lifecycle, publisher Publisher) *Manager {
	return &Manager{store: store, templates: templates, lifecycle: lifecycle, publisher: publisher, locks: map[string]*sync.Mutex{}, pendingPermissions: map[string]map[string]pendingPermission{}}
}

func (m *Manager) Start(ctx context.Context, request StartRequest) (RunDetail, bool, error) {
	requestHash, err := startRequestHash(request)
	if err != nil {
		return RunDetail{}, false, err
	}
	if detail, found, err := m.LookupStart(request); err != nil || found {
		return detail, found, err
	}
	release, err := m.acquireProjectStart(ctx, request.Project)
	if err != nil {
		return RunDetail{}, false, err
	}
	defer release()
	record, startErr := m.validateStart(ctx, &request)
	if startErr != nil {
		return RunDetail{}, false, startErr
	}
	runID, err := m.store.NewPipelineRunID()
	if err != nil {
		return RunDetail{}, false, err
	}
	taskID, err := m.store.NewTaskID()
	if err != nil {
		return RunDetail{}, false, err
	}
	templateJSON, _ := json.Marshal(record.Template)
	inputsJSON, _ := json.Marshal(request.Inputs)
	assignmentsJSON, _ := json.Marshal(request.Assignments)
	now := time.Now().UTC()
	first := record.Template.Stages[0]
	run := state.PipelineRunRecord{
		RunID: runID, TemplateID: request.TemplateID, TemplateSnapshot: templateJSON,
		DisplayName: request.DisplayName, Project: request.Project, Goal: request.Goal,
		Inputs: inputsJSON, Assignments: assignmentsJSON, State: "queued", Revision: 1,
		PendingAction: "dispatch_stage_task", CurrentStageID: first.ID, CreatedAt: now, UpdatedAt: now,
	}
	values := make([]state.PipelineValueRecord, 0, len(request.Inputs))
	for name, value := range request.Inputs {
		values = append(values, state.PipelineValueRecord{RunID: runID, Name: name, Value: value, SourceKind: "run_input", UpdatedAt: now})
	}
	assignmentText, assignmentHash := renderAssignment(run, record.Template, first, values, nil, "")
	assignment := standingAssignment(request.Assignments, first.ID)
	coordinator, err := m.coordinatorTask(run.Project, run.Goal, first, request.Assignments[first.ID])
	if err != nil {
		return RunDetail{}, false, err
	}
	created, replay, err := m.store.CreatePipelineRun(state.CreatePipelineRunParams{
		Run: run, RequestID: request.RequestID, RequestHash: requestHash, Values: values,
		InitialStageTask: &state.CreatePipelineStageTaskParams{RunID: runID, ExpectedRevision: 1, StageIndex: 0, AttemptNumber: 1, StageID: first.ID, AssignmentDigest: assignmentHash, OutputValues: stageOutputValues(first), Coordinator: coordinator, Task: state.Task{TaskID: taskID, Project: request.Project, DisplayName: first.Title, Instruction: assignmentText, TargetKind: state.TargetLaunch, Role: record.Template.OrchestratorRole, Backend: assignment.Backend, Model: assignment.Model, Effort: assignment.Effort, Fast: assignment.Fast, CreatedByKind: "pipeline"}},
	})
	if err != nil {
		if errors.Is(err, state.ErrPipelineRequestConflict) {
			return RunDetail{}, false, controlError("request_conflict", "request_id was already used with different content")
		}
		return RunDetail{}, false, err
	}
	// The run is durable, so the proposal that asked for it is no longer pending.
	// A proposal's request id is its own content-addressed id, so a manual start
	// simply matches no record (FS-14.R33, TS-09.R26).
	m.consumeProposal(request.RequestID)
	m.publish(created)
	detail, err := m.Detail(created.RunID)
	return detail, replay, err
}

func (m *Manager) coordinatorTask(project, goal string, stage Stage, assignment RuntimeAssignment) (*state.Task, error) {
	if stage.Coordination != "dedicated" {
		return nil, nil
	}
	id, err := m.store.NewTaskID()
	if err != nil {
		return nil, err
	}
	instruction := fmt.Sprintf("Coordinate pipeline stage %q. Run goal: %s\nStage objective: %s\nCreate and manage durable child work, report progress and your final result to the standing owner, and do not report the pipeline stage itself.", stage.Title, goal, stage.Objective)
	return &state.Task{TaskID: id, Project: project, DisplayName: stage.Title + " coordinator", Instruction: instruction, TargetKind: state.TargetLaunch, Role: stage.DedicatedRole, Backend: assignment.Backend, Model: assignment.Model, Effort: assignment.Effort, Fast: assignment.Fast, CreatedByKind: "pipeline_coordinator"}, nil
}

// OnStageTaskInterrupted projects an unexpected standing-owner exit onto the
// run without guessing an outcome. Retry remains an explicit operator action.
func (m *Manager) OnStageTaskInterrupted(taskID string) error {
	stage, err := m.store.ReadPipelineStageTaskByTask(taskID)
	if errors.Is(err, state.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	run, err := m.store.ReadPipelineRun(stage.RunID)
	if err != nil {
		return err
	}
	if run.State == "completed" || run.State == "stopped" || run.State == "stopping" {
		return nil
	}
	updated, err := m.store.UpdatePipelineRunCAS(run.RunID, run.Revision, state.PipelineRunUpdate{
		State: "paused", PendingAction: "", CurrentStageID: stage.StageID,
		CurrentAgentID: stage.StandingAgentID, AttentionReason: "interrupted",
	})
	if errors.Is(err, state.ErrPipelineConflict) {
		return nil
	}
	if err == nil {
		m.publish(updated)
		m.notify(updated, "needs_attention")
	}
	return err
}

func stageOutputValues(stage Stage) map[string]string {
	out := make(map[string]string, len(stage.Outputs))
	for _, output := range stage.Outputs {
		out[output.Name] = output.Value
	}
	return out
}

// standingAssignment accepts the temporary stage-keyed HTTP shape while v2
// callers move to the explicit standing key. The standing owner is the only
// stage executor; dedicated coordinators are child-task configuration.
func standingAssignment(assignments map[string]RuntimeAssignment, firstStageID string) RuntimeAssignment {
	if a, ok := assignments["standing"]; ok {
		return a
	}
	return assignments[firstStageID]
}

func startRequestHash(request StartRequest) (string, error) {
	request.RequestID = ""
	return Digest(request)
}

// LookupStart resolves an exact idempotent replay without consulting mutable
// templates/config or workspace conflicts. The original frozen run wins.
func (m *Manager) LookupStart(request StartRequest) (RunDetail, bool, error) {
	if strings.TrimSpace(request.RequestID) == "" {
		return RunDetail{}, false, nil
	}
	runID, storedHash, err := m.store.ReadPipelineRequest(request.RequestID)
	if errors.Is(err, state.ErrNotFound) {
		return RunDetail{}, false, nil
	}
	if err != nil {
		return RunDetail{}, false, err
	}
	hash, err := startRequestHash(request)
	if err != nil {
		return RunDetail{}, false, err
	}
	if hash != storedHash {
		return RunDetail{}, false, controlError("request_conflict", "request_id was already used with different content")
	}
	detail, err := m.Detail(runID)
	return detail, true, err
}

func (m *Manager) acquireProjectStart(ctx context.Context, project string) (func(), error) {
	if m.lifecycle == nil {
		return func() {}, nil
	}
	return m.lifecycle.AcquirePipelineStart(ctx, project)
}

func (m *Manager) validateStart(ctx context.Context, request *StartRequest) (TemplateRecord, error) {
	// V2 names the standing owner and optional dedicated coordinators directly.
	// Keep one canonical frozen assignment map internally while older local
	// callers finish migrating.
	if request.Assignments == nil {
		request.Assignments = map[string]RuntimeAssignment{}
	}
	if request.Orchestrator.Backend != "" || request.Orchestrator.Model != "" {
		request.Assignments["standing"] = request.Orchestrator
	}
	for stageID, assignment := range request.DedicatedAssignments {
		request.Assignments[stageID] = assignment
	}
	diagnostics := []Diagnostic{}
	add := func(field, code, message string) {
		diagnostics = appendBounded(diagnostics, Diagnostic{Field: field, Code: code, Message: message})
	}
	if strings.TrimSpace(request.RequestID) == "" || utf8.RuneCountInString(request.RequestID) > MaxTitleRunes {
		add("request_id", "invalid", "request_id is required and must be at most 120 characters")
	}
	if !config.ValidSlug(request.TemplateID) {
		add("template_id", "invalid_slug", "template_id must be a lowercase slug")
	}
	if !config.ValidSlug(request.Project) {
		add("project", "invalid_slug", "project must be a lowercase slug")
	}
	if strings.TrimSpace(request.Goal) == "" || utf8.RuneCountInString(request.Goal) > MaxGoalRunes {
		add("goal", "invalid", fmt.Sprintf("goal is required and must be at most %d characters", MaxGoalRunes))
	}
	record, err := m.templates.Read(request.TemplateID)
	if err != nil {
		if errors.Is(err, ErrTemplateNotFound) {
			add("template_id", "not_found", "pipeline template does not exist")
			return TemplateRecord{}, validationError("run cannot start", diagnostics)
		}
		return TemplateRecord{}, err
	}
	if !record.Valid {
		diagnostics = append(diagnostics, record.Diagnostics...)
	}
	if request.DisplayName == "" {
		request.DisplayName = record.Template.Title
	}
	if utf8.RuneCountInString(request.DisplayName) > MaxTitleRunes {
		add("display_name", "too_long", fmt.Sprintf("display_name must be at most %d characters", MaxTitleRunes))
	}
	if request.Inputs == nil {
		request.Inputs = map[string]string{}
	}
	declaredInputs := map[string]ValueDecl{}
	for _, input := range record.Template.Inputs {
		declaredInputs[input.Name] = input
		value, ok := request.Inputs[input.Name]
		if input.Required && (!ok || strings.TrimSpace(value) == "") {
			add("inputs."+input.Name, "required", "required run input is missing")
		}
		if utf8.RuneCountInString(value) > MaxValueRunes {
			add("inputs."+input.Name, "too_long", fmt.Sprintf("input must be at most %d characters", MaxValueRunes))
		}
	}
	for name := range request.Inputs {
		if _, ok := declaredInputs[name]; !ok {
			add("inputs."+name, "unknown", "input is not declared by the template")
		}
	}
	if len(record.Template.Stages) > 0 {
		for _, input := range record.Template.Stages[0].Inputs {
			if input.Required && strings.TrimSpace(request.Inputs[input.Value]) == "" {
				add("inputs."+input.Value, "required_for_first_stage", "the first stage requires a non-empty value named "+input.Value)
			}
		}
	}
	for _, stage := range record.Template.Stages {
		if stage.Coordination != "dedicated" {
			continue
		}
		assignment := request.Assignments[stage.ID]
		field := "dedicated_assignments." + stage.ID
		if assignment.Backend == "" || assignment.Model == "" {
			add(field, "required", "a dedicated coordinator requires a configured backend and model")
			continue
		}
		if m.lifecycle != nil {
			if err := m.lifecycle.ValidateStage(ctx, StageExecution{StageID: stage.ID, StageTitle: stage.Title, Role: stage.DedicatedRole, Project: request.Project, Backend: assignment.Backend, Model: assignment.Model, Effort: assignment.Effort, Fast: assignment.Fast}); err != nil {
				add(field, "unavailable", err.Error())
			}
		}
	}
	if len(record.Template.Stages) > 0 {
		assignment := standingAssignment(request.Assignments, record.Template.Stages[0].ID)
		if assignment.Backend == "" || assignment.Model == "" {
			add("assignments.standing", "required", "the standing orchestrator requires a configured backend and model")
		} else if m.lifecycle != nil {
			if err := m.lifecycle.ValidateStage(ctx, StageExecution{StageID: record.Template.Stages[0].ID, StageTitle: record.Template.Stages[0].Title, Role: record.Template.OrchestratorRole, Project: request.Project, Backend: assignment.Backend, Model: assignment.Model, Effort: assignment.Effort, Fast: assignment.Fast}); err != nil {
				add("assignments.standing", "unavailable", err.Error())
			}
		}
	}
	if len(diagnostics) > 0 {
		return record, validationError("run cannot start", diagnostics)
	}
	return record, nil
}

func (m *Manager) Detail(runID string) (RunDetail, error) {
	run, err := m.store.ReadPipelineRun(runID)
	if err != nil {
		return RunDetail{}, err
	}
	detail := RunDetail{Run: run, Inputs: map[string]string{}, Assignments: map[string]RuntimeAssignment{}, Attempts: []state.PipelineAttemptRecord{}, Values: []state.PipelineValueRecord{}, Diagnostics: []Diagnostic{}}
	if err := json.Unmarshal(run.TemplateSnapshot, &detail.Template); err != nil {
		return RunDetail{}, fmt.Errorf("pipeline: decode template snapshot: %w", err)
	}
	detail.Template = NormalizeTemplate(detail.Template)
	if err := json.Unmarshal(run.Inputs, &detail.Inputs); err != nil {
		return RunDetail{}, fmt.Errorf("pipeline: decode run inputs: %w", err)
	}
	if detail.Inputs == nil {
		detail.Inputs = map[string]string{}
	}
	if err := json.Unmarshal(run.Assignments, &detail.Assignments); err != nil {
		return RunDetail{}, fmt.Errorf("pipeline: decode run assignments: %w", err)
	}
	if detail.Assignments == nil {
		detail.Assignments = map[string]RuntimeAssignment{}
	}
	detail.Attempts, err = m.store.ListPipelineAttempts(runID)
	if err != nil {
		return RunDetail{}, err
	}
	detail.Values, err = m.store.ListPipelineValues(runID)
	if stages, stageErr := m.store.ListPipelineStageTasks(runID); stageErr != nil {
		return RunDetail{}, stageErr
	} else if len(stages) > 0 {
		current := stages[len(stages)-1]
		detail.Run.CurrentTaskID = current.TaskID
		detail.Run.OrchestratorAgentID = current.StandingAgentID
		detail.Run.CurrentAgentID = current.StandingAgentID
	}
	if err == nil && m.hasPendingPermission(runID, detail.Run.CurrentAgentID) {
		detail.Run.AttentionReason = "awaiting permission approval"
	}
	return detail, err
}

func (m *Manager) List(limit, offset int) ([]RunSummary, error) {
	runs, _, err := m.ListPage(limit, offset)
	return runs, err
}

// ListPage builds the bounded Runs projection without loading per-run attempts
// or values. The exact retained total is returned with the same state snapshot
// as the page for the additive HTTP pagination contract.
func (m *Manager) ListPage(limit, offset int) ([]RunSummary, int, error) {
	if limit <= 0 || limit > MaxListPage {
		limit = MaxListPage
	}
	page, err := m.store.ListPipelineRunPage(limit, offset)
	if err != nil {
		return nil, 0, err
	}
	out := make([]RunSummary, 0, len(page.Runs))
	for _, run := range page.Runs {
		if m.hasPendingPermission(run.RunID, run.CurrentAgentID) {
			run.AttentionReason = "awaiting permission approval"
		}
		diagnostics := []Diagnostic{}
		stageTitle := run.CurrentStageID
		var snapshot Template
		if err := json.Unmarshal(run.TemplateSnapshot, &snapshot); err != nil {
			// Only the frozen snapshot failed to decode: this projection never
			// reads full run detail, so it must not claim it did (INV §8).
			diagnostics = appendBounded(diagnostics, Diagnostic{Field: "current_stage_title", Code: "frozen_stage_title_unavailable", Message: "frozen template snapshot could not be decoded"})
		} else {
			foundTitle := false
			for _, stage := range snapshot.Stages {
				if stage.ID == run.CurrentStageID && stage.Title != "" {
					stageTitle = stage.Title
					foundTitle = true
					break
				}
			}
			if !foundTitle && run.CurrentStageID != "" {
				diagnostics = appendBounded(diagnostics, Diagnostic{Field: "current_stage_title", Code: "frozen_stage_title_unavailable", Message: "frozen template snapshot has no current stage title"})
			}
		}
		out = append(out, RunSummary{
			RunID: run.RunID, TemplateID: run.TemplateID, DisplayName: run.DisplayName,
			Project: run.Project, State: run.State, Revision: run.Revision,
			PendingAction: run.PendingAction, CurrentStageID: run.CurrentStageID,
			CurrentStageTitle: stageTitle,
			CurrentAgentID:    run.CurrentAgentID, AttentionReason: run.AttentionReason,
			FinalOutcome: run.FinalOutcome, UpdatedAt: run.UpdatedAt.Format(time.RFC3339Nano),
			Diagnostics: diagnostics,
		})
	}
	return out, page.Total, nil
}

func (m *Manager) hasPendingPermission(runID, agentID string) bool {
	m.attentionMu.Lock()
	defer m.attentionMu.Unlock()
	for _, pending := range m.pendingPermissions[runID] {
		if pending.agentID == agentID {
			return true
		}
	}
	return false
}

// OnPermissionEvent derives pipeline attention from the current stage agent's
// process-lifetime permission state. It changes no durable run state.
func (m *Manager) OnPermissionEvent(agentID, generation, toolCallID string, pending bool) error {
	stage, task, err := m.store.PipelineStageTaskForAssignee(agentID, generation)
	if errors.Is(err, state.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	run, err := m.store.ReadPipelineRun(stage.RunID)
	if err != nil {
		return err
	}
	if stage.State != "open" || (task.State != state.TaskStarting && task.State != state.TaskRunning) {
		return nil
	}
	m.attentionMu.Lock()
	requests := m.pendingPermissions[run.RunID]
	wasPending := len(requests) > 0
	changed := false
	if pending {
		if requests == nil {
			requests = map[string]pendingPermission{}
			m.pendingPermissions[run.RunID] = requests
		}
		if current, exists := requests[toolCallID]; !exists || current.agentID != agentID || current.generation != generation {
			requests[toolCallID] = pendingPermission{agentID: agentID, generation: generation, toolCallID: toolCallID}
		}
	} else if current, exists := requests[toolCallID]; exists && current.agentID == agentID && current.generation == generation {
		delete(requests, toolCallID)
		if len(requests) == 0 {
			delete(m.pendingPermissions, run.RunID)
		}
	}
	isPending := len(requests) > 0
	changed = wasPending != isPending
	m.attentionMu.Unlock()
	if !changed || m.publisher == nil {
		return nil
	}
	reason := ""
	if isPending {
		reason = "awaiting permission approval"
	}
	update := PipelineUpdate{RunID: run.RunID, DisplayName: run.DisplayName, Revision: run.Revision, State: run.State, CurrentStageID: run.CurrentStageID, CurrentAgentID: run.CurrentAgentID, AttentionReason: reason, FinalOutcome: run.FinalOutcome}
	m.publisher.PublishPipelineUpdate(update)
	if isPending {
		m.publisher.PublishPipelineNotification(update, "needs_attention")
	}
	return nil
}

func (m *Manager) ClearPermissionAttention(agentID, generation string) {
	m.attentionMu.Lock()
	for runID, requests := range m.pendingPermissions {
		for toolCallID, pending := range requests {
			if pending.agentID == agentID && pending.generation == generation {
				delete(requests, toolCallID)
			}
		}
		if len(requests) == 0 {
			delete(m.pendingPermissions, runID)
		}
	}
	m.attentionMu.Unlock()
}

func (m *Manager) Startup(ctx context.Context) error {
	if err := m.resetLegacyPipelines(ctx); err != nil {
		return err
	}
	runs, err := m.store.ListActivePipelineRuns()
	if err != nil {
		return err
	}
	for _, run := range runs {
		if run.PendingAction == "" || run.PendingAction == "await_approval" {
			continue
		}
		if err := m.Reconcile(ctx, run.RunID); err != nil {
			if pauseErr := m.pauseStartupRun(ctx, run.RunID, "restart_reconcile_failed"); pauseErr != nil {
				return errors.Join(err, pauseErr)
			}
		}
	}
	return nil
}

// resetLegacyPipelines is the authorized v1 cutover. It stops recorded legacy
// agents before deleting only runs with no task-stage provenance, then removes
// explicitly versioned v1 template files. A durable checkpoint makes retries
// safe after a crash or filesystem failure.
func (m *Manager) resetLegacyPipelines(ctx context.Context) error {
	run, agents, err := m.store.BeginLegacyPipelineReset()
	if err != nil || !run {
		return err
	}
	if m.lifecycle != nil {
		for _, agentID := range agents {
			if err := m.lifecycle.StopStage(ctx, agentID); err != nil {
				return err
			}
		}
	}
	if err := m.store.RemoveLegacyPipelineRecords(); err != nil {
		return err
	}
	if err := m.templates.ResetVersion1Files(); err != nil {
		return err
	}
	return m.store.CompleteLegacyPipelineReset()
}

func (m *Manager) pauseStartupRun(ctx context.Context, runID, reason string) error {
	run, err := m.store.ReadPipelineRun(runID)
	if err != nil {
		return err
	}
	if run.State == "completed" || run.State == "stopped" {
		return nil
	}
	updated, err := m.store.UpdatePipelineRunCAS(run.RunID, run.Revision, state.PipelineRunUpdate{
		State: "paused", PendingAction: "", CurrentStageID: run.CurrentStageID,
		CurrentAttemptID: run.CurrentAttemptID, CurrentAgentID: run.CurrentAgentID,
		AttentionReason: reason,
	})
	if errors.Is(err, state.ErrPipelineConflict) {
		return nil
	}
	if err != nil {
		return err
	}
	m.publish(updated)
	m.notify(updated, "needs_attention")
	if m.lifecycle != nil && run.CurrentAgentID != "" {
		_ = m.lifecycle.StopStage(ctx, run.CurrentAgentID)
	}
	return nil
}

func (m *Manager) runLock(runID string) *sync.Mutex {
	m.locksMu.Lock()
	defer m.locksMu.Unlock()
	lock := m.locks[runID]
	if lock == nil {
		lock = &sync.Mutex{}
		m.locks[runID] = lock
	}
	return lock
}

func (m *Manager) publish(run state.PipelineRunRecord) {
	if m.publisher == nil {
		return
	}
	m.publisher.PublishPipelineUpdate(PipelineUpdate{RunID: run.RunID, DisplayName: run.DisplayName, Revision: run.Revision, State: run.State, CurrentStageID: run.CurrentStageID, CurrentAgentID: run.CurrentAgentID, AttentionReason: run.AttentionReason, FinalOutcome: run.FinalOutcome})
}

func stageByID(template Template, stageID string) (Stage, bool) {
	for _, stage := range template.Stages {
		if stage.ID == stageID {
			return stage, true
		}
	}
	return Stage{}, false
}
