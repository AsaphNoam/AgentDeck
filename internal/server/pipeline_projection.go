package server

import (
	"time"

	"github.com/agentdeck/agentdeck/internal/pipeline"
	"github.com/agentdeck/agentdeck/internal/state"
)

type pipelineRunDetailResponse struct {
	pipeline.RunDetail
	Orchestrator         pipeline.RuntimeAssignment            `json:"orchestrator"`
	DedicatedAssignments map[string]pipeline.RuntimeAssignment `json:"dedicated_assignments"`
	StageTasks           []pipelineStageTaskDetail             `json:"stage_tasks"`
	Controls             pipelineRunControls                   `json:"controls"`
}

type pipelineRunControl struct {
	Eligible bool   `json:"eligible"`
	Reason   string `json:"reason,omitempty"`
}

type pipelineRunControls struct {
	Continue      pipelineRunControl `json:"continue"`
	Retry         pipelineRunControl `json:"retry"`
	Replace       pipelineRunControl `json:"replace"`
	Stop          pipelineRunControl `json:"stop"`
	RepairCleanup pipelineRunControl `json:"repair_cleanup"`
}

type pipelineStageTaskDetail struct {
	TaskID         string                    `json:"task_id"`
	RunID          string                    `json:"run_id"`
	StageID        string                    `json:"stage_id"`
	StageIndex     int                       `json:"stage_index"`
	AttemptNumber  int                       `json:"attempt_number"`
	State          string                    `json:"state"`
	AssignmentText string                    `json:"assignment_text"`
	StandingOwner  pipelineStageOwner        `json:"standing_owner"`
	Coordinator    *pipelineStageCoordinator `json:"coordinator,omitempty"`
	Result         *pipelineStageResult      `json:"result,omitempty"`
	Work           []pipelineTaskWork        `json:"work"`
	Cleanup        *pipelineStageCleanup     `json:"cleanup,omitempty"`
	CreatedAt      time.Time                 `json:"created_at"`
	UpdatedAt      time.Time                 `json:"updated_at"`
}

// pipelineStageCleanup is the bounded, in-vocabulary view of a stage task's
// retained cleanup. Supervision already renders this shape; the projection never
// emitted it, so the stored phase, unsafe flag and error were invisible and a
// stage holding unrecoverable cleanup looked idle (TS-09.R42, FS-14.R44,
// INV §8/§10/§11).
type pipelineStageCleanup struct {
	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
}

func stageCleanupDetail(task state.Task) *pipelineStageCleanup {
	if task.CleanupPhase == "" {
		return nil
	}
	// `retrying` is the transient schedule the dispatcher owns; `needs_attention`
	// is the persistent condition that earns the repair control.
	phase := task.CleanupPhase + "_retrying"
	if task.CleanupUnsafe {
		phase = task.CleanupPhase + "_needs_attention"
	}
	return &pipelineStageCleanup{State: phase, Reason: clipPreview(task.CleanupLastError, detailPreviewLimit)}
}

type pipelineStageCoordinator struct {
	TaskID        string                     `json:"task_id"`
	AgentID       string                     `json:"agent_id,omitempty"`
	Name          string                     `json:"name,omitempty"`
	State         string                     `json:"state,omitempty"`
	Route         string                     `json:"route"`
	ReportSummary string                     `json:"report_summary,omitempty"`
	Runtime       pipeline.RuntimeAssignment `json:"runtime"`
}

type pipelineStageOwner struct {
	AgentID string                     `json:"agent_id,omitempty"`
	Name    string                     `json:"name,omitempty"`
	State   string                     `json:"state,omitempty"`
	Route   string                     `json:"route"`
	Runtime pipeline.RuntimeAssignment `json:"runtime"`
}

type pipelineStageResult struct {
	Outcome string            `json:"outcome"`
	Summary string            `json:"summary"`
	Details string            `json:"details"`
	Checks  string            `json:"checks"`
	Outputs map[string]string `json:"outputs"`
}

type pipelineTaskWork struct {
	TaskID      string             `json:"task_id"`
	DisplayName string             `json:"display_name"`
	State       string             `json:"state"`
	Outcome     string             `json:"outcome,omitempty"`
	Summary     string             `json:"summary,omitempty"`
	AgentID     string             `json:"agent_id,omitempty"`
	Route       string             `json:"route"`
	Children    []pipelineTaskWork `json:"children"`
}

