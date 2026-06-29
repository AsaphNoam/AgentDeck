package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/index"
	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
)

// seedSessionForTracking creates an agent + sessions row + running row so file/command
// endpoints and hook capture have a valid target.
func seedSessionForTracking(t *testing.T, srv *Server) string {
	t.Helper()
	agentID := "a_fc_test"
	agent := state.Agent{
		AgentID: agentID, Name: "Atlas", Role: "implementer", Project: "my-app",
		Backend: "claude", Model: "sonnet-4-6", Interface: "chat",
		CreatedAt: time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC),
	}
	if err := srv.stateStore.WriteAgent(agent); err != nil {
		t.Fatalf("WriteAgent: %v", err)
	}
	// Upsert the sessions row so /files and /commands don't 404.
	ix := index.New(srv.stateStore.DB())
	meta := runtime.SessionMetaData{
		Name: "Atlas", Role: "implementer", Project: "my-app",
		Backend: "claude", Model: "sonnet-4-6", Interface: "chat",
		Cwd: "/workspace/my-app", CreatedAt: "2026-06-28T10:00:00Z", SessionID: "sess-fc",
	}
	if err := ix.UpsertSessionMeta(agentID, meta); err != nil {
		t.Fatalf("UpsertSessionMeta: %v", err)
	}
	// Running row for hook token validation.
	if err := srv.stateStore.WriteRunning(state.RunningEntry{
		AgentID: agentID, PID: 12345, SessionID: "sess-fc",
		Interface: "chat", HookToken: "tok_fc",
		StartedAt: time.Date(2026, 6, 28, 10, 0, 1, 0, time.UTC),
	}); err != nil {
		t.Fatalf("WriteRunning: %v", err)
	}
	return agentID
}

func TestFilesEndpointEmptyList(t *testing.T) {
	srv := testServer(t, false)
	seedSessionForTracking(t, srv)
	h := srv.routes()

	rec := doGET(t, h, "/api/sessions/a_fc_test/files")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	files, _ := resp["files"].([]any)
	if len(files) != 0 {
		t.Fatalf("expected empty files, got %d", len(files))
	}
}

func TestCommandsEndpointEmptyList(t *testing.T) {
	srv := testServer(t, false)
	seedSessionForTracking(t, srv)
	h := srv.routes()

	rec := doGET(t, h, "/api/sessions/a_fc_test/commands")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	cmds, _ := resp["commands"].([]any)
	if len(cmds) != 0 {
		t.Fatalf("expected empty commands, got %d", len(cmds))
	}
}

