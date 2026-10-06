package state

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ThinkTankActivity is one normalized room-turn record copied into room-owned
// storage as it arrives, with frozen actor/project/cwd provenance, so tools,
// diffs and approvals outlive the source agent (TS-14.R10, R12).
type ThinkTankActivity struct {
	RoomID     string
	Seq        int64
	AttemptID  string
	AgentID    string
	AgentName  string
	Project    string
	Cwd        string
	Generation string
	TurnID     string
	SourceSeq  int64
	Payload    string
	Truncated  bool
	CreatedAt  time.Time
}

// thinkTankTruncatedPayload replaces an over-limit record while keeping its
// identity and anchors for display.
func thinkTankTruncatedPayload(eventType string) string {
	return fmt.Sprintf(`{"type":%q,"truncated":true}`, eventType)
}

// AppendThinkTankActivity stores one record for a still-running attempt. A
// record over the 8 MiB ceiling, or past the attempt's 64 MiB budget, keeps
// an explicit truncation marker instead (TS-14.R17). ErrThinkTankStale means
// the attempt already finished; late activity cannot reopen it.
func (s *Store) AppendThinkTankActivity(a ThinkTankActivity, eventType string) (ThinkTankActivity, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return ThinkTankActivity{}, fmt.Errorf("state: begin think tank activity: %w", err)
	}
	defer tx.Rollback()
	var state string
	var used int64
	if err := tx.QueryRow(`SELECT state, activity_bytes FROM think_tank_attempts WHERE attempt_id = ? AND room_id = ?`,
		a.AttemptID, a.RoomID).Scan(&state, &used); errors.Is(err, sql.ErrNoRows) {
		return ThinkTankActivity{}, ErrThinkTankStale
	} else if err != nil {
		return ThinkTankActivity{}, fmt.Errorf("state: read think tank attempt: %w", err)
	}
	if state != ThinkTankAttemptRunning {
		return ThinkTankActivity{}, ErrThinkTankStale
	}
	if len(a.Payload) > ThinkTankMaxActivityBytes || used+int64(len(a.Payload)) > ThinkTankMaxAttemptBytes {
		a.Payload, a.Truncated = thinkTankTruncatedPayload(eventType), true
	}
	if _, err := tx.Exec(`UPDATE think_tank_attempts SET activity_bytes = activity_bytes + ? WHERE attempt_id = ?`,
		len(a.Payload), a.AttemptID); err != nil {
		return ThinkTankActivity{}, fmt.Errorf("state: count think tank activity: %w", err)
	}
	if err := tx.QueryRow(`SELECT COALESCE(MAX(seq), 0) + 1 FROM think_tank_activity WHERE room_id = ?`, a.RoomID).Scan(&a.Seq); err != nil {
		return ThinkTankActivity{}, fmt.Errorf("state: next think tank activity seq: %w", err)
	}
	a.CreatedAt = timeNow()
	if _, err := tx.Exec(`
INSERT INTO think_tank_activity(room_id, seq, attempt_id, agent_id, agent_name, project, cwd, generation,
  turn_id, source_seq, payload, truncated, created_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.RoomID, a.Seq, a.AttemptID, a.AgentID, a.AgentName, a.Project, a.Cwd, a.Generation, a.TurnID,
		a.SourceSeq, a.Payload, a.Truncated, formatTime(a.CreatedAt)); err != nil {
		return ThinkTankActivity{}, fmt.Errorf("state: insert think tank activity: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return ThinkTankActivity{}, fmt.Errorf("state: commit think tank activity: %w", err)
	}
	return a, nil
}

// ListThinkTankActivity returns visible activity after afterSeq. Activity of
// an opening still withheld behind the barrier is not offered (TS-14.R16).
func (s *Store) ListThinkTankActivity(roomID string, afterSeq int64, limit int) ([]ThinkTankActivity, error) {
	return s.listThinkTankActivity(roomID, afterSeq, limit)
}

// ThinkTankActivityWindowStart identifies the newest visible window without
// loading all of its payloads. The returned sequence is excluded by List.
func (s *Store) ThinkTankActivityWindowStart(roomID string, limit int) (int64, bool, error) {
	var after int64
	err := s.db.QueryRow(`SELECT v.seq FROM think_tank_activity v
JOIN think_tank_attempts a ON a.attempt_id = v.attempt_id
WHERE v.room_id = ? AND NOT (a.turn = 'opening' AND a.state != 'finalized')
ORDER BY v.seq DESC LIMIT 1 OFFSET ?`, roomID, limit).Scan(&after)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("state: recent think tank window: %w", err)
	}
	return after, true, nil
}

func (s *Store) listThinkTankActivity(roomID string, afterSeq int64, limit int) ([]ThinkTankActivity, error) {
	rows, err := s.db.Query(`
SELECT v.room_id, v.seq, v.attempt_id, v.agent_id, v.agent_name, v.project, v.cwd, v.generation, v.turn_id,
  v.source_seq, v.payload, v.truncated, v.created_at
FROM think_tank_activity v JOIN think_tank_attempts a ON a.attempt_id = v.attempt_id
WHERE v.room_id = ? AND v.seq > ? AND NOT (a.turn = 'opening' AND a.state != 'finalized')
ORDER BY v.seq LIMIT ?`, roomID, afterSeq, limit)
	if err != nil {
		return nil, fmt.Errorf("state: list think tank activity: %w", err)
	}
	out := []ThinkTankActivity{}
	for rows.Next() {
		var a ThinkTankActivity
		var created string
		if err := rows.Scan(&a.RoomID, &a.Seq, &a.AttemptID, &a.AgentID, &a.AgentName, &a.Project, &a.Cwd,
			&a.Generation, &a.TurnID, &a.SourceSeq, &a.Payload, &a.Truncated, &created); err != nil {
			rows.Close()
			return nil, fmt.Errorf("state: scan think tank activity: %w", err)
		}
		if a.CreatedAt, err = parseTime(created); err != nil {
			rows.Close()
			return nil, wrapTimeErr("think_tank_activity.created_at", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("state: iterate think tank activity: %w", err)
	}
	rows.Close()
	return out, nil
}

// ResolveThinkTankMember returns the room an agent may read: the explicit
// room when the caller is a recorded member, or the room of its current
// attempt when roomID is empty (TS-14.R11).
func (s *Store) ResolveThinkTankMember(callerAgentID, roomID string) (string, error) {
	if roomID == "" {
		err := s.db.QueryRow(`SELECT room_id FROM think_tank_attempts WHERE agent_id = ? AND state = ?
ORDER BY created_at DESC LIMIT 1`, callerAgentID, ThinkTankAttemptRunning).Scan(&roomID)
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrThinkTankNoTurn
		} else if err != nil {
			return "", fmt.Errorf("state: resolve think tank turn: %w", err)
		}
	}
	var one int
	err := s.db.QueryRow(`SELECT 1 FROM think_tank_members WHERE room_id = ? AND agent_id = ? LIMIT 1`, roomID, callerAgentID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrThinkTankNotMember
	} else if err != nil {
		return "", fmt.Errorf("state: read think tank membership: %w", err)
	}
	return roomID, nil
}

// ThinkTankSource is one participant workspace context recorded with room
// activity. Its id is an opaque room-owned reference (TS-14 §3).
type ThinkTankSource struct {
	SourceID   string
	AgentID    string
	AgentName  string
	Project    string
	Cwd        string
	AttemptIDs []string
}

// ThinkTankSources lists the distinct recorded workspace contexts.
func (s *Store) ThinkTankSources(roomID string) ([]ThinkTankSource, error) {
	rows, err := s.db.Query(`
SELECT MIN(seq), agent_id, agent_name, project, cwd, GROUP_CONCAT(DISTINCT attempt_id) FROM think_tank_activity
WHERE room_id = ? GROUP BY agent_id, cwd ORDER BY MIN(seq)`, roomID)
	if err != nil {
		return nil, fmt.Errorf("state: list think tank sources: %w", err)
	}
	out := []ThinkTankSource{}
	for rows.Next() {
		var src ThinkTankSource
		var seq int64
		var attempts string
		if err := rows.Scan(&seq, &src.AgentID, &src.AgentName, &src.Project, &src.Cwd, &attempts); err != nil {
			rows.Close()
			return nil, fmt.Errorf("state: scan think tank source: %w", err)
		}
		src.SourceID = fmt.Sprintf("src_%d", seq)
		src.AttemptIDs = strings.Split(attempts, ",")
		out = append(out, src)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("state: iterate think tank sources: %w", err)
	}
	rows.Close()
	return out, nil
}
