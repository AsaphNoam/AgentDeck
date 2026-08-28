package state

import (
	"fmt"
	"strings"
)

// PipelineAgentSnapshots reads identity, running, and status for all requested
// agents in one durable query. It is intentionally separate from dashboard
// hydration so a freshly reloaded run detail has a complete initial card state.
func (s *Store) PipelineAgentSnapshots(agentIDs []string) (map[string]PipelineAgentSnapshot, error) {
	out := make(map[string]PipelineAgentSnapshot, len(agentIDs))
	ids := distinctNonEmpty(agentIDs)
	if len(ids) == 0 {
		return out, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := s.db.Query(`
SELECT a.agent_id, a.name, s.agent_id IS NOT NULL, r.agent_id IS NOT NULL,
       COALESCE(s.state, ''), COALESCE(s.detail, '')
FROM agents a
LEFT JOIN status s ON s.agent_id = a.agent_id
LEFT JOIN running r ON r.agent_id = a.agent_id
WHERE a.agent_id IN (`+placeholders+`)
ORDER BY a.agent_id`, args...)
	if err != nil {
		return nil, fmt.Errorf("state: read pipeline agent snapshots: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var snapshot PipelineAgentSnapshot
		if err := rows.Scan(&snapshot.AgentID, &snapshot.Name, &snapshot.StatusFound,
			&snapshot.Running, &snapshot.State, &snapshot.Detail); err != nil {
			return nil, fmt.Errorf("state: scan pipeline agent snapshot: %w", err)
		}
		snapshot.IdentityFound = true
		out[snapshot.AgentID] = snapshot
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("state: iterate pipeline agent snapshots: %w", err)
	}
	return out, nil
}

// PipelineDelegatedTasks returns the capped direct-delegate task rows for the
// supplied attempt creator windows. The query uses no task arms or attachments;
// counts remain true even when the visible per-attempt rows are capped.
func (s *Store) PipelineDelegatedTasks(project string, windows []PipelineAttemptWindow, limit int) ([]PipelineDelegatedTask, error) {
	if len(windows) == 0 || limit <= 0 {
		return []PipelineDelegatedTask{}, nil
	}
	query, args := pipelineDelegatedTaskQuery(project, windows, limit)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("state: list pipeline delegated tasks: %w", err)
	}
	defer rows.Close()
	out := []PipelineDelegatedTask{}
	for rows.Next() {
		var task PipelineDelegatedTask
		var identityFound, statusFound, running bool
		if err := rows.Scan(
			&task.AttemptID, &task.TaskID, &task.DisplayName, &task.State, &task.Outcome,
			&task.OutcomeSummary, &task.AssignedAgentID, &task.Agent.AgentID, &task.Agent.Name,
			&identityFound, &statusFound, &running, &task.Agent.State, &task.Agent.Detail,
			&task.DelegatedTotal, &task.DelegatedRunningCount,
		); err != nil {
			return nil, fmt.Errorf("state: scan pipeline delegated task: %w", err)
		}
		task.Agent.IdentityFound = identityFound
		task.Agent.StatusFound = statusFound
		task.Agent.Running = running
		out = append(out, task)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("state: iterate pipeline delegated tasks: %w", err)
	}
	return out, nil
}

func pipelineDelegatedTaskQuery(project string, windows []PipelineAttemptWindow, limit int) (string, []any) {
	values := make([]string, 0, len(windows))
	args := make([]any, 0, len(windows)*5+2)
	for _, window := range windows {
		values = append(values, "(?, ?, ?, ?, ?)")
		args = append(args, window.AttemptID, window.AgentID, window.AgentGeneration,
			formatTime(window.CreatedAt), formatOptionalTime(window.NextCreatedAt))
	}
	args = append(args, project, limit)
	return `
WITH attempt_windows(attempt_id, creator_id, creator_generation, created_at, next_created_at) AS (
  VALUES ` + strings.Join(values, ",") + `
), matched AS (
  SELECT w.attempt_id, t.task_id, t.display_name, t.state, t.outcome,
         t.outcome_summary, t.assigned_agent_id,
         COALESCE(a.agent_id, '') AS identity_agent_id, COALESCE(a.name, '') AS agent_name,
         a.agent_id IS NOT NULL AS identity_found,
         s.agent_id IS NOT NULL AS status_found,
         r.agent_id IS NOT NULL AS running,
         COALESCE(s.state, '') AS agent_state, COALESCE(s.detail, '') AS agent_detail,
         t.created_at
  FROM attempt_windows w
  JOIN tasks t ON t.project = ?
    AND t.created_by_kind = 'agent'
    AND t.assigned_agent_id IS NOT NULL
    AND t.created_by_agent_id = w.creator_id
    AND t.created_by_generation = w.creator_generation
    AND t.created_at >= w.created_at
    AND (w.next_created_at IS NULL OR t.created_at < w.next_created_at)
  LEFT JOIN agents a ON a.agent_id = t.assigned_agent_id
  LEFT JOIN status s ON s.agent_id = t.assigned_agent_id
  LEFT JOIN running r ON r.agent_id = t.assigned_agent_id
), counts AS (
  SELECT attempt_id, COUNT(*) AS delegated_total,
         COUNT(DISTINCT CASE WHEN running THEN assigned_agent_id END) AS delegated_running_count
  FROM matched GROUP BY attempt_id
), ranked AS (
  SELECT matched.*, ROW_NUMBER() OVER (
    PARTITION BY attempt_id ORDER BY created_at DESC, task_id
  ) AS row_number
  FROM matched
)
SELECT ranked.attempt_id, ranked.task_id, ranked.display_name, ranked.state,
       ranked.outcome, ranked.outcome_summary, ranked.assigned_agent_id,
       ranked.identity_agent_id, ranked.agent_name, ranked.identity_found,
       ranked.status_found, ranked.running, ranked.agent_state, ranked.agent_detail,
       counts.delegated_total, counts.delegated_running_count
FROM ranked JOIN counts USING (attempt_id)
WHERE ranked.row_number <= ?
ORDER BY ranked.attempt_id, ranked.created_at DESC, ranked.task_id`, args
}

func distinctNonEmpty(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
