package pipeline

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/agentdeck/agentdeck/internal/state"
)

func (m *Manager) Continue(ctx context.Context, runID string, expectedRevision int64, input string) (RunDetail, error) {
	lock := m.runLock(runID)
	lock.Lock()
	run, err := m.store.ReadPipelineRun(runID)
	if err != nil {
		lock.Unlock()
		return RunDetail{}, err
	}
	if run.Revision != expectedRevision {
		lock.Unlock()
		return RunDetail{}, controlError("revision_conflict", "run changed; refresh before continuing")
	}
	release, err := m.acquireProjectStart(ctx, run.Project)
	if err != nil {
		lock.Unlock()
		return RunDetail{}, err
	}
	defer release()
	detail, err := m.Detail(runID)
	if err != nil {
		lock.Unlock()
		return RunDetail{}, err
	}
	attempt, ok := currentAttempt(detail)
	if !ok {
		lock.Unlock()
		return RunDetail{}, controlError("invalid_state", "current attempt is missing")
	}
	switch {
	case run.State == "paused" && run.AttentionReason == "blocked" && attempt.ReportOutcome == "blocked":
		if strings.TrimSpace(input) == "" || utf8.RuneCountInString(input) > MaxValueRunes {
			lock.Unlock()
			return RunDetail{}, validationError("continuation input is required", []Diagnostic{{Field: "input", Code: "invalid", Message: "input is required and must fit the pipeline value limit"}})
		}
		_, err = m.createStageAttempt(run, attempt.StageID, false, false, input)
	case run.State == "paused" && run.PendingAction == "await_approval":
		stage, found := stageByID(detail.Template, attempt.StageID)
		if !found {
			err = controlError("invalid_state", "current stage is missing")
			break
		}
		transition := stage.Transitions.Success
		if attempt.ReportOutcome == "failure" {
			transition = stage.Transitions.Failure
		}
		if transition.Final != "" {
			var completed state.PipelineRunRecord
			completed, err = m.store.UpdatePipelineRunCAS(runID, run.Revision, state.PipelineRunUpdate{
				State: "completed", PendingAction: "", CurrentStageID: run.CurrentStageID,
				CurrentAttemptID: run.CurrentAttemptID, CurrentAgentID: "", FinalOutcome: transition.Final,
			})
			if err == nil {
				m.publish(completed)
				m.notify(completed, "completed")
			}
		} else {
			_, err = m.createStageAttempt(run, transition.Stage, false, true, "")
		}
	default:
		err = controlError("invalid_state", "continue is not valid for the current run state")
	}
	lock.Unlock()
	if err != nil {
		return RunDetail{}, err
	}
	if err := m.Reconcile(ctx, runID); err != nil {
		return RunDetail{}, err
	}
	return m.Detail(runID)
}

func (m *Manager) Retry(ctx context.Context, runID string, expectedRevision int64) (RunDetail, error) {
	lock := m.runLock(runID)
	lock.Lock()
	run, err := m.store.ReadPipelineRun(runID)
	if err != nil {
		lock.Unlock()
		return RunDetail{}, err
	}
	if run.Revision != expectedRevision {
		lock.Unlock()
		return RunDetail{}, controlError("revision_conflict", "run changed; refresh before retrying")
	}
	release, err := m.acquireProjectStart(ctx, run.Project)
	if err != nil {
		lock.Unlock()
		return RunDetail{}, err
	}
	defer release()
	if run.State != "paused" || run.PendingAction == "await_approval" || run.AttentionReason == "loop_limit_reached" {
		lock.Unlock()
		return RunDetail{}, controlError("invalid_state", "retry is not valid for the current run state")
	}
	updated, err := m.store.UpdatePipelineRunCAS(runID, run.Revision, state.PipelineRunUpdate{
		State: "paused", PendingAction: "retry_stop_agent", CurrentStageID: run.CurrentStageID,
		CurrentAttemptID: run.CurrentAttemptID, CurrentAgentID: run.CurrentAgentID,
	})
	if err == nil {
		m.publish(updated)
	}
	lock.Unlock()
	if err != nil {
		return RunDetail{}, err
	}
	if err := m.Reconcile(ctx, runID); err != nil {
		return RunDetail{}, err
	}
	return m.Detail(runID)
}