func TestFilesEndpointRows(t *testing.T) {
	srv := testServer(t, false)
	seedSessionForTracking(t, srv)
	ix := srv.indexer

	// Simulate 3 file-edit events: 2 edits to src/auth.ts, 1 edit to src/db.ts.
	// ToolCallData shape: Name must be a known file tool; path lives in Args.
	makeEditEv := func(seq int64, path, tcID string) runtime.Event {
		args, _ := json.Marshal(map[string]any{"path": path})
		raw, _ := json.Marshal(runtime.ToolCallData{
			ToolCallID: tcID, Name: "Edit", Args: args, Status: "in_progress",
		})
		return runtime.Event{
			AgentID: "a_fc_test", Seq: seq,
			Type: runtime.EvToolCall, Ts: "2026-06-28T10:00:0" + string(rune('0'+seq)) + "Z", Data: raw,
		}
	}
	if err := ix.OnEvent("a_fc_test", makeEditEv(1, "src/auth.ts", "tc_1")); err != nil {
		t.Fatalf("OnEvent 1: %v", err)
	}
	if err := ix.OnEvent("a_fc_test", makeEditEv(2, "src/auth.ts", "tc_2")); err != nil {
		t.Fatalf("OnEvent 2: %v", err)
	}
	if err := ix.OnEvent("a_fc_test", makeEditEv(3, "src/db.ts", "tc_3")); err != nil {
		t.Fatalf("OnEvent 3: %v", err)
	}

	// Also add a diff event for auth.ts → has_diff should be true.
	diffRaw, _ := json.Marshal(runtime.DiffData{Path: "src/auth.ts", ToolCallID: "tc_diff", NewText: "..."})
	diffEv := runtime.Event{
		AgentID: "a_fc_test", Seq: 4,
		Type: runtime.EvDiff, Ts: "2026-06-28T10:00:04Z", Data: diffRaw,
	}
	if err := ix.OnEvent("a_fc_test", diffEv); err != nil {
		t.Fatalf("OnEvent diff: %v", err)
	}

	h := srv.routes()
	rec := doGET(t, h, "/api/sessions/a_fc_test/files")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		AgentID string `json:"agent_id"`
		Files   []struct {
			Path      string `json:"path"`
			EditCount int    `json:"edit_count"`
			HasDiff   bool   `json:"has_diff"`
			DiffRefs  []any  `json:"diff_refs"`
		} `json:"files"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.AgentID != "a_fc_test" {
		t.Fatalf("agent_id = %q, want a_fc_test", resp.AgentID)
	}
	// 2 unique paths: src/auth.ts (3 edits = 2 tool_call + 1 diff), src/db.ts (1 edit)
	if len(resp.Files) != 2 {
		t.Fatalf("files len = %d, want 2; got %+v", len(resp.Files), resp.Files)
	}
	// auth.ts should come first (most recently touched: seq 4)
	if !strings.HasSuffix(resp.Files[0].Path, "auth.ts") {
		t.Fatalf("first file = %q, want auth.ts", resp.Files[0].Path)
	}
	if resp.Files[0].EditCount < 2 {
		t.Fatalf("auth.ts edit_count = %d, want >= 2", resp.Files[0].EditCount)
	}
	if !resp.Files[0].HasDiff {
		t.Fatalf("auth.ts has_diff = false, want true")
	}
	if len(resp.Files[0].DiffRefs) == 0 {
		t.Fatalf("auth.ts diff_refs is empty, want non-empty")
	}
	if !strings.HasSuffix(resp.Files[1].Path, "db.ts") {
		t.Fatalf("second file = %q, want db.ts", resp.Files[1].Path)
	}
	if resp.Files[1].HasDiff {
		t.Fatalf("db.ts has_diff = true, want false")
	}
}

func TestCommandsEndpointRows(t *testing.T) {
	srv := testServer(t, false)
	seedSessionForTracking(t, srv)
	ix := srv.indexer

	// 2 bash tool_calls. ToolCallData.Name must be a known command tool; command lives in Args.
	for _, tc := range []struct {
		seq  int64
		cmd  string
		tcID string
	}{
		{1, "npm test -- --watch=false", "tc_1"},
		{2, "git status", "tc_2"},
	} {
		args, _ := json.Marshal(map[string]any{"command": tc.cmd})
		raw, _ := json.Marshal(runtime.ToolCallData{
			ToolCallID: tc.tcID, Name: "Bash", Args: args, Status: "in_progress",
		})
		callEv := runtime.Event{
			AgentID: "a_fc_test", Seq: tc.seq,
			Type: runtime.EvToolCall, Ts: "2026-06-28T10:00:0" + string(rune('0'+tc.seq)) + "Z", Data: raw,
		}
		if err := ix.OnEvent("a_fc_test", callEv); err != nil {
			t.Fatalf("OnEvent tool_call seq=%d: %v", tc.seq, err)
		}
		// Correlate a tool_result.
		resRaw, _ := json.Marshal(runtime.ToolResultData{ToolCallID: tc.tcID, Status: "completed"})
		resEv := runtime.Event{
			AgentID: "a_fc_test", Seq: tc.seq + 10,
			Type: runtime.EvToolResult, Ts: "2026-06-28T10:00:1" + string(rune('0'+tc.seq)) + "Z", Data: resRaw,
		}
		if err := ix.OnEvent("a_fc_test", resEv); err != nil {
			t.Fatalf("OnEvent tool_result seq=%d: %v", tc.seq+10, err)
		}
	}

	h := srv.routes()
	rec := doGET(t, h, "/api/sessions/a_fc_test/commands")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Commands []struct {
			Command    string `json:"command"`
			ExitStatus string `json:"exit_status"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Commands) != 2 {
		t.Fatalf("commands len = %d, want 2; got %+v", len(resp.Commands), resp.Commands)
	}
	for _, c := range resp.Commands {
		if c.ExitStatus != "completed" {
			t.Fatalf("command %q exit_status = %q, want completed", c.Command, c.ExitStatus)
		}
	}
}

