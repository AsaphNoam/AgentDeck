//go:build sqlite_fts5

package state

import (
	"strings"
	"testing"
)

// Regression (review fix): a plain fallback sessions_fts left by a prior non-FTS5
// build must be upgraded to the FTS5 virtual table once an FTS5-capable binary
// opens the DB — previously `CREATE VIRTUAL TABLE IF NOT EXISTS` silently no-oped
// on the existing plain table, leaving search degraded forever.
func TestEnsureSessionsFTSUpgradesFallback(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	// Simulate a DB first created by a non-FTS5 build: replace the virtual table
	// with a plain fallback of the same name.
	if _, err := st.db.Exec(`DROP TABLE sessions_fts`); err != nil {
		t.Fatalf("drop virtual: %v", err)
	}
	if _, err := st.db.Exec(`CREATE TABLE sessions_fts (
	  agent_id TEXT, name TEXT, role TEXT, project TEXT, grp TEXT, model TEXT, backend TEXT, content TEXT)`); err != nil {
		t.Fatalf("create fallback: %v", err)
	}
	if _, err := st.db.Exec(`INSERT INTO sessions_fts VALUES
	  ('a_legacy', 'Atlas', 'implementer', 'app', 'auth', 'sonnet', 'claude', 'old searchable marker')`); err != nil {
		t.Fatalf("insert legacy fallback row: %v", err)
	}
	st.Close()

	// Reopen with the FTS5-capable binary → ensureSessionsFTS must upgrade it.
	st2, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer st2.Close()

	var createSQL string
	if err := st2.db.QueryRow(
		`SELECT sql FROM sqlite_master WHERE type='table' AND name='sessions_fts'`,
	).Scan(&createSQL); err != nil {
		t.Fatalf("inspect sessions_fts: %v", err)
	}
	if !strings.Contains(strings.ToUpper(createSQL), "VIRTUAL") {
		t.Fatalf("sessions_fts not upgraded to an FTS5 virtual table: %q", createSQL)
	}
	var metadataName, legacyContent string
	if err := st2.db.QueryRow(`SELECT name FROM sessions_fts WHERE agent_id = 'a_legacy' AND document_id = 'metadata'`).Scan(&metadataName); err != nil {
		t.Fatalf("read migrated metadata document: %v", err)
	}
	if err := st2.db.QueryRow(`SELECT content FROM sessions_fts WHERE agent_id = 'a_legacy' AND document_id = 'legacy'`).Scan(&legacyContent); err != nil {
		t.Fatalf("read migrated legacy document: %v", err)
	}
	if metadataName != "Atlas" || legacyContent != "old searchable marker" {
		t.Fatalf("migrated documents = metadata %q legacy %q", metadataName, legacyContent)
	}
}

func TestEnsureSessionsFTSMigratesLegacyVirtualDocuments(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := st.db.Exec(`DROP TABLE sessions_fts`); err != nil {
		t.Fatalf("drop current virtual table: %v", err)
	}
	if _, err := st.db.Exec(`
CREATE VIRTUAL TABLE sessions_fts USING fts5(
  agent_id UNINDEXED, name, role, project, grp, model, backend, content
)`); err != nil {
		t.Fatalf("create legacy virtual table: %v", err)
	}
	if _, err := st.db.Exec(`INSERT INTO sessions_fts VALUES
  ('a_legacy', 'Atlas', 'implementer', 'app', 'auth', 'sonnet', 'claude', 'historic quartz marker')`); err != nil {
		t.Fatalf("insert legacy virtual row: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("close legacy store: %v", err)
	}

	reopened, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer reopened.Close()
	var metadataName, legacyContent string
	if err := reopened.db.QueryRow(`SELECT name FROM sessions_fts WHERE agent_id = 'a_legacy' AND document_id = 'metadata'`).Scan(&metadataName); err != nil {
		t.Fatalf("read metadata document: %v", err)
	}
	if err := reopened.db.QueryRow(`SELECT content FROM sessions_fts WHERE agent_id = 'a_legacy' AND document_id = 'legacy'`).Scan(&legacyContent); err != nil {
		t.Fatalf("read legacy document: %v", err)
	}
	if metadataName != "Atlas" || legacyContent != "historic quartz marker" {
		t.Fatalf("migrated documents = metadata %q legacy %q", metadataName, legacyContent)
	}
}