func (m *Manager) Stop(ctx context.Context, runID string, expectedRevision int64) (RunDetail, error) {
	lock := m.runLock(runID)
	lock.Lock()
	run, err := m.store.ReadPipelineRun(runID)
	if err != nil {
		lock.Unlock()
		return RunDetail{}, err
	}
	if run.Revision != expectedRevision {
		lock.Unlock()
		return RunDetail{}, controlError("revision_conflict", "run changed; refresh before stopping")
	}
	if run.State == "completed" || run.State == "stopped" {
		lock.Unlock()
		return m.Detail(runID)
	}
	updated, err := m.store.UpdatePipelineRunCAS(runID, run.Revision, state.PipelineRunUpdate{
		State: "stopped", PendingAction: "stop_run_agent", CurrentStageID: run.CurrentStageID,
		CurrentAttemptID: run.CurrentAttemptID, CurrentAgentID: run.CurrentAgentID, FinalOutcome: "stopped",
	})
	if err == nil {
		m.publish(updated)
	}
	lock.Unlock()
	if err != nil {
		return RunDetail{}, err
	}
	if err := m.Reconcile(ctx, runID); err != nil {
		return RunDetail{}, err
	}
	return m.Detail(runID)
}

// StopProject uses the ordinary durable stop path for every non-terminal run
// owned by a project. Project archival calls it before archiving stage agents.
func (m *Manager) StopProject(ctx context.Context, project string) error {
	runs, err := m.store.ListActivePipelineRuns()
	if err != nil {
		return err
	}
	for _, run := range runs {
		if run.Project != project {
			continue
		}
		if _, err := m.Stop(ctx, run.RunID, run.Revision); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) Delete(runID string) error {
	return m.store.DeletePipelineRun(runID)
}

func (m *Manager) Report(agentID, generation string, report StageReport) (RunDetail, error) {
	run, attempt, err := m.store.CurrentPipelineAttemptForAgent(agentID)
	if err != nil {
		if errors.Is(err, state.ErrNotFound) {
			return refuseReport(run, attempt, agentID, generation, controlError("assignment_unknown", "caller has no current pipeline assignment"))
		}
		return RunDetail{}, err
	}
	lock := m.runLock(run.RunID)
	lock.Lock()
	defer lock.Unlock()
	run, err = m.store.ReadPipelineRun(run.RunID)
	if err != nil {
		return RunDetail{}, err
	}
	attempt, err = m.store.ReadPipelineAttempt(run.CurrentAttemptID)
	if err != nil {
		return RunDetail{}, err
	}
	if !state.OwnsReportedWork(agentID, generation, attempt.AgentID, attempt.AgentGeneration) {
		return refuseReport(run, attempt, agentID, generation, controlError("stale_assignment", "caller is not the current stage attempt"))
	}
	if run.PendingAction != "await_result" {
		if attempt.ReportOutcome != "" {
			return refuseReport(run, attempt, agentID, generation, controlError("already_reported", "this attempt already reported a result and its participation has ended; work done since then cannot be recorded against the run. Wait for the run's human Continue action, which arrives as a new assignment"))
		}
		return refuseReport(run, attempt, agentID, generation, controlError("stale_assignment", "the current stage attempt is no longer awaiting a result"))
	}
	// The vocabulary and the field bounds are the shared work-result rules, so a
	// stage report and a task report can never drift apart (TS-10.R7).
	switch err := state.ValidateAgentReport(report.Outcome, report.Summary, report.Details, report.Checks); {
	case errors.Is(err, state.ErrInvalidOutcome):
		return refuseReport(run, attempt, agentID, generation, validationError("invalid stage result", []Diagnostic{{Field: "outcome", Code: "invalid", Message: "outcome must be success, failure, or blocked"}}))
	case err != nil:
		return refuseReport(run, attempt, agentID, generation, validationError("invalid stage result", []Diagnostic{{Field: "summary", Code: "invalid", Message: "summary is required and report fields must fit their documented limits"}}))
	}
	if report.Outputs == nil {
		report.Outputs = map[string]string{}
	}
	detail, err := m.Detail(run.RunID)
	if err != nil {
		return RunDetail{}, err
	}
	stage, ok := stageByID(detail.Template, attempt.StageID)
	if !ok {
		return refuseReport(run, attempt, agentID, generation, controlError("invalid_state", "current stage is missing"))
	}
	declared := map[string]StageOutput{}
	for _, output := range stage.Outputs {
		declared[output.Name] = output
	}
	outputValues := []state.PipelineValueRecord{}
	available := map[string]string{}
	for _, value := range detail.Values {
		available[value.Name] = value.Value
	}
	for localName, value := range report.Outputs {
		output, exists := declared[localName]
		if !exists {
			return refuseReport(run, attempt, agentID, generation, validationError("invalid stage result", []Diagnostic{{Field: "outputs." + localName, Code: "undeclared", Message: "output is not declared by the current stage"}}))
		}
		if utf8.RuneCountInString(value) > MaxValueRunes {
			return refuseReport(run, attempt, agentID, generation, validationError("invalid stage result", []Diagnostic{{Field: "outputs." + localName, Code: "too_long", Message: "output exceeds the pipeline value limit"}}))
		}
		available[output.Value] = value
		outputValues = append(outputValues, state.PipelineValueRecord{RunID: run.RunID, Name: output.Value, Value: value, SourceAttemptID: attempt.AttemptID})
	}
	if report.Outcome != "blocked" {
		transition := stage.Transitions.Success
		if report.Outcome == "failure" {
			transition = stage.Transitions.Failure
		}
		if transition.Stage != "" {
			destination, found := stageByID(detail.Template, transition.Stage)
			if !found {
				return refuseReport(run, attempt, agentID, generation, controlError("invalid_state", "transition destination is missing"))
			}
			missing := []Diagnostic{}
			for _, input := range destination.Inputs {
				if input.Required && strings.TrimSpace(available[input.Value]) == "" {
					missing = append(missing, Diagnostic{Field: "outputs." + input.Value, Code: "missing_destination_value", Message: "destination requires a non-empty value named " + input.Value})
				}
			}
			if len(missing) > 0 {
				return refuseReport(run, attempt, agentID, generation, validationError("destination inputs are unresolved", missing))
			}
		}
	}
	updated, _, err := m.store.AcceptPipelineReport(state.PipelineReportInput{
		RunID: run.RunID, AttemptID: attempt.AttemptID, AgentID: agentID, AgentGeneration: generation,
		ExpectedRevision: run.Revision, Outcome: report.Outcome, Summary: report.Summary,
		Details: report.Details, Checks: report.Checks, Outputs: outputValues, ReportedAt: time.Now().UTC(),
	})
	if err != nil {
		if errors.Is(err, state.ErrPipelineConflict) {
			return refuseReport(run, attempt, agentID, generation, controlError("stale_assignment", "stage result was already accepted or the run changed"))
		}
		return RunDetail{}, err
	}
	m.publish(updated)
	return m.Detail(run.RunID)
}

// refuseReport logs every refused stage result at Warn before returning it. The
// server log is otherwise silent about control-plane refusals, so a field report
// of a refused report could only be corroborated from the agent's own
// transcript. The fields carried here are exactly what separates the refusal
// conditions from one another (stale_assignment vs already_reported vs the rest).
func refuseReport(run state.PipelineRunRecord, attempt state.PipelineAttemptRecord, agentID, generation string, refusal *ControlError) (RunDetail, error) {
	slog.Warn("pipeline: stage result refused",
		"code", refusal.Code, "run", run.RunID, "attempt", attempt.AttemptID,
		"caller_agent", agentID, "caller_generation", generation,
		"attempt_agent", attempt.AgentID, "attempt_generation", attempt.AgentGeneration,
		"pending_action", run.PendingAction)
	return RunDetail{}, refusal
}

func (m *Manager) OnTurnEnd(agentID, generation string) error {
	run, attempt, err := m.store.CurrentPipelineAttemptForAgent(agentID)
	if errors.Is(err, state.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if attempt.AgentGeneration != generation || attempt.ReportOutcome == "" || run.PendingAction != "await_quiescence" {
		return nil
	}
	lock := m.runLock(run.RunID)
	lock.Lock()
	// The pre-lock read only filters; the quiescence claim must use the state
	// this critical section observed, exactly as Report re-reads it. A revision
	// bump inside that window otherwise fails the CAS and parks the run at
	// await_quiescence with no further turn boundary coming (INV §5).
	run, attempt, err = m.currentUnderLock(run.RunID, agentID, generation)
	if err != nil {
		lock.Unlock()
		return err
	}
	if attempt.AttemptID == "" || attempt.ReportOutcome == "" || run.PendingAction != "await_quiescence" {
		lock.Unlock()
		slog.Warn("pipeline: turn end skipped; run changed before the quiescence claim",
			"run", run.RunID, "attempt", run.CurrentAttemptID, "caller_agent", agentID,
			"caller_generation", generation, "pending_action", run.PendingAction)
		return nil
	}
	updated, err := m.store.MarkPipelineQuiescent(run.RunID, attempt.AttemptID, agentID, generation, run.Revision, time.Now().UTC())
	if err == nil {
		m.publish(updated)
		if updated.AttentionReason == "blocked" {
			m.notify(updated, "needs_attention")
		}
	}
	lock.Unlock()
	if err != nil {
		if errors.Is(err, state.ErrPipelineConflict) {
			slog.Warn("pipeline: quiescence claim conflicted",
				"run", run.RunID, "attempt", attempt.AttemptID, "caller_agent", agentID,
				"caller_generation", generation, "revision", run.Revision)
			return nil
		}
		return err
	}
	return m.Reconcile(context.Background(), run.RunID)
}

// currentUnderLock re-reads the run and its current attempt while the run lock
// is held, and reports whether that attempt is still the caller's. Callers that
// resolved an attempt before taking the lock must refresh through it so a CAS
// never carries a revision from outside the critical section (INV §5).
func (m *Manager) currentUnderLock(runID, agentID, generation string) (state.PipelineRunRecord, state.PipelineAttemptRecord, error) {
	run, err := m.store.ReadPipelineRun(runID)
	if err != nil {
		return state.PipelineRunRecord{}, state.PipelineAttemptRecord{}, err
	}
	attempt, err := m.store.ReadPipelineAttempt(run.CurrentAttemptID)
	if errors.Is(err, state.ErrNotFound) {
		return run, state.PipelineAttemptRecord{}, nil
	}
	if err != nil {
		return state.PipelineRunRecord{}, state.PipelineAttemptRecord{}, err
	}
	if attempt.AgentID != agentID || attempt.AgentGeneration != generation {
		return run, state.PipelineAttemptRecord{}, nil
	}
	return run, attempt, nil
}

func (m *Manager) OnExit(agentID, generation, cause string) error {
	run, attempt, err := m.store.CurrentPipelineAttemptForAgent(agentID)
	if errors.Is(err, state.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if attempt.AgentGeneration != generation {
		return nil
	}
	if stoppingAgent(run) {
		return m.Reconcile(context.Background(), run.RunID)
	}
	lock := m.runLock(run.RunID)
	lock.Lock()
	// Same INV §5 refresh as OnTurnEnd: the crash pause CAS must not carry the
	// revision read before the lock, or a run whose revision moved inside the
	// window stays running with no attention reason and no notification.
	run, attempt, err = m.currentUnderLock(run.RunID, agentID, generation)
	if err != nil {
		lock.Unlock()
		return err
	}
	if attempt.AttemptID == "" {
		lock.Unlock()
		slog.Warn("pipeline: exit skipped; run changed before the crash pause",
			"run", run.RunID, "attempt", run.CurrentAttemptID, "caller_agent", agentID,
			"caller_generation", generation, "pending_action", run.PendingAction)
		return nil
	}
	if stoppingAgent(run) {
		lock.Unlock()
		return m.Reconcile(context.Background(), run.RunID)
	}
	attemptState := "crashed"
	attentionReason := "agent_crash"
	if cause == "requested_stop" {
		attemptState = "stopped"
		attentionReason = "agent_stopped"
	}
	updated, err := m.store.UpdatePipelineAttemptAndRunCAS(run.RunID, run.Revision, attempt.AttemptID, attemptState, generation, state.PipelineRunUpdate{
		State: "paused", PendingAction: "", CurrentStageID: run.CurrentStageID,
		CurrentAttemptID: run.CurrentAttemptID, CurrentAgentID: run.CurrentAgentID, AttentionReason: attentionReason,
	})
	if err == nil {
		m.publish(updated)
		m.notify(updated, "needs_attention")
	}
	lock.Unlock()
	if errors.Is(err, state.ErrPipelineConflict) {
		slog.Warn("pipeline: crash pause conflicted",
			"run", run.RunID, "attempt", attempt.AttemptID, "caller_agent", agentID,
			"caller_generation", generation, "revision", run.Revision)
		return nil
	}
	return err
}

// stoppingAgent reports the pending actions that already own stopping the run's
// agent, so an exit callback reconciles instead of raising a crash pause.
func stoppingAgent(run state.PipelineRunRecord) bool {
	return run.State == "stopped" || run.PendingAction == "stopping_agent" ||
		run.PendingAction == "stop_run_agent" || run.PendingAction == "retry_stopping_agent"
}