func TestFilesEndpointUnknownAgent(t *testing.T) {
	srv := testServer(t, false)
	h := srv.routes()

	rec := doGET(t, h, "/api/sessions/a_unknown/files")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestCommandsEndpointUnknownAgent(t *testing.T) {
	srv := testServer(t, false)
	h := srv.routes()

	rec := doGET(t, h, "/api/sessions/a_unknown/commands")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func postHookJSON(t *testing.T, h http.Handler, body string, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/hook", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("X-AgentDeck-Token", token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHookFileEditCapture(t *testing.T) {
	srv := testServer(t, false)
	seedSessionForTracking(t, srv)
	h := srv.routes()

	body := `{"agent_id":"a_fc_test","event":"file_edit","path":"src/main.go","timestamp":"2026-06-28T10:00:01Z"}`
	rec := postHookJSON(t, h, body, "tok_fc")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("hook file_edit status = %d body=%s, want 204", rec.Code, rec.Body.String())
	}

	// Verify the row was inserted.
	rec2 := doGET(t, h, "/api/sessions/a_fc_test/files")
	if rec2.Code != http.StatusOK {
		t.Fatalf("files status = %d", rec2.Code)
	}
	var resp struct {
		Files []struct {
			Path string `json:"path"`
		} `json:"files"`
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Files) != 1 || !strings.HasSuffix(resp.Files[0].Path, "main.go") {
		t.Fatalf("files = %+v, want one main.go entry", resp.Files)
	}
}

func TestHookCommandCapture(t *testing.T) {
	srv := testServer(t, false)
	seedSessionForTracking(t, srv)
	h := srv.routes()

	body := `{"agent_id":"a_fc_test","event":"command","command":"make build","timestamp":"2026-06-28T10:00:02Z"}`
	rec := postHookJSON(t, h, body, "tok_fc")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("hook command status = %d body=%s, want 204", rec.Code, rec.Body.String())
	}

	rec2 := doGET(t, h, "/api/sessions/a_fc_test/commands")
	if rec2.Code != http.StatusOK {
		t.Fatalf("commands status = %d", rec2.Code)
	}
	var resp struct {
		Commands []struct {
			Command    string `json:"command"`
			ExitStatus string `json:"exit_status"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Commands) != 1 || resp.Commands[0].Command != "make build" {
		t.Fatalf("commands = %+v, want [make build]", resp.Commands)
	}
	if resp.Commands[0].ExitStatus != "completed" {
		t.Fatalf("exit_status = %q, want completed", resp.Commands[0].ExitStatus)
	}
}

func TestHookTrackingValidationErrors(t *testing.T) {
	srv := testServer(t, false)
	seedSessionForTracking(t, srv)
	h := srv.routes()

	cases := []struct {
		name     string
		body     string
		wantCode int
	}{
		{"missing path", `{"agent_id":"a_fc_test","event":"file_edit"}`, http.StatusBadRequest},
		{"missing command", `{"agent_id":"a_fc_test","event":"command"}`, http.StatusBadRequest},
		{"wrong token file_edit", `{"agent_id":"a_fc_test","event":"file_edit","path":"x.go"}`, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			token := "tok_fc"
			if tc.name == "wrong token file_edit" {
				token = "bad_token"
			}
			rec := postHookJSON(t, h, tc.body, token)
			if rec.Code != tc.wantCode {
				t.Fatalf("%s: status = %d body=%s, want %d", tc.name, rec.Code, rec.Body.String(), tc.wantCode)
			}
		})
	}
}
