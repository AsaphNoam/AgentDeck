package state

import (
	"database/sql"
	"errors"
	"fmt"
)

// ReadStatus returns one live status row by agent id.
func (s *Store) ReadStatus(id string) (Status, error) {
	var st Status
	var busySince sql.NullString
	err := s.db.QueryRow(`
SELECT agent_id, state, detail, last_trace, busy_since, context_pct
FROM status
WHERE agent_id = ?`, id).Scan(
		&st.AgentID, &st.State, &st.Detail, &st.LastTrace, &busySince, &st.ContextPct,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Status{}, ErrNotFound
	}
	if err != nil {
		return Status{}, fmt.Errorf("state: read status: %w", err)
	}
	st.BusySince, err = parseOptionalTime(busySince)
	if err != nil {
		return Status{}, wrapTimeErr("status.busy_since", err)
	}
	return st, nil
}

// WriteStatus inserts or updates a live status row.
func (s *Store) WriteStatus(st Status) error {
	_, err := s.db.Exec(`
INSERT INTO status(agent_id, state, detail, last_trace, busy_since, context_pct)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(agent_id) DO UPDATE SET
    state = excluded.state,
    detail = excluded.detail,
    last_trace = excluded.last_trace,
    busy_since = excluded.busy_since,
    context_pct = excluded.context_pct`,
		st.AgentID, st.State, st.Detail, st.LastTrace, formatOptionalTime(st.BusySince), st.ContextPct,
	)
	if err != nil {
		return fmt.Errorf("state: write status: %w", err)
	}
	return nil
}

// ListStatus returns all status rows.
func (s *Store) ListStatus() ([]Status, error) {
	rows, err := s.db.Query(`
SELECT agent_id, state, detail, last_trace, busy_since, context_pct
FROM status
ORDER BY agent_id`)
	if err != nil {
		return nil, fmt.Errorf("state: list status: %w", err)
	}
	defer rows.Close()

	out := []Status{}
	for rows.Next() {
		var st Status
		var busySince sql.NullString
		if err := rows.Scan(&st.AgentID, &st.State, &st.Detail, &st.LastTrace, &busySince, &st.ContextPct); err != nil {
			return nil, fmt.Errorf("state: scan status: %w", err)
		}
		st.BusySince, err = parseOptionalTime(busySince)
		if err != nil {
			return nil, wrapTimeErr("status.busy_since", err)
		}
		out = append(out, st)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("state: iterate status: %w", err)
	}
	return out, nil
}

// DeleteStatus deletes one live status row.
func (s *Store) DeleteStatus(id string) error {
	if _, err := s.db.Exec(`DELETE FROM status WHERE agent_id = ?`, id); err != nil {
		return fmt.Errorf("state: delete status: %w", err)
	}
	return nil
}
