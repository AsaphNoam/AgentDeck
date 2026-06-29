//go:build sqlite_fts5

package index

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/agentdeck/agentdeck/internal/runtime"
)

func TestIndexerFTSMatch(t *testing.T) {
	st, _ := openTestDB(t)
	indexFixture(t, st.DB())

	var agentID string
	if err := st.DB().QueryRow(`SELECT agent_id FROM sessions_fts WHERE sessions_fts MATCH ?`, `"distinctive quartz"`).Scan(&agentID); err != nil {
		t.Fatalf("fts search: %v", err)
	}
	if agentID != "a_index" {
		t.Fatalf("fts agent = %q, want a_index", agentID)
	}
}

// TestIndexerFTSLongTranscript guards the BLOCKING finding that archive FTS used
// to drop transcript content beyond a 1 MiB cap (keeping only the newest bytes).
// An early distinctive phrase followed by >1 MiB of later content must remain
// searchable — proving complete transcript content is indexed.
func TestIndexerFTSLongTranscript(t *testing.T) {
	st, _ := openTestDB(t)
	ix := New(st.DB())

	m := meta()
	metaRaw, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal meta: %v", err)
	}
	if err := ix.OnEvent("a_index", runtime.Event{AgentID: "a_index", Seq: 0, Type: runtime.EvSessionMeta, Data: metaRaw, Ts: m.CreatedAt}); err != nil {
		t.Fatalf("OnEvent meta: %v", err)
	}

	// Seq 1: the early phrase that the old cap would have evicted.
	if err := ix.OnEvent("a_index", ev(t, 1, runtime.EvAssistantText, runtime.AssistantTextData{Delta: "earlymarker_zebra_phrase"})); err != nil {
		t.Fatalf("OnEvent early: %v", err)
	}
	// Then push well past 1 MiB of later, distinct content.
	filler := strings.Repeat("lorem ipsum dolor ", 4000) // ~72 KiB per event
	var seq int64 = 2
	for total := 0; total < (1 << 20 + 256<<10); total += len(filler) {
		if err := ix.OnEvent("a_index", ev(t, seq, runtime.EvAssistantText, runtime.AssistantTextData{Delta: filler})); err != nil {
			t.Fatalf("OnEvent filler seq %d: %v", seq, err)
		}
		seq++
	}
	if err := ix.OnEvent("a_index", ev(t, seq, runtime.EvTurnEnd, runtime.TurnEndData{StopReason: "end_turn", ContextPct: 0.9})); err != nil {
		t.Fatalf("OnEvent turn_end: %v", err)
	}
	if err := ix.OnTurnEnd("a_index", runtime.TurnRollup{LastSeq: seq, LastContextPct: 0.9, UpdatedAt: m.CreatedAt}); err != nil {
		t.Fatalf("OnTurnEnd: %v", err)
	}

	var agentID string
	if err := st.DB().QueryRow(`SELECT agent_id FROM sessions_fts WHERE sessions_fts MATCH ?`, `"earlymarker_zebra_phrase"`).Scan(&agentID); err != nil {
		t.Fatalf("early phrase no longer searchable after >1 MiB of later content: %v", err)
	}
	if agentID != "a_index" {
		t.Fatalf("fts agent = %q, want a_index", agentID)
	}
}
