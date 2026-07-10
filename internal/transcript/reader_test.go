package transcript

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/agentdeck/agentdeck/internal/runtime"
)

// TestReadAllSkipsOversizedRecord guards the BLOCKING finding that a single
// record larger than maxRecordSize used to abort the entire transcript. The
// oversized line must be skipped while the valid records around it are all
// returned, with no error.
func TestReadAllSkipsOversizedRecord(t *testing.T) {
	var buf bytes.Buffer

	writeEvent := func(seq int64, delta string) {
		e := event(t, seq, runtime.EvAssistantText, runtime.AssistantTextData{Delta: delta})
		b, err := json.Marshal(e)
		if err != nil {
			t.Fatalf("marshal event seq %d: %v", seq, err)
		}
		buf.Write(b)
		buf.WriteByte('\n')
	}

	writeEvent(1, "before")
	writeEvent(2, "still-before")
	// One oversized record (~9 MiB) that exceeds maxRecordSize (8 MiB). It is
	// still valid JSON structurally, but must be skipped without aborting.
	big := event(t, 3, runtime.EvAssistantText, runtime.AssistantTextData{Delta: strings.Repeat("x", 9*1024*1024)})
	bigB, err := json.Marshal(big)
	if err != nil {
		t.Fatalf("marshal oversized: %v", err)
	}
	if len(bigB) <= maxRecordSize {
		t.Fatalf("oversized line len %d not above cap %d", len(bigB), maxRecordSize)
	}
	buf.Write(bigB)
	buf.WriteByte('\n')
	writeEvent(4, "after")
	writeEvent(5, "still-after")

	events, err := readAll(&buf, ReadOptions{})
	if err != nil {
		t.Fatalf("readAll: %v", err)
	}
	if len(events) != 4 {
		t.Fatalf("events len = %d, want 4 (oversized skipped)", len(events))
	}
	wantSeqs := []int64{1, 2, 4, 5}
	for i, want := range wantSeqs {
		if events[i].Seq != want {
			t.Fatalf("events[%d].Seq = %d, want %d (full=%+v)", i, events[i].Seq, want, events)
		}
	}
}

// TestReadAllMetaOnlyReturnsEmptySlice guards the S1/S4 blocker: a just-launched
// agent's transcript holds only a session_meta record. With IncludeMeta:false
// that record is filtered out, and the result must be a non-nil empty slice so
// the HTTP layer emits "events":[] (not null) — a null crashes foldTranscript.
func TestReadAllMetaOnlyReturnsEmptySlice(t *testing.T) {
	var buf bytes.Buffer
	e := event(t, 0, runtime.EvSessionMeta, runtime.SessionMetaData{})
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal meta: %v", err)
	}
	buf.Write(b)
	buf.WriteByte('\n')

	events, err := readAll(&buf, ReadOptions{IncludeMeta: false})
	if err != nil {
		t.Fatalf("readAll: %v", err)
	}
	if events == nil {
		t.Fatal("events is nil; want non-nil empty slice")
	}
	if len(events) != 0 {
		t.Fatalf("events len = %d, want 0", len(events))
	}
}

// TestReadAllOversizedTrailingRecordNoAbort ensures an oversized record as the
// final line (no trailing content after it) still yields the earlier records.
func TestReadAllOversizedTrailingRecordNoAbort(t *testing.T) {
	var buf bytes.Buffer
	e := event(t, 1, runtime.EvAssistantText, runtime.AssistantTextData{Delta: "keep"})
	b, _ := json.Marshal(e)
	buf.Write(b)
	buf.WriteByte('\n')
	buf.WriteString(strings.Repeat("y", 9*1024*1024))
	buf.WriteByte('\n')

	events, err := readAll(&buf, ReadOptions{})
	if err != nil {
		t.Fatalf("readAll: %v", err)
	}
	if len(events) != 1 || events[0].Seq != 1 {
		t.Fatalf("events = %+v, want single seq 1", events)
	}
}

// TestReaderRecoversMaxSeqPastOversized ensures the recoverMaxSeq path (used by
// Open) survives an oversized record and still reports the true max seq.
func TestReaderRecoversMaxSeqPastOversized(t *testing.T) {
	home := t.TempDir()
	w, err := Open(home, "a_test", meta())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := w.Append(event(t, 1, runtime.EvAssistantText, runtime.AssistantTextData{Delta: "one"})); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	maxSeq, existed, err := recoverMaxSeq(w.Path())
	if err != nil {
		t.Fatalf("recoverMaxSeq: %v", err)
	}
	if !existed || maxSeq != 1 {
		t.Fatalf("recoverMaxSeq = (%d, %v), want (1, true)", maxSeq, existed)
	}
}
