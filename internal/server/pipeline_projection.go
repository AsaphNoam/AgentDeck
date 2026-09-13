package server

import (
	"time"

	"github.com/agentdeck/agentdeck/internal/pipeline"
	"github.com/agentdeck/agentdeck/internal/state"
)

type pipelineRunDetailResponse struct {
	pipeline.RunDetail
	AgentsByAttempt      map[string]pipelineAttemptAgents      `json:"agents_by_attempt"`
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
	TaskID         string               `json:"task_id"`
	RunID          string               `json:"run_id"`
	StageID        string               `json:"stage_id"`
	StageIndex     int                  `json:"stage_index"`
	AttemptNumber  int                  `json:"attempt_number"`
	State          string               `json:"state"`
	AssignmentText string               `json:"assignment_text"`
	StandingOwner  pipelineStageOwner   `json:"standing_owner"`
	Result         *pipelineStageResult `json:"result,omitempty"`
	Work           []pipelineTaskWork   `json:"work"`
	CreatedAt      time.Time            `json:"created_at"`
	UpdatedAt      time.Time            `json:"updated_at"`
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

func (s *Server) pipelineTaskRunProjection(detail pipeline.RunDetail) ([]pipelineStageTaskDetail, pipelineRunControls, error) {
	stages, err := s.stateStore.ListPipelineStageTasks(detail.Run.RunID)
	if err != nil {
		return nil, pipelineRunControls{}, err
	}
	agentIDs := make([]string, 0, len(stages))
	for _, stage := range stages {
		if stage.StandingAgentID != "" {
			agentIDs = append(agentIDs, stage.StandingAgentID)
		}
	}
	snapshots, err := s.stateStore.PipelineAgentSnapshots(agentIDs)
	if err != nil {
		return nil, pipelineRunControls{}, err
	}
	runTasks, err := s.stateStore.ListTasksForPipelineRun(detail.Run.RunID)
	if err != nil {
		return nil, pipelineRunControls{}, err
	}
	tasksByID := make(map[string]state.Task, len(runTasks))
	childrenByParent := map[string][]string{}
	for _, task := range runTasks {
		tasksByID[task.TaskID] = task
		lineage, lineageErr := s.stateStore.ReadTaskLineage(task.TaskID)
		if lineageErr != nil {
			return nil, pipelineRunControls{}, lineageErr
		}
		if lineage.ParentTaskID != "" {
			childrenByParent[lineage.ParentTaskID] = append(childrenByParent[lineage.ParentTaskID], task.TaskID)
		}
	}
	var projectWork func(string) []pipelineTaskWork
	projectWork = func(parentID string) []pipelineTaskWork {
		items := []pipelineTaskWork{}
		for _, taskID := range childrenByParent[parentID] {
			task := tasksByID[taskID]
			route := "unavailable"
			if task.AssignedAgentID != "" {
				if _, readErr := s.stateStore.ReadAgent(task.AssignedAgentID); readErr == nil {
					route = "archive"
					if _, runningErr := s.stateStore.ReadRunning(task.AssignedAgentID); runningErr == nil {
						route = "live"
					}
				}
			}
			items = append(items, pipelineTaskWork{TaskID: task.TaskID, DisplayName: task.DisplayName, State: task.State, Outcome: task.Outcome, Summary: task.OutcomeSummary, AgentID: task.AssignedAgentID, Route: route, Children: projectWork(task.TaskID)})
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
		item := pipelineStageTaskDetail{TaskID: task.TaskID, RunID: stage.RunID, StageID: stage.StageID, StageIndex: stage.StageIndex, AttemptNumber: stage.AttemptNumber, State: task.State, AssignmentText: task.Instruction, StandingOwner: owner, Work: projectWork(task.TaskID), CreatedAt: stage.CreatedAt, UpdatedAt: task.UpdatedAt}
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
		controls.Replace = pipelineRunControl{Eligible: false, Reason: "Standing-owner replacement is unavailable for this run state."}
		controls.RepairCleanup = pipelineRunControl{Eligible: detail.Run.State == "stopping" && detail.Run.PendingAction == "cleanup_run", Reason: "Retries only the retained cleanup effects."}
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
