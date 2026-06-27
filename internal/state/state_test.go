package state

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"
	"time"
)

func newTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	home := t.TempDir()
	st, err := Open(home)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := st.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})
	return st, home
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse time %q: %v", s, err)
	}
	return tm
}

func testAgent(id string, createdAt time.Time) Agent {
	return Agent{
		AgentID:   id,
		Name:      "Atlas",
		Role:      "implementer",
		Project:   "my-app",
		Backend:   "claude",
		Model:     "sonnet-4-6",
		Interface: "chat",
		CreatedAt: createdAt,
		Group:     "auth-migration",
	}
}

func TestOpenMigratesAndConfiguresSQLite(t *testing.T) {
	st, home := newTestStore(t)
	if _, err := os.Stat(filepath.Join(home, "state.db")); err != nil {
		t.Fatalf("state.db missing: %v", err)
	}

	var journalMode string
	if err := st.DB().QueryRow(`PRAGMA journal_mode`).Scan(&journalMode); err != nil {
		t.Fatalf("journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", journalMode)
	}
	var foreignKeys int
	if err := st.DB().QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		t.Fatalf("foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeys)
	}

	for _, table := range []string{"schema_migrations", "agents", "running", "status", "messages"} {
		var name string
		err := st.DB().QueryRow(
			`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`,
			table,
		).Scan(&name)
		if err != nil {
			t.Fatalf("table %s missing: %v", table, err)
		}
	}
	var version int
	if err := st.DB().QueryRow(`SELECT version FROM schema_migrations`).Scan(&version); err != nil {
		t.Fatalf("schema_migrations version: %v", err)
	}
	if version != 1 {
		t.Fatalf("migration version = %d, want 1", version)
	}
}