// maxProjectedRunTasks bounds the task history one run read carries. A long run
// accumulates descendants without limit, and a supervision read must not grow
// with them (TS-09.R28, INV §16).
const maxProjectedRunTasks = 200

// agentRoute derives one task's route from the batched snapshot set rather than
// a per-task identity and liveness read.
func agentRoute(snapshots map[string]state.PipelineAgentSnapshot, agentID string) string {
	if agentID == "" {
		return "unavailable"
	}
	snapshot, ok := snapshots[agentID]
	if !ok || !snapshot.IdentityFound {
		return "unavailable"
	}
	if snapshot.Running {
		return "live"
	}
	return "archive"
}

func (s *Server) pipelineTaskRunProjection(detail pipeline.RunDetail) ([]pipelineStageTaskDetail, pipelineRunControls, error) {
	stages, err := s.stateStore.ListPipelineStageTasks(detail.Run.RunID)
	if err != nil {
		return nil, pipelineRunControls{}, err
	}
	runTasks, err := s.stateStore.ListTasksForPipelineRun(detail.Run.RunID, maxProjectedRunTasks)
	if err != nil {
		return nil, pipelineRunControls{}, err
	}
	// One identity/liveness read for every agent this projection can mention, and
	// one provenance read for the whole run. Reading them per task made a long
	// run's supervision read grow with its descendant count (TS-09.R28/R42,
	// INV §7/§16).
	agentIDs := make([]string, 0, len(stages)+len(runTasks))
	for _, stage := range stages {
		if stage.StandingAgentID != "" {
			agentIDs = append(agentIDs, stage.StandingAgentID)
		}
	}
	for _, task := range runTasks {
		if task.AssignedAgentID != "" {
			agentIDs = append(agentIDs, task.AssignedAgentID)
		}
	}
	snapshots, err := s.stateStore.PipelineAgentSnapshots(agentIDs)
	if err != nil {
		return nil, pipelineRunControls{}, err
	}
	lineageByTask, err := s.stateStore.ListPipelineRunTaskLineage(detail.Run.RunID, maxProjectedRunTasks)
	if err != nil {
		return nil, pipelineRunControls{}, err
	}
	// A stage task's parent is its predecessor stage task: that edge is stage
	// succession, not delegated work. Immutable lineage keeps it, but rendering it
	// as subordinate work nested stage two under stage one and repeated the whole
	// remaining stage tail under every earlier card (TS-09.R44, FS-14.R39).
	// Each stage has its own card, so succession edges never enter the work tree.
	stageTaskIDs := make(map[string]struct{}, len(stages))
	for _, stage := range stages {
		stageTaskIDs[stage.TaskID] = struct{}{}
	}
	tasksByID := make(map[string]state.Task, len(runTasks))
	childrenByParent := map[string][]string{}
	for _, task := range runTasks {
		tasksByID[task.TaskID] = task
		if _, isStage := stageTaskIDs[task.TaskID]; isStage {
			continue
		}
		if parent := lineageByTask[task.TaskID].ParentTaskID; parent != "" {
			childrenByParent[parent] = append(childrenByParent[parent], task.TaskID)
		}
	}
	var projectWork func(string) []pipelineTaskWork
	projectWork = func(parentID string) []pipelineTaskWork {
		items := []pipelineTaskWork{}
		for _, taskID := range childrenByParent[parentID] {
			task := tasksByID[taskID]
			items = append(items, pipelineTaskWork{TaskID: task.TaskID, DisplayName: task.DisplayName, State: task.State, Outcome: task.Outcome, Summary: task.OutcomeSummary, AgentID: task.AssignedAgentID, Route: agentRoute(snapshots, task.AssignedAgentID), Children: projectWork(task.TaskID)})
		}
		return items
	}
	out := make([]pipelineStageTaskDetail, 0, len(stages))
	for _, stage := range stages {
		task, err := s.stateStore.ReadTask(stage.TaskID)
		if err != nil {
			return nil, pipelineRunControls{}, err
		}
		owner := pipelineStageOwner{AgentID: stage.StandingAgentID, Name: stage.StandingAgentID, State: "unknown", Route: "unavailable", Runtime: pipeline.RuntimeAssignment{Backend: task.Backend, Model: task.Model, Effort: task.Effort, Fast: task.Fast}}
		if stage.StandingAgentID != "" {
			card := pipelineAgentCard(snapshots[stage.StandingAgentID], stage.StandingAgentID, stage.StandingAgentID, task.OutcomeSummary)
			owner.Name, owner.State, owner.Route = card.Name, card.State, card.Route
		}
		work := projectWork(task.TaskID)
		if stage.CoordinatorTaskID != "" {
			filtered := work[:0]
			for _, child := range work {
				if child.TaskID != stage.CoordinatorTaskID {
					filtered = append(filtered, child)
				}
			}
			work = filtered
		}
		item := pipelineStageTaskDetail{TaskID: task.TaskID, RunID: stage.RunID, StageID: stage.StageID, StageIndex: stage.StageIndex, AttemptNumber: stage.AttemptNumber, State: task.State, AssignmentText: task.Instruction, StandingOwner: owner, Work: work, Cleanup: stageCleanupDetail(task), CreatedAt: stage.CreatedAt, UpdatedAt: task.UpdatedAt}
		if stage.CoordinatorTaskID != "" {
			coordinator, readErr := s.stateStore.ReadTask(stage.CoordinatorTaskID)
			if readErr != nil {
				return nil, pipelineRunControls{}, readErr
			}
			name := coordinator.AssignedAgentID
			if snapshot, ok := snapshots[coordinator.AssignedAgentID]; ok && snapshot.Name != "" {
				name = snapshot.Name
			}
			item.Coordinator = &pipelineStageCoordinator{TaskID: coordinator.TaskID, AgentID: coordinator.AssignedAgentID, Name: name, State: coordinator.State, Route: agentRoute(snapshots, coordinator.AssignedAgentID), ReportSummary: coordinator.OutcomeSummary, Runtime: pipeline.RuntimeAssignment{Backend: coordinator.Backend, Model: coordinator.Model, Effort: coordinator.Effort, Fast: coordinator.Fast}}
		}
		if task.Outcome != "" {
			outputs, err := s.stateStore.ReadTaskResultOutputs(task.TaskID)
			if err != nil {
				return nil, pipelineRunControls{}, err
			}
			item.Result = &pipelineStageResult{Outcome: task.Outcome, Summary: task.OutcomeSummary, Details: task.OutcomeDetails, Outputs: outputs}
		}
		out = append(out, item)
	}
	controls := pipelineRunControls{}
	terminal := detail.Run.State == "completed" || detail.Run.State == "stopped"
	controls.Stop = pipelineRunControl{Eligible: !terminal && detail.Run.State != "stopping", Reason: "Stops the run and cancels unfinished work."}
	if len(out) > 0 {
		current := out[len(out)-1]
		controls.Continue = pipelineRunControl{Eligible: detail.Run.State == "paused" && (detail.Run.PendingAction == "await_approval" || current.Result != nil && (current.Result.Outcome == state.OutcomeFailure || current.Result.Outcome == state.OutcomeBlocked)), Reason: "Approves success or sends recovery input to a new stage attempt."}
		controls.Retry = pipelineRunControl{Eligible: detail.Run.State == "paused" && current.State == state.TaskInterrupted, Reason: "Retries the interrupted assignment on the same standing owner."}
		controls.Replace = pipelineRunControl{Eligible: detail.Run.State == "paused" && current.State == state.TaskInterrupted, Reason: "Replaces only the interrupted standing owner and retains stage work."}
		controls.RepairCleanup = pipelineRunControl{Eligible: pipeline.CleanupRepairable(detail.Run.State, detail.Run.PendingAction), Reason: "Retries only the retained cleanup effects."}
	}
	return out, controls, nil
}

