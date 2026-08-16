package runtime

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

// Regression (review fix): a tool_call_update must only produce a tool_result on a
// TERMINAL status (completed/failed). An in-progress update — or one that omits
// status — previously defaulted to "completed", flipping the tool to done early
// and repeating tool_results in the transcript.
func TestMapToolCallUpdateOnlyTerminalStatusEmitsResult(t *testing.T) {
	countResults := func(raw string) int {
		n := 0
		for _, e := range mapSessionUpdate(json.RawMessage(raw)) {
			if e.Type == EvToolResult {
				n++
			}
		}
		return n
	}

	cases := []struct {
		name string
		raw  string
		want int
	}{
		{"in_progress", `{"update":{"sessionUpdate":"tool_call_update","toolCallId":"tc1","status":"in_progress"}}`, 0},
		{"no_status", `{"update":{"sessionUpdate":"tool_call_update","toolCallId":"tc1"}}`, 0},
		{"completed", `{"update":{"sessionUpdate":"tool_call_update","toolCallId":"tc1","status":"completed"}}`, 1},
		{"failed", `{"update":{"sessionUpdate":"tool_call_update","toolCallId":"tc1","status":"failed"}}`, 1},
	}
	for _, c := range cases {
		if got := countResults(c.raw); got != c.want {
			t.Errorf("%s: %d tool_result(s), want %d", c.name, got, c.want)
		}
	}
}

// A diff block still streams on an in-progress update even though no tool_result
// is emitted for it.
func TestMapToolCallUpdateEmitsDiffOnInProgress(t *testing.T) {
	raw := `{"update":{"sessionUpdate":"tool_call_update","toolCallId":"tc1","status":"in_progress",
	  "content":[{"type":"diff","path":"a.go","oldText":"x","newText":"y"}]}}`
	var diffs, results int
	for _, e := range mapSessionUpdate(json.RawMessage(raw)) {
		switch e.Type {
		case EvDiff:
			diffs++
		case EvToolResult:
			results++
		}
	}
	if diffs != 1 || results != 0 {
		t.Fatalf("in-progress diff update: diffs=%d results=%d, want diffs=1 results=0", diffs, results)
	}
}

// TestContextUsageFromRealClaudeAdapterShapes is the reproduction for the field
// bug "the context bar is broken, just stuck at 0%" (HANDOFF review finding;
// governs FS-02.R26 and the ACP→context_pct mapping under TS-04). Committed
// SKIPPED so the tree stays green; the /fix session un-skips it as the
// regression.
//
// The pinned Claude adapter 0.59.0 reports context usage in two places, and
// AgentDeck reads neither correctly:
//
//  1. The session/prompt result `usage` object is
//     {inputTokens,outputTokens,cachedReadTokens,cachedWriteTokens,totalTokens}
//     — it has no `used` and no `window`. mapPromptResult reads
//     r.Usage.Used/r.Usage.Window, so both decode to 0, the Window>0 guard is
//     false, and context_pct stays 0 for every real turn.
//  2. The adapter also emits a session/update notification
//     {sessionUpdate:"usage_update", used:<tokens>, size:<contextWindow>} — the
//     actual context channel — which mapSessionUpdate drops (default → nil).
//
// fakeacp emits the AgentDeck-invented {used,window} shape, so the existing
// chat/event tests pass against a double that does not match the real adapter
// (INV §11). This test uses the adapter's true shapes and must fail against the
// current mapping.
func TestContextUsageFromRealClaudeAdapterShapes(t *testing.T) {
	t.Skip("reproduction for context-bar-stuck-at-0pct; un-skip in the fix (FS-02.R26)")

	// (1) Real prompt-result usage shape: token counts only, no used/window.
	result := `{"stopReason":"end_turn","usage":{"inputTokens":42000,"outputTokens":800,` +
		`"cachedReadTokens":1200,"cachedWriteTokens":0,"totalTokens":44000}}`
	td, hasPct := mapPromptResult(json.RawMessage(result))
	if !hasPct || td.ContextPct <= 0 {
		t.Fatalf("prompt-result usage produced context_pct=%v hasPct=%v, want a populated non-zero fraction", td.ContextPct, hasPct)
	}

	// (2) Real usage_update session notification: used + size (context window).
	raw := `{"sessionId":"s","update":{"sessionUpdate":"usage_update","used":44000,"size":200000}}`
	if len(mapSessionUpdate(json.RawMessage(raw))) == 0 {
		t.Fatalf("usage_update notification produced no event; the real context channel is dropped")
	}
}

