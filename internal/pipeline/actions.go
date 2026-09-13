package pipeline

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/agentdeck/agentdeck/internal/state"
)

func (m *Manager) Continue(ctx context.Context, runID string, expectedRevision int64, input string) (RunDetail, error) {
	unlock := m.lockRun(runID)
	run, err := m.store.ReadPipelineRun(runID)
	if err != nil {
		unlock()
		return RunDetail{}, err
	}
	if run.Revision != expectedRevision {
		unlock()
		return RunDetail{}, controlError("revision_conflict", "run changed; refresh before continuing")
	}
	if current, found, stageErr := m.currentStageTask(runID); stageErr == nil && found {
		detail, detailErr := m.Detail(runID)
		if detailErr != nil {
			unlock()
			return RunDetail{}, detailErr
		}
		task, taskErr := m.store.ReadTask(current.TaskID)
		if taskErr != nil {
			unlock()
			return RunDetail{}, taskErr
		}
		stage, found := stageByID(detail.Template, current.StageID)
		if !found {
			unlock()
			return RunDetail{}, controlError("invalid_state", "current stage is missing")
		}
		var continueErr error
		switch {
		case run.State == "paused" && run.PendingAction == "await_approval" && task.Outcome == state.OutcomeSuccess:
			continueErr = m.advanceTaskStage(run, detail, current, stage)
		case run.State == "paused" && run.PendingAction == "" && (task.Outcome == state.OutcomeFailure || task.Outcome == state.OutcomeBlocked):
			if strings.TrimSpace(input) == "" || utf8.RuneCountInString(input) > MaxValueRunes {
				continueErr = validationError("continuation input is required", []Diagnostic{{Field: "input", Code: "invalid", Message: "input is required and must fit the pipeline value limit"}})
				break
			}
			continueErr = m.continueTaskStage(run, detail, current, stage, input)
		default:
			continueErr = controlError("invalid_state", "continue is not valid for the current run state")
		}
		unlock()
		if continueErr != nil {
			return RunDetail{}, continueErr
		}
		return m.Detail(runID)
	} else if stageErr != nil {
		unlock()
		return RunDetail{}, stageErr
	}
	unlock()
	return RunDetail{}, controlError("invalid_state", "run has no durable stage task")
}

func (m *Manager) continueTaskStage(run state.PipelineRunRecord, detail RunDetail, current state.PipelineStageTask, stage Stage, input string) error {
	taskID, err := m.store.NewTaskID()
	if err != nil {
		return err
	}
	assignment := standingAssignment(detail.Assignments, stage.ID)
	coordinator, err := m.coordinatorTask(run.Project, run.Goal, taskID, stage, detail.Assignments[stage.ID])
	if err != nil {
		return err
	}
	prior, err := m.priorStageResults(run.RunID, "")
	if err != nil {
		return err
	}
	instruction, digest := renderAssignment(run, detail.Template, stage, detail.Values, assignmentContext{
		Coordinator: coordinatorContext(coordinator, stage), PriorResults: prior, Continuation: input,
	})
	targetKind := state.TargetLaunch
	if current.StandingAgentID != "" {
		targetKind = state.TargetAgent
	}
	_, _, _, err = m.store.CreatePipelineStageTask(state.CreatePipelineStageTaskParams{
		RunID: run.RunID, ExpectedRevision: run.Revision, StageIndex: current.StageIndex,
		AttemptNumber: current.AttemptNumber + 1, StageID: stage.ID, AssignmentDigest: digest,
		ParentTaskID: current.TaskID, OutputValues: stageOutputValues(stage), Coordinator: coordinator, Task: state.Task{
			TaskID: taskID, Project: run.Project, DisplayName: stage.Title, Instruction: instruction,
			TargetKind: targetKind, TargetAgentID: current.StandingAgentID, Role: detail.Template.OrchestratorRole,
			Backend: assignment.Backend, Model: assignment.Model, Effort: assignment.Effort, Fast: assignment.Fast,
			CreatedByKind: "pipeline",
		},
	})
	if err == nil {
		updated, readErr := m.store.ReadPipelineRun(run.RunID)
		if readErr == nil {
			m.publish(updated)
		}
	}
	return err
}

