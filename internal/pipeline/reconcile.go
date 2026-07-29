package pipeline

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/agentdeck/agentdeck/internal/state"
)

func (m *Manager) Reconcile(ctx context.Context, runID string) error {
	// Recovery can call Reconcile without a user control method. Claim the
	// project before a pending launch/resume transition is advanced so a project
	// archive cannot observe and stop an incomplete snapshot.
	if run, err := m.store.ReadPipelineRun(runID); err != nil {
		return err
	} else if startsProcess(run.PendingAction) {
		release, err := m.acquireProjectStart(ctx, run.Project)
		if err != nil {
			return err
		}
		defer release()
	}
	lock := m.runLock(runID)
	lock.Lock()
	defer lock.Unlock()
	for step := 0; step < maxReconcileSteps; step++ {
		run, err := m.store.ReadPipelineRun(runID)
		if err != nil {
			return err
		}
		switch run.PendingAction {
		case "launch_stage":
			claimed, err := m.store.UpdatePipelineRunCAS(runID, run.Revision, state.PipelineRunUpdate{
				State: "running", PendingAction: "launching_stage", CurrentStageID: run.CurrentStageID,
				CurrentAttemptID: run.CurrentAttemptID, CurrentAgentID: run.CurrentAgentID,
			})
			if err != nil {
				if errors.Is(err, state.ErrPipelineConflict) {
					continue
				}
				return err
			}
			m.publish(claimed)
		case "launching_stage":
			if err := m.reconcileLaunch(ctx, run); err != nil {
				return err
			}
			return nil
		case "stop_agent":
			claimed, err := m.store.UpdatePipelineRunCAS(runID, run.Revision, state.PipelineRunUpdate{
				State: run.State, PendingAction: "stopping_agent", CurrentStageID: run.CurrentStageID,
				CurrentAttemptID: run.CurrentAttemptID, CurrentAgentID: run.CurrentAgentID,
			})
			if err != nil {
				if errors.Is(err, state.ErrPipelineConflict) {
					continue
				}
				return err
			}
			m.publish(claimed)
		case "stopping_agent":
			if err := m.stopThenAdvance(ctx, run); err != nil {
				return err
			}
			continue
		case "resume_blocked":
			claimed, err := m.store.UpdatePipelineRunCAS(runID, run.Revision, state.PipelineRunUpdate{
				State: "running", PendingAction: "resuming_blocked", CurrentStageID: run.CurrentStageID,
				CurrentAttemptID: run.CurrentAttemptID, CurrentAgentID: run.CurrentAgentID,
			})
			if err != nil {
				if errors.Is(err, state.ErrPipelineConflict) {
					continue
				}
				return err
			}
			m.publish(claimed)
		case "resuming_blocked":
			if err := m.reconcileContinuation(ctx, run); err != nil {
				return err
			}
			return nil
		case "retry_stop_agent":
			claimed, err := m.store.UpdatePipelineRunCAS(runID, run.Revision, state.PipelineRunUpdate{
				State: "paused", PendingAction: "retry_stopping_agent", CurrentStageID: run.CurrentStageID,
				CurrentAttemptID: run.CurrentAttemptID, CurrentAgentID: run.CurrentAgentID,
			})
			if err != nil {
				if errors.Is(err, state.ErrPipelineConflict) {
					continue
				}
				return err
			}
			m.publish(claimed)
		case "retry_stopping_agent":
			if run.CurrentAgentID != "" && m.lifecycle != nil {
				if err := m.lifecycle.StopStage(ctx, run.CurrentAgentID); err != nil {
					return m.pauseLifecycle(run, "stop_failed", err)
				}
			}
			if _, err := m.createStageAttempt(run, run.CurrentStageID, true, false, ""); err != nil {
				return err
			}
		case "stop_run_agent":
			if run.CurrentAgentID != "" && m.lifecycle != nil {
				if err := m.lifecycle.StopStage(ctx, run.CurrentAgentID); err != nil {
					return err
				}
			}
			finished, err := m.store.UpdatePipelineRunCAS(runID, run.Revision, state.PipelineRunUpdate{
				State: "stopped", PendingAction: "", CurrentStageID: run.CurrentStageID,
				CurrentAttemptID: run.CurrentAttemptID, CurrentAgentID: "", FinalOutcome: "stopped",
			})
			if err != nil {
				if errors.Is(err, state.ErrPipelineConflict) {
					continue
				}
				return err
			}
			m.publish(finished)
			return nil
		default:
			return nil
		}
	}
	return fmt.Errorf("pipeline: reconcile step limit reached for %s", runID)
}

