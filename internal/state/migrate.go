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
		var err error
		if m.apply != nil {
			err = m.apply(tx)
		} else {
			_, err = tx.Exec(m.sql)
		}
		if err != nil {
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
	// The FTS projection is capability-dependent, so this migration cannot be a
	// static schema string. Preserve existing documents while upgrading either the
	// legacy whole-session shape or a plain fallback table to the current FTS5
	// shape. An untagged binary leaves an existing virtual table untouched: it
	// cannot read that table, and archive search will use its metadata fallback.
	var createSQL string
	err := tx.QueryRow(
		`SELECT sql FROM sqlite_master WHERE type='table' AND name='sessions_fts'`,
	).Scan(&createSQL)
	exists := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("state: inspect sessions_fts: %w", err)
	}

	wantVirtual := fts5Available(tx)
	isVirtual := exists && strings.Contains(strings.ToUpper(createSQL), "VIRTUAL")
	if isVirtual && !wantVirtual {
		return nil
	}
	hasDocumentID := false
	if exists {
		rows, qerr := tx.Query(`PRAGMA table_info(sessions_fts)`)
		if qerr != nil {
			return fmt.Errorf("state: inspect sessions_fts columns: %w", qerr)
		}
		for rows.Next() {
			var cid int
			var name, typ string
			var notNull, pk int
			var defaultValue any
			if qerr := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); qerr != nil {
				rows.Close()
				return fmt.Errorf("state: scan sessions_fts columns: %w", qerr)
			}
			if name == "document_id" {
				hasDocumentID = true
			}
		}
		if qerr := rows.Err(); qerr != nil {
			rows.Close()
			return fmt.Errorf("state: iterate sessions_fts columns: %w", qerr)
		}
		if qerr := rows.Close(); qerr != nil {
			return fmt.Errorf("state: close sessions_fts columns: %w", qerr)
		}
	}
	if exists && hasDocumentID && isVirtual == wantVirtual {
		return nil
	}

	if _, err := tx.Exec(`DROP TABLE IF EXISTS temp.sessions_fts_backup`); err != nil {
		return fmt.Errorf("state: clear sessions_fts migration backup: %w", err)
	}
	if _, err := tx.Exec(`
CREATE TEMP TABLE sessions_fts_backup (
  agent_id TEXT NOT NULL,
  document_id TEXT NOT NULL,
  name TEXT NOT NULL,
  role TEXT NOT NULL,
  project TEXT NOT NULL,
  grp TEXT NOT NULL,
  model TEXT NOT NULL,
  backend TEXT NOT NULL,
  content TEXT NOT NULL
)`); err != nil {
		return fmt.Errorf("state: create sessions_fts migration backup: %w", err)
	}
	if exists {
		if hasDocumentID {
			_, err = tx.Exec(`
INSERT INTO sessions_fts_backup(agent_id, document_id, name, role, project, grp, model, backend, content)
SELECT agent_id, document_id, name, role, project, grp, model, backend, content FROM sessions_fts`)
		} else {
			_, err = tx.Exec(`
INSERT INTO sessions_fts_backup(agent_id, document_id, name, role, project, grp, model, backend, content)
SELECT agent_id, 'metadata', name, role, project, grp, model, backend, '' FROM sessions_fts;
INSERT INTO sessions_fts_backup(agent_id, document_id, name, role, project, grp, model, backend, content)
SELECT agent_id, 'legacy', '', '', '', '', '', '', content FROM sessions_fts WHERE content <> ''`)
		}
		if err != nil {
			return fmt.Errorf("state: preserve sessions_fts documents: %w", err)
		}
		if _, err := tx.Exec(`DROP TABLE sessions_fts`); err != nil {
			return fmt.Errorf("state: drop old sessions_fts: %w", err)
		}
	}

	if err := createSessionsFTS(tx, wantVirtual); err != nil {
		return err
	}
	if _, err := tx.Exec(`
INSERT INTO sessions_fts(agent_id, document_id, name, role, project, grp, model, backend, content)
SELECT agent_id, document_id, name, role, project, grp, model, backend, content
FROM sessions_fts_backup`); err != nil {
		return fmt.Errorf("state: restore sessions_fts documents: %w", err)
	}
	if _, err := tx.Exec(`DROP TABLE sessions_fts_backup`); err != nil {
		return fmt.Errorf("state: drop sessions_fts migration backup: %w", err)
	}
	return nil
}

func createSessionsFTS(tx *sql.Tx, virtual bool) error {
	if virtual {
		if _, err := tx.Exec(`
CREATE VIRTUAL TABLE sessions_fts USING fts5(
  agent_id UNINDEXED,
  document_id UNINDEXED,
  name,
  role,
  project,
  grp,
  model,
  backend,
  content,
  tokenize = 'unicode61 remove_diacritics 2'
	)`); err != nil {
			return fmt.Errorf("state: create sessions_fts: %w", err)
		}
		return nil
	}
	if _, err := tx.Exec(`
CREATE TABLE sessions_fts (
  agent_id TEXT NOT NULL,
  document_id TEXT NOT NULL,
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
	if _, err := tx.Exec(`CREATE INDEX sessions_fts_agent_document ON sessions_fts(agent_id, document_id)`); err != nil {
		return fmt.Errorf("state: index fallback sessions_fts: %w", err)
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
