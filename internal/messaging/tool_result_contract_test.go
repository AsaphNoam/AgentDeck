package messaging

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// FS-17.A2: the classifier is closed over the specified refusal vocabulary and
// fails safely for a newly emitted code until the guard is updated.
func TestRefusalRetryClasses(t *testing.T) {
	want := map[string]retryClass{
		"validation": retryNever, "invalid_body": retryNever, "invalid_subject": retryNever,
		"invalid_outcome": retryNever, "invalid_state": retryNever, "invalid_cursor": retryNever,
		"dependency_cycle": retryNever, "target_ineligible": retryNever, "already_reported": retryNever,
		"not_assigned": retryNever, "not_creator": retryNever, "retry_requires_rearm": retryNever,
		"task_not_found": retryNever, "context_not_found": retryNever,
		"context_source_unavailable": retryNever, "proposal_forbidden": retryNever,
		"session_unknown":     retryNever,
		"ambiguous_recipient": retryAfterChange, "recipient_not_found": retryAfterChange,
		"source_unavailable":      retryAfterChange,
		"message_budget_exceeded": retryNextTurn,
		"internal":                retryTransient, "store_unavailable": retryTransient,
		"context_unavailable": retryTransient, "pipeline_unavailable": retryTransient,
	}
	if !reflect.DeepEqual(refusalRetryClasses, want) {
		t.Fatalf("refusal classifier = %#v, want %#v", refusalRetryClasses, want)
	}
	if got := classifyRetry("new_unclassified_refusal"); got != retryTransient {
		t.Fatalf("unclassified refusal = %q, want %q", got, retryTransient)
	}
}

// FS-17.A4/A5: both result helpers preserve their text encoding and mirror JSON
// objects into structuredContent. A non-object result still answers in text.
func TestResultHelpersMirrorStructuredContent(t *testing.T) {
	success := map[string]any{"ok": true, "value": "kept"}
	result, _, err := jsonResult(success)
	assertResultChannels(t, result, err, success, false)
	if _, exists := success["retry"]; exists {
		t.Fatal("jsonResult added retry to a successful payload")
	}

	refusal := map[string]any{"ok": false, "error": "message_budget_exceeded", "message": "stop"}
	result, _, err = errResult(refusal)
	wantRefusal := map[string]any{
		"ok": false, "error": "message_budget_exceeded", "message": "stop",
		"retry": map[string]any{"class": retryNextTurn},
	}
	assertResultChannels(t, result, err, wantRefusal, true)
	if _, exists := refusal["retry"]; exists {
		t.Fatal("errResult mutated its caller's payload")
	}

	result, _, err = jsonResult([]string{"text-only"})
	if err != nil {
		t.Fatalf("non-object jsonResult: %v", err)
	}
	if result.StructuredContent != nil {
		t.Fatalf("non-object structuredContent = %#v, want nil", result.StructuredContent)
	}
}

// FS-17.A4/A5: derive the surface from tools/list, prove every registered tool
// returns the shared refusal contract, and keep output schemas deferred.
func TestRegisteredToolsShareResultContract(t *testing.T) {
	srv := New(newStore(t), nil)
	cs := connect(t, srv, "unknown-token")
	listed, err := cs.ListTools(t.Context(), &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	if len(listed.Tools) == 0 {
		t.Fatal("tools/list returned no tools")
	}
	validArgs := map[string]map[string]any{
		"list_agents": {}, "send_message": {"to": "nobody", "body": "hello"},
		"check_messages":               {},
		"report_pipeline_stage_result": {"outcome": "success", "summary": "done"},
		"propose_pipeline_template": {"id": "sample", "template": map[string]any{
			"version": 1, "title": "sample", "inputs": []any{}, "stages": []any{},
		}},
		"propose_pipeline_run": {"run": map[string]any{
			"request_id": "request_1", "template_id": "sample", "display_name": "sample",
			"project": "sample", "goal": "sample", "inputs": map[string]any{},
			"assignments": map[string]any{},
		}},
		"get_assigned_task":           {},
		"create_task":                 {"display_name": "sample", "instruction": "do work"},
		"cancel_task":                 {"task_id": "task_1"},
		"report_task_result":          {"outcome": "success", "summary": "done"},
		"share_context":               {"to": "nobody", "source": "current_turn"},
		"list_context_links":          {},
		"read_context_link":           {"context_ref_id": "context_1"},
		"set_context_link_visibility": {"grant_id": "grant_1", "hidden": true},
		"revoke_context_grant":        {"grant_id": "grant_1"},
	}
	for _, tool := range listed.Tools {
		if tool.OutputSchema != nil {
			t.Errorf("%s advertises output schema %#v", tool.Name, tool.OutputSchema)
		}
		args, covered := validArgs[tool.Name]
		if !covered {
			t.Fatalf("registered tool %q is missing from the contract enumeration", tool.Name)
		}
		res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: tool.Name, Arguments: args})
		if err != nil {
			t.Fatalf("call %s: %v", tool.Name, err)
		}
		if !res.IsError {
			t.Errorf("%s unknown session IsError = false", tool.Name)
		}
		assertWireChannelsEqual(t, tool.Name, res)
		object := res.StructuredContent.(map[string]any)
		if object["error"] != "session_unknown" {
			t.Errorf("%s error = %#v", tool.Name, object["error"])
		}
		retry := object["retry"].(map[string]any)
		if retry["class"] != string(retryNever) {
			t.Errorf("%s retry = %#v", tool.Name, retry)
		}
	}
}

func assertResultChannels(t *testing.T, result *mcp.CallToolResult, err error, want map[string]any, isError bool) {
	t.Helper()
	if err != nil {
		t.Fatalf("result helper: %v", err)
	}
	if result.IsError != isError {
		t.Fatalf("IsError = %v, want %v", result.IsError, isError)
	}
	assertWireChannelsEqual(t, "helper", result)
	got := result.StructuredContent.(map[string]any)
	wantJSON, _ := json.Marshal(want)
	var normalized map[string]any
	_ = json.Unmarshal(wantJSON, &normalized)
	if !reflect.DeepEqual(got, normalized) {
		t.Fatalf("structuredContent = %#v, want %#v", got, normalized)
	}
}

func assertWireChannelsEqual(t *testing.T, name string, result *mcp.CallToolResult) {
	t.Helper()
	if len(result.Content) != 1 {
		t.Fatalf("%s content count = %d", name, len(result.Content))
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("%s content type = %T", name, result.Content[0])
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(text.Text), &decoded); err != nil {
		t.Fatalf("%s text JSON: %v", name, err)
	}
	if !reflect.DeepEqual(decoded, result.StructuredContent) {
		t.Fatalf("%s channels differ: text=%#v structured=%#v", name, decoded, result.StructuredContent)
	}
}
