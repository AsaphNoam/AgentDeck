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
	unlock := m.lockRun(runID)
	defer unlock()
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
			current, found, err := m.currentStageTask(run.RunID)
			if err != nil || !found {
				return err
			}
			updated, err := m.store.ActivatePipelineStageReplacement(run.RunID, current.TaskID, run.Revision)
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

// settleCleanup is the one stage/run cleanup completion contract. It keyset-pages
// the members in scope, cancels the unfinished ones through the shared task
// cancellation helper, and reports whether every release and yield intent has
// settled. It never drops a claim early: an effect that has not settled keeps
// the run in `finishing`/`stopping` until the dispatcher's cleanup pass finishes
// it and re-drives this cursor (TS-09.R40/R42/R43, INV §5/§15).
//
// An empty stageID scopes it to the whole run, which is what Stop cleans up. A
// stage id and attempt scope it to that stage attempt's own members, so stage
// completion converges on exactly the work that stage started.
func (m *Manager) settleCleanup(runID, stageID, attempt string) (bool, error) {
	settled := true
	after := ""
	for {
		tasks, err := m.store.ListPipelineCleanupMembers(runID, stageID, attempt, after, cleanupPageSize)
		if err != nil {
			return false, err
		}
		for _, task := range tasks {
			after = task.TaskID
			if task.State != state.TaskFinished {
				cancelled, cancelErr := m.store.CancelTask(task.TaskID)
				if cancelErr != nil && !errors.Is(cancelErr, state.ErrTaskNotReportable) {
					return false, cancelErr
				}
				if cancelErr == nil {
					task = cancelled
				}
			}
			if task.State != state.TaskFinished || task.PendingRelease || task.PendingYield {
				settled = false
			}
		}
		if len(tasks) < cleanupPageSize {
			return settled, nil
		}
	}
}

func (m *Manager) reconcileRunCleanup(run state.PipelineRunRecord) error {
	settled, err := m.settleCleanup(run.RunID, "", "")
	if err != nil {
		return err
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
	current, found, err := m.currentStageTask(run.RunID)
	if err != nil || !found {
		return err
	}
	task, err := m.store.ReadTask(current.TaskID)
	if err != nil {
		return err
	}
	// Waiting for the owner's own release alone advanced the run with the stage's
	// unfinished descendants still alive. The whole stage attempt must converge
	// before any next-stage or final write (TS-09.R42).
	settled, err := m.settleCleanup(run.RunID, current.StageID, fmt.Sprintf("%d", current.AttemptNumber))
	if err != nil {
		return err
	}
	if !settled {
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
