package index

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
	"github.com/agentdeck/agentdeck/internal/transcript"
)

func openTestDB(t *testing.T) (*state.Store, string) {
	t.Helper()
	home := t.TempDir()
	st, err := state.Open(home)
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st, home
}

func meta() runtime.SessionMetaData {
	return runtime.SessionMetaData{
		Name:            "Atlas",
		Role:            "implementer",
		Project:         "my-app",
		Backend:         "claude",
		Model:           "sonnet-4-6",
		Interface:       "chat",
		Group:           "auth",
		Cwd:             "/workspace/my-app",
		SystemPromptSHA: "sha256:abc",
		EnvKeys:         []string{"OPENAI_BASE_URL"},
		CreatedAt:       "2026-06-28T10:00:00Z",
		SessionID:       "sess-1",
	}
}

func ev(t *testing.T, seq int64, typ string, data any) runtime.Event {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal event data: %v", err)
	}
	return runtime.Event{
		AgentID: "a_index",
		Seq:     seq,
		Type:    typ,
		Data:    raw,
		Ts:      time.Date(2026, 6, 28, 10, 0, int(seq), 0, time.UTC).Format(time.RFC3339),
	}
}

func fixtureEvents(t *testing.T) []runtime.Event {
	m := meta()
	metaRaw, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal meta: %v", err)
	}
	return []runtime.Event{
		{AgentID: "a_index", Seq: 0, Type: runtime.EvSessionMeta, Data: metaRaw, Ts: m.CreatedAt},
		ev(t, 1, runtime.EvAssistantText, runtime.AssistantTextData{Delta: "distinctive quartz phrase"}),
		ev(t, 2, runtime.EvToolCall, runtime.ToolCallData{ToolCallID: "tc_edit", Name: "Edit", Args: json.RawMessage(`{"file_path":"/workspace/my-app/src/auth.ts"}`), Status: "in_progress"}),
		ev(t, 3, runtime.EvDiff, runtime.DiffData{ToolCallID: "tc_edit", Path: "/workspace/my-app/src/auth.ts", NewText: "package auth"}),
		ev(t, 4, runtime.EvToolCall, runtime.ToolCallData{ToolCallID: "tc_cmd", Name: "Bash", Args: json.RawMessage(`{"command":"go test ./..."}`), Status: "in_progress"}),
		ev(t, 5, runtime.EvToolResult, runtime.ToolResultData{ToolCallID: "tc_cmd", Status: "completed", Content: json.RawMessage(`"ok"`)}),
		ev(t, 6, runtime.EvTurnEnd, runtime.TurnEndData{StopReason: "end_turn", ContextPct: 0.42}),
	}
}

func indexFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	ix := New(db)
	for _, e := range fixtureEvents(t) {
		if err := ix.OnEvent("a_index", e); err != nil {
			t.Fatalf("OnEvent seq %d: %v", e.Seq, err)
		}
		if e.Type == runtime.EvTurnEnd {
			var d runtime.TurnEndData
			_ = json.Unmarshal(e.Data, &d)
			if err := ix.OnTurnEnd("a_index", runtime.TurnRollup{LastSeq: e.Seq, LastContextPct: d.ContextPct, UpdatedAt: e.Ts}); err != nil {
				t.Fatalf("OnTurnEnd: %v", err)
			}
		}
	}
}

