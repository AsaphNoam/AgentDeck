package pipeline

import (
	"context"
	"errors"
	"fmt"

	"github.com/agentdeck/agentdeck/internal/state"
)

// Reconcile advances only durable task-backed pipeline effects. Process launch,
// resume, report, and release belong to the shared task dispatcher.
func (m *Manager) Reconcile(_ context.Context, runID string) error {
	lock := m.runLock(runID)
	lock.Lock()
	defer lock.Unlock()
	for step := 0; step < maxReconcileSteps; step++ {
		run, err := m.store.ReadPipelineRun(runID)
		if err != nil {
			return err
		}
		switch run.PendingAction {
		case "release_stage_task":
			return m.reconcileTaskStageRelease(run)
		case "cleanup_run":
			return m.reconcileRunCleanup(run)
		case "activate_replacement":
			stages, err := m.store.ListPipelineStageTasks(run.RunID)
			if err != nil || len(stages) == 0 {
				return err
			}
			updated, err := m.store.ActivatePipelineStageReplacement(run.RunID, stages[len(stages)-1].TaskID, run.Revision)
			if err == nil {
				m.publish(updated)
			}
			return err
		default:
			return nil
		}
	}
	return fmt.Errorf("pipeline: reconcile step limit reached for %s", runID)
}

func (m *Manager) reconcileRunCleanup(run state.PipelineRunRecord) error {
	tasks, err := m.store.ListTasksForPipelineRun(run.RunID)
	if err != nil {
		return err
	}
	settled := true
	for _, task := range tasks {
		if task.State != state.TaskFinished {
			cancelled, cancelErr := m.store.CancelTask(task.TaskID)
			if cancelErr != nil && !errors.Is(cancelErr, state.ErrTaskNotReportable) {
				return cancelErr
			}
			if cancelErr == nil {
				task = cancelled
			}
		}
		if task.PendingRelease || task.PendingYield {
			settled = false
		}
	}
	if !settled {
		return nil
	}
	stopped, err := m.store.UpdatePipelineRunCAS(run.RunID, run.Revision, state.PipelineRunUpdate{
		State: "stopped", PendingAction: "", CurrentStageID: run.CurrentStageID,
		CurrentAgentID: "", FinalOutcome: state.OutcomeCancelled,
	})
	if err == nil {
		m.publish(stopped)
		m.notify(stopped, "completed")
	}
	return err
}

func (m *Manager) reconcileTaskStageRelease(run state.PipelineRunRecord) error {
	stages, err := m.store.ListPipelineStageTasks(run.RunID)
	if err != nil {
		return err
	}
	if len(stages) == 0 {
		return nil
	}
	current := stages[len(stages)-1]
	task, err := m.store.ReadTask(current.TaskID)
	if err != nil {
		return err
	}
	if task.PendingRelease {
		return nil
	}
	detail, err := m.Detail(run.RunID)
	if err != nil {
		return err
	}
	stage, found := stageByID(detail.Template, current.StageID)
	if !found {
		return controlError("invalid_state", "current stage is missing")
	}
	if task.Outcome != state.OutcomeSuccess {
		paused, err := m.store.UpdatePipelineRunCAS(run.RunID, run.Revision, state.PipelineRunUpdate{State: "paused", PendingAction: "", CurrentStageID: stage.ID, AttentionReason: task.Outcome})
		if err == nil {
			m.publish(paused)
			m.notify(paused, "needs_attention")
		}
		return err
	}
	if stage.ApprovalAfterSuccess {
		paused, err := m.store.UpdatePipelineRunCAS(run.RunID, run.Revision, state.PipelineRunUpdate{State: "paused", PendingAction: "await_approval", CurrentStageID: stage.ID, AttentionReason: "approval_required"})
		if err == nil {
			m.publish(paused)
			m.notify(paused, "needs_attention")
		}
		return err
	}
	return m.advanceTaskStage(run, detail, current, stage)
}

func (m *Manager) notify(run state.PipelineRunRecord, kind string) {
	if m.publisher != nil {
		m.publisher.PublishPipelineNotification(PipelineUpdate{RunID: run.RunID, DisplayName: run.DisplayName, Revision: run.Revision, State: run.State, CurrentStageID: run.CurrentStageID, CurrentAgentID: run.CurrentAgentID, AttentionReason: run.AttentionReason, FinalOutcome: run.FinalOutcome}, kind)
	}
}
