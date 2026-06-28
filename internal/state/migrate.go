package state

import (
	"database/sql"
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
	return nil
}

func ensureSessionsFTS(tx *sql.Tx) error {
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
