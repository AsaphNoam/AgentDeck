package server

import (
	"time"

	"github.com/agentdeck/agentdeck/internal/pipeline"
	"github.com/agentdeck/agentdeck/internal/state"
)

type pipelineRunDetailResponse struct {
	pipeline.RunDetail
	AgentsByAttempt map[string]pipelineAttemptAgents `json:"agents_by_attempt"`
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
