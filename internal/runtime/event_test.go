package runtime

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestPayloadRoundTrip asserts every *Data payload struct survives a
// JSON marshal→unmarshal cycle unchanged. These payloads are the frozen
// contract Phase 2 (streaming) and Phase 4 (persistence) consume, so a silent
// field drop here would be a cross-phase break.
func TestPayloadRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		val  any
	}{
		{"assistant_text", AssistantTextData{Delta: "Sure, I'll "}},
		{"tool_call", ToolCallData{
			ToolCallID: "tc_42", Name: "Edit", Title: "Edit main.go",
			Args: json.RawMessage(`{"path":"main.go"}`), Status: "in_progress",
		}},
		{"tool_result", ToolResultData{
			ToolCallID: "tc_42", Status: "completed",
			Content: json.RawMessage(`{"ok":true}`), Error: "",
		}},
		{"tool_result_failed", ToolResultData{
			ToolCallID: "tc_43", Status: "failed",
			Content: json.RawMessage(`null`), Error: "boom",
		}},
		{"diff", DiffData{
			ToolCallID: "tc_42", Path: "main.go",
			OldText: "a", NewText: "b", Patch: "@@ -1 +1 @@\n-a\n+b\n",
		}},
		{"permission_request", PermissionRequestData{
			ToolCallID: "tc_42", Name: "Bash", Reason: "run a command",
			Args:         json.RawMessage(`{"cmd":"ls"}`),
			Options:      []PermOption{{OptionID: "o1", Label: "Allow", Kind: "allow_once"}},
			AutoApproved: false, ExpiresAt: "2026-06-22T10:04:12Z",
		}},
		{"turn_end", TurnEndData{StopReason: "end_turn", ContextPct: 0.42}},
		{"error", ErrorData{Scope: "process", Message: "crashed", Fatal: true}},
		{"event_envelope", Event{
			AgentID: "a_8f3c12", Seq: 7, Type: EvAssistantText,
			Data: json.RawMessage(`{"delta":"hi"}`), Ts: "2026-06-22T10:04:12Z",
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(tc.val)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			// Round-trip into a fresh zero value of the same type.
			out := reflect.New(reflect.TypeOf(tc.val)).Interface()
			if err := json.Unmarshal(b, out); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			got := reflect.ValueOf(out).Elem().Interface()
			if !reflect.DeepEqual(tc.val, got) {
				t.Fatalf("round-trip mismatch:\n in:  %#v\n out: %#v", tc.val, got)
			}
		})
	}
}

// TestEventTypeConstants locks the wire string for each event type. These are
// part of the cross-phase contract (techspec §11) and must not drift silently.
func TestEventTypeConstants(t *testing.T) {
	want := map[string]string{
		EvAssistantText:     "assistant_text",
		EvToolCall:          "tool_call",
		EvToolResult:        "tool_result",
		EvDiff:              "diff",
		EvPermissionRequest: "permission_request",
		EvTurnEnd:           "turn_end",
		EvError:             "error",
	}
	for got, expect := range want {
		if got != expect {
			t.Errorf("event type constant = %q, want %q", got, expect)
		}
	}
}
