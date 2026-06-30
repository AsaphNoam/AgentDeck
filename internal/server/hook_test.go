package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/state"
)

type serverCapturePublisher struct {
	updates []state.AgentStateUpdate
}

func (p *serverCapturePublisher) PublishStateUpdate(update state.AgentStateUpdate) {
	p.updates = append(p.updates, update)
}

func postHook(t *testing.T, h http.Handler, body string, token string) *httptest.ResponseRecorder {
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

func seedHookAgent(t *testing.T, srv *Server) string {
	t.Helper()
	agent := state.Agent{
		AgentID: "a_8f3c12", Name: "Atlas", Role: "implementer", Project: "my-app",
		Backend: "claude", Model: "sonnet-4-6", Interface: "chat",
		CreatedAt: time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC),
	}
	if err := srv.stateStore.WriteAgent(agent); err != nil {
		t.Fatalf("WriteAgent: %v", err)
	}
	if err := srv.stateStore.WriteRunning(state.RunningEntry{
		AgentID: agent.AgentID, PID: 48213, SessionID: "claude-sess-xyz",
		Interface: "chat", HookToken: "tok_live",
		StartedAt: time.Date(2026, 6, 22, 10, 0, 1, 0, time.UTC),
	}); err != nil {
		t.Fatalf("WriteRunning: %v", err)
	}
	return agent.AgentID
}

func TestHookStatusAppliesAndPublishes(t *testing.T) {
	srv := testServer(t, true)
	pub := &serverCapturePublisher{}
	srv.stateMgr = state.NewManager(srv.stateStore, pub)
	agentID := seedHookAgent(t, srv)
	h := srv.routes()

	rec := postHook(t, h, `{"agent_id":"a_8f3c12","event":"status","state":"busy","detail":"Editing src/auth.ts","last_trace":"PostToolUse: Edit","context_pct":0.42}`, "tok_live")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("hook status code = %d body=%s, want 204", rec.Code, rec.Body.String())
	}
	status, err := srv.stateStore.ReadStatus(agentID)
	if err != nil {
		t.Fatalf("ReadStatus: %v", err)
	}
	if status.State != "busy" || status.Detail != "Editing src/auth.ts" || status.ContextPct != 0.42 {
		t.Fatalf("status = %+v", status)
	}
	if len(pub.updates) != 1 || pub.updates[0].AgentID != agentID || pub.updates[0].State != "busy" {
		t.Fatalf("published = %+v", pub.updates)
	}
}

// TestHookLifecycleIngest covers the terminal-CLI lifecycle events (§4.2, §8.6):
// SessionStart refreshes the running row's session_id/tty AND applies idle;
// PreToolUse applies busy + publishes a state_update; a stale token is rejected
// 401 before any write.
func TestHookLifecycleIngest(t *testing.T) {
	srv := testServer(t, true)
	pub := &serverCapturePublisher{}
	srv.stateMgr = state.NewManager(srv.stateStore, pub)
	agentID := seedHookAgent(t, srv)
	h := srv.routes()

	// SessionStart with a fresh session_id + tty refreshes the running row.
	rec := postHook(t, h, `{"agent_id":"a_8f3c12","event":"SessionStart","state":"idle","detail":"session started","last_trace":"SessionStart","session_id":"term-sess-1","tty":"/dev/ttys007"}`, "tok_live")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("SessionStart code = %d body=%s, want 204", rec.Code, rec.Body.String())
	}
	run, err := srv.stateStore.ReadRunning(agentID)
	if err != nil {
		t.Fatalf("ReadRunning: %v", err)
	}
	if run.SessionID != "term-sess-1" || run.TTY != "/dev/ttys007" {
		t.Fatalf("running after SessionStart = %+v, want session_id/tty refreshed", run)
	}
	// The running row must NOT be cleared by a lifecycle event.
	if run.HookToken != "tok_live" {
		t.Fatalf("running token lost: %+v", run)
	}

	// PreToolUse applies busy and publishes.
	pub.updates = nil
	rec = postHook(t, h, `{"agent_id":"a_8f3c12","event":"PreToolUse","state":"busy","detail":"Edit: src/auth.ts","last_trace":"PreToolUse: Edit","context_pct":0.42}`, "tok_live")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("PreToolUse code = %d body=%s, want 204", rec.Code, rec.Body.String())
	}
	st, err := srv.stateStore.ReadStatus(agentID)
	if err != nil || st.State != "busy" || st.ContextPct != 0.42 {
		t.Fatalf("status after PreToolUse = %+v err=%v, want busy/0.42", st, err)
	}
	if len(pub.updates) != 1 || pub.updates[0].State != "busy" {
		t.Fatalf("published = %+v, want one busy update", pub.updates)
	}

	// Stop does not clear the running row (per-turn, §4.3).
	rec = postHook(t, h, `{"agent_id":"a_8f3c12","event":"Stop","state":"idle","detail":"turn complete","last_trace":"Stop"}`, "tok_live")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("Stop code = %d, want 204", rec.Code)
	}
	if _, err := srv.stateStore.ReadRunning(agentID); err != nil {
		t.Fatalf("running row gone after Stop (should survive): %v", err)
	}

	// Stale token → 401 bad_token, no write.
	rec = postHook(t, h, `{"agent_id":"a_8f3c12","event":"UserPromptSubmit","state":"busy"}`, "stale")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("stale-token code = %d, want 401", rec.Code)
	}
	var body hookErrorBody
	if json.Unmarshal(rec.Body.Bytes(), &body); body.Error != "bad_token" {
		t.Fatalf("stale-token error = %q, want bad_token", body.Error)
	}
}

func TestHookValidationErrors(t *testing.T) {
	srv := testServer(t, true)
	seedHookAgent(t, srv)
	h := srv.routes()

	cases := []struct {
		name      string
		body      string
		token     string
		wantCode  int
		wantError string
	}{
		{"missing token", `{"agent_id":"a_8f3c12","event":"status","state":"busy"}`, "", http.StatusUnauthorized, "bad_token"},
		{"wrong token", `{"agent_id":"a_8f3c12","event":"status","state":"busy"}`, "wrong", http.StatusUnauthorized, "bad_token"},
		{"unknown agent", `{"agent_id":"a_nope","event":"status","state":"busy"}`, "tok_live", http.StatusNotFound, "agent_not_found"},
		{"malformed body", `{`, "tok_live", http.StatusBadRequest, "bad_request"},
		{"bad context", `{"agent_id":"a_8f3c12","event":"status","state":"busy","context_pct":1.2}`, "tok_live", http.StatusBadRequest, "bad_request"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := postHook(t, h, tc.body, tc.token)
			if rec.Code != tc.wantCode {
				t.Fatalf("status = %d body=%s, want %d", rec.Code, rec.Body.String(), tc.wantCode)
			}
			var body hookErrorBody
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("error body JSON: %v", err)
			}
			if body.Error != tc.wantError || body.Message == "" {
				t.Fatalf("error body = %+v, want error %q with message", body, tc.wantError)
			}
		})
	}
}

func TestHookBodyTokenFallback(t *testing.T) {
	srv := testServer(t, true)
	seedHookAgent(t, srv)
	h := srv.routes()

	rec := postHook(t, h, `{"agent_id":"a_8f3c12","event":"status","state":"idle","token":"tok_live"}`, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("body-token hook status = %d body=%s, want 204", rec.Code, rec.Body.String())
	}
}
