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
}
