package index

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/transcript"
)

// Reindex wipes and rebuilds the archive index (sessions, sessions_fts,
// tracked_files, tracked_commands) from raw transcript.ndjson files.
// It is NOT safe to run while the server is live: resetTables deletes all
// sessions rows outside the replay transaction, so any agents active during
// the wipe are permanently lost from the index. Always stop the server before
// running reindex.
func Reindex(home string, db *sql.DB) error {
	if home == "" {
		return fmt.Errorf("index: home is required")
	}
	if err := resetTables(db); err != nil {
		return err
	}
	root := filepath.Join(home, "sessions")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("index: read sessions dir: %w", err)
	}
	ix := New(db)
	var failures []error
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		agentID := entry.Name()
		// Isolate each agent: a bad transcript (unreadable, or an OnEvent/
		// OnTurnEnd/flush error) is captured and skipped so the remaining
		// agents are still reindexed rather than leaving the archive wiped.
		if err := reindexAgent(ix, root, agentID); err != nil {
			failures = append(failures, err)
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("index: reindex skipped %d agent(s): %w", len(failures), errors.Join(failures...))
	}
	return nil
}

// reindexAgent replays one agent's transcript into the index. Any error is
// returned (not fatal to the whole reindex) so the caller can continue.
func reindexAgent(ix *Indexer, root, agentID string) error {
	path := filepath.Join(root, agentID, "transcript.ndjson")
	events, err := transcript.NewReader(path).ReadAll(transcript.ReadOptions{IncludeMeta: true})
	if err != nil {
		return fmt.Errorf("index: replay %s: %w", agentID, err)
	}
	var lastSeq int64
	var lastContext float64
	var updatedAt string
	var sawTurnEnd bool
	for _, ev := range events {
		if err := ix.OnEvent(agentID, ev); err != nil {
			return fmt.Errorf("index: event %s seq %d: %w", agentID, ev.Seq, err)
		}
		if ev.Seq > lastSeq {
			lastSeq = ev.Seq
			updatedAt = ev.Ts
		}
		if ev.Type == runtime.EvTurnEnd {
			var d runtime.TurnEndData
			_ = json.Unmarshal(ev.Data, &d)
			lastContext = d.ContextPct
			sawTurnEnd = true
			if err := ix.OnTurnEnd(agentID, runtime.TurnRollup{LastSeq: ev.Seq, LastContextPct: d.ContextPct, UpdatedAt: ev.Ts}); err != nil {
				return fmt.Errorf("index: turn_end %s seq %d: %w", agentID, ev.Seq, err)
			}
		}
	}
	if lastSeq > 0 && updatedAt != "" && !sawTurnEnd {
		if err := ix.flush(agentID, runtime.TurnRollup{LastSeq: lastSeq, LastContextPct: lastContext, UpdatedAt: updatedAt}, false); err != nil {
			return fmt.Errorf("index: final flush %s: %w", agentID, err)
		}
	}
	return nil
}

func resetTables(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("index: begin reset: %w", err)
	}
	defer tx.Rollback()
	for _, stmt := range []string{
		`DELETE FROM sessions_fts`,
		`DELETE FROM tracked_commands`,
		`DELETE FROM tracked_files`,
		`DELETE FROM sessions`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("index: reset tables: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("index: commit reset: %w", err)
	}
	return nil
}