func startsProcess(action string) bool {
	switch action {
	case "launch_stage", "launching_stage", "resume_blocked", "resuming_blocked", "retry_stop_agent", "retry_stopping_agent":
		return true
	default:
		return false
	}
}

func (m *Manager) reconcileLaunch(ctx context.Context, run state.PipelineRunRecord) error {
	detail, err := m.Detail(run.RunID)
	if err != nil {
		return err
	}
	attempt, ok := currentAttempt(detail)
	if !ok {
		return m.pauseLifecycle(run, "attempt_missing", errors.New("current attempt is missing"))
	}
	stage, ok := stageByID(detail.Template, attempt.StageID)
	if !ok {
		return m.pauseLifecycle(run, "stage_missing", errors.New("snapshot stage is missing"))
	}
	execution := stageExecution(detail, attempt, stage)
	if m.lifecycle != nil && !m.lifecycle.IsRunning(attempt.AgentID) {
		if err := m.lifecycle.LaunchStage(ctx, execution); err != nil {
			return m.pauseAttempt(run, attempt, "launch_failed", err)
		}
	}
	updated, err := m.store.UpdatePipelineAttemptAndRunCAS(run.RunID, run.Revision, attempt.AttemptID, "running", attempt.AgentGeneration, state.PipelineRunUpdate{
		State: "running", PendingAction: "await_result", CurrentStageID: attempt.StageID,
		CurrentAttemptID: attempt.AttemptID, CurrentAgentID: attempt.AgentID,
	})
	if err != nil {
		return err
	}
	m.publish(updated)
	return nil
}

func (m *Manager) reconcileContinuation(ctx context.Context, run state.PipelineRunRecord) error {
	detail, err := m.Detail(run.RunID)
	if err != nil {
		return err
	}
	attempt, ok := currentAttempt(detail)
	if !ok {
		return m.pauseLifecycle(run, "attempt_missing", errors.New("current continuation attempt is missing"))
	}
	stage, ok := stageByID(detail.Template, attempt.StageID)
	if !ok {
		return m.pauseLifecycle(run, "stage_missing", errors.New("snapshot stage is missing"))
	}
	if m.lifecycle != nil {
		if err := m.lifecycle.ContinueStage(ctx, stageExecution(detail, attempt, stage)); err != nil {
			return m.pauseAttempt(run, attempt, "resume_failed", err)
		}
	}
	updated, err := m.store.UpdatePipelineAttemptAndRunCAS(run.RunID, run.Revision, attempt.AttemptID, "running", attempt.AgentGeneration, state.PipelineRunUpdate{
		State: "running", PendingAction: "await_result", CurrentStageID: attempt.StageID,
		CurrentAttemptID: attempt.AttemptID, CurrentAgentID: attempt.AgentID,
	})
	if err != nil {
		return err
	}
	m.publish(updated)
	return nil
}

