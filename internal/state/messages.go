package state

import (
	"database/sql"
	"errors"
	"fmt"
)

// ReadMessage reads one message by id.
func (s *Store) ReadMessage(id int64) (Message, error) {
	var m Message
	var createdAt string
	var readAt sql.NullString
	err := s.db.QueryRow(`
SELECT id, from_agent, to_agent, body, created_at, read_at
FROM messages
WHERE id = ?`, id).Scan(&m.ID, &m.FromAgent, &m.ToAgent, &m.Body, &createdAt, &readAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Message{}, ErrNotFound
	}
	if err != nil {
		return Message{}, fmt.Errorf("state: read message %d: %w", id, err)
	}
	m.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return Message{}, wrapTimeErr("message.created_at", err)
	}
	m.ReadAt, err = parseOptionalTime(readAt)
	if err != nil {
		return Message{}, wrapTimeErr("message.read_at", err)
	}
	return m, nil
}

// WriteMessage inserts a message and returns its id.
func (s *Store) WriteMessage(m Message) (int64, error) {
	res, err := s.db.Exec(`
INSERT INTO messages(from_agent, to_agent, body, created_at, read_at)
VALUES (?, ?, ?, ?, ?)`,
		m.FromAgent, m.ToAgent, m.Body, formatTime(m.CreatedAt), formatOptionalTime(m.ReadAt),
	)
	if err != nil {
		return 0, fmt.Errorf("state: write message: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("state: message id: %w", err)
	}
	return id, nil
}

// ListMessages returns messages addressed to toAgent.
func (s *Store) ListMessages(toAgent string) ([]Message, error) {
	rows, err := s.db.Query(`
SELECT id, from_agent, to_agent, body, created_at, read_at
FROM messages
WHERE to_agent = ?
ORDER BY created_at, id`, toAgent)
	if err != nil {
		return nil, fmt.Errorf("state: list messages: %w", err)
	}
	defer rows.Close()

	out := []Message{}
	for rows.Next() {
		var m Message
		var createdAt string
		var readAt sql.NullString
		if err := rows.Scan(&m.ID, &m.FromAgent, &m.ToAgent, &m.Body, &createdAt, &readAt); err != nil {
			return nil, fmt.Errorf("state: scan message: %w", err)
		}
		m.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, wrapTimeErr("message.created_at", err)
		}
		m.ReadAt, err = parseOptionalTime(readAt)
		if err != nil {
			return nil, wrapTimeErr("message.read_at", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("state: iterate messages: %w", err)
	}
	return out, nil
}

// DeleteMessage deletes one message.
func (s *Store) DeleteMessage(id int64) error {
	if _, err := s.db.Exec(`DELETE FROM messages WHERE id = ?`, id); err != nil {
		return fmt.Errorf("state: delete message: %w", err)
	}
	return nil
}
