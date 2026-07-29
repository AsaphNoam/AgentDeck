package state

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
)

var randRead = rand.Read

// NewAgentID generates "a_" + 6 lowercase hex chars and retries collisions
// against the agents table up to 10 times.
func (s *Store) NewAgentID() (string, error) {
	for i := 0; i < 10; i++ {
		var b [3]byte
		if _, err := randRead(b[:]); err != nil {
			return "", fmt.Errorf("state: read random: %w", err)
		}
		id := "a_" + hex.EncodeToString(b[:])
		var exists int
		err := s.db.QueryRow(`SELECT 1 FROM agents WHERE agent_id = ?`, id).Scan(&exists)
		if errors.Is(err, sql.ErrNoRows) {
			return id, nil
		}
		if err != nil {
			return "", fmt.Errorf("state: check agent_id collision: %w", err)
		}
	}
	return "", errors.New("state: could not mint unique agent_id after 10 tries")
}

// ReadAgent returns one agent by id.
func (s *Store) ReadAgent(id string) (Agent, error) {
	var a Agent
	var createdAt string
	err := s.db.QueryRow(`
SELECT agent_id, name, role, project, backend, model, interface, created_at, grp, archived
FROM agents
WHERE agent_id = ?`, id).Scan(
		&a.AgentID, &a.Name, &a.Role, &a.Project, &a.Backend, &a.Model,
		&a.Interface, &createdAt, &a.Group, &a.Archived,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Agent{}, ErrNotFound
	}
	if err != nil {
		return Agent{}, fmt.Errorf("state: read agent: %w", err)
	}
	a.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return Agent{}, wrapTimeErr("agent.created_at", err)
	}
	return a, nil
}

// WriteAgent inserts or updates an agent.
func (s *Store) WriteAgent(a Agent) error {
	_, err := s.db.Exec(`
INSERT INTO agents(agent_id, name, role, project, backend, model, interface, created_at, grp, archived)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(agent_id) DO UPDATE SET
    name = excluded.name,
    role = excluded.role,
    project = excluded.project,
    backend = excluded.backend,
    model = excluded.model,
    interface = excluded.interface,
    created_at = excluded.created_at,
    grp = excluded.grp,
    archived = excluded.archived`,
		a.AgentID, a.Name, a.Role, a.Project, a.Backend, a.Model, a.Interface,
		formatTime(a.CreatedAt), a.Group, a.Archived,
	)
	if err != nil {
		return fmt.Errorf("state: write agent: %w", err)
	}
	return nil
}

// ListAgents returns agents ordered by created_at.
func (s *Store) ListAgents() ([]Agent, error) {
	rows, err := s.db.Query(`
SELECT agent_id, name, role, project, backend, model, interface, created_at, grp, archived
FROM agents
ORDER BY created_at, agent_id`)
	if err != nil {
		return nil, fmt.Errorf("state: list agents: %w", err)
	}
	defer rows.Close()

	out := []Agent{}
	for rows.Next() {
		var a Agent
		var createdAt string
		if err := rows.Scan(
			&a.AgentID, &a.Name, &a.Role, &a.Project, &a.Backend, &a.Model,
			&a.Interface, &createdAt, &a.Group, &a.Archived,
		); err != nil {
			return nil, fmt.Errorf("state: scan agent: %w", err)
		}
		a.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, wrapTimeErr("agent.created_at", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("state: iterate agents: %w", err)
	}
	return out, nil
}

// SetAgentsArchived updates every requested durable identity in one transaction.
// Callers stop all running agents before marking them archived.
func (s *Store) SetAgentsArchived(ids []string, archived bool) error {
	if len(ids) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("state: begin archive update: %w", err)
	}
	defer tx.Rollback()
	for _, id := range ids {
		if _, err := tx.Exec(`UPDATE agents SET archived = ? WHERE agent_id = ?`, archived, id); err != nil {
			return fmt.Errorf("state: update agent archive: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("state: commit archive update: %w", err)
	}
	return nil
}

// RestoreAgentArchiveStates compensates a failed cross-store project archive
// with each agent's exact pre-transition flag, rather than assuming every
// project member was active before the attempt.
func (s *Store) RestoreAgentArchiveStates(states map[string]bool) error {
	if len(states) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("state: begin archive compensation: %w", err)
	}
	defer tx.Rollback()
	for id, archived := range states {
		if _, err := tx.Exec(`UPDATE agents SET archived = ? WHERE agent_id = ?`, archived, id); err != nil {
			return fmt.Errorf("state: restore agent archive state: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("state: commit archive compensation: %w", err)
	}
	return nil
}

// DeleteAgent deletes an agent and cascades dependent state rows.
func (s *Store) DeleteAgent(id string) error {
	if _, err := s.db.Exec(`DELETE FROM agents WHERE agent_id = ?`, id); err != nil {
		return fmt.Errorf("state: delete agent: %w", err)
	}
	return nil
}