func TestOpenIsIdempotentAndPreservesRows(t *testing.T) {
	st, home := newTestStore(t)
	agent := testAgent("a_8f3c12", mustTime(t, "2026-06-22T10:00:00Z"))
	if err := st.WriteAgent(agent); err != nil {
		t.Fatalf("WriteAgent: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close before reopen: %v", err)
	}

	reopened, err := Open(home)
	if err != nil {
		t.Fatalf("Open again: %v", err)
	}
	defer reopened.Close()

	got, err := reopened.ReadAgent(agent.AgentID)
	if err != nil {
		t.Fatalf("ReadAgent after reopen: %v", err)
	}
	if !reflect.DeepEqual(got, agent) {
		t.Fatalf("agent after reopen = %+v, want %+v", got, agent)
	}
	var count int
	if err := reopened.DB().QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = 1`).Scan(&count); err != nil {
		t.Fatalf("count migration: %v", err)
	}
	if count != 1 {
		t.Fatalf("migration version rows = %d, want 1", count)
	}
}

func TestRoundTripStateObjects(t *testing.T) {
	st, _ := newTestStore(t)
	busy := mustTime(t, "2026-06-22T10:00:05Z")
	read := mustTime(t, "2026-06-22T10:00:10Z")

	agent := testAgent("a_8f3c12", mustTime(t, "2026-06-22T10:00:00Z"))
	if err := st.WriteAgent(agent); err != nil {
		t.Fatalf("WriteAgent: %v", err)
	}
	if got, err := st.ReadAgent(agent.AgentID); err != nil || !reflect.DeepEqual(got, agent) {
		t.Fatalf("Agent round-trip: got %+v err %v, want %+v", got, err, agent)
	}

	run := RunningEntry{
		AgentID:   agent.AgentID,
		PID:       48213,
		SessionID: "claude-sess-xyz",
		Interface: "chat",
		TTY:       "/dev/ttys001",
		StartedAt: mustTime(t, "2026-06-22T10:00:01Z"),
	}
	if err := st.WriteRunning(run); err != nil {
		t.Fatalf("WriteRunning: %v", err)
	}
	if got, err := st.ReadRunning(run.AgentID); err != nil || !reflect.DeepEqual(got, run) {
		t.Fatalf("RunningEntry round-trip: got %+v err %v, want %+v", got, err, run)
	}

	status := Status{
		AgentID:    agent.AgentID,
		State:      "busy",
		Detail:     "Editing src/auth.ts",
		LastTrace:  "tool: edit",
		BusySince:  &busy,
		ContextPct: 0.42,
	}
	if err := st.WriteStatus(status); err != nil {
		t.Fatalf("WriteStatus: %v", err)
	}
	if got, err := st.ReadStatus(status.AgentID); err != nil || !reflect.DeepEqual(got, status) {
		t.Fatalf("Status round-trip: got %+v err %v, want %+v", got, err, status)
	}

	recipient := testAgent("a_123abc", mustTime(t, "2026-06-22T10:00:02Z"))
	if err := st.WriteAgent(recipient); err != nil {
		t.Fatalf("WriteAgent recipient: %v", err)
	}
	msg := Message{
		FromAgent: agent.AgentID,
		ToAgent:   recipient.AgentID,
		Body:      "Please review auth.",
		CreatedAt: mustTime(t, "2026-06-22T10:00:03Z"),
		ReadAt:    &read,
	}
	id, err := st.WriteMessage(msg)
	if err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}
	msg.ID = id
	if got, err := st.ReadMessage(id); err != nil || !reflect.DeepEqual(got, msg) {
		t.Fatalf("ReadMessage round-trip: got %+v err %v, want %+v", got, err, msg)
	}
	messages, err := st.ListMessages(recipient.AgentID)
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if !reflect.DeepEqual(messages, []Message{msg}) {
		t.Fatalf("messages = %+v, want %+v", messages, []Message{msg})
	}
}

func TestListEmptyAndReadNotFound(t *testing.T) {
	st, _ := newTestStore(t)
	if agents, err := st.ListAgents(); err != nil || len(agents) != 0 {
		t.Fatalf("ListAgents empty = %+v err %v, want [] nil", agents, err)
	}
	if running, err := st.ListRunning(); err != nil || len(running) != 0 {
		t.Fatalf("ListRunning empty = %+v err %v, want [] nil", running, err)
	}
	if statuses, err := st.ListStatus(); err != nil || len(statuses) != 0 {
		t.Fatalf("ListStatus empty = %+v err %v, want [] nil", statuses, err)
	}
	if messages, err := st.ListMessages("a_nope"); err != nil || len(messages) != 0 {
		t.Fatalf("ListMessages empty = %+v err %v, want [] nil", messages, err)
	}

	if _, err := st.ReadAgent("a_nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ReadAgent missing err = %v, want ErrNotFound", err)
	}
	if _, err := st.ReadRunning("a_nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ReadRunning missing err = %v, want ErrNotFound", err)
	}
	if _, err := st.ReadStatus("a_nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ReadStatus missing err = %v, want ErrNotFound", err)
	}
	if _, err := st.ReadMessage(123); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ReadMessage missing err = %v, want ErrNotFound", err)
	}
}

func TestNewAgentIDFormatUniquenessAndCollisionRetry(t *testing.T) {
	st, _ := newTestStore(t)
	re := regexp.MustCompile(`^a_[0-9a-f]{6}$`)
	seen := make(map[string]bool, 1000)
	for i := 0; i < 1000; i++ {
		id, err := st.NewAgentID()
		if err != nil {
			t.Fatalf("NewAgentID: %v", err)
		}
		if !re.MatchString(id) {
			t.Fatalf("NewAgentID = %q, want ^a_[0-9a-f]{6}$", id)
		}
		if seen[id] {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = true
	}

	originalRandRead := randRead
	defer func() { randRead = originalRandRead }()
	calls := 0
	randRead = func(b []byte) (int, error) {
		calls++
		if calls == 1 {
			copy(b, []byte{0x8f, 0x3c, 0x12})
			return len(b), nil
		}
		copy(b, []byte{0xab, 0xcd, 0xef})
		return len(b), nil
	}

	if err := st.WriteAgent(testAgent("a_8f3c12", mustTime(t, "2026-06-22T10:00:00Z"))); err != nil {
		t.Fatalf("WriteAgent collision row: %v", err)
	}
	id, err := st.NewAgentID()
	if err != nil {
		t.Fatalf("NewAgentID after collision: %v", err)
	}
	if id != "a_abcdef" {
		t.Fatalf("NewAgentID after collision = %q, want a_abcdef", id)
	}
	if calls != 2 {
		t.Fatalf("randRead calls = %d, want 2", calls)
	}
}

func TestDeleteAgentCascades(t *testing.T) {
	st, _ := newTestStore(t)
	agent := testAgent("a_8f3c12", mustTime(t, "2026-06-22T10:00:00Z"))
	recipient := testAgent("a_123abc", mustTime(t, "2026-06-22T10:00:01Z"))
	for _, a := range []Agent{agent, recipient} {
		if err := st.WriteAgent(a); err != nil {
			t.Fatalf("WriteAgent: %v", err)
		}
	}
	if err := st.WriteRunning(RunningEntry{
		AgentID: agent.AgentID, PID: 1, SessionID: "s", Interface: "chat",
		StartedAt: mustTime(t, "2026-06-22T10:00:02Z"),
	}); err != nil {
		t.Fatalf("WriteRunning: %v", err)
	}
	if err := st.WriteStatus(Status{AgentID: agent.AgentID, State: "idle"}); err != nil {
		t.Fatalf("WriteStatus: %v", err)
	}
	if _, err := st.WriteMessage(Message{
		FromAgent: agent.AgentID, ToAgent: recipient.AgentID, Body: "hello",
		CreatedAt: mustTime(t, "2026-06-22T10:00:03Z"),
	}); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	if err := st.DeleteAgent(agent.AgentID); err != nil {
		t.Fatalf("DeleteAgent: %v", err)
	}
	if _, err := st.ReadRunning(agent.AgentID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ReadRunning after DeleteAgent err = %v, want ErrNotFound", err)
	}
	if _, err := st.ReadStatus(agent.AgentID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ReadStatus after DeleteAgent err = %v, want ErrNotFound", err)
	}
	messages, err := st.ListMessages(recipient.AgentID)
	if err != nil {
		t.Fatalf("ListMessages after DeleteAgent: %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("messages after DeleteAgent = %+v, want []", messages)
	}
}

func TestDeleteMethodsAreIdempotent(t *testing.T) {
	st, _ := newTestStore(t)
	if err := st.DeleteAgent("a_nope"); err != nil {
		t.Fatalf("DeleteAgent missing: %v", err)
	}
	if err := st.DeleteRunning("a_nope"); err != nil {
		t.Fatalf("DeleteRunning missing: %v", err)
	}
	if err := st.DeleteStatus("a_nope"); err != nil {
		t.Fatalf("DeleteStatus missing: %v", err)
	}
	if err := st.DeleteMessage(42); err != nil {
		t.Fatalf("DeleteMessage missing: %v", err)
	}
}