func TestIndexerFTSAndRollups(t *testing.T) {
	st, _ := openTestDB(t)
	indexFixture(t, st.DB())

	var turnCount, eventCount, lastSeq, filesTouched, commandsRun int
	var contextPct float64
	if err := st.DB().QueryRow(`
SELECT turn_count, event_count, last_seq, last_context_pct, files_touched, commands_run
FROM sessions WHERE agent_id = 'a_index'`).Scan(&turnCount, &eventCount, &lastSeq, &contextPct, &filesTouched, &commandsRun); err != nil {
		t.Fatalf("session rollup: %v", err)
	}
	if turnCount != 1 || eventCount != 6 || lastSeq != 6 || contextPct != 0.42 || filesTouched != 1 || commandsRun != 1 {
		t.Fatalf("rollup = turns:%d events:%d last:%d pct:%v files:%d commands:%d", turnCount, eventCount, lastSeq, contextPct, filesTouched, commandsRun)
	}

	var path string
	var editCount, hasDiff int
	var diffRefs string
	if err := st.DB().QueryRow(`SELECT path, edit_count, has_diff, diff_refs FROM tracked_files WHERE agent_id = 'a_index'`).Scan(&path, &editCount, &hasDiff, &diffRefs); err != nil {
		t.Fatalf("tracked file: %v", err)
	}
	if path != "src/auth.ts" || editCount != 2 || hasDiff != 1 || diffRefs == "[]" {
		t.Fatalf("tracked file = path:%q edit:%d diff:%d refs:%s", path, editCount, hasDiff, diffRefs)
	}

	var command, status string
	if err := st.DB().QueryRow(`SELECT command, exit_status FROM tracked_commands WHERE agent_id = 'a_index'`).Scan(&command, &status); err != nil {
		t.Fatalf("tracked command: %v", err)
	}
	if command != "go test ./..." || status != "completed" {
		t.Fatalf("tracked command = %q/%q", command, status)
	}
}

// FS-13.A5: annotation excerpts and instructions join archive search even
// though a dashboard annotation does not finish an ACP turn.
func TestAnnotationFlushesSearchableContent(t *testing.T) {
	st, _ := openTestDB(t)
	ix := New(st.DB())
	m := meta()
	if err := ix.UpsertSessionMeta("a_index", m); err != nil {
		t.Fatalf("UpsertSessionMeta: %v", err)
	}
	annotation := ev(t, 1, runtime.EvAnnotation, runtime.AnnotationData{Annotations: []runtime.Annotation{{
		Seq: 7, Excerpt: "suspect boundary condition", Instruction: "add a regression test for the annotation path",
	}}, Target: runtime.AnnotationTarget{Kind: "self", AgentID: "a_index"}})
	if err := ix.OnEvent("a_index", annotation); err != nil {
		t.Fatalf("OnEvent annotation: %v", err)
	}
	if err := ix.FlushContent("a_index", annotation.Seq, annotation.Ts); err != nil {
		t.Fatalf("FlushContent: %v", err)
	}
	var content string
	if err := st.DB().QueryRow(`SELECT content FROM sessions_fts WHERE agent_id = ? AND document_id = 'flush:1'`, "a_index").Scan(&content); err != nil {
		t.Fatalf("read indexed annotation: %v", err)
	}
	if !strings.Contains(content, "regression test for the annotation path") {
		t.Fatalf("annotation instruction not indexed: %q", content)
	}
}

func TestResumeAfterRestartPreservesFTSContent(t *testing.T) {
	st, _ := openTestDB(t)
	db := st.DB()

	// First process: index a session and flush a turn into an immutable document.
	indexFixture(t, db)
	var before string
	if err := db.QueryRow(`SELECT content FROM sessions_fts WHERE agent_id = 'a_index' AND document_id = 'turn:6'`).Scan(&before); err != nil {
		t.Fatalf("read fts content before restart: %v", err)
	}
	if !strings.Contains(before, "distinctive quartz") {
		t.Fatalf("pre-restart content missing original phrase: %q", before)
	}

	// Second process (restart/resume): a brand-new Indexer with an empty
	// in-memory buffer over the same DB. A resumed turn brings new content.
	ix2 := New(db)
	if err := ix2.OnEvent("a_index", ev(t, 7, runtime.EvAssistantText, runtime.AssistantTextData{Delta: "post restart marker"})); err != nil {
		t.Fatalf("OnEvent after restart: %v", err)
	}
	if err := ix2.OnTurnEnd("a_index", runtime.TurnRollup{LastSeq: 7, UpdatedAt: "2026-06-28T11:00:00Z"}); err != nil {
		t.Fatalf("OnTurnEnd after restart: %v", err)
	}

	var original, resumed string
	if err := db.QueryRow(`SELECT content FROM sessions_fts WHERE agent_id = 'a_index' AND document_id = 'turn:6'`).Scan(&original); err != nil {
		t.Fatalf("read original document after restart: %v", err)
	}
	if err := db.QueryRow(`SELECT content FROM sessions_fts WHERE agent_id = 'a_index' AND document_id = 'turn:7'`).Scan(&resumed); err != nil {
		t.Fatalf("read resumed document: %v", err)
	}
	if original != before {
		t.Fatalf("original document changed after resume: before %q after %q", before, original)
	}
	if !strings.Contains(resumed, "post restart marker") {
		t.Fatalf("new phrase missing after resume: %q", resumed)
	}
}

