package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/agentdeck/agentdeck/internal/pipeline"
)

func TestParsePipelineCLIValues(t *testing.T) {
	inputs, err := parseKeyValues([]string{"spec=Do it", "note=a=b"})
	if err != nil || !reflect.DeepEqual(inputs, map[string]string{"spec": "Do it", "note": "a=b"}) {
		t.Fatalf("inputs = %v err=%v", inputs, err)
	}
	assignments, err := parseStageAssignments([]string{"work=codex/gpt", "review=claude/sonnet"})
	if err != nil || assignments["work"] != (pipeline.RuntimeAssignment{Backend: "codex", Model: "gpt"}) {
		t.Fatalf("assignments = %v err=%v", assignments, err)
	}
	if _, err := parseStageAssignments([]string{"work=codex"}); err == nil {
		t.Fatal("invalid assignment succeeded")
	}
}

// FS-14.A1: CLI commands remain thin clients over the same REST contract.
func TestPipelineAPIRequestContract(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"run":{"run":{"run_id":"pr_test"}}}`))
	}))
	defer server.Close()
	portPart := strings.TrimPrefix(server.URL, "http://127.0.0.1:")
	var port int
	if _, err := fmt.Sscanf(portPart, "%d", &port); err != nil {
		t.Fatal(err)
	}
	status, _, err := pipelineAPIRequest(port, http.MethodPost, "/api/pipeline-runs", map[string]any{"request_id": "r1"})
	if err != nil || status != http.StatusCreated || gotMethod != http.MethodPost || gotPath != "/api/pipeline-runs" || gotBody["request_id"] != "r1" {
		t.Fatalf("request status=%d err=%v method=%s path=%s body=%v", status, err, gotMethod, gotPath, gotBody)
	}
}
