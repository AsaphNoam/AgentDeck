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
		_, err := s.db.Exec(`
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
	q := `UPDATE messages SET read = 1, read_at = ? WHERE message_id IN (` + placeholders + `)`
	if _, err := s.db.Exec(q, args...); err != nil {
		return fmt.Errorf("state: mark messages read: %w", err)
	}
	return nil
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