func (m *Manager) advanceTaskStage(run state.PipelineRunRecord, detail RunDetail, current state.PipelineStageTask, stage Stage) error {
	if current.StageIndex+1 >= len(detail.Template.Stages) {
		completed, err := m.store.UpdatePipelineRunCAS(run.RunID, run.Revision, state.PipelineRunUpdate{
			State: "completed", PendingAction: "", CurrentStageID: stage.ID,
			CurrentAgentID: "", FinalOutcome: state.OutcomeSuccess,
		})
		if err == nil {
			m.publish(completed)
			m.notify(completed, "completed")
		}
		return err
	}
	next := detail.Template.Stages[current.StageIndex+1]
	taskID, err := m.store.NewTaskID()
	if err != nil {
		return err
	}
	assignment := standingAssignment(detail.Assignments, next.ID)
	coordinator, err := m.coordinatorTask(run.Project, run.Goal, taskID, next, detail.Assignments[next.ID])
	if err != nil {
		return err
	}
	prior, err := m.priorStageResults(run.RunID, "")
	if err != nil {
		return err
	}
	instruction, digest := renderAssignment(run, detail.Template, next, detail.Values, assignmentContext{
		Coordinator: coordinatorContext(coordinator, next), PriorResults: prior,
	})
	targetKind := state.TargetLaunch
	if current.StandingAgentID != "" {
		targetKind = state.TargetAgent
	}
	_, _, _, err = m.store.CreatePipelineStageTask(state.CreatePipelineStageTaskParams{
		RunID: run.RunID, ExpectedRevision: run.Revision, StageIndex: current.StageIndex + 1,
		AttemptNumber: 1, StageID: next.ID, AssignmentDigest: digest, ParentTaskID: current.TaskID,
		OutputValues: stageOutputValues(next), Coordinator: coordinator, Task: state.Task{
			TaskID: taskID, Project: run.Project, DisplayName: next.Title, Instruction: instruction,
			TargetKind: targetKind, TargetAgentID: current.StandingAgentID, Role: detail.Template.OrchestratorRole,
			Backend: assignment.Backend, Model: assignment.Model, Effort: assignment.Effort, Fast: assignment.Fast,
			CreatedByKind: "pipeline",
		},
	})
	if err == nil {
		updated, readErr := m.store.ReadPipelineRun(run.RunID)
		if readErr == nil {
			m.publish(updated)
		}
	}
	return err
}

func (m *Manager) Retry(ctx context.Context, runID string, expectedRevision int64) (RunDetail, error) {
	unlock := m.lockRun(runID)
	run, err := m.store.ReadPipelineRun(runID)
	if err != nil {
		unlock()
		return RunDetail{}, err
	}
	if run.Revision != expectedRevision {
		unlock()
		return RunDetail{}, controlError("revision_conflict", "run changed; refresh before retrying")
	}
	if current, found, stageErr := m.currentStageTask(runID); stageErr == nil && found {
		updated, retryErr := m.store.RetryInterruptedPipelineStageTask(runID, current.TaskID, expectedRevision)
		if retryErr != nil {
			unlock()
			if errors.Is(retryErr, state.ErrPipelineStageConflict) {
				return RunDetail{}, controlError("invalid_state", "retry is not valid for the current stage task")
			}
			return RunDetail{}, retryErr
		}
		m.publish(updated)
		unlock()
		return m.Detail(runID)
	} else if stageErr != nil {
		unlock()
		return RunDetail{}, stageErr
	}
	unlock()
	return RunDetail{}, controlError("invalid_state", "run has no durable stage task")
}

