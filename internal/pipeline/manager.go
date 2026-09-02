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
	pendingPermissions map[string]pendingPermission
}

type pendingPermission struct {
	agentID, generation, toolCallID string
}

func NewManager(store *state.Store, templates *TemplateStore, lifecycle Lifecycle, publisher Publisher) *Manager {
	return &Manager{store: store, templates: templates, lifecycle: lifecycle, publisher: publisher, locks: map[string]*sync.Mutex{}, pendingPermissions: map[string]pendingPermission{}}
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
	attemptID, err := m.store.NewPipelineAttemptID()
	if err != nil {
		return RunDetail{}, false, err
	}
	agentID, err := m.store.NewAgentID()
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
		PendingAction: "launch_stage", CurrentStageID: first.ID, CurrentAttemptID: attemptID,
		CurrentAgentID: agentID, CreatedAt: now, UpdatedAt: now,
	}
	values := make([]state.PipelineValueRecord, 0, len(request.Inputs))
	for name, value := range request.Inputs {
		values = append(values, state.PipelineValueRecord{RunID: runID, Name: name, Value: value, SourceKind: "run_input", UpdatedAt: now})
	}
	assignmentText, assignmentHash := renderAssignment(run, record.Template, first, values, nil, "")
	assignment := request.Assignments[first.ID]
	attempt := &state.PipelineAttemptRecord{
		AttemptID: attemptID, RunID: runID, StageID: first.ID, AttemptNo: 1, VisitNo: 1,
		AgentID: agentID, AgentGeneration: attemptID, Backend: assignment.Backend, Model: assignment.Model, Effort: assignment.Effort,
		State: "queued", AssignmentText: assignmentText, AssignmentHash: assignmentHash,
		AssignmentVersion: assignmentVersion, ReportOutputs: json.RawMessage(`{}`), CreatedAt: now, UpdatedAt: now,
	}
	created, replay, err := m.store.CreatePipelineRun(state.CreatePipelineRunParams{
		Run: run, RequestID: request.RequestID, RequestHash: requestHash, Values: values, InitialAttempt: attempt,
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
	if err := m.Reconcile(ctx, created.RunID); err != nil {
		return RunDetail{}, replay, err
	}
	detail, err := m.Detail(created.RunID)
	return detail, replay, err
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
	if request.Assignments == nil {
		request.Assignments = map[string]RuntimeAssignment{}
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
	stages := map[string]Stage{}
	for _, stage := range record.Template.Stages {
		stages[stage.ID] = stage
		assignment, ok := request.Assignments[stage.ID]
		if !ok || assignment.Backend == "" || assignment.Model == "" {
			add("assignments."+stage.ID, "required", "every stage requires a configured backend and model")
			continue
		}
		if m.lifecycle != nil {
			if err := m.lifecycle.ValidateStage(ctx, StageExecution{StageID: stage.ID, StageTitle: stage.Title, Role: stage.Role, Project: request.Project, Backend: assignment.Backend, Model: assignment.Model, Effort: assignment.Effort}); err != nil {
				add("assignments."+stage.ID, "unavailable", err.Error())
			}
		}
	}
	for stageID := range request.Assignments {
		if _, ok := stages[stageID]; !ok {
			add("assignments."+stageID, "unknown", "assignment does not match a template stage")
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
	if err == nil && m.hasPendingPermission(runID, run.CurrentAgentID) {
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
	pending, ok := m.pendingPermissions[runID]
	return ok && pending.agentID == agentID
}

// OnPermissionEvent derives pipeline attention from the current stage agent's
// process-lifetime permission state. It changes no durable run state.
func (m *Manager) OnPermissionEvent(agentID, generation, toolCallID string, pending bool) error {
	run, attempt, err := m.store.CurrentPipelineAttemptForAgent(agentID)
	if errors.Is(err, state.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if attempt.AgentGeneration != generation || run.CurrentAgentID != agentID || run.CurrentAttemptID != attempt.AttemptID || run.PendingAction != "await_result" {
		return nil
	}
	m.attentionMu.Lock()
	current, exists := m.pendingPermissions[run.RunID]
	changed := false
	if pending {
		if !exists || current.toolCallID != toolCallID || current.generation != generation {
			m.pendingPermissions[run.RunID] = pendingPermission{agentID: agentID, generation: generation, toolCallID: toolCallID}
			changed = true
		}
	} else if exists && current.agentID == agentID && current.generation == generation && current.toolCallID == toolCallID {
		delete(m.pendingPermissions, run.RunID)
		changed = true
	}
	m.attentionMu.Unlock()
	if !changed || m.publisher == nil {
		return nil
	}
	reason := ""
	if pending {
		reason = "awaiting permission approval"
	}
	update := PipelineUpdate{RunID: run.RunID, DisplayName: run.DisplayName, Revision: run.Revision, State: run.State, CurrentStageID: run.CurrentStageID, CurrentAgentID: run.CurrentAgentID, AttentionReason: reason, FinalOutcome: run.FinalOutcome}
	m.publisher.PublishPipelineUpdate(update)
	if pending {
		m.publisher.PublishPipelineNotification(update, "needs_attention")
	}
	return nil
}

func (m *Manager) ClearPermissionAttention(agentID, generation string) {
	m.attentionMu.Lock()
	for runID, pending := range m.pendingPermissions {
		if pending.agentID == agentID && pending.generation == generation {
			delete(m.pendingPermissions, runID)
		}
	}
	m.attentionMu.Unlock()
}

func (m *Manager) Startup(ctx context.Context) error {
	runs, err := m.store.ListActivePipelineRuns()
	if err != nil {
		return err
	}
	for _, run := range runs {
		if run.PendingAction == "" || run.PendingAction == "await_approval" {
			continue
		}
		if run.PendingAction == "await_result" || run.PendingAction == "await_quiescence" {
			detail, detailErr := m.Detail(run.RunID)
			if detailErr != nil {
				if pauseErr := m.pauseStartupRun(ctx, run.RunID, "restart_state_invalid"); pauseErr != nil {
					return pauseErr
				}
				continue
			}
			attempt, ok := currentAttempt(detail)
			if !ok {
				if pauseErr := m.pauseStartupRun(ctx, run.RunID, "restart_state_invalid"); pauseErr != nil {
					return pauseErr
				}
				continue
			}
			reason := "restart_recovery"
			if run.PendingAction == "await_quiescence" {
				reason = "restart_awaiting_quiescence"
			}
			updated, updateErr := m.store.UpdatePipelineAttemptAndRunCAS(run.RunID, run.Revision, attempt.AttemptID, reason, attempt.AgentGeneration, state.PipelineRunUpdate{
				State: "paused", PendingAction: "", CurrentStageID: run.CurrentStageID,
				CurrentAttemptID: run.CurrentAttemptID, CurrentAgentID: run.CurrentAgentID, AttentionReason: reason,
			})
			if updateErr != nil {
				return updateErr
			}
			m.publish(updated)
			m.notify(updated, "needs_attention")
			if m.lifecycle != nil && run.CurrentAgentID != "" {
				_ = m.lifecycle.StopStage(ctx, run.CurrentAgentID)
			}
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