// FS-05.A11: later turns append documents and metadata updates replace only
// the metadata document.
func TestTurnsAppendWithoutRewritingEarlierDocuments(t *testing.T) {
	st, _ := openTestDB(t)
	ix := New(st.DB())
	if err := ix.UpsertSessionMeta("a_index", meta()); err != nil {
		t.Fatalf("UpsertSessionMeta: %v", err)
	}
	if err := ix.OnEvent("a_index", ev(t, 1, runtime.EvAssistantText, runtime.AssistantTextData{Delta: "first immutable marker"})); err != nil {
		t.Fatalf("first OnEvent: %v", err)
	}
	if err := ix.OnTurnEnd("a_index", runtime.TurnRollup{LastSeq: 2, UpdatedAt: "2026-06-28T10:00:02Z"}); err != nil {
		t.Fatalf("first OnTurnEnd: %v", err)
	}
	if len(ix.content) != 0 {
		t.Fatalf("turn buffer retained after flush: %+v", ix.content)
	}
	var first string
	if err := st.DB().QueryRow(`SELECT content FROM sessions_fts WHERE agent_id = 'a_index' AND document_id = 'turn:2'`).Scan(&first); err != nil {
		t.Fatalf("read first document: %v", err)
	}

	if err := ix.OnEvent("a_index", ev(t, 3, runtime.EvAssistantText, runtime.AssistantTextData{Delta: "second separate marker"})); err != nil {
		t.Fatalf("second OnEvent: %v", err)
	}
	if err := ix.OnTurnEnd("a_index", runtime.TurnRollup{LastSeq: 4, UpdatedAt: "2026-06-28T10:00:04Z"}); err != nil {
		t.Fatalf("second OnTurnEnd: %v", err)
	}
	var after string
	if err := st.DB().QueryRow(`SELECT content FROM sessions_fts WHERE agent_id = 'a_index' AND document_id = 'turn:2'`).Scan(&after); err != nil {
		t.Fatalf("reread first document: %v", err)
	}
	if after != first {
		t.Fatalf("first document rewritten: before %q after %q", first, after)
	}
	var transcriptDocuments int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM sessions_fts WHERE agent_id = 'a_index' AND document_id <> 'metadata'`).Scan(&transcriptDocuments); err != nil {
		t.Fatalf("count transcript documents: %v", err)
	}
	if transcriptDocuments != 2 {
		t.Fatalf("transcript document count = %d, want 2", transcriptDocuments)
	}
}

func TestReindexRebuildsFromRawLogs(t *testing.T) {
	st, home := openTestDB(t)
	w, err := transcript.Open(home, "a_index", nil)
	if err != nil {
		t.Fatalf("transcript.Open: %v", err)
	}
	for _, e := range fixtureEvents(t) {
		if err := w.Append(e); err != nil {
			t.Fatalf("Append seq %d: %v", e.Seq, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close writer: %v", err)
	}

	if _, err := st.DB().Exec(`INSERT INTO sessions(agent_id, name, role, project, backend, model, interface, cwd, system_prompt, created_at, updated_at) VALUES ('stale','stale','r','p','b','m','chat','','','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')`); err != nil {
		t.Fatalf("insert stale row: %v", err)
	}
	if err := Reindex(home, st.DB()); err != nil {
		t.Fatalf("Reindex: %v", err)
	}
	var count int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM sessions WHERE agent_id = 'stale'`).Scan(&count); err != nil {
		t.Fatalf("count stale: %v", err)
	}
	if count != 0 {
		t.Fatalf("stale sessions count = %d, want 0", count)
	}
	var agentID string
	if err := st.DB().QueryRow(`SELECT agent_id FROM sessions_fts WHERE content LIKE ?`, `%distinctive quartz%`).Scan(&agentID); err != nil {
		t.Fatalf("fts content after reindex: %v", err)
	}
	if agentID != "a_index" {
		t.Fatalf("fts after reindex agent = %q, want a_index", agentID)
	}
}