func (m *Manager) stopThenAdvance(ctx context.Context, run state.PipelineRunRecord) error {
	if run.CurrentAgentID != "" && m.lifecycle != nil {
		if err := m.lifecycle.StopStage(ctx, run.CurrentAgentID); err != nil {
			return m.pauseLifecycle(run, "stop_failed", err)
		}
	}
	detail, err := m.Detail(run.RunID)
	if err != nil {
		return err
	}
	attempt, ok := currentAttempt(detail)
	if !ok || attempt.ReportOutcome == "" {
		return m.pauseLifecycle(run, "result_missing", errors.New("accepted result is missing"))
	}
	stage, ok := stageByID(detail.Template, attempt.StageID)
	if !ok {
		return m.pauseLifecycle(run, "stage_missing", errors.New("snapshot stage is missing"))
	}
	transition := stage.Transitions.Success
	if attempt.ReportOutcome == "failure" {
		transition = stage.Transitions.Failure
	}
	if transition.Approval == "required" {
		paused, err := m.store.UpdatePipelineRunCAS(run.RunID, run.Revision, state.PipelineRunUpdate{
			State: "paused", PendingAction: "await_approval", CurrentStageID: run.CurrentStageID,
			CurrentAttemptID: run.CurrentAttemptID, CurrentAgentID: "", AttentionReason: "approval_required",
		})
		if err != nil {
			return err
		}
		m.publish(paused)
		m.notify(paused, "needs_attention")
		return nil
	}
	if transition.Final != "" {
		completed, err := m.store.UpdatePipelineRunCAS(run.RunID, run.Revision, state.PipelineRunUpdate{
			State: "completed", PendingAction: "", CurrentStageID: run.CurrentStageID,
			CurrentAttemptID: run.CurrentAttemptID, CurrentAgentID: "", FinalOutcome: transition.Final,
		})
		if err != nil {
			return err
		}
		m.publish(completed)
		m.notify(completed, "completed")
		return nil
	}
	_, err = m.createStageAttempt(run, transition.Stage, false, true, "")
	return err
}

func (m *Manager) createStageAttempt(run state.PipelineRunRecord, stageID string, retry, newVisit bool, continuation string) (state.PipelineRunRecord, error) {
	detail, err := m.Detail(run.RunID)
	if err != nil {
		return state.PipelineRunRecord{}, err
	}
	stage, ok := stageByID(detail.Template, stageID)
	if !ok {
		return state.PipelineRunRecord{}, controlError("invalid_state", "destination stage is missing from the run snapshot")
	}
	attemptNo := 1
	visitNo := 1
	var parent string
	var sameAgent string
	var generation string
	for _, previous := range detail.Attempts {
		if previous.AttemptNo >= attemptNo {
			attemptNo = previous.AttemptNo + 1
		}
		if previous.StageID == stageID && previous.VisitNo >= visitNo {
			visitNo = previous.VisitNo
		}
		if previous.AttemptID == run.CurrentAttemptID {
			parent = previous.AttemptID
			if continuation != "" {
				sameAgent = previous.AgentID
				generation = previous.AgentGeneration
			}
		}
	}
	if newVisit {
		seen := false
		for _, previous := range detail.Attempts {
			if previous.StageID == stageID {
				seen = true
				break
			}
		}
		if seen {
			visitNo++
		}
	}
	limit := stage.MaxVisits
	if limit <= 0 {
		limit = 1
	}
	if visitNo > limit {
		paused, err := m.store.UpdatePipelineRunCAS(run.RunID, run.Revision, state.PipelineRunUpdate{
			State: "paused", PendingAction: "", CurrentStageID: stageID,
			CurrentAttemptID: run.CurrentAttemptID, CurrentAgentID: "", AttentionReason: "loop_limit_reached",
		})
		if err == nil {
			m.publish(paused)
			m.notify(paused, "needs_attention")
		}
		return paused, err
	}
	attemptID, err := m.store.NewPipelineAttemptID()
	if err != nil {
		return state.PipelineRunRecord{}, err
	}
	agentID := sameAgent
	if agentID == "" || retry {
		agentID, err = m.store.NewAgentID()
		if err != nil {
			return state.PipelineRunRecord{}, err
		}
		generation = attemptID
	} else if m.lifecycle != nil && !m.lifecycle.IsRunning(agentID) {
		generation = attemptID
	}
	assignment := detail.Assignments[stageID]
	shadowRun := run
	shadowRun.CurrentStageID = stageID
	text, hash := renderAssignment(shadowRun, detail.Template, stage, detail.Values, detail.Attempts, continuation)
	now := time.Now().UTC()
	attempt := state.PipelineAttemptRecord{
		AttemptID: attemptID, RunID: run.RunID, StageID: stageID, AttemptNo: attemptNo, VisitNo: visitNo,
		ParentAttemptID: parent, AgentID: agentID, AgentGeneration: generation,
		Backend: assignment.Backend, Model: assignment.Model, State: "queued", AssignmentText: text,
		AssignmentHash: hash, AssignmentVersion: assignmentVersion, CreatedAt: now, UpdatedAt: now,
	}
	pending := "launch_stage"
	if continuation != "" {
		pending = "resume_blocked"
	}
	updated, err := m.store.CreatePipelineAttemptCAS(run.RunID, run.Revision, attempt, state.PipelineRunUpdate{
		State: "queued", PendingAction: pending, CurrentStageID: stageID,
		CurrentAttemptID: attemptID, CurrentAgentID: agentID, UpdatedAt: now,
	})
	if err != nil {
		return state.PipelineRunRecord{}, err
	}
	m.publish(updated)
	return updated, nil
}

