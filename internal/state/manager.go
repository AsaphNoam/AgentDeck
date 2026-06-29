package state

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
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

var errNoStateChange = errors.New("state: no state change")

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

// ApplyHook validates a per-launch token, applies one lifecycle/status update,
// and publishes the recomputed AgentState.
func (m *Manager) ApplyHook(token string, payload HookPayload) (AgentStateUpdate, error) {
	if strings.TrimSpace(token) == "" {
		return AgentStateUpdate{}, fmt.Errorf("%w: missing token", ErrTokenMismatch)
	}
	if err := validateHookPayload(payload); err != nil {
		return AgentStateUpdate{}, err
	}
	now := timeNow()
	return m.apply(payload.AgentID, func(tx *sql.Tx) error {
		if err := hookAgentExists(tx, payload.AgentID); err != nil {
			return err
		}
		current, err := hookRunning(tx, payload.AgentID)
		if err != nil {
			return err
		}
		if current.HookToken == "" || current.HookToken != token {
			return ErrTokenMismatch
		}

		switch payload.Event {
		case "running":
			pid := payload.PID
			if pid == 0 {
				pid = current.PID
			}
			sessionID := payload.SessionID
			if sessionID == "" {
				sessionID = current.SessionID
			}
			_, err := tx.Exec(`
INSERT INTO running(agent_id, pid, session_id, interface, tty, hook_token, started_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(agent_id) DO UPDATE SET
    pid = excluded.pid,
    session_id = excluded.session_id,
    interface = excluded.interface,
    tty = excluded.tty,
    hook_token = excluded.hook_token,
    started_at = excluded.started_at`,
				payload.AgentID, pid, sessionID, current.Interface, current.TTY, current.HookToken, formatTime(now),
			)
			if err != nil {
				return fmt.Errorf("state: apply running hook: %w", err)
			}
		case "status":
			return applyStatusHook(tx, payload, now)
		case "stopped":
			if _, err := tx.Exec(`DELETE FROM running WHERE agent_id = ?`, payload.AgentID); err != nil {
				return fmt.Errorf("state: apply stopped hook: %w", err)
			}
		case "SessionStart":
			// A terminal CLI's SessionStart hook refreshes the running row's
			// session_id/tty (if the CLI exposed them) before applying the idle
			// status (techspec §4.2, §4.4). The running row itself already exists
			// (the runtime wrote it at launch with the live hook token).
			if err := refreshRunningFromHook(tx, payload, current); err != nil {
				return err
			}
			return applyStatusHook(tx, payload, now)
		case "UserPromptSubmit", "PreToolUse", "PostToolUse", "Stop":
			// Lifecycle hooks from a terminal agent are pure status producers
			// (§4.3). They never clear the running row — Stop fires at the END OF
			// EACH TURN, not on CLI exit; the running row is cleared by the
			// runtime's Stop / the liveness sweep / the explicit "stopped" event.
			return applyStatusHook(tx, payload, now)
		}
		return nil
	})
}

// ApplyStaleCorrection applies a conservative status detail update for a live
// agent only when the current status row predates staleBefore.
func (m *Manager) ApplyStaleCorrection(agentID, detail string, staleBefore time.Time) (AgentStateUpdate, bool, error) {
	if strings.TrimSpace(agentID) == "" {
		return AgentStateUpdate{}, false, fmt.Errorf("%w: agent_id is required", ErrInvalidHook)
	}
	applied := false
	update, err := m.apply(agentID, func(tx *sql.Tx) error {
		if err := hookAgentExists(tx, agentID); err != nil {
			return err
		}
		if _, err := hookRunning(tx, agentID); err != nil {
			if errors.Is(err, ErrTokenMismatch) {
				return ErrNotFound
			}
			return err
		}

		var curState, curDetail, curTrace string
		var curBusy sql.NullString
		var curPct sql.NullFloat64
		var curUpdated sql.NullInt64
		err := tx.QueryRow(`
SELECT state, detail, last_trace, busy_since, context_pct, updated_at
FROM status
WHERE agent_id = ?`, agentID).Scan(&curState, &curDetail, &curTrace, &curBusy, &curPct, &curUpdated)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("state: stale correction read status: %w", err)
		}
		if curUpdated.Valid && curUpdated.Int64 >= staleBefore.UnixMilli() {
			return errNoStateChange
		}
		if curState == "" {
			curState = "idle"
		}
		if strings.TrimSpace(detail) != "" {
			curDetail = detail
		}
		contextPct := 0.0
		if curPct.Valid {
			contextPct = curPct.Float64
		}
		var busySince any
		if curBusy.Valid {
			busySince = curBusy.String
		}
		_, err = tx.Exec(`
INSERT INTO status(agent_id, state, detail, last_trace, busy_since, context_pct, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(agent_id) DO UPDATE SET
    state = excluded.state,
    detail = excluded.detail,
    last_trace = excluded.last_trace,
    busy_since = excluded.busy_since,
    context_pct = excluded.context_pct,
    updated_at = excluded.updated_at`,
			agentID, curState, curDetail, "ReconcileSweep", busySince, contextPct, timeNow().UnixMilli(),
		)
		if err != nil {
			return fmt.Errorf("state: apply stale correction: %w", err)
		}
		applied = true
		return nil
	})
	if err != nil {
		return AgentStateUpdate{}, false, err
	}
	return update, applied, nil
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
		if errors.Is(err, errNoStateChange) {
			return AgentStateUpdate{}, nil
		}
		return AgentStateUpdate{}, err
	}
	if err := tx.Commit(); err != nil {
		return AgentStateUpdate{}, fmt.Errorf("state: commit apply: %w", err)
	}
	return m.recomputeAndPublish(agentID)
}