// commandsUpdateParams builds an available_commands_update session/update params
// with a raw availableCommands array (TS-04.R24).
func commandsUpdateParams(commandsJSON string) json.RawMessage {
	return json.RawMessage(`{"sessionId":"s","update":{"sessionUpdate":"available_commands_update","availableCommands":` + commandsJSON + `}}`)
}

// TestDecodeAvailableCommands covers the ACP command decode boundary (TS-04.R24):
// valid mapping incl. the optional input hint, skipping nameless entries without
// losing valid ones (INV §7), rune-clamping (INV §8), the entry cap, empty-clears,
// and ok=false for a non-command update so mapping falls through.
func TestDecodeAvailableCommands(t *testing.T) {
	items, ok := decodeAvailableCommands(commandsUpdateParams(
		`[{"name":"review","description":"Review code","input":{"hint":"branch"}},{"description":"no name"},{"name":"$plan","description":"Plan"}]`))
	if !ok {
		t.Fatalf("ok = false, want true for available_commands_update")
	}
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2 (nameless skipped); got %+v", len(items), items)
	}
	if items[0].Name != "review" || items[0].Description != "Review code" || items[0].InputHint != "branch" {
		t.Fatalf("item[0] = %+v, want review/Review code/branch", items[0])
	}
	if items[1].Name != "$plan" || items[1].InputHint != "" {
		t.Fatalf("item[1] = %+v, want $plan with no hint", items[1])
	}

	// Empty update returns an empty (non-nil) slice that clears the snapshot.
	empty, ok := decodeAvailableCommands(commandsUpdateParams(`[]`))
	if !ok || empty == nil || len(empty) != 0 {
		t.Fatalf("empty update: ok=%v items=%v, want ok=true, empty non-nil", ok, empty)
	}

	// A non-command update is not recognized so mapping proceeds normally.
	if _, ok := decodeAvailableCommands(json.RawMessage(`{"update":{"sessionUpdate":"agent_message_chunk"}}`)); ok {
		t.Fatalf("ok = true for agent_message_chunk, want false")
	}

	// Rune clamping and the entry cap.
	long := `[{"name":"` + strings.Repeat("n", 300) + `","description":"` + strings.Repeat("d", 3000) + `"}]`
	clamped, _ := decodeAvailableCommands(commandsUpdateParams(long))
	if len([]rune(clamped[0].Name)) != maxCommandName || len([]rune(clamped[0].Description)) != maxCommandText {
		t.Fatalf("clamp: name=%d desc=%d, want %d/%d", len([]rune(clamped[0].Name)), len([]rune(clamped[0].Description)), maxCommandName, maxCommandText)
	}
	var many []string
	for i := 0; i < maxCommandEntries+10; i++ {
		many = append(many, `{"name":"c`+strconv.Itoa(i)+`","description":""}`)
	}
	capped, _ := decodeAvailableCommands(commandsUpdateParams(`[` + strings.Join(many, ",") + `]`))
	if len(capped) != maxCommandEntries {
		t.Fatalf("cap: items=%d, want %d", len(capped), maxCommandEntries)
	}
}

// TestAvailableCommandsSnapshotReplaceOnly proves the runtime snapshot is
// replace-only live state (TS-04.R24): each update overwrites the prior list, an
// empty update clears it, and an unknown agent yields ErrNoHandle.
func TestAvailableCommandsSnapshotReplaceOnly(t *testing.T) {
	c := NewChatRuntime(nil)
	as := &agentState{agentID: "a1"}
	c.agents["a1"] = as

	c.onNotification(as, "session/update", commandsUpdateParams(`[{"name":"review","description":"Review"}]`))
	got, err := c.Commands("a1")
	if err != nil || len(got) != 1 || got[0].Name != "review" {
		t.Fatalf("after first update: got=%+v err=%v, want [review]", got, err)
	}

	c.onNotification(as, "session/update", commandsUpdateParams(`[{"name":"plan","description":"Plan"}]`))
	got, _ = c.Commands("a1")
	if len(got) != 1 || got[0].Name != "plan" {
		t.Fatalf("after replace: got=%+v, want [plan] only (not merged)", got)
	}

	c.onNotification(as, "session/update", commandsUpdateParams(`[]`))
	got, _ = c.Commands("a1")
	if len(got) != 0 {
		t.Fatalf("after empty update: got=%+v, want cleared", got)
	}

	if _, err := c.Commands("nope"); err == nil {
		t.Fatalf("Commands(unknown) err = nil, want ErrNoHandle")
	}
}
