package state

import (
	"database/sql"
	"errors"
	"fmt"
)

const statusColumns = `agent_id, state, detail, last_trace, busy_since, context_pct, context_used, context_size, updated_at`

func scanStatus(row rowScanner) (Status, error) {
	var st Status
	var busySince sql.NullString
	var used, size sql.NullInt64
	if err := row.Scan(&st.AgentID, &st.State, &st.Detail, &st.LastTrace, &busySince, &st.ContextPct, &used, &size, &st.UpdatedAt); err != nil {
		return Status{}, err
	}
	st.ContextCounts = contextCountsFromColumns(used, size)
	var err error
	st.BusySince, err = parseOptionalTime(busySince)
	if err != nil {
		return Status{}, wrapTimeErr("status.busy_since", err)
	}
	return st, nil
}

// ReadStatus returns one live status row by agent id.
func (s *Store) ReadStatus(id string) (Status, error) {
	st, err := scanStatus(s.db.QueryRow(`SELECT `+statusColumns+` FROM status WHERE agent_id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Status{}, ErrNotFound
	}
	if err != nil {
		return Status{}, fmt.Errorf("state: read status: %w", err)
	}
	return st, nil
}

// WriteStatus inserts or updates a live status row. The percentage and the
// optional exact pair are written together (TS-02.R39).
func (s *Store) WriteStatus(st Status) error {
	if st.UpdatedAt == 0 {
		st.UpdatedAt = timeNow().UnixMilli()
	}
	used, size := contextCountsColumns(st.ContextCounts)
	_, err := s.db.Exec(`
INSERT INTO status(`+statusColumns+`)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(agent_id) DO UPDATE SET
    state = excluded.state,
    detail = excluded.detail,
    last_trace = excluded.last_trace,
    busy_since = excluded.busy_since,
    context_pct = excluded.context_pct,
    context_used = excluded.context_used,
    context_size = excluded.context_size,
    updated_at = excluded.updated_at`,
		st.AgentID, st.State, st.Detail, st.LastTrace, formatOptionalTime(st.BusySince), st.ContextPct, used, size, st.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("state: write status: %w", err)
	}
	return nil
}

// ListStatus returns all status rows.
func (s *Store) ListStatus() ([]Status, error) {
	rows, err := s.db.Query(`SELECT ` + statusColumns + ` FROM status ORDER BY agent_id`)
	if err != nil {
		return nil, fmt.Errorf("state: list status: %w", err)
	}
	defer rows.Close()

	out := []Status{}
	for rows.Next() {
		st, err := scanStatus(rows)
		if err != nil {
			return nil, fmt.Errorf("state: scan status: %w", err)
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
