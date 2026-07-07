package state

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

func migrate(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("state: begin migration: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL
)`); err != nil {
		return fmt.Errorf("state: create schema_migrations: %w", err)
	}

	applied := map[int]bool{}
	rows, err := tx.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("state: read schema_migrations: %w", err)
	}
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			rows.Close()
			return fmt.Errorf("state: scan schema_migrations: %w", err)
		}
		applied[version] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("state: iterate schema_migrations: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("state: close schema_migrations rows: %w", err)
	}

	for _, m := range migrations {
		if applied[m.version] {
			continue
		}
		if _, err := tx.Exec(m.sql); err != nil {
			return fmt.Errorf("state: apply migration %04d: %w", m.version, err)
		}
		if _, err := tx.Exec(
			`INSERT INTO schema_migrations(version, applied_at) VALUES (?, ?)`,
			m.version,
			formatTime(time.Now().UTC()),
		); err != nil {
			return fmt.Errorf("state: record migration %04d: %w", m.version, err)
		}
	}

	if err := ensureSessionsFTS(tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("state: commit migration: %w", err)
	}

	// Guard against running an older binary against a schema written by a newer one.
	// Derived from the migrations slice itself so a new migration can never forget
	// to bump the floor (a hand-maintained constant once risked self-bricking).
	latestKnownMigration := migrations[len(migrations)-1].version
	var maxApplied int
	if err := db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&maxApplied); err != nil {
		return fmt.Errorf("state: check max migration: %w", err)
	}
	if maxApplied > latestKnownMigration {
		return fmt.Errorf("state: database was created by a newer binary (migration %d > %d known); upgrade agentdeck", maxApplied, latestKnownMigration)
	}
	return nil
}

func ensureSessionsFTS(tx *sql.Tx) error {
	// If a plain fallback sessions_fts (from a prior non-FTS5 build) exists and an
	// FTS5-capable binary is now running, drop it so the CREATE VIRTUAL below can
	// upgrade it in place — otherwise `CREATE VIRTUAL TABLE IF NOT EXISTS` sees the
	// plain table and silently no-ops, leaving search stuck in degraded mode
	// forever. Search content repopulates on the next index/`reindex`.
	var createSQL string
	switch err := tx.QueryRow(
		`SELECT sql FROM sqlite_master WHERE type='table' AND name='sessions_fts'`,
	).Scan(&createSQL); {
	case errors.Is(err, sql.ErrNoRows):
		// no table yet — first create below
	case err != nil:
		return fmt.Errorf("state: inspect sessions_fts: %w", err)
	default:
		if !strings.Contains(strings.ToUpper(createSQL), "VIRTUAL") && fts5Available(tx) {
			if _, derr := tx.Exec(`DROP TABLE sessions_fts`); derr != nil {
				return fmt.Errorf("state: drop stale fallback sessions_fts: %w", derr)
			}
		}
	}

	_, err := tx.Exec(`
CREATE VIRTUAL TABLE IF NOT EXISTS sessions_fts USING fts5(
  agent_id UNINDEXED,
  name,
  role,
  project,
  grp,
  model,
  backend,
  content,
  tokenize = 'unicode61 remove_diacritics 2'
)`)
	if err == nil {
		return nil
	}
	if !strings.Contains(err.Error(), "no such module: fts5") {
		return fmt.Errorf("state: create sessions_fts: %w", err)
	}
	if _, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS sessions_fts (
  agent_id TEXT NOT NULL,
  name TEXT NOT NULL,
  role TEXT NOT NULL,
  project TEXT NOT NULL,
  grp TEXT NOT NULL,
  model TEXT NOT NULL,
  backend TEXT NOT NULL,
  content TEXT NOT NULL
)`); err != nil {
		return fmt.Errorf("state: create fallback sessions_fts: %w", err)
	}
	return nil
}

// fts5Available reports whether the SQLite build has the FTS5 module, probed by
// attempting to create (and drop) a throwaway virtual table.
func fts5Available(tx *sql.Tx) bool {
	if _, err := tx.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS __fts5_probe USING fts5(x)`); err != nil {
		return false
	}
	_, _ = tx.Exec(`DROP TABLE IF EXISTS __fts5_probe`)
	return true
}
