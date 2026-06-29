package state

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrRecipientNotFound is returned by ResolveRecipient when no live agent
// matches the `to` address (techspec §9).
var ErrRecipientNotFound = errors.New("state: recipient not found")

// ErrAmbiguousRecipient is returned by ResolveRecipient when more than one live
// agent matches the `to` address; the candidates accompany the error so the
// caller can re-address by agent_id (techspec §9).
var ErrAmbiguousRecipient = errors.New("state: ambiguous recipient")

// AmbiguousError carries the candidate agents for an ambiguous `to` resolution.
type AmbiguousError struct {
	Candidates []AgentRef
}

func (e *AmbiguousError) Error() string { return ErrAmbiguousRecipient.Error() }
func (e *AmbiguousError) Unwrap() error { return ErrAmbiguousRecipient }

// address builds the canonical "role@project" addressable form.
func address(role, project string) string { return role + "@" + project }

// LiveAgents returns every currently-running agent (a row in the running
// registry) joined with identity and latest status (techspec §3.2). Agents with
// no status row report state "unknown".
func (s *Store) LiveAgents() ([]LiveAgent, error) {
	rows, err := s.db.Query(`
SELECT a.agent_id, a.name, a.role, a.project,
       COALESCE(st.state, 'unknown'), COALESCE(st.detail, ''), COALESCE(st.context_pct, 0)
FROM running r
JOIN agents a ON a.agent_id = r.agent_id
LEFT JOIN status st ON st.agent_id = r.agent_id
ORDER BY a.name`)
	if err != nil {
		return nil, fmt.Errorf("state: list live agents: %w", err)
	}
	defer rows.Close()

	out := []LiveAgent{}
	for rows.Next() {
		var la LiveAgent
		if err := rows.Scan(&la.AgentID, &la.Name, &la.Role, &la.Project, &la.State, &la.Detail, &la.ContextPct); err != nil {
			return nil, fmt.Errorf("state: scan live agent: %w", err)
		}
		la.Address = address(la.Role, la.Project)
		out = append(out, la)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("state: iterate live agents: %w", err)
	}
	return out, nil
}

// ResolveRecipient resolves a `to` string to a single live agent_id following
// the techspec §3.4 order (exact agent_id, then role@project, then
// case-insensitive name). Only live agents are considered. On more than one
// match it returns an *AmbiguousError (wrapping ErrAmbiguousRecipient); on no
// match, ErrRecipientNotFound.
func (s *Store) ResolveRecipient(to string) (string, []AgentRef, error) {
	to = strings.TrimSpace(to)
	if to == "" {
		return "", nil, ErrRecipientNotFound
	}
	live, err := s.LiveAgents()
	if err != nil {
		return "", nil, err
	}

	// 1. Exact agent_id.
	for _, a := range live {
		if a.AgentID == to {
			return a.AgentID, nil, nil
		}
	}

	// 2. role@project.
	if role, project, ok := strings.Cut(to, "@"); ok {
		var matches []LiveAgent
		for _, a := range live {
			if a.Role == role && a.Project == project {
				matches = append(matches, a)
			}
		}
		if len(matches) == 1 {
			return matches[0].AgentID, nil, nil
		}
		if len(matches) > 1 {
			return "", nil, &AmbiguousError{Candidates: toRefs(matches)}
		}
	}

	// 3. Name (case-insensitive).
	var matches []LiveAgent
	for _, a := range live {
		if strings.EqualFold(a.Name, to) {
			matches = append(matches, a)
		}
	}
	if len(matches) == 1 {
		return matches[0].AgentID, nil, nil
	}
	if len(matches) > 1 {
		return "", nil, &AmbiguousError{Candidates: toRefs(matches)}
	}

	return "", nil, ErrRecipientNotFound
}

func toRefs(agents []LiveAgent) []AgentRef {
	refs := make([]AgentRef, len(agents))
	for i, a := range agents {
		refs[i] = AgentRef{AgentID: a.AgentID, Name: a.Name, Address: a.Address}
	}
	return refs
}