type pipelineAttemptAgents struct {
	StageAgent            *pipelineAgentSummary    `json:"stage_agent"`
	DelegatedAgents       []pipelineDelegatedAgent `json:"delegated_agents"`
	DelegatedTotal        int                      `json:"delegated_total"`
	DelegatedRunningCount int                      `json:"delegated_running_count"`
}

type pipelineAgentSummary struct {
	AgentID   string `json:"agent_id"`
	Name      string `json:"name"`
	Running   bool   `json:"running"`
	State     string `json:"state"`
	Preview   string `json:"preview"`
	Route     string `json:"route"`
	Available bool   `json:"available"`
	Fast      bool   `json:"fast"`
}

type pipelineDelegatedAgent struct {
	pipelineAgentSummary
	TaskID      string `json:"task_id"`
	DisplayName string `json:"display_name"`
	TaskState   string `json:"task_state"`
	Outcome     string `json:"outcome"`
}

// pipelineAttemptAgents joins a pipeline-only detail with the task domain at
// the HTTP boundary. It never asks the pipeline manager to know about tasks.
func (s *Server) pipelineAttemptAgents(detail pipeline.RunDetail) (map[string]pipelineAttemptAgents, error) {
	out := make(map[string]pipelineAttemptAgents, len(detail.Attempts))
	stageIDs := make([]string, 0, len(detail.Attempts))
	for _, attempt := range detail.Attempts {
		out[attempt.AttemptID] = pipelineAttemptAgents{DelegatedAgents: []pipelineDelegatedAgent{}}
		if attempt.AgentID != "" {
			stageIDs = append(stageIDs, attempt.AgentID)
		}
	}

	snapshots, err := s.stateStore.PipelineAgentSnapshots(stageIDs)
	if err != nil {
		return nil, err
	}
	for _, attempt := range detail.Attempts {
		entry := out[attempt.AttemptID]
		if attempt.AgentID != "" {
			snapshot := snapshots[attempt.AgentID]
			summary := pipelineAgentCard(snapshot, attempt.AgentID, attempt.AgentID, attempt.ReportSummary)
			entry.StageAgent = &summary
		}
		out[attempt.AttemptID] = entry
	}

	delegates, err := s.stateStore.PipelineDelegatedTasks(detail.Run.Project, pipelineAttemptWindows(detail.Attempts), pipeline.MaxDelegatedAgents)
	if err != nil {
		return nil, err
	}
	for _, task := range delegates {
		entry := out[task.AttemptID]
		summary := pipelineAgentCard(task.Agent, task.AssignedAgentID, task.DisplayName, task.OutcomeSummary)
		entry.DelegatedAgents = append(entry.DelegatedAgents, pipelineDelegatedAgent{
			pipelineAgentSummary: summary,
			TaskID:               task.TaskID,
			DisplayName:          task.DisplayName,
			TaskState:            task.State,
			Outcome:              task.Outcome,
		})
		entry.DelegatedTotal = task.DelegatedTotal
		entry.DelegatedRunningCount = task.DelegatedRunningCount
		out[task.AttemptID] = entry
	}
	return out, nil
}

