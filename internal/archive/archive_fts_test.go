//go:build sqlite_fts5

package archive

import (
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/index"
	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
	"github.com/agentdeck/agentdeck/internal/transcript"
)

func openArchiveTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	st, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	seedArchiveRows(t, st.DB())
	return st.DB(), func() { _ = st.Close() }
}

func seedArchiveRows(t *testing.T, db *sql.DB) {
	t.Helper()
	created := "2026-06-28T10:00:00Z"
	updated1 := "2026-06-28T10:10:00Z"
	updated2 := "2026-06-28T10:05:00Z"
	if _, err := db.Exec(`
INSERT INTO agents(agent_id, name, role, project, backend, model, interface, created_at, grp)
VALUES ('a_active','Atlas','implementer','my-app','claude','sonnet','chat',?,'auth')`, created); err != nil {
		t.Fatalf("insert agent: %v", err)
	}
	if err := insertSession(db, "a_active", "Atlas", "implementer", "my-app", "auth", updated1, "fixed a distinctive quartz issue"); err != nil {
		t.Fatalf("insert active session: %v", err)
	}
	if err := insertSession(db, "a_inactive", "Beta", "reviewer", "docs", "", updated2, "reviewed migration notes"); err != nil {
		t.Fatalf("insert inactive session: %v", err)
	}
	if _, err := db.Exec(`
INSERT INTO running(agent_id, pid, session_id, interface, started_at)
VALUES ('a_active', 1234, 'sess-active', 'chat', ?)`, time.Now().UTC().Format(time.RFC3339)); err != nil {
		t.Fatalf("insert running: %v", err)
	}
}

func insertSession(db *sql.DB, id, name, role, project, group, updated, content string) error {
	_, err := db.Exec(`
INSERT INTO sessions(agent_id, name, role, project, backend, model, interface, grp, cwd, system_prompt, created_at, updated_at, turn_count, files_touched, commands_run)
VALUES (?, ?, ?, ?, 'claude', 'sonnet', 'chat', ?, '/tmp/app', 'prompt', '2026-06-28T10:00:00Z', ?, 1, 2, 1)`,
		id, name, role, project, group, updated)
	if err != nil {
		return err
	}
	_, err = db.Exec(`
INSERT INTO sessions_fts(agent_id, name, role, project, grp, model, backend, content)
VALUES (?, ?, ?, ?, ?, 'sonnet', 'claude', ?)`, id, name, role, project, group, content)
	return err
}