func (m *Manager) Replace(ctx context.Context, runID string, expectedRevision int64, runtime RuntimeAssignment) (RunDetail, error) {
	unlock := m.lockRun(runID)
	defer unlock()
	run, err := m.store.ReadPipelineRun(runID)
	if err != nil {
		return RunDetail{}, err
	}
	if run.Revision != expectedRevision {
		return RunDetail{}, controlError("revision_conflict", "run changed; refresh before replacing the standing owner")
	}
	detail, err := m.Detail(runID)
	if err != nil {
		return RunDetail{}, err
	}
	current, found, err := m.currentStageTask(runID)
	if err != nil || !found {
		return RunDetail{}, controlError("invalid_state", "there is no current stage task to replace")
	}
	task, err := m.store.ReadTask(current.TaskID)
	if err != nil {
		return RunDetail{}, err
	}
	stage, found := stageByID(detail.Template, current.StageID)
	if !found || run.State != "paused" || task.State != state.TaskInterrupted || runtime.Backend == "" || runtime.Model == "" {
		return RunDetail{}, controlError("invalid_state", "replacement requires an interrupted standing-owner task and a valid runtime")
	}
	if m.lifecycle != nil {
		if err := m.lifecycle.ValidateStage(ctx, StageExecution{StageID: stage.ID, StageTitle: stage.Title, Role: detail.Template.OrchestratorRole, Project: run.Project, Backend: runtime.Backend, Model: runtime.Model, Effort: runtime.Effort, Fast: runtime.Fast}); err != nil {
			return RunDetail{}, validationError("replacement runtime is unavailable", []Diagnostic{{Field: "orchestrator", Code: "unavailable", Message: err.Error()}})
		}
	}
	id, err := m.store.NewTaskID()
	if err != nil {
		return RunDetail{}, err
	}
	// A replacement keeps the predecessor's coordinator binding, so its assignment
	// names that same child rather than minting a second one (TS-09.R37/R41).
	var replacementCoordinator *coordinatorHandoff
	if current.CoordinatorTaskID != "" {
		replacementCoordinator = &coordinatorHandoff{TaskID: current.CoordinatorTaskID, Role: stage.DedicatedRole, Objective: stage.Objective}
	}
	prior, err := m.priorStageResults(runID, "")
	if err != nil {
		return RunDetail{}, err
	}
	instruction, digest := renderAssignment(run, detail.Template, stage, detail.Values, assignmentContext{
		Coordinator: replacementCoordinator, PriorResults: prior,
		Continuation: "Replacement standing owner: inspect the retained stage work and continue from durable results.",
	})
	prepared, err := m.store.PreparePipelineStageReplacement(state.CreatePipelineStageTaskParams{RunID: runID, ExpectedRevision: run.Revision, StageIndex: current.StageIndex, AttemptNumber: current.AttemptNumber + 1, StageID: stage.ID, AssignmentDigest: digest, ParentTaskID: current.TaskID, OutputValues: stageOutputValues(stage), Task: state.Task{TaskID: id, Project: run.Project, DisplayName: stage.Title, Instruction: instruction, TargetKind: state.TargetLaunch, Role: detail.Template.OrchestratorRole, Backend: runtime.Backend, Model: runtime.Model, Effort: runtime.Effort, Fast: runtime.Fast, CreatedByKind: "pipeline"}}, current.TaskID)
	if err != nil {
		if errors.Is(err, state.ErrPipelineStageConflict) {
			return RunDetail{}, controlError("invalid_state", "replacement lost to a newer stage state")
		}
		return RunDetail{}, err
	}
	updated, err := m.store.ActivatePipelineStageReplacement(runID, id, prepared.Revision)
	if err != nil {
		return RunDetail{}, err
	}
	m.publish(updated)
	return m.Detail(runID)
}

