package transcript

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/runtime"
)

func meta() *runtime.SessionMetaData {
	return &runtime.SessionMetaData{
		Name:            "Atlas",
		Role:            "implementer",
		Project:         "my-app",
		Backend:         "claude",
		Model:           "sonnet-4-6",
		Interface:       "chat",
		Cwd:             "/tmp/my-app",
		SystemPromptSHA: "sha256:abc",
		EnvKeys:         []string{"OPENAI_BASE_URL"},
		CreatedAt:       "2026-06-28T00:00:00Z",
	}
}

func event(t *testing.T, seq int64, typ string, data any) runtime.Event {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal data: %v", err)
	}
	return runtime.Event{
		AgentID: "a_test",
		Seq:     seq,
		Type:    typ,
		Data:    raw,
		Ts:      time.Date(2026, 6, 28, 12, 0, int(seq), 0, time.UTC).Format(time.RFC3339),
	}
}

func TestAppendReadRoundTrip(t *testing.T) {
	home := t.TempDir()
	w, err := Open(home, "a_test", meta())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if got := w.NextSeq(); got != 1 {
		t.Fatalf("NextSeq = %d, want 1", got)
	}
	if err := w.Append(event(t, 1, runtime.EvAssistantText, runtime.AssistantTextData{Delta: "hello"})); err != nil {
		t.Fatalf("Append text: %v", err)
	}
	if err := w.Append(event(t, 2, runtime.EvPermissionResolved, runtime.PermissionResolvedData{ToolCallID: "tc_1", Decision: "approve"})); err != nil {
		t.Fatalf("Append permission_resolved: %v", err)
	}
	if err := w.Append(event(t, 3, runtime.EvTurnEnd, runtime.TurnEndData{StopReason: "end_turn"})); err != nil {
		t.Fatalf("Append turn_end: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	events, err := ReadFile(home, "a_test", ReadOptions{})
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("events len = %d, want 3", len(events))
	}
	if events[0].Seq != 1 || events[0].Type != runtime.EvAssistantText {
		t.Fatalf("first event = %+v, want seq 1 assistant_text", events[0])
	}
	if events[1].Type != runtime.EvPermissionResolved {
		t.Fatalf("second event type = %q, want permission_resolved", events[1].Type)
	}

	withMeta, err := ReadFile(home, "a_test", ReadOptions{IncludeMeta: true})
	if err != nil {
		t.Fatalf("ReadFile include meta: %v", err)
	}
	if len(withMeta) != 4 || withMeta[0].Type != runtime.EvSessionMeta || withMeta[0].Seq != 0 {
		t.Fatalf("withMeta[0] = %+v len=%d, want seq 0 session_meta plus events", withMeta[0], len(withMeta))
	}

	since, err := ReadFile(home, "a_test", ReadOptions{SinceSeq: 1})
	if err != nil {
		t.Fatalf("ReadFile since: %v", err)
	}
	if len(since) != 2 || since[0].Seq != 2 {
		t.Fatalf("since = %+v, want seqs > 1", since)
	}
}

func TestReopenContinuesSeq(t *testing.T) {
	home := t.TempDir()
	w, err := Open(home, "a_test", meta())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := w.Append(event(t, 0, runtime.EvAssistantText, runtime.AssistantTextData{Delta: "one"})); err != nil {
		t.Fatalf("Append auto seq: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	w, err = Open(home, "a_test", meta())
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if got := w.NextSeq(); got != 2 {
		t.Fatalf("reopen NextSeq = %d, want 2", got)
	}
	if err := w.Append(event(t, 0, runtime.EvAssistantText, runtime.AssistantTextData{Delta: "two"})); err != nil {
		t.Fatalf("Append after reopen: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close reopen: %v", err)
	}
	events, err := ReadFile(home, "a_test", ReadOptions{})
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(events) != 2 || events[0].Seq != 1 || events[1].Seq != 2 {
		t.Fatalf("events seq = %+v, want 1,2", events)
	}
}

func TestReaderSkipsPartialTrailingAndBadMiddleLine(t *testing.T) {
	home := t.TempDir()
	w, err := Open(home, "a_test", meta())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := w.Append(event(t, 1, runtime.EvAssistantText, runtime.AssistantTextData{Delta: "before"})); err != nil {
		t.Fatalf("Append before: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	f, err := os.OpenFile(w.Path(), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("OpenFile append: %v", err)
	}
	if _, err := f.WriteString("{bad json}\n"); err != nil {
		t.Fatalf("write bad line: %v", err)
	}
	good := event(t, 2, runtime.EvAssistantText, runtime.AssistantTextData{Delta: "after"})
	b, _ := json.Marshal(good)
	if _, err := f.Write(append(b, '\n')); err != nil {
		t.Fatalf("write good line: %v", err)
	}
	if _, err := f.WriteString(`{"agent_id":"a_test","seq":3,"type":"assistant_text"`); err != nil {
		t.Fatalf("write partial line: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close append file: %v", err)
	}

	events, err := ReadFile(home, "a_test", ReadOptions{})
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(events) != 2 || events[0].Seq != 1 || events[1].Seq != 2 {
		t.Fatalf("events = %+v, want valid seq 1 and 2 only", events)
	}
}

func TestLargeLineRoundTrips(t *testing.T) {
	home := t.TempDir()
	w, err := Open(home, "a_test", meta())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	large := strings.Repeat("x", 128*1024)
	if err := w.Append(event(t, 1, runtime.EvDiff, runtime.DiffData{
		ToolCallID: "tc_1",
		Path:       "big.txt",
		NewText:    large,
		Patch:      large,
	})); err != nil {
		t.Fatalf("Append large: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	events, err := ReadFile(home, "a_test", ReadOptions{})
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events len = %d, want 1", len(events))
	}
	var diff runtime.DiffData
	if err := json.Unmarshal(events[0].Data, &diff); err != nil {
		t.Fatalf("unmarshal diff: %v", err)
	}
	if len(diff.NewText) != len(large) || len(diff.Patch) != len(large) {
		t.Fatalf("large diff lengths = %d/%d, want %d", len(diff.NewText), len(diff.Patch), len(large))
	}
}
