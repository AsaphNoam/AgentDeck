package state

import (
	"database/sql"
	"fmt"
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

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("state: commit migration: %w", err)
	}
	return nil
}
