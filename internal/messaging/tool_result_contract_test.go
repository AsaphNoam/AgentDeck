package messaging

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/agentdeck/agentdeck/internal/pipeline"
)

// FS-17.A2: the classifier is closed over the specified refusal vocabulary and
// fails safely for a newly emitted code until the guard is updated.
func TestRefusalRetryClasses(t *testing.T) {
	emitted := map[string]bool{}
	literal := regexp.MustCompile(`"error"\s*:\s*"([^"]+)"`)
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if filepath.Ext(file) != ".go" || filepath.Base(file) == "tool_result_contract_test.go" ||
			len(file) >= 8 && file[len(file)-8:] == "_test.go" {
			continue
		}
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range literal.FindAllSubmatch(source, -1) {
			emitted[string(match[1])] = true
		}
	}
	// pipelineToolError forwards these control-plane codes dynamically.
	for _, code := range []string{"assignment_unknown", "stale_assignment", "validation_failed"} {
		emitted[code] = true
	}
	for code := range emitted {
		if _, ok := refusalRetryClasses[code]; !ok {
			t.Errorf("emitted refusal %q has no retry classification", code)
		}
	}
	if got := classifyRetry("new_unclassified_refusal"); got != retryTransient {
		t.Fatalf("unclassified refusal = %q, want %q", got, retryTransient)
	}
}

func TestPipelineRefusalsUseDeclaredRetryClasses(t *testing.T) {
	cases := map[string]retryClass{
		"assignment_unknown": retryNever,
		"stale_assignment":   retryNever,
		"validation_failed":  retryAfterChange,
	}
	for code, want := range cases {
		result, _, err := pipelineToolError(&pipeline.ControlError{Code: code, Message: "refused"})
		if err != nil {
			t.Fatal(err)
		}
		object := result.StructuredContent.(map[string]any)
		retry := object["retry"].(map[string]any)
		if got := retry["class"]; got != string(want) {
			t.Errorf("%s retry class = %v, want %s", code, got, want)
		}
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

// FS-17 §6: malformed arguments rejected by the pinned SDK never reach an
// AgentDeck handler and therefore retain the SDK's plain-text error boundary.
func TestSDKArgumentRejectionIsOutsideResultContract(t *testing.T) {
	srv := New(newStore(t), nil)
	cs := connect(t, srv, "unknown-token")
	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: "check_messages", Arguments: map[string]any{"limit": "many"}})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError || res.StructuredContent != nil {
		t.Fatalf("SDK argument refusal = %+v", res)
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
