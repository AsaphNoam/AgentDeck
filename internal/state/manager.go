package state

import (
	"database/sql"
	"errors"
	"fmt"
	"sync"
)

// StatePublisher receives effective-state updates from Manager. Subphase 2.3
// wires this to the SSE bus; tests and 2.1 use a stub implementation.
type StatePublisher interface {
	PublishStateUpdate(AgentStateUpdate)
}

// Manager serializes state mutations and emits dashboard-ready AgentState
// updates after each committed change.
type Manager struct {
	store     *Store
	publisher StatePublisher

	writeMu sync.Mutex
	knownMu sync.Mutex
	known   map[string]bool
}

// NewManager wraps an opened Store. publisher may be nil.
func NewManager(store *Store, publisher StatePublisher) *Manager {
	return &Manager{
		store:     store,
		publisher: publisher,
		known:     map[string]bool{},
	}
}

// Store returns the underlying typed SQLite store for existing callers.
func (m *Manager) Store() *Store {
	if m == nil {
		return nil
	}
	return m.store
}

// Start scans all known identities and publishes their effective state.
func (m *Manager) Start() error {
	if m == nil || m.store == nil {
		return errors.New("state: manager has no store")
	}
	rows, err := m.store.db.Query(`SELECT agent_id FROM agents ORDER BY created_at, agent_id`)
	if err != nil {
		return fmt.Errorf("state: manager startup scan: %w", err)
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("state: manager startup scan row: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("state: manager startup scan close: %w", err)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("state: manager startup scan rows: %w", err)
	}
	for _, id := range ids {
		if _, err := m.recomputeAndPublish(id); err != nil {
			return err
		}
	}
	return nil
}

// Touch recomputes and publishes the current effective state for agentID.
func (m *Manager) Touch(agentID string) (AgentStateUpdate, error) {
	return m.recomputeAndPublish(agentID)
}

func (m *Manager) apply(agentID string, mutate func(*sql.Tx) error) (AgentStateUpdate, error) {
	if m == nil || m.store == nil {
		return AgentStateUpdate{}, errors.New("state: manager has no store")
	}
	m.writeMu.Lock()
	defer m.writeMu.Unlock()

	tx, err := m.store.db.Begin()
	if err != nil {
		return AgentStateUpdate{}, fmt.Errorf("state: begin apply: %w", err)
	}
	if err := mutate(tx); err != nil {
		_ = tx.Rollback()
		return AgentStateUpdate{}, err
	}
	if err := tx.Commit(); err != nil {
		return AgentStateUpdate{}, fmt.Errorf("state: commit apply: %w", err)
	}
	return m.recomputeAndPublish(agentID)
}

func (m *Manager) recomputeAndPublish(agentID string) (AgentStateUpdate, error) {
	update, err := m.recompute(agentID)
	if err != nil {
		return AgentStateUpdate{}, err
	}
	if update.AgentID == "" {
		return AgentStateUpdate{}, nil
	}
	if m.publisher != nil {
		m.publisher.PublishStateUpdate(update)
	}
	return update, nil
}

func (m *Manager) recompute(agentID string) (AgentStateUpdate, error) {
	now := timeNow().UnixMilli()
	row := m.store.db.QueryRow(`
SELECT
    a.agent_id, a.name, a.role, a.project, a.backend, a.model, a.interface, a.grp, a.created_at,
    r.pid, r.session_id, r.started_at,
    st.state, st.detail, st.last_trace, st.busy_since, st.context_pct
FROM agents a
LEFT JOIN running r ON r.agent_id = a.agent_id
LEFT JOIN status st ON st.agent_id = a.agent_id
WHERE a.agent_id = ?`, agentID)

	var out AgentState
	var pid sql.NullInt64
	var sessionID, startedAt sql.NullString
	var state, detail, lastTrace, busySince sql.NullString
	var contextPct sql.NullFloat64
	err := row.Scan(
		&out.AgentID, &out.Name, &out.Role, &out.Project, &out.Backend, &out.Model,
		&out.Interface, &out.Group, &out.CreatedAt,
		&pid, &sessionID, &startedAt,
		&state, &detail, &lastTrace, &busySince, &contextPct,
	)
	if errors.Is(err, sql.ErrNoRows) {
		if !m.isKnown(agentID) {
			return AgentStateUpdate{}, nil
		}
		m.setKnown(agentID, false)
		return AgentStateUpdate{AgentState: AgentState{AgentID: agentID, UpdatedAt: now}, Removed: true}, nil
	}
	if err != nil {
		return AgentStateUpdate{}, fmt.Errorf("state: recompute agent: %w", err)
	}

	out.UpdatedAt = now
	out.State = "unknown"
	if pid.Valid {
		out.Running = true
		out.PID = int(pid.Int64)
	}
	if sessionID.Valid {
		out.SessionID = sessionID.String
	}
	if startedAt.Valid {
		out.StartedAt = startedAt.String
	}
	if state.Valid && state.String != "" {
		out.State = state.String
	}
	if detail.Valid {
		out.Detail = detail.String
	}
	if lastTrace.Valid {
		out.LastTrace = lastTrace.String
	}
	if busySince.Valid {
		out.BusySince = busySince.String
	}
	if contextPct.Valid {
		out.ContextPct = contextPct.Float64
	}

	m.setKnown(agentID, true)
	return AgentStateUpdate{AgentState: out}, nil
}

func (m *Manager) isKnown(agentID string) bool {
	m.knownMu.Lock()
	defer m.knownMu.Unlock()
	return m.known[agentID]
}

func (m *Manager) setKnown(agentID string, known bool) {
	m.knownMu.Lock()
	defer m.knownMu.Unlock()
	if known {
		m.known[agentID] = true
		return
	}
	delete(m.known, agentID)
}