func (m *Manager) pauseAttempt(run state.PipelineRunRecord, attempt state.PipelineAttemptRecord, reason string, cause error) error {
	updated, err := m.store.UpdatePipelineAttemptAndRunCAS(run.RunID, run.Revision, attempt.AttemptID, reason, attempt.AgentGeneration, state.PipelineRunUpdate{
		State: "paused", PendingAction: "", CurrentStageID: run.CurrentStageID,
		CurrentAttemptID: run.CurrentAttemptID, CurrentAgentID: run.CurrentAgentID, AttentionReason: reason,
	})
	if err != nil {
		return err
	}
	m.publish(updated)
	m.notify(updated, "needs_attention")
	return nil
}

func (m *Manager) pauseLifecycle(run state.PipelineRunRecord, reason string, cause error) error {
	updated, err := m.store.UpdatePipelineRunCAS(run.RunID, run.Revision, state.PipelineRunUpdate{
		State: "paused", PendingAction: "", CurrentStageID: run.CurrentStageID,
		CurrentAttemptID: run.CurrentAttemptID, CurrentAgentID: run.CurrentAgentID, AttentionReason: reason,
	})
	if err != nil {
		return err
	}
	m.publish(updated)
	m.notify(updated, "needs_attention")
	return nil
}

func (m *Manager) notify(run state.PipelineRunRecord, kind string) {
	if m.publisher != nil {
		m.publisher.PublishPipelineNotification(PipelineUpdate{RunID: run.RunID, DisplayName: run.DisplayName, Revision: run.Revision, State: run.State, CurrentStageID: run.CurrentStageID, CurrentAgentID: run.CurrentAgentID, AttentionReason: run.AttentionReason, FinalOutcome: run.FinalOutcome}, kind)
	}
}

func currentAttempt(detail RunDetail) (state.PipelineAttemptRecord, bool) {
	for _, attempt := range detail.Attempts {
		if attempt.AttemptID == detail.Run.CurrentAttemptID {
			return attempt, true
		}
	}
	return state.PipelineAttemptRecord{}, false
}

func stageExecution(detail RunDetail, attempt state.PipelineAttemptRecord, stage Stage) StageExecution {
	return StageExecution{RunID: detail.Run.RunID, RunName: detail.Run.DisplayName, AttemptID: attempt.AttemptID,
		StageID: stage.ID, StageTitle: stage.Title, Role: stage.Role, Project: detail.Run.Project,
		Backend: attempt.Backend, Model: attempt.Model, AgentID: attempt.AgentID,
		Generation: attempt.AgentGeneration, AgentName: stage.Title + " — " + detail.Run.DisplayName,
		Assignment: attempt.AssignmentText}
}