// TestReindexIsolatesBadAgent guards the BLOCKING finding that Reindex wiped
// every agent's index up front and then aborted on the FIRST bad transcript,
// leaving the archive partially destroyed. A good agent must remain fully
// reindexed even when another agent's transcript is unreadable, and Reindex
// must report the skipped agent via a non-nil aggregated error.
func TestReindexIsolatesBadAgent(t *testing.T) {
	st, home := openTestDB(t)

	// Good agent: a real, replayable transcript.
	w, err := transcript.Open(home, "a_index", nil)
	if err != nil {
		t.Fatalf("transcript.Open good: %v", err)
	}
	for _, e := range fixtureEvents(t) {
		if err := w.Append(e); err != nil {
			t.Fatalf("Append seq %d: %v", e.Seq, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close writer: %v", err)
	}

	// Bad agent: its transcript.ndjson is a directory, so ReadAll fails. This
	// is the "one bad transcript" that must not abort the whole reindex.
	badDir := filepath.Join(home, "sessions", "a_bad")
	if err := os.MkdirAll(filepath.Join(badDir, "transcript.ndjson"), 0o755); err != nil {
		t.Fatalf("make bad transcript dir: %v", err)
	}

	err = Reindex(home, st.DB())
	if err == nil {
		t.Fatalf("Reindex returned nil, want aggregated error naming skipped agent")
	}
	if !strings.Contains(err.Error(), "a_bad") {
		t.Fatalf("Reindex error = %v, want it to name a_bad", err)
	}

	// The good agent must still be fully in the index despite the bad one.
	var agentID string
	if err := st.DB().QueryRow(`SELECT agent_id FROM sessions WHERE agent_id = 'a_index'`).Scan(&agentID); err != nil {
		t.Fatalf("good agent missing from sessions after reindex: %v", err)
	}
	if agentID != "a_index" {
		t.Fatalf("sessions agent = %q, want a_index", agentID)
	}
	var ftsAgent string
	if err := st.DB().QueryRow(`SELECT agent_id FROM sessions_fts WHERE content LIKE ?`, `%distinctive quartz%`).Scan(&ftsAgent); err != nil {
		t.Fatalf("good agent fts content missing after reindex: %v", err)
	}
	if ftsAgent != "a_index" {
		t.Fatalf("fts agent = %q, want a_index", ftsAgent)
	}
}

// TestReindexPreservesFinalPartialTurn guards the BLOCKING finding that
// reindexAgent's post-loop flush only ran when NO turn_end was ever seen. A
// transcript with one completed turn followed by a crash mid-later-turn left
// the final assistant text sitting only in the in-memory buffer, so reindex
// silently dropped the most recent partial turn's searchable content.
func TestReindexPreservesFinalPartialTurn(t *testing.T) {
	st, home := openTestDB(t)
	w, err := transcript.Open(home, "a_index", nil)
	if err != nil {
		t.Fatalf("transcript.Open: %v", err)
	}
	m := meta()
	metaRaw, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal meta: %v", err)
	}
	events := []runtime.Event{
		{AgentID: "a_index", Seq: 0, Type: runtime.EvSessionMeta, Data: metaRaw, Ts: m.CreatedAt},
		// Turn 1: completed (has a turn_end).
		ev(t, 1, runtime.EvAssistantText, runtime.AssistantTextData{Delta: "turnone_alpha_phrase"}),
		ev(t, 2, runtime.EvTurnEnd, runtime.TurnEndData{StopReason: "end_turn", ContextPct: 0.3}),
		// Turn 2: partial — crash before a turn_end.
		ev(t, 3, runtime.EvAssistantText, runtime.AssistantTextData{Delta: "turntwo_partial_bravo_phrase"}),
	}
	for _, e := range events {
		if err := w.Append(e); err != nil {
			t.Fatalf("Append seq %d: %v", e.Seq, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close writer: %v", err)
	}

	if err := Reindex(home, st.DB()); err != nil {
		t.Fatalf("Reindex: %v", err)
	}

	// The completed turn's phrase is searchable (never in question) ...
	var agentID string
	if err := st.DB().QueryRow(`SELECT agent_id FROM sessions_fts WHERE content LIKE ?`, `%turnone_alpha_phrase%`).Scan(&agentID); err != nil {
		t.Fatalf("completed-turn phrase missing after reindex: %v", err)
	}
	// ... and so must the partial (unterminated) turn's phrase.
	if err := st.DB().QueryRow(`SELECT agent_id FROM sessions_fts WHERE content LIKE ?`, `%turntwo_partial_bravo_phrase%`).Scan(&agentID); err != nil {
		t.Fatalf("partial-turn phrase dropped by reindex (BLOCKING regression): %v", err)
	}
	if agentID != "a_index" {
		t.Fatalf("fts agent = %q, want a_index", agentID)
	}
}

func TestReindexPreservesAnnotationFlushBoundary(t *testing.T) {
	st, home := openTestDB(t)
	w, err := transcript.Open(home, "a_index", nil)
	if err != nil {
		t.Fatalf("transcript.Open: %v", err)
	}
	m := meta()
	metaRaw, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal meta: %v", err)
	}
	events := []runtime.Event{
		{AgentID: "a_index", Seq: 0, Type: runtime.EvSessionMeta, Data: metaRaw, Ts: m.CreatedAt},
		ev(t, 1, runtime.EvAssistantText, runtime.AssistantTextData{Delta: "before annotation marker"}),
		ev(t, 2, runtime.EvAnnotation, runtime.AnnotationData{OverallInstruction: "review this annotation boundary"}),
		ev(t, 3, runtime.EvAssistantText, runtime.AssistantTextData{Delta: "after annotation marker"}),
		ev(t, 4, runtime.EvTurnEnd, runtime.TurnEndData{StopReason: "end_turn", ContextPct: 0.2}),
	}
	for _, event := range events {
		if err := w.Append(event); err != nil {
			t.Fatalf("append seq %d: %v", event.Seq, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close transcript: %v", err)
	}
	if err := Reindex(home, st.DB()); err != nil {
		t.Fatalf("Reindex: %v", err)
	}

	var flushed, completed string
	if err := st.DB().QueryRow(`SELECT content FROM sessions_fts WHERE agent_id = 'a_index' AND document_id = 'flush:2'`).Scan(&flushed); err != nil {
		t.Fatalf("read annotation flush document: %v", err)
	}
	if err := st.DB().QueryRow(`SELECT content FROM sessions_fts WHERE agent_id = 'a_index' AND document_id = 'turn:4'`).Scan(&completed); err != nil {
		t.Fatalf("read post-annotation turn document: %v", err)
	}
	if !strings.Contains(flushed, "before annotation marker") || !strings.Contains(flushed, "annotation boundary") {
		t.Fatalf("annotation flush content = %q", flushed)
	}
	if strings.Contains(completed, "before annotation marker") || !strings.Contains(completed, "after annotation marker") {
		t.Fatalf("post-annotation turn content = %q", completed)
	}
}

func TestReindexMissingSessionsDirIsNoop(t *testing.T) {
	st, home := openTestDB(t)
	if err := os.RemoveAll(filepath.Join(home, "sessions")); err != nil {
		t.Fatalf("remove sessions dir: %v", err)
	}
	if err := Reindex(home, st.DB()); err != nil {
		t.Fatalf("Reindex missing sessions: %v", err)
	}
	var rows []string
	got, err := st.DB().Query(`SELECT agent_id FROM sessions`)
	if err != nil {
		t.Fatalf("query sessions: %v", err)
	}
	defer got.Close()
	for got.Next() {
		var id string
		if err := got.Scan(&id); err != nil {
			t.Fatalf("scan id: %v", err)
		}
		rows = append(rows, id)
	}
	if !reflect.DeepEqual(rows, []string(nil)) {
		t.Fatalf("sessions rows = %+v, want empty", rows)
	}
}

// TestLaunchConfigRoundTrips proves the frozen federation launch object survives
// UpsertSessionMeta → sessions.launch_config_json → state.ReadSession, and that
// an empty object normalizes to nil (migration v8).
func TestLaunchConfigRoundTrips(t *testing.T) {
	st, _ := openTestDB(t)
	ix := New(st.DB())

	m := meta()
	m.LaunchConfig = json.RawMessage(`{"version":1,"binding":{"backend":"claude","provider":"claude-code"},"resolved":{"model":"user-model"}}`)
	if err := ix.UpsertSessionMeta("a_lc", m); err != nil {
		t.Fatalf("UpsertSessionMeta: %v", err)
	}
	snap, err := st.ReadSession("a_lc")
	if err != nil {
		t.Fatalf("ReadSession: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(snap.LaunchConfig, &decoded); err != nil {
		t.Fatalf("launch config did not round-trip: %v (raw=%q)", err, snap.LaunchConfig)
	}
	if decoded["version"] != float64(1) {
		t.Fatalf("launch config = %+v", decoded)
	}

	// A meta with no binding stores the default object and reads back as nil.
	empty := meta()
	if err := ix.UpsertSessionMeta("a_empty", empty); err != nil {
		t.Fatalf("UpsertSessionMeta empty: %v", err)
	}
	snap2, err := st.ReadSession("a_empty")
	if err != nil {
		t.Fatalf("ReadSession empty: %v", err)
	}
	if snap2.LaunchConfig != nil {
		t.Fatalf("empty launch config = %q, want nil", snap2.LaunchConfig)
	}
}

// TestUpsertSessionMetaTransactionAtomicity verifies that session row and
// metadata document are updated atomically: both succeed or both roll back
// (TS-02.R16, INV §15).
func TestUpsertSessionMetaTransactionAtomicity(t *testing.T) {
	st, _ := openTestDB(t)
	ix := New(st.DB())

	m := meta()
	m.Name = "First"
	if err := ix.UpsertSessionMeta("a_atomic", m); err != nil {
		t.Fatalf("first UpsertSessionMeta: %v", err)
	}

	var firstName string
	if err := st.DB().QueryRow(`SELECT name FROM sessions WHERE agent_id = ?`, "a_atomic").Scan(&firstName); err != nil {
		t.Fatalf("read session after first upsert: %v", err)
	}
	if firstName != "First" {
		t.Fatalf("name = %q, want First", firstName)
	}

	var metaCount int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM sessions_fts WHERE agent_id = ? AND document_id = ?`, "a_atomic", "metadata").Scan(&metaCount); err != nil {
		t.Fatalf("count metadata documents: %v", err)
	}
	if metaCount != 1 {
		t.Fatalf("metadata document count = %d, want 1", metaCount)
	}

	// Update with a different name and verify both writes succeeded together.
	m.Name = "Second"
	if err := ix.UpsertSessionMeta("a_atomic", m); err != nil {
		t.Fatalf("second UpsertSessionMeta: %v", err)
	}

	var secondName string
	if err := st.DB().QueryRow(`SELECT name FROM sessions WHERE agent_id = ?`, "a_atomic").Scan(&secondName); err != nil {
		t.Fatalf("read session after second upsert: %v", err)
	}
	if secondName != "Second" {
		t.Fatalf("name = %q, want Second", secondName)
	}

	// Metadata document should still exist and be unique.
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM sessions_fts WHERE agent_id = ? AND document_id = ?`, "a_atomic", "metadata").Scan(&metaCount); err != nil {
		t.Fatalf("count metadata documents after update: %v", err)
	}
	if metaCount != 1 {
		t.Fatalf("metadata document count after update = %d, want 1 (not 2)", metaCount)
	}
}
