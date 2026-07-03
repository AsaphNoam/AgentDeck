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

func TestResumeAfterRestartPreservesFTSContent(t *testing.T) {
	st, _ := openTestDB(t)
	db := st.DB()

	// First process: index a session and flush a turn. "distinctive quartz
	// phrase" lands in the durable sessions_fts.content column.
	indexFixture(t, db)
	var before string
	if err := db.QueryRow(`SELECT content FROM sessions_fts WHERE agent_id = 'a_index'`).Scan(&before); err != nil {
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

	var after string
	if err := db.QueryRow(`SELECT content FROM sessions_fts WHERE agent_id = 'a_index'`).Scan(&after); err != nil {
		t.Fatalf("read fts content after restart: %v", err)
	}
	if !strings.Contains(after, "distinctive quartz") {
		t.Fatalf("original phrase wiped after resume: %q", after)
	}
	if !strings.Contains(after, "post restart marker") {
		t.Fatalf("new phrase missing after resume: %q", after)
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
