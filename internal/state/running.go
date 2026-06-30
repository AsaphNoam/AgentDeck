package state

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// ReadRunning returns one running entry by agent id.
func (s *Store) ReadRunning(id string) (RunningEntry, error) {
	var r RunningEntry
	var startedAt, driverIDs string
	err := s.db.QueryRow(`
SELECT agent_id, pid, session_id, interface, tty, driver, driver_ids, hook_token, started_at
FROM running
WHERE agent_id = ?`, id).Scan(
		&r.AgentID, &r.PID, &r.SessionID, &r.Interface, &r.TTY, &r.Driver, &driverIDs, &r.HookToken, &startedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return RunningEntry{}, ErrNotFound
	}
	if err != nil {
		return RunningEntry{}, fmt.Errorf("state: read running: %w", err)
	}
	r.DriverIDs = decodeDriverIDs(driverIDs)
	r.StartedAt, err = parseTime(startedAt)
	if err != nil {
		return RunningEntry{}, wrapTimeErr("running.started_at", err)
	}
	return r, nil
}

// WriteRunning inserts or updates a running entry.
func (s *Store) WriteRunning(r RunningEntry) error {
	_, err := s.db.Exec(`
INSERT INTO running(agent_id, pid, session_id, interface, tty, driver, driver_ids, hook_token, started_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(agent_id) DO UPDATE SET
    pid = excluded.pid,
    session_id = excluded.session_id,
    interface = excluded.interface,
    tty = excluded.tty,
    driver = excluded.driver,
    driver_ids = excluded.driver_ids,
    hook_token = excluded.hook_token,
    started_at = excluded.started_at`,
		r.AgentID, r.PID, r.SessionID, r.Interface, r.TTY, r.Driver, encodeDriverIDs(r.DriverIDs), r.HookToken, formatTime(r.StartedAt),
	)
	if err != nil {
		return fmt.Errorf("state: write running: %w", err)
	}
	return nil
}

// ListRunning returns all running entries.
func (s *Store) ListRunning() ([]RunningEntry, error) {
	rows, err := s.db.Query(`
SELECT agent_id, pid, session_id, interface, tty, driver, driver_ids, hook_token, started_at
FROM running
ORDER BY started_at, agent_id`)
	if err != nil {
		return nil, fmt.Errorf("state: list running: %w", err)
	}
	defer rows.Close()

	out := []RunningEntry{}
	for rows.Next() {
		var r RunningEntry
		var startedAt, driverIDs string
		if err := rows.Scan(&r.AgentID, &r.PID, &r.SessionID, &r.Interface, &r.TTY, &r.Driver, &driverIDs, &r.HookToken, &startedAt); err != nil {
			return nil, fmt.Errorf("state: scan running: %w", err)
		}
		r.DriverIDs = decodeDriverIDs(driverIDs)
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

// ValidateHookToken checks that the given token matches the hook_token in the
// running row for agentID. Returns ErrNotFound if no running row, ErrTokenMismatch
// if the token is wrong.
func (s *Store) ValidateHookToken(agentID, token string) error {
	var stored string
	err := s.db.QueryRow(`SELECT hook_token FROM running WHERE agent_id = ?`, agentID).Scan(&stored)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("state: validate token: %w", err)
	}
	if stored == "" || stored != token {
		return ErrTokenMismatch
	}
	return nil
}

// encodeDriverIDs marshals the driver-id map to a JSON object string for the
// running.driver_ids column. A nil/empty map becomes "{}".
func encodeDriverIDs(ids map[string]string) string {
	if len(ids) == 0 {
		return "{}"
	}
	b, err := json.Marshal(ids)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// decodeDriverIDs parses the driver_ids column; an empty/"{}"/invalid value
// yields a nil map (omitted from the API JSON).
func decodeDriverIDs(s string) map[string]string {
	if s == "" || s == "{}" {
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil
	}
	if len(m) == 0 {
		return nil
	}
	return m
}

// DeleteRunning deletes one running entry.
func (s *Store) DeleteRunning(id string) error {
	if _, err := s.db.Exec(`DELETE FROM running WHERE agent_id = ?`, id); err != nil {
		return fmt.Errorf("state: delete running: %w", err)
	}
	return nil
}
