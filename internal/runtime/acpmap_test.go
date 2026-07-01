package runtime

import (
	"encoding/json"
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
