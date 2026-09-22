package runtime

import (
	"encoding/json"
	"strconv"

	"github.com/agentdeck/agentdeck/internal/strutil"
)

// Reasoning is live-only runtime activity (FS-03.R57, TS-01.R35, TS-04.R63):
// it has a generation, optional activity scope and a span id but no transcript
// sequence, so it never reaches the writer, SQLite, FTS, tracking, reindex or
// SSE gap recovery.

const (
	ActivityReasoningDelta = "reasoning_delta"
	// maxReasoningDelta bounds one chunk before fan-out (INV §16).
	maxReasoningDelta = 16 << 10
)

// ActivityNotice is the ephemeral `runtime_activity` SSE payload (TS-03.R44).
type ActivityNotice struct {
	AgentID    string `json:"agent_id"`
	Generation string `json:"generation"`
	ActivityID string `json:"activity_id,omitempty"`
	SpanID     string `json:"span_id"`
	Kind       string `json:"kind"`
	Delta      string `json:"delta"`
}

// SetActivitySink mirrors live-only runtime activity into an external bus.
func (c *ChatRuntime) SetActivitySink(sink func(ActivityNotice)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activitySink = sink
}

// decodeThoughtChunk reads an `agent_thought_chunk` text block. Anything else,
// or an empty chunk, is not reasoning.
func decodeThoughtChunk(params json.RawMessage) (string, bool) {
	var su acpSessionUpdate
	if json.Unmarshal(params, &su) != nil || su.Update.SessionUpdate != "agent_thought_chunk" {
		return "", false
	}
	var block acpContentBlock
	if json.Unmarshal(su.Update.Content, &block) != nil || block.Type != "text" || block.Text == "" {
		return "", true
	}
	return strutil.ClipRunes(block.Text, maxReasoningDelta), true
}

// reasoningSpan returns the activity's current span id, opening a new one when
// that activity's previous update was not reasoning.
func (as *agentState) reasoningSpan(activityID string) string {
	as.mu.Lock()
	defer as.mu.Unlock()
	if as.spanOpen == nil {
		as.spanOpen = map[string]string{}
	}
	span := as.spanOpen[activityID]
	if span == "" {
		as.spans++
		span = "r" + strconv.Itoa(as.spans)
		as.spanOpen[activityID] = span
	}
	return span
}

// closeReasoningSpan ends the activity's open span; its next thought starts a
// new one.
func (as *agentState) closeReasoningSpan(activityID string) {
	as.mu.Lock()
	delete(as.spanOpen, activityID)
	as.mu.Unlock()
}

func (c *ChatRuntime) publishReasoning(as *agentState, activityID, delta string) {
	c.mu.Lock()
	sink := c.activitySink
	c.mu.Unlock()
	if sink == nil || delta == "" {
		return
	}
	sink(ActivityNotice{
		AgentID: as.agentID, Generation: as.generation, ActivityID: activityID,
		SpanID: as.reasoningSpan(activityID), Kind: ActivityReasoningDelta, Delta: delta,
	})
}
