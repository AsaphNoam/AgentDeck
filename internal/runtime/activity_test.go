package runtime

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
)

// Reasoning streams to the live-only sink in chronological spans and never
// becomes a sequenced transcript event (FS-03.A39, TS-04.R63).
func TestThoughtChunksAreLiveOnlyReasoningSpans(t *testing.T) {
	c, spec := newChatTest(t, "thought_stream")
	var mu sync.Mutex
	var notices []ActivityNotice
	c.SetActivitySink(func(n ActivityNotice) {
		mu.Lock()
		notices = append(notices, n)
		mu.Unlock()
	})
	ctx := context.Background()
	h, err := c.Start(ctx, spec)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = c.Stop(ctx, h.AgentID) })
	ch, unsub, err := c.Subscribe(h.AgentID)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	t.Cleanup(unsub)
	if err := c.SendPrompt(ctx, h.AgentID, "think"); err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}
	evs := drainTurn(t, ch)

	var answer string
	for _, ev := range evs {
		if ev.Type == EvAssistantText {
			var d AssistantTextData
			_ = json.Unmarshal(ev.Data, &d)
			answer += d.Delta
		}
	}
	if answer != "Answer.Done." {
		t.Fatalf("transcript text = %q; reasoning must not enter it", answer)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(notices) != 3 {
		t.Fatalf("notices = %+v, want 3 non-empty deltas", notices)
	}
	for _, n := range notices {
		if n.AgentID != h.AgentID || n.Kind != ActivityReasoningDelta || n.Generation != spec.Generation || n.ActivityID != "" {
			t.Fatalf("notice = %+v", n)
		}
	}
	if notices[0].SpanID != notices[1].SpanID || notices[1].SpanID == notices[2].SpanID {
		t.Fatalf("spans = %q %q %q, want one span per contiguous run", notices[0].SpanID, notices[1].SpanID, notices[2].SpanID)
	}
	if notices[0].Delta+notices[1].Delta != "Consider the plan." {
		t.Fatalf("first span text = %q", notices[0].Delta+notices[1].Delta)
	}
}
