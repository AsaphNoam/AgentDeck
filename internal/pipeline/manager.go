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

	locksMu sync.Mutex
	locks   map[string]*sync.Mutex
}

func NewManager(store *state.Store, templates *TemplateStore, lifecycle Lifecycle, publisher Publisher) *Manager {
	return &Manager{store: store, templates: templates, lifecycle: lifecycle, publisher: publisher, locks: map[string]*sync.Mutex{}}
}

func (m *Manager) Start(ctx context.Context, request StartRequest) (RunDetail, bool, error) {
	requestHash, err := startRequestHash(request)
	if err != nil {
		return RunDetail{}, false, err
	}
	if detail, found, err := m.LookupStart(request); err != nil || found {
		return detail, found, err
	}
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
		AgentID: agentID, AgentGeneration: attemptID, Backend: assignment.Backend, Model: assignment.Model,
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
		if !ok || !config.ValidSlug(assignment.Backend) || !config.ValidSlug(assignment.Model) {
			add("assignments."+stage.ID, "required", "every stage requires a configured backend and model")
			continue
		}
		if m.lifecycle != nil {
			if err := m.lifecycle.ValidateStage(ctx, StageExecution{StageID: stage.ID, StageTitle: stage.Title, Role: stage.Role, Project: request.Project, Backend: assignment.Backend, Model: assignment.Model}); err != nil {
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
	return detail, err
}

func (m *Manager) List(limit, offset int) ([]RunSummary, error) {
	if limit <= 0 || limit > MaxListPage {
		limit = MaxListPage
	}
	runs, err := m.store.ListPipelineRuns(limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]RunSummary, 0, len(runs))
	for _, run := range runs {
		diagnostics := []Diagnostic{}
		if _, detailErr := m.Detail(run.RunID); detailErr != nil {
			diagnostics = appendBounded(diagnostics, Diagnostic{Field: "", Code: "run_read_failed", Message: "run detail could not be decoded"})
		}
		out = append(out, RunSummary{
			RunID: run.RunID, TemplateID: run.TemplateID, DisplayName: run.DisplayName,
			Project: run.Project, State: run.State, Revision: run.Revision,
			PendingAction: run.PendingAction, CurrentStageID: run.CurrentStageID,
			CurrentAgentID: run.CurrentAgentID, AttentionReason: run.AttentionReason,
			FinalOutcome: run.FinalOutcome, UpdatedAt: run.UpdatedAt.Format(time.RFC3339Nano),
			Diagnostics: diagnostics,
		})
	}
	return out, nil
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
	m.publisher.PublishPipelineUpdate(PipelineUpdate{RunID: run.RunID, Revision: run.Revision, State: run.State, CurrentStageID: run.CurrentStageID, CurrentAgentID: run.CurrentAgentID, AttentionReason: run.AttentionReason})
}

func stageByID(template Template, stageID string) (Stage, bool) {
	for _, stage := range template.Stages {
		if stage.ID == stageID {
			return stage, true
		}
	}
	return Stage{}, false
}