func (m *Manager) Stop(ctx context.Context, runID string, expectedRevision int64) (RunDetail, error) {
	unlock := m.lockRun(runID)
	run, err := m.store.ReadPipelineRun(runID)
	if err != nil {
		unlock()
		return RunDetail{}, err
	}
	if run.Revision != expectedRevision {
		unlock()
		return RunDetail{}, controlError("revision_conflict", "run changed; refresh before stopping")
	}
	if run.State == "completed" || run.State == "stopped" {
		unlock()
		return m.Detail(runID)
	}
	if current, found, stageErr := m.currentStageTask(runID); stageErr == nil && found {
		updated, stopErr := m.store.UpdatePipelineRunCAS(runID, run.Revision, state.PipelineRunUpdate{
			State: "stopping", PendingAction: "cleanup_run", CurrentStageID: run.CurrentStageID,
			CurrentAgentID: current.StandingAgentID, FinalOutcome: "",
		})
		if stopErr == nil {
			m.publish(updated)
		}
		unlock()
		if stopErr != nil {
			return RunDetail{}, stopErr
		}
		return m.Detail(runID)
	} else if stageErr != nil {
		unlock()
		return RunDetail{}, stageErr
	}
	unlock()
	return RunDetail{}, controlError("invalid_state", "run has no durable stage task")
}

// FinishStopCleanup commits the terminal run outcome only after every lineage
// member is terminal and its runtime release has settled.
func (m *Manager) FinishStopCleanup(runID string, expectedRevision int64) (RunDetail, error) {
	unlock := m.lockRun(runID)
	defer unlock()
	run, err := m.store.ReadPipelineRun(runID)
	if err != nil {
		return RunDetail{}, err
	}
	if run.Revision != expectedRevision {
		return RunDetail{}, controlError("revision_conflict", "run changed; refresh before finishing cleanup")
	}
	if run.State != "stopping" || run.PendingAction != "cleanup_run" {
		return RunDetail{}, controlError("invalid_state", "run cleanup is not pending")
	}
	settled, err := m.settleCleanup(runID, "", "")
	if err != nil {
		return RunDetail{}, err
	}
	if !settled {
		return RunDetail{}, controlError("cleanup_pending", "run cleanup still has unfinished task effects")
	}
	stopped, err := m.store.UpdatePipelineRunCAS(runID, run.Revision, state.PipelineRunUpdate{
		State: "stopped", PendingAction: "", CurrentStageID: run.CurrentStageID,
		CurrentAgentID: "", FinalOutcome: state.OutcomeCancelled,
	})
	if err != nil {
		return RunDetail{}, err
	}
	m.publish(stopped)
	m.notify(stopped, "completed")
	return m.Detail(runID)
}

func (m *Manager) RepairCleanup(ctx context.Context, runID string, expectedRevision int64) (RunDetail, error) {
	run, err := m.store.ReadPipelineRun(runID)
	if err != nil {
		return RunDetail{}, err
	}
	if run.Revision != expectedRevision {
		return RunDetail{}, controlError("revision_conflict", "run changed; refresh before repairing cleanup")
	}
	// Stage completion retains cleanup exactly as Stop does, so `finishing` needs
	// the same repair route: without it a stage whose cleanup failed persistently
	// had no operator action at all (TS-09.R42).
	if !cleanupRepairable(run) {
		return RunDetail{}, controlError("invalid_state", "cleanup repair is not valid for the current run state")
	}
	if err := m.Reconcile(ctx, runID); err != nil {
		return RunDetail{}, err
	}
	return m.Detail(runID)
}

// CleanupRepairable reports whether a run is holding retained cleanup that the
// operator-facing repair control can retry. Both the stopping run's cleanup and
// a completing stage's release are retained the same way, so both expose the
// same recovery action (TS-09.R42, FS-14.R44).
func CleanupRepairable(state, pendingAction string) bool {
	return (state == "stopping" && pendingAction == "cleanup_run") ||
		(state == "finishing" && pendingAction == "release_stage_task")
}

func cleanupRepairable(run state.PipelineRunRecord) bool {
	return CleanupRepairable(run.State, run.PendingAction)
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
