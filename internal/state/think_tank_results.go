package state

import (
	"database/sql"
	"fmt"
)

// ThinkTankResult is the immutable agent-owned read projection of a judge's
// successfully finalized synthesis. It follows the judge's ordinary history,
// not the room: room deletion keeps it (FS-21.R50, TS-14.R26).
type ThinkTankResult struct {
	ResultID    int64
	AgentID     string
	RoomID      string
	RoomTitle   string
	EntrySeq    int64
	AttemptID   string
	Body        string
	Generation  string
	TurnID      string
	EventSeq    int64
	CompletedAt string
}

func migrateThinkTankResults(tx *sql.Tx) error {
	_, err := tx.Exec(`
CREATE TABLE think_tank_results (
  result_id    INTEGER PRIMARY KEY AUTOINCREMENT,
  agent_id     TEXT NOT NULL REFERENCES agents(agent_id) ON DELETE CASCADE,
  room_id      TEXT NOT NULL,
  room_title   TEXT NOT NULL,
  entry_seq    INTEGER NOT NULL,
  attempt_id   TEXT NOT NULL,
  body         TEXT NOT NULL,
  generation   TEXT NOT NULL,
  turn_id      TEXT NOT NULL,
  event_seq    INTEGER NOT NULL DEFAULT 0,
  completed_at TEXT NOT NULL,
  UNIQUE(room_id, entry_seq)
);
CREATE INDEX idx_think_tank_results_agent ON think_tank_results(agent_id, result_id);
`)
	return err
}

// insertThinkTankResultTx commits the result with the canonical synthesis
// entry. The room/entry key makes a duplicate finalization a no-op; a judge
// whose agent history is already deleted has nowhere to retain it.
func insertThinkTankResultTx(tx *sql.Tx, room ThinkTank, a ThinkTankAttempt, e ThinkTankEntry, eventSeq int64) error {
	if _, err := tx.Exec(`
INSERT INTO think_tank_results(agent_id, room_id, room_title, entry_seq, attempt_id, body, generation, turn_id, event_seq, completed_at)
SELECT ?, ?, ?, ?, ?, ?, ?, ?, ?, ? WHERE EXISTS (SELECT 1 FROM agents WHERE agent_id = ?)
ON CONFLICT(room_id, entry_seq) DO NOTHING`,
		a.AgentID, room.RoomID, room.Title, e.Seq, a.AttemptID, e.Body, a.Generation, a.TurnID, eventSeq,
		formatTime(timeNow()), a.AgentID); err != nil {
		return fmt.Errorf("state: insert think tank result: %w", err)
	}
	return nil
}

// ListThinkTankResults pages an agent's retained synthesis results in order
// after the given result id.
func (s *Store) ListThinkTankResults(agentID string, afterID int64, limit int) ([]ThinkTankResult, error) {
	rows, err := s.db.Query(`
SELECT result_id, agent_id, room_id, room_title, entry_seq, attempt_id, body, generation, turn_id, event_seq, completed_at
FROM think_tank_results WHERE agent_id = ? AND result_id > ? ORDER BY result_id LIMIT ?`, agentID, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("state: list think tank results: %w", err)
	}
	defer rows.Close()
	out := []ThinkTankResult{}
	for rows.Next() {
		var r ThinkTankResult
		if err := rows.Scan(&r.ResultID, &r.AgentID, &r.RoomID, &r.RoomTitle, &r.EntrySeq, &r.AttemptID, &r.Body,
			&r.Generation, &r.TurnID, &r.EventSeq, &r.CompletedAt); err != nil {
			return nil, fmt.Errorf("state: scan think tank result: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
