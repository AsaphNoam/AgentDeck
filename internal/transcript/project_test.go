package transcript

import (
	"encoding/json"
	"testing"

	"github.com/agentdeck/agentdeck/internal/runtime"
)

func projectionEvent(t *testing.T, typ string, data any) runtime.Event {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal %s: %v", typ, err)
	}
	return runtime.Event{AgentID: "a_1", Seq: 7, Type: typ, Ts: "2026-08-22T10:00:00Z", Data: raw}
}

// sampleEvents supplies one populated event per normalized type. The exhaustive
// test below indexes it by runtime.AllEventTypes, so a new Ev* constant fails
// here until it has both a sample and a ProjectEvent rule (TS-01.R22, INV §2).
func sampleEvents(t *testing.T) map[string]runtime.Event {
	t.Helper()
	return map[string]runtime.Event{
		runtime.EvUserPrompt:    projectionEvent(t, runtime.EvUserPrompt, runtime.UserPromptData{Text: "  ship it  "}),
		runtime.EvAssistantText: projectionEvent(t, runtime.EvAssistantText, runtime.AssistantTextData{Delta: "on it"}),
		runtime.EvToolCall: projectionEvent(t, runtime.EvToolCall, runtime.ToolCallData{
			ToolCallID: "tc1", Name: "Bash", Args: json.RawMessage(`{"command":"ls"}`), Status: "pending"}),
		runtime.EvToolResult: projectionEvent(t, runtime.EvToolResult, runtime.ToolResultData{
			ToolCallID: "tc1", Status: "completed", Content: json.RawMessage(`"file.go"`), Error: "boom"}),
		runtime.EvDiff: projectionEvent(t, runtime.EvDiff, runtime.DiffData{
			ToolCallID: "tc1", Path: "main.go", OldText: "a", NewText: "b", Patch: "@@ -1 +1 @@"}),
		runtime.EvPermissionRequest: projectionEvent(t, runtime.EvPermissionRequest, runtime.PermissionRequestData{
			ToolCallID: "tc1", Name: "Bash", Reason: "runs a command"}),
		runtime.EvPermissionResolved: projectionEvent(t, runtime.EvPermissionResolved, runtime.PermissionResolvedData{
			ToolCallID: "tc1", Decision: "approve"}),
		runtime.EvSessionMeta:   projectionEvent(t, runtime.EvSessionMeta, runtime.SessionMetaData{Name: "alpha", Backend: "claude-acp"}),
		runtime.EvTurnEnd:       projectionEvent(t, runtime.EvTurnEnd, runtime.TurnEndData{StopReason: "end_turn", ContextPct: 0.4}),
		runtime.EvError:         projectionEvent(t, runtime.EvError, runtime.ErrorData{Scope: "protocol", Message: "handshake failed"}),
		runtime.EvBackendSwitch: projectionEvent(t, runtime.EvBackendSwitch, runtime.BackendSwitchData{From: "a", To: "b"}),
		runtime.EvAnnotation: projectionEvent(t, runtime.EvAnnotation, runtime.AnnotationData{
			Annotations:        []runtime.Annotation{{Seq: 3, Excerpt: "line", Instruction: "fix"}},
			OverallInstruction: "tidy up"}),
	}
}

// TestProjectEventCoversEveryType is the closed-registry guard: every current
// normalized event is classified as rendered or deliberately metadata-only, and
// none falls through to the unknown marker.
func TestProjectEventCoversEveryType(t *testing.T) {
	samples := sampleEvents(t)
	metadataOnly := map[string]bool{
		runtime.EvSessionMeta:   true,
		runtime.EvBackendSwitch: true,
	}
	for _, typ := range runtime.AllEventTypes {
		ev, ok := samples[typ]
		if !ok {
			t.Fatalf("%s has no sample event; add one with its ProjectEvent rule", typ)
		}
		p, err := ProjectEvent(ev)
		if err != nil {
			t.Fatalf("%s: project: %v", typ, err)
		}
		switch {
		case metadataOnly[typ]:
			if p.Disposition != DispositionMetadata {
				t.Errorf("%s: disposition = %q, want metadata", typ, p.Disposition)
			}
			if len(p.Parts) != 0 {
				t.Errorf("%s: metadata projection carries %d parts", typ, len(p.Parts))
			}
		default:
			if p.Disposition != DispositionText {
				t.Errorf("%s: disposition = %q, want text", typ, p.Disposition)
			}
			if len(p.Parts) == 0 {
				t.Errorf("%s: text projection carries no parts", typ)
			}
			if p.Role == "" {
				t.Errorf("%s: text projection has no role", typ)
			}
		}
		if p.Seq != ev.Seq || p.Type != ev.Type || p.Ts != ev.Ts {
			t.Errorf("%s: projection lost event identity: %+v", typ, p)
		}
	}
}

func TestProjectEventUnknownTypeIsExplicit(t *testing.T) {
	p, err := ProjectEvent(runtime.Event{Seq: 2, Type: "future_kind", Data: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatalf("project unknown: %v", err)
	}
	if p.Disposition != DispositionUnknown {
		t.Fatalf("disposition = %q, want unknown", p.Disposition)
	}
	if p.SearchText() != "" {
		t.Fatalf("unknown event contributed search text %q", p.SearchText())
	}
}

// TestProjectEventSearchText pins the exact search bag each type contributed
// before the projection was extracted, so routing the indexer through it is a
// refactor rather than a silent change to archive search.
func TestProjectEventSearchText(t *testing.T) {
	samples := sampleEvents(t)
	want := map[string]string{
		runtime.EvUserPrompt:         "ship it",
		runtime.EvAssistantText:      "on it",
		runtime.EvToolCall:           `Bash {"command":"ls"}`,
		runtime.EvToolResult:         `"file.go"`,
		runtime.EvDiff:               "main.go b",
		runtime.EvPermissionRequest:  "runs a command",
		runtime.EvPermissionResolved: "",
		runtime.EvSessionMeta:        "",
		runtime.EvTurnEnd:            "",
		runtime.EvError:              "",
		runtime.EvBackendSwitch:      "",
		runtime.EvAnnotation:         "line fix tidy up",
	}
	for typ, expected := range want {
		p, err := ProjectEvent(samples[typ])
		if err != nil {
			t.Fatalf("%s: project: %v", typ, err)
		}
		if got := p.SearchText(); got != expected {
			t.Errorf("%s: search text = %q, want %q", typ, got, expected)
		}
	}
}

func TestProjectEventRejectsMalformedPayload(t *testing.T) {
	_, err := ProjectEvent(runtime.Event{Seq: 4, Type: runtime.EvToolCall, Data: json.RawMessage(`not json`)})
	if err == nil {
		t.Fatal("expected a decode error for a malformed tool_call payload")
	}
}