func pipelineAttemptWindows(attempts []state.PipelineAttemptRecord) []state.PipelineAttemptWindow {
	windows := make([]state.PipelineAttemptWindow, 0, len(attempts))
	nextByCreator := map[string]*time.Time{}
	for i := len(attempts) - 1; i >= 0; i-- {
		attempt := attempts[i]
		if attempt.AgentID == "" || attempt.AgentGeneration == "" {
			continue
		}
		key := attempt.AgentID + "\x00" + attempt.AgentGeneration
		windows = append(windows, state.PipelineAttemptWindow{
			AttemptID: attempt.AttemptID, AgentID: attempt.AgentID, AgentGeneration: attempt.AgentGeneration,
			CreatedAt: attempt.CreatedAt, NextCreatedAt: nextByCreator[key],
		})
		next := attempt.CreatedAt
		nextByCreator[key] = &next
	}
	// The SQL does not rely on this ordering, but execution order makes query
	// plans and diagnostics reproducible when multiple creator windows exist.
	for i, j := 0, len(windows)-1; i < j; i, j = i+1, j-1 {
		windows[i], windows[j] = windows[j], windows[i]
	}
	return windows
}

func pipelineAgentCard(snapshot state.PipelineAgentSnapshot, agentID, fallbackName, fallbackPreview string) pipelineAgentSummary {
	available := snapshot.IdentityFound && snapshot.StatusFound
	summary := pipelineAgentSummary{
		AgentID: agentID, Name: fallbackName, State: "unknown", Preview: pipelinePreview(fallbackPreview),
		Route: "unavailable", Available: available,
	}
	if snapshot.IdentityFound && snapshot.Name != "" {
		summary.Name = snapshot.Name
	}
	if !available {
		return summary
	}
	summary.Running = snapshot.Running
	summary.Fast = snapshot.Fast
	summary.State = snapshot.State
	if summary.State == "" {
		summary.State = "unknown"
	}
	if snapshot.Detail != "" {
		summary.Preview = pipelinePreview(snapshot.Detail)
	}
	if summary.Running {
		summary.Route = "live"
	} else {
		summary.Route = "archive"
	}
	return summary
}

func pipelinePreview(value string) string {
	if value == "" {
		value = "No recent activity"
	}
	return clipPreview(value, detailPreviewLimit)
}
