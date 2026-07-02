package state

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// SessionSnapshot holds the frozen launch config stored in the sessions table.
// Resume rebuilds a LaunchSpec from this snapshot (not from current role/project files).
type SessionSnapshot struct {
	AgentID        string
	Name           string
	Role           string
	Project        string
	Backend        string
	Model          string
	Interface      string
	Group          string
	Cwd            string
	SystemPrompt   string
	EnvKeys        []string // key names only; values re-resolved at resume time
	LastSessionID  string
	LastSeq        int64
	LastContextPct float64
	CreatedAt      string
}

// ReadSession returns the frozen launch snapshot for the given agent_id.
// Returns ErrNotFound if no sessions row exists for the agent.
func (s *Store) ReadSession(agentID string) (SessionSnapshot, error) {
	var snap SessionSnapshot
	var envKeysJSON string
	err := s.db.QueryRow(`
SELECT agent_id, name, role, project, backend, model, interface, grp, cwd, system_prompt,
       env_keys, last_session_id, last_seq, last_context_pct, created_at
FROM sessions WHERE agent_id = ?`, agentID).Scan(
		&snap.AgentID, &snap.Name, &snap.Role, &snap.Project,
		&snap.Backend, &snap.Model, &snap.Interface, &snap.Group,
		&snap.Cwd, &snap.SystemPrompt, &envKeysJSON,
		&snap.LastSessionID, &snap.LastSeq, &snap.LastContextPct, &snap.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return SessionSnapshot{}, ErrNotFound
	}
	if err != nil {
		return SessionSnapshot{}, fmt.Errorf("state: read session: %w", err)
	}
	if err := json.Unmarshal([]byte(envKeysJSON), &snap.EnvKeys); err != nil {
		snap.EnvKeys = nil
	}
	return snap, nil
}

// ListInactiveSessions returns sessions rows that have no matching running row,
// filtered by role and/or project (empty = wildcard), ordered by updated_at DESC.
func (s *Store) ListInactiveSessions(role, project string) ([]SessionSnapshot, error) {
	q := `
SELECT s.agent_id, s.name, s.role, s.project, s.backend, s.model, s.interface, s.grp,
       s.cwd, s.system_prompt, s.env_keys, s.last_session_id, s.last_seq, s.last_context_pct, s.created_at
FROM sessions s
LEFT JOIN running r ON r.agent_id = s.agent_id
WHERE r.agent_id IS NULL`
	args := []any{}
	if role != "" {
		q += " AND s.role = ?"
		args = append(args, role)
	}
	if project != "" {
		q += " AND s.project = ?"
		args = append(args, project)
	}
	q += " ORDER BY s.updated_at DESC"

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("state: list inactive sessions: %w", err)
	}
	defer rows.Close()
	var out []SessionSnapshot
	for rows.Next() {
		var snap SessionSnapshot
		var envKeysJSON string
		if err := rows.Scan(
			&snap.AgentID, &snap.Name, &snap.Role, &snap.Project,
			&snap.Backend, &snap.Model, &snap.Interface, &snap.Group,
			&snap.Cwd, &snap.SystemPrompt, &envKeysJSON,
			&snap.LastSessionID, &snap.LastSeq, &snap.LastContextPct, &snap.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("state: scan inactive session: %w", err)
		}
		if err := json.Unmarshal([]byte(envKeysJSON), &snap.EnvKeys); err != nil {
			snap.EnvKeys = nil
		}
		out = append(out, snap)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("state: list inactive sessions: %w", err)
	}
	return out, nil
}