// InsertMessage writes one message row, minting a unique message_id ("m_" + 6
// hex, retrying on the rare collision) and stamping created_at (now if unset),
// read=false, delivered_via="pending" unless the caller set it (techspec §3.2,
// §4.1). Returns the minted message_id.
func (s *Store) InsertMessage(m Message) (string, error) {
	id, err := insertMessageTx(s.db, m)
	if err != nil {
		return "", err
	}
	return id, nil
}

type messageExecer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func insertMessageTx(exec messageExecer, m Message) (string, error) {
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now().UTC()
	}
	if m.DeliveredVia == "" {
		m.DeliveredVia = "pending"
	}
	var inReplyTo sql.NullString
	if m.InReplyTo != "" {
		inReplyTo = sql.NullString{String: m.InReplyTo, Valid: true}
	}
	for i := 0; i < 10; i++ {
		var b [3]byte
		if _, err := randRead(b[:]); err != nil {
			return "", fmt.Errorf("state: read random: %w", err)
		}
		id := "m_" + hex.EncodeToString(b[:])
		_, err := exec.Exec(`
INSERT INTO messages(message_id, from_agent, from_address, from_name, to_agent, subject, body, created_at, read, read_at, delivered_via, in_reply_to)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, NULL, ?, ?)`,
			id, m.FromAgent, m.FromAddress, m.FromName, m.ToAgent, m.Subject, m.Body,
			formatTime(m.CreatedAt), m.DeliveredVia, inReplyTo)
		if err == nil {
			return id, nil
		}
		if isUniqueViolation(err) {
			continue // regenerate on the rare PK collision
		}
		return "", fmt.Errorf("state: insert message: %w", err)
	}
	return "", errors.New("state: could not mint unique message_id after 10 tries")
}

// InsertMessageWithBudget writes one outbound message and increments the
// sender's turn budget in the same transaction. On breach, the message is not
// inserted and the budget row is marked breached.
func (s *Store) InsertMessageWithBudget(m Message, limit int) (string, BudgetStatus, bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return "", BudgetStatus{}, false, fmt.Errorf("state: begin message insert budget: %w", err)
	}
	defer tx.Rollback()

	budget, breached, err := consumeBudgetTx(tx, m.FromAgent, 0, 1, limit)
	if err != nil {
		return "", BudgetStatus{}, false, err
	}
	if breached {
		if err := tx.Commit(); err != nil {
			return "", BudgetStatus{}, false, fmt.Errorf("state: commit message budget breach: %w", err)
		}
		return "", budget, true, nil
	}
	id, err := insertMessageTx(tx, m)
	if err != nil {
		return "", BudgetStatus{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return "", BudgetStatus{}, false, fmt.Errorf("state: commit message insert budget: %w", err)
	}
	return id, budget, false, nil
}

// isUniqueViolation reports whether err is a SQLite UNIQUE/PK constraint error.
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// ListMessages returns the recipient's mailbox ordered by created_at ascending,
// optionally restricted to unread, capped at limit (limit <= 0 means no cap)
// (techspec §3.2, §3.5).
func (s *Store) ListMessages(recipientID string, unreadOnly bool, limit int) ([]Message, error) {
	q := `
SELECT message_id, from_agent, from_address, from_name, to_agent, subject, body, created_at, read, read_at, delivered_via, in_reply_to
FROM messages
WHERE to_agent = ?`
	if unreadOnly {
		q += ` AND read = 0`
	}
	q += ` ORDER BY created_at, message_id`
	args := []any{recipientID}
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("state: list messages: %w", err)
	}
	defer rows.Close()

	out := []Message{}
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("state: iterate messages: %w", err)
	}
	return out, nil
}

func scanMessage(rows *sql.Rows) (Message, error) {
	var m Message
	var createdAt string
	var readAt sql.NullString
	var inReplyTo sql.NullString
	var readInt int
	if err := rows.Scan(&m.MessageID, &m.FromAgent, &m.FromAddress, &m.FromName, &m.ToAgent,
		&m.Subject, &m.Body, &createdAt, &readInt, &readAt, &m.DeliveredVia, &inReplyTo); err != nil {
		return Message{}, fmt.Errorf("state: scan message: %w", err)
	}
	m.Read = readInt != 0
	m.InReplyTo = inReplyTo.String
	var err error
	if m.CreatedAt, err = parseTime(createdAt); err != nil {
		return Message{}, wrapTimeErr("message.created_at", err)
	}
	if m.ReadAt, err = parseOptionalTime(readAt); err != nil {
		return Message{}, wrapTimeErr("message.read_at", err)
	}
	return m, nil
}

