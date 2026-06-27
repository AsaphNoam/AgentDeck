package state

import (
	"database/sql"
	"errors"
	"fmt"
)

// ReadRunning returns one running entry by agent id.
func (s *Store) ReadRunning(id string) (RunningEntry, error) {
	var r RunningEntry
	var startedAt string
	err := s.db.QueryRow(`
SELECT agent_id, pid, session_id, interface, tty, started_at
FROM running
WHERE agent_id = ?`, id).Scan(
		&r.AgentID, &r.PID, &r.SessionID, &r.Interface, &r.TTY, &startedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return RunningEntry{}, ErrNotFound
	}
	if err != nil {
		return RunningEntry{}, fmt.Errorf("state: read running: %w", err)
	}
	r.StartedAt, err = parseTime(startedAt)
	if err != nil {
		return RunningEntry{}, wrapTimeErr("running.started_at", err)
	}
	return r, nil
}

// WriteRunning inserts or updates a running entry.
func (s *Store) WriteRunning(r RunningEntry) error {
	_, err := s.db.Exec(`
INSERT INTO running(agent_id, pid, session_id, interface, tty, started_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(agent_id) DO UPDATE SET
    pid = excluded.pid,
    session_id = excluded.session_id,
    interface = excluded.interface,
    tty = excluded.tty,
    started_at = excluded.started_at`,
		r.AgentID, r.PID, r.SessionID, r.Interface, r.TTY, formatTime(r.StartedAt),
	)
	if err != nil {
		return fmt.Errorf("state: write running: %w", err)
	}
	return nil
}

// ListRunning returns all running entries.
func (s *Store) ListRunning() ([]RunningEntry, error) {
	rows, err := s.db.Query(`
SELECT agent_id, pid, session_id, interface, tty, started_at
FROM running
ORDER BY started_at, agent_id`)
	if err != nil {
		return nil, fmt.Errorf("state: list running: %w", err)
	}
	defer rows.Close()

	out := []RunningEntry{}
	for rows.Next() {
		var r RunningEntry
		var startedAt string
		if err := rows.Scan(&r.AgentID, &r.PID, &r.SessionID, &r.Interface, &r.TTY, &startedAt); err != nil {
			return nil, fmt.Errorf("state: scan running: %w", err)
		}
		r.StartedAt, err = parseTime(startedAt)
		if err != nil {
			return nil, wrapTimeErr("running.started_at", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("state: iterate running: %w", err)
	}
	return out, nil
}

// DeleteRunning deletes one running entry.
func (s *Store) DeleteRunning(id string) error {
	if _, err := s.db.Exec(`DELETE FROM running WHERE agent_id = ?`, id); err != nil {
		return fmt.Errorf("state: delete running: %w", err)
	}
	return nil
}