func TestArchiveListAndActiveFilter(t *testing.T) {
	db, cleanup := openArchiveTestDB(t)
	defer cleanup()
	a := New(db)

	resp, err := a.Search(Query{Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if resp.Total != 2 || len(resp.Results) != 2 {
		t.Fatalf("list total/results = %d/%d, want 2/2", resp.Total, len(resp.Results))
	}
	if resp.Results[0].AgentID != "a_active" || !resp.Results[0].Active {
		t.Fatalf("first result = %+v, want active newest", resp.Results[0])
	}

	active := false
	resp, err = a.Search(Query{Limit: 10, Active: &active})
	if err != nil {
		t.Fatalf("inactive list: %v", err)
	}
	if resp.Total != 1 || resp.Results[0].AgentID != "a_inactive" || resp.Results[0].Active {
		t.Fatalf("inactive results = %+v", resp.Results)
	}
}

func TestArchiveSearchFTSMetadataTranscriptAndPagination(t *testing.T) {
	db, cleanup := openArchiveTestDB(t)
	defer cleanup()
	a := New(db)

	resp, err := a.Search(Query{Q: "distinctive quartz", Limit: 10})
	if err != nil {
		t.Fatalf("transcript search: %v", err)
	}
	if resp.Total != 1 || resp.Results[0].AgentID != "a_active" {
		t.Fatalf("transcript search results = %+v", resp.Results)
	}
	if got := resp.Results[0].MatchedIn; len(got) != 1 || got[0] != "transcript" {
		t.Fatalf("matched_in = %+v, want transcript", got)
	}
	if resp.Results[0].Snippet == "" {
		t.Fatalf("snippet empty for transcript hit")
	}

	resp, err = a.Search(Query{Q: "Atlas", Limit: 10})
	if err != nil {
		t.Fatalf("metadata search: %v", err)
	}
	if resp.Total != 1 || resp.Results[0].AgentID != "a_active" {
		t.Fatalf("metadata search results = %+v", resp.Results)
	}
	if got := resp.Results[0].MatchedIn; len(got) == 0 || got[0] != "metadata" {
		t.Fatalf("matched_in = %+v, want metadata", got)
	}

	resp, err = a.Search(Query{Limit: 1, Offset: 1})
	if err != nil {
		t.Fatalf("pagination list: %v", err)
	}
	if resp.Total != 2 || len(resp.Results) != 1 || resp.Results[0].AgentID != "a_inactive" {
		t.Fatalf("pagination result = total %d %+v", resp.Total, resp.Results)
	}
}

func TestArchiveSearchNegative(t *testing.T) {
	db, cleanup := openArchiveTestDB(t)
	defer cleanup()
	resp, err := New(db).Search(Query{Q: "missing-token", Limit: 10})
	if err != nil {
		t.Fatalf("negative search: %v", err)
	}
	if resp.Total != 0 || len(resp.Results) != 0 {
		t.Fatalf("negative search = total %d %+v, want empty", resp.Total, resp.Results)
	}
}

// TestArchiveSearchANDSemantics verifies that multi-token queries use AND:
// a term present in only one of two sessions must not match the other.
func TestArchiveSearchANDSemantics(t *testing.T) {
	db, cleanup := openArchiveTestDB(t)
	defer cleanup()
	a := New(db)

	// "distinctive" is only in a_active; "migration" is only in a_inactive.
	// A query for both should match neither.
	resp, err := a.Search(Query{Q: "distinctive migration", Limit: 10})
	if err != nil {
		t.Fatalf("AND query: %v", err)
	}
	if resp.Total != 0 {
		t.Fatalf("AND semantics: got %d results, want 0 (tokens in different sessions)", resp.Total)
	}

	// Both tokens in a_active's content → exactly one match.
	resp, err = a.Search(Query{Q: "distinctive quartz", Limit: 10})
	if err != nil {
		t.Fatalf("AND same-session query: %v", err)
	}
	if resp.Total != 1 || resp.Results[0].AgentID != "a_active" {
		t.Fatalf("AND same-session: got total=%d %+v, want 1 a_active", resp.Total, resp.Results)
	}
}

// TestArchiveReindexEquivalence drops the Phase-4 tables and rebuilds from
// transcript.ndjson files, then asserts archive search returns the same results.
func TestArchiveReindexEquivalence(t *testing.T) {
	home := t.TempDir()
	st, err := state.Open(home)
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	defer st.Close()
	db := st.DB()

	// Write two transcript files to disk and index them via Reindex.
	writeTranscript(t, home, "r_active", "Gamma", "implementer", "my-app", "spectacular ruby feature")
	writeTranscript(t, home, "r_inactive", "Delta", "reviewer", "docs", "reviewed spectacular notes")

	if err := index.Reindex(home, db); err != nil {
		t.Fatalf("initial reindex: %v", err)
	}
	// Insert an agents row (required by the running FK) and a running row so r_active shows as active.
	if _, err := db.Exec(`INSERT INTO agents(agent_id, name, role, project, backend, model, interface, created_at) VALUES ('r_active','Gamma','implementer','my-app','claude','sonnet','chat','2026-06-28T10:00:00Z')`); err != nil {
		t.Fatalf("insert agent: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO running(agent_id, pid, session_id, interface, started_at) VALUES ('r_active', 9999, 'sess-rx', 'chat', ?)`,
		time.Now().UTC().Format(time.RFC3339)); err != nil {
		t.Fatalf("insert running: %v", err)
	}

	a := New(db)
	before, err := a.Search(Query{Q: "spectacular ruby", Limit: 10})
	if err != nil {
		t.Fatalf("search before reindex: %v", err)
	}
	if before.Total != 1 || before.Results[0].AgentID != "r_active" {
		t.Fatalf("before reindex: got total=%d %+v", before.Total, before.Results)
	}

	// Drop and rebuild.
	if err := index.Reindex(home, db); err != nil {
		t.Fatalf("second reindex: %v", err)
	}
	// Restore the running row (Reindex clears tracked tables but not running).
	// running is in Phase-1 tables and is NOT cleared by resetTables, so it persists.

	after, err := a.Search(Query{Q: "spectacular ruby", Limit: 10})
	if err != nil {
		t.Fatalf("search after reindex: %v", err)
	}
	if after.Total != before.Total || after.Results[0].AgentID != before.Results[0].AgentID {
		t.Fatalf("reindex equivalence: before=%+v after=%+v", before, after)
	}
}

// writeTranscript writes a minimal transcript.ndjson (session_meta + assistant_text + turn_end)
// to home/sessions/{agentID}/ so that index.Reindex can build from it.
func writeTranscript(t *testing.T, home, agentID, name, role, project, content string) {
	t.Helper()
	meta := &runtime.SessionMetaData{
		Name: name, Role: role, Project: project,
		Backend: "claude", Model: "sonnet", Interface: "chat",
		Cwd: "/tmp", CreatedAt: "2026-06-28T10:00:00Z",
	}
	w, err := transcript.Open(home, agentID, meta)
	if err != nil {
		t.Fatalf("transcript.Open %s: %v", agentID, err)
	}
	defer w.Close()

	textData, _ := json.Marshal(runtime.AssistantTextData{Delta: content})
	if err := w.Append(runtime.Event{
		AgentID: agentID, Seq: 1, Type: runtime.EvAssistantText,
		Ts: "2026-06-28T10:01:00Z", Data: textData,
	}); err != nil {
		t.Fatalf("append text %s: %v", agentID, err)
	}

	endData, _ := json.Marshal(runtime.TurnEndData{StopReason: "end_turn", ContextPct: 0.1})
	if err := w.Append(runtime.Event{
		AgentID: agentID, Seq: 2, Type: runtime.EvTurnEnd,
		Ts: "2026-06-28T10:02:00Z", Data: endData,
	}); err != nil {
		t.Fatalf("append turn_end %s: %v", agentID, err)
	}
}