// MarkRead flags the given messages read, stamping read_at with now (techspec §3.5).
func (s *Store) MarkRead(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders, args := inClause(ids)
	args = append([]any{formatTime(time.Now().UTC())}, args...)
	q := `UPDATE messages
SET read = 1,
    read_at = ?,
    delivered_via = CASE WHEN delivered_via = 'pending' THEN 'poll' ELSE delivered_via END
WHERE message_id IN (` + placeholders + `)`
	if _, err := s.db.Exec(q, args...); err != nil {
		return fmt.Errorf("state: mark messages read: %w", err)
	}
	return nil
}

// MarkUnreadDeliveredVia stamps unread pending messages for a recipient with a
// delivery mechanism ("nudge" today) without marking them read.
func (s *Store) MarkUnreadDeliveredVia(agentID, via string) (int64, error) {
	res, err := s.db.Exec(`
UPDATE messages
SET delivered_via = ?
WHERE to_agent = ? AND read = 0 AND delivered_via = 'pending'`, via, agentID)
	if err != nil {
		return 0, fmt.Errorf("state: mark messages delivered: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// DeleteMessages removes the given messages (techspec §3.5).
func (s *Store) DeleteMessages(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders, args := inClause(ids)
	q := `DELETE FROM messages WHERE message_id IN (` + placeholders + `)`
	if _, err := s.db.Exec(q, args...); err != nil {
		return fmt.Errorf("state: delete messages: %w", err)
	}
	return nil
}

// UnreadCount returns the number of unread messages addressed to agentID
// (techspec §3.2); drives the card badge and the nudger.
func (s *Store) UnreadCount(agentID string) (int, error) {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE to_agent = ? AND read = 0`, agentID).Scan(&n); err != nil {
		return 0, fmt.Errorf("state: unread count: %w", err)
	}
	return n, nil
}

// ResetTurnBudget starts a new messaging-budget window for an agent turn. It is
// the single source of truth for the agent's current turn: it keeps at most one
// turn_budget row per agent by deleting any other rows in the same transaction.
// This matters across a server restart + resume — agentState.turnSeq resets to
// 0 on a fresh process, so a resumed agent re-emits low turn_ids while the prior
// session's higher-numbered (and possibly breached) rows linger with the highest
// rowids. Without this cleanup, currentBudgetTx's ORDER BY rowid DESC would read
// a stale row and could block send_message/check_messages for the resumed agent.
// Collapsing to one row also caps turn_budget's otherwise unbounded growth.
func (s *Store) ResetTurnBudget(agentID, turnID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("state: begin reset turn budget: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM turn_budget WHERE agent_id = ? AND turn_id <> ?`, agentID, turnID); err != nil {
		return fmt.Errorf("state: prune turn budget rows: %w", err)
	}
	if _, err := tx.Exec(`
INSERT INTO turn_budget(agent_id, turn_id, inbound, outbound, breached)
VALUES (?, ?, 0, 0, 0)
ON CONFLICT(agent_id, turn_id) DO UPDATE SET
    inbound = 0,
    outbound = 0,
    breached = 0`, agentID, turnID); err != nil {
		return fmt.Errorf("state: reset turn budget: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("state: commit reset turn budget: %w", err)
	}
	return nil
}

// BudgetStatus is the current per-turn messaging budget row for an agent.
type BudgetStatus struct {
	AgentID   string
	TurnID    string
	Inbound   int
	Outbound  int
	Breached  bool
	Remaining int
}

// ConsumeTurnBudget atomically increments the latest budget row for agentID by
// inbound/outbound deltas unless that would exceed limit. If no runtime-created
// row exists yet, it creates an implicit t_000000000000 row so direct MCP calls
// in tests/manual sessions still have deterministic accounting.
func (s *Store) ConsumeTurnBudget(agentID string, inboundDelta, outboundDelta, limit int) (BudgetStatus, bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return BudgetStatus{}, false, fmt.Errorf("state: begin budget: %w", err)
	}
	defer tx.Rollback()

	cur, breached, err := consumeBudgetTx(tx, agentID, inboundDelta, outboundDelta, limit)
	if err != nil {
		return BudgetStatus{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return BudgetStatus{}, false, fmt.Errorf("state: commit budget: %w", err)
	}
	return cur, breached, nil
}

func consumeBudgetTx(tx *sql.Tx, agentID string, inboundDelta, outboundDelta, limit int) (BudgetStatus, bool, error) {
	cur, err := currentBudgetTx(tx, agentID, limit)
	if err != nil {
		return BudgetStatus{}, false, err
	}
	used := cur.Inbound + cur.Outbound
	nextUsed := used + inboundDelta + outboundDelta
	if nextUsed > limit {
		if _, err := tx.Exec(`UPDATE turn_budget SET breached = 1 WHERE agent_id = ? AND turn_id = ?`, agentID, cur.TurnID); err != nil {
			return BudgetStatus{}, false, fmt.Errorf("state: mark budget breached: %w", err)
		}
		cur.Breached = true
		cur.Remaining = max(0, limit-used)
		return cur, true, nil
	}
	cur.Inbound += inboundDelta
	cur.Outbound += outboundDelta
	cur.Remaining = max(0, limit-nextUsed)
	if _, err := tx.Exec(`
UPDATE turn_budget SET inbound = ?, outbound = ?
WHERE agent_id = ? AND turn_id = ?`, cur.Inbound, cur.Outbound, agentID, cur.TurnID); err != nil {
		return BudgetStatus{}, false, fmt.Errorf("state: update budget: %w", err)
	}
	return cur, false, nil
}

// CurrentTurnBudget returns the latest budget row for agentID, creating the same
// implicit row used by ConsumeTurnBudget if none exists yet.
func (s *Store) CurrentTurnBudget(agentID string, limit int) (BudgetStatus, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return BudgetStatus{}, fmt.Errorf("state: begin budget read: %w", err)
	}
	defer tx.Rollback()
	cur, err := currentBudgetTx(tx, agentID, limit)
	if err != nil {
		return BudgetStatus{}, err
	}
	if err := tx.Commit(); err != nil {
		return BudgetStatus{}, fmt.Errorf("state: commit budget read: %w", err)
	}
	return cur, nil
}

func currentBudgetTx(tx *sql.Tx, agentID string, limit int) (BudgetStatus, error) {
	var cur BudgetStatus
	var breached int
	err := tx.QueryRow(`
SELECT agent_id, turn_id, inbound, outbound, breached
FROM turn_budget
WHERE agent_id = ?
ORDER BY rowid DESC
LIMIT 1`, agentID).Scan(&cur.AgentID, &cur.TurnID, &cur.Inbound, &cur.Outbound, &breached)
	if errors.Is(err, sql.ErrNoRows) {
		cur = BudgetStatus{AgentID: agentID, TurnID: "t_000000000000"}
		if _, err := tx.Exec(`
INSERT INTO turn_budget(agent_id, turn_id, inbound, outbound, breached)
VALUES (?, ?, 0, 0, 0)`, cur.AgentID, cur.TurnID); err != nil {
			return BudgetStatus{}, fmt.Errorf("state: create implicit budget: %w", err)
		}
	} else if err != nil {
		return BudgetStatus{}, fmt.Errorf("state: read budget: %w", err)
	}
	cur.Breached = breached != 0
	cur.Remaining = max(0, limit-cur.Inbound-cur.Outbound)
	return cur, nil
}

// TakeMessagesWithBudget returns and optionally marks/deletes messages while
// incrementing the inbound turn budget in one transaction.
func (s *Store) TakeMessagesWithBudget(recipientID string, unreadOnly bool, limit, budgetLimit int, markRead, deleteAfter bool) ([]Message, BudgetStatus, bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, BudgetStatus{}, false, fmt.Errorf("state: begin message take budget: %w", err)
	}
	defer tx.Rollback()

	budget, err := currentBudgetTx(tx, recipientID, budgetLimit)
	if err != nil {
		return nil, BudgetStatus{}, false, err
	}
	if limit > budget.Remaining {
		limit = budget.Remaining
	}

	msgs := []Message{}
	if limit > 0 {
		q := `
SELECT message_id, from_agent, from_address, from_name, to_agent, subject, body, created_at, read, read_at, delivered_via, in_reply_to
FROM messages
WHERE to_agent = ?`
		if unreadOnly {
			q += ` AND read = 0`
		}
		q += ` ORDER BY created_at, message_id LIMIT ?`
		rows, err := tx.Query(q, recipientID, limit)
		if err != nil {
			return nil, BudgetStatus{}, false, fmt.Errorf("state: list messages for budget: %w", err)
		}
		for rows.Next() {
			m, err := scanMessage(rows)
			if err != nil {
				rows.Close()
				return nil, BudgetStatus{}, false, err
			}
			msgs = append(msgs, m)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, BudgetStatus{}, false, fmt.Errorf("state: iterate messages for budget: %w", err)
		}
		rows.Close()
	}

	budget, breached, err := consumeBudgetTx(tx, recipientID, len(msgs), 0, budgetLimit)
	if err != nil {
		return nil, BudgetStatus{}, false, err
	}
	if !breached && len(msgs) > 0 {
		ids := make([]string, len(msgs))
		for i, m := range msgs {
			ids[i] = m.MessageID
		}
		placeholders, args := inClause(ids)
		if deleteAfter {
			if _, err := tx.Exec(`DELETE FROM messages WHERE message_id IN (`+placeholders+`)`, args...); err != nil {
				return nil, BudgetStatus{}, false, fmt.Errorf("state: delete messages with budget: %w", err)
			}
		} else if markRead {
			args = append([]any{formatTime(time.Now().UTC())}, args...)
			q := `UPDATE messages
SET read = 1,
    read_at = ?,
    delivered_via = CASE WHEN delivered_via = 'pending' THEN 'poll' ELSE delivered_via END
WHERE message_id IN (` + placeholders + `)`
			if _, err := tx.Exec(q, args...); err != nil {
				return nil, BudgetStatus{}, false, fmt.Errorf("state: mark messages read with budget: %w", err)
			}
		}
	}
	if budget.Remaining == 0 && len(msgs) == 0 {
		budget, breached, err = consumeBudgetTx(tx, recipientID, 1, 0, budgetLimit)
		if err != nil {
			return nil, BudgetStatus{}, false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, BudgetStatus{}, false, fmt.Errorf("state: commit message take budget: %w", err)
	}
	if breached {
		msgs = nil
	}
	return msgs, budget, breached, nil
}

// DeleteExpiredMessages applies the Phase 5 retention policy. It deletes read
// messages older than readTTL and any message older than hardTTL.
func (s *Store) DeleteExpiredMessages(now time.Time, readTTL, hardTTL time.Duration) (readDeleted, hardDeleted int64, err error) {
	readCutoff := formatTime(now.UTC().Add(-readTTL))
	hardCutoff := formatTime(now.UTC().Add(-hardTTL))
	res, err := s.db.Exec(`DELETE FROM messages WHERE read = 1 AND read_at IS NOT NULL AND read_at < ?`, readCutoff)
	if err != nil {
		return 0, 0, fmt.Errorf("state: delete read expired messages: %w", err)
	}
	readDeleted, _ = res.RowsAffected()
	res, err = s.db.Exec(`DELETE FROM messages WHERE created_at < ?`, hardCutoff)
	if err != nil {
		return readDeleted, 0, fmt.Errorf("state: delete hard expired messages: %w", err)
	}
	hardDeleted, _ = res.RowsAffected()
	return readDeleted, hardDeleted, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// inClause builds an "?, ?, ..." placeholder string and the matching args slice.
func inClause(ids []string) (string, []any) {
	ph := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		ph[i] = "?"
		args[i] = id
	}
	return strings.Join(ph, ", "), args
}