func validateHookPayload(payload HookPayload) error {
	if strings.TrimSpace(payload.AgentID) == "" {
		return fmt.Errorf("%w: agent_id is required", ErrInvalidHook)
	}
	switch payload.Event {
	case "running":
		if payload.PID < 0 {
			return fmt.Errorf("%w: pid must be positive", ErrInvalidHook)
		}
	case "status", "SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "Stop":
		// "status" is the runtime-internal event; the rest are the terminal CLI
		// lifecycle events (techspec §4.2). All carry an explicit state.
		switch payload.State {
		case "busy", "idle", "waiting_input", "done", "error":
		default:
			return fmt.Errorf("%w: invalid state", ErrInvalidHook)
		}
	case "stopped":
	default:
		return fmt.Errorf("%w: invalid event", ErrInvalidHook)
	}
	if payload.ContextPct != nil && (*payload.ContextPct < 0 || *payload.ContextPct > 1) {
		return fmt.Errorf("%w: context_pct out of range", ErrInvalidHook)
	}
	return nil
}

func hookAgentExists(tx *sql.Tx, agentID string) error {
	var exists int
	err := tx.QueryRow(`SELECT 1 FROM agents WHERE agent_id = ?`, agentID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("state: hook read agent: %w", err)
	}
	return nil
}

func hookRunning(tx *sql.Tx, agentID string) (RunningEntry, error) {
	var r RunningEntry
	var startedAt string
	err := tx.QueryRow(`
SELECT agent_id, pid, session_id, interface, tty, hook_token, started_at
FROM running
WHERE agent_id = ?`, agentID).Scan(
		&r.AgentID, &r.PID, &r.SessionID, &r.Interface, &r.TTY, &r.HookToken, &startedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return RunningEntry{}, ErrTokenMismatch
	}
	if err != nil {
		return RunningEntry{}, fmt.Errorf("state: hook read running: %w", err)
	}
	started, err := parseTime(startedAt)
	if err != nil {
		return RunningEntry{}, wrapTimeErr("running.started_at", err)
	}
	r.StartedAt = started
	return r, nil
}

// refreshRunningFromHook updates the running row's session_id/tty from a
// SessionStart hook when the CLI exposes them, preserving everything else
// (pid/interface/token/started_at). A SessionStart that omits both is a no-op.
func refreshRunningFromHook(tx *sql.Tx, payload HookPayload, current RunningEntry) error {
	sessionID := current.SessionID
	if payload.SessionID != "" {
		sessionID = payload.SessionID
	}
	tty := current.TTY
	if payload.TTY != "" {
		tty = payload.TTY
	}
	if sessionID == current.SessionID && tty == current.TTY {
		return nil
	}
	_, err := tx.Exec(`UPDATE running SET session_id = ?, tty = ? WHERE agent_id = ?`,
		sessionID, tty, payload.AgentID)
	if err != nil {
		return fmt.Errorf("state: refresh running from hook: %w", err)
	}
	return nil
}

func applyStatusHook(tx *sql.Tx, payload HookPayload, now time.Time) error {
	var curState string
	var curBusy sql.NullString
	var curPct sql.NullFloat64
	err := tx.QueryRow(`
SELECT state, busy_since, context_pct
FROM status
WHERE agent_id = ?`, payload.AgentID).Scan(&curState, &curBusy, &curPct)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("state: hook read status: %w", err)
	}

	var busySince any
	if payload.State == "busy" {
		if curState == "busy" && curBusy.Valid {
			busySince = curBusy.String
		} else {
			busySince = formatTime(now)
		}
	}
	contextPct := 0.0
	if curPct.Valid {
		contextPct = curPct.Float64
	}
	if payload.ContextPct != nil {
		contextPct = *payload.ContextPct
	}

	_, err = tx.Exec(`
INSERT INTO status(agent_id, state, detail, last_trace, busy_since, context_pct, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(agent_id) DO UPDATE SET
    state = excluded.state,
    detail = excluded.detail,
    last_trace = excluded.last_trace,
    busy_since = excluded.busy_since,
    context_pct = excluded.context_pct,
    updated_at = excluded.updated_at`,
		payload.AgentID, payload.State, payload.Detail, payload.LastTrace, busySince, contextPct, now.UnixMilli(),
	)
	if err != nil {
		return fmt.Errorf("state: apply status hook: %w", err)
	}
	return nil
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
	if unread, err := m.store.UnreadCount(agentID); err == nil {
		out.UnreadMessages = unread
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
