package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
	"github.com/agentdeck/agentdeck/internal/transcript"
)

// writeAnnotationPair seeds an inactive chat source and a running, idle chat
// recipient — the FS-13.R7 assign-to-another-agent shape.
func writeAnnotationPair(t *testing.T, srv *Server) (state.Agent, state.Agent) {
	t.Helper()
	now := time.Now().UTC()
	source := state.Agent{AgentID: "a_source", Name: "Source", Role: "implementer", Project: "demo", Backend: "claude", Model: "sonnet", Interface: "chat", CreatedAt: now}
	target := state.Agent{AgentID: "a_target", Name: "Target", Role: "reviewer", Project: "demo", Backend: "claude", Model: "sonnet", Interface: "chat", CreatedAt: now}
	for _, agent := range []state.Agent{source, target} {
		if err := srv.stateStore.WriteAgent(agent); err != nil {
			t.Fatalf("WriteAgent %s: %v", agent.AgentID, err)
		}
	}
	if err := srv.stateStore.WriteRunning(state.RunningEntry{AgentID: target.AgentID, PID: 42, Interface: "chat", StartedAt: now}); err != nil {
		t.Fatalf("WriteRunning: %v", err)
	}
	if err := srv.stateStore.WriteStatus(state.Status{AgentID: target.AgentID, State: "idle"}); err != nil {
		t.Fatalf("WriteStatus: %v", err)
	}
	// The source needs an indexed session row: appending its annotation event
	// flushes that event's text into the session's searchable content (FS-13.R10).
	if err := srv.indexer.UpsertSessionMeta(source.AgentID, runtime.SessionMetaData{
		Name: source.Name, Role: source.Role, Project: source.Project, Backend: source.Backend,
		Model: source.Model, Interface: source.Interface, CreatedAt: now.Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("UpsertSessionMeta: %v", err)
	}
	return source, target
}

func TestAnnotationMailBodyClipsOnlyExcerpts(t *testing.T) {
	data := runtime.AnnotationData{
		Annotations: []runtime.Annotation{
			{Seq: 1, Path: "main.go", Side: "new", StartLine: 3, EndLine: 4, Excerpt: strings.Repeat("x", 2000), Instruction: "inspect the first change"},
			{Seq: 2, Excerpt: strings.Repeat("y", 2000), Instruction: "inspect the second change"},
			{Seq: 3, Excerpt: strings.Repeat("z", 2000), Instruction: "inspect the final change"},
			{Seq: 4, Excerpt: strings.Repeat("w", 2000), Instruction: "keep every instruction intact"},
		},
		OverallInstruction: "send a concise review",
		Target:             runtime.AnnotationTarget{Kind: "agent", AgentID: "a_target"},
	}
	body, err := annotationMailBody(data)
	if err != nil {
		t.Fatalf("annotationMailBody: %v", err)
	}
	if len(body) > maxAnnotationMailBytes {
		t.Fatalf("mail body length = %d, want <= %d", len(body), maxAnnotationMailBytes)
	}
	for _, text := range []string{"inspect the first change", "inspect the second change", "inspect the final change", "keep every instruction intact", "send a concise review"} {
		if !strings.Contains(body, text) {
			t.Fatalf("mail body clipped protected text %q", text)
		}
	}
}

func TestValidateAnnotationsClipsLongExcerptAndRejectsEmptyInstruction(t *testing.T) {
	data := runtime.AnnotationData{Annotations: []runtime.Annotation{{Seq: 1, Excerpt: strings.Repeat("x", 2100), Instruction: "check it"}}, Target: runtime.AnnotationTarget{Kind: "self"}}
	if err := validateAnnotations(&data); err != nil {
		t.Fatalf("validateAnnotations: %v", err)
	}
	if got := runeCount(data.Annotations[0].Excerpt); got != maxAnnotationChars {
		t.Fatalf("excerpt runes = %d, want %d", got, maxAnnotationChars)
	}
	data.Annotations[0].Instruction = "  "
	if err := validateAnnotations(&data); err == nil {
		t.Fatal("validateAnnotations accepted an empty instruction")
	}
}

// FS-13.A3/A5 and FS-06.A10: an inactive source can assign annotations to a
// running chat agent; the source event is durable/indexed and user mail neither
// impersonates an agent nor creates a turn-budget row.
func TestAnnotationAgentDeliveryPersistsUserMailAndTranscriptEvent(t *testing.T) {
	srv := testServer(t, true)
	source, target := writeAnnotationPair(t, srv)
	body := runtime.AnnotationData{Annotations: []runtime.Annotation{{Seq: 4, Path: "main.go", Side: "new", StartLine: 9, EndLine: 10, Excerpt: "target phrase", Instruction: "review this branch"}}, Target: runtime.AnnotationTarget{Kind: "agent", AgentID: target.AgentID}}
	raw, _ := json.Marshal(body)
	req := newLocalRequest(http.MethodPost, "/api/sessions/a_source/annotations", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("annotation status = %d: %s", rec.Code, rec.Body.String())
	}
	mail, err := srv.stateStore.ListMessages(target.AgentID, true, 10)
	if err != nil || len(mail) != 1 {
		t.Fatalf("ListMessages = %d, %v; want 1", len(mail), err)
	}
	if mail[0].FromAgent != "user" || mail[0].FromAddress != "user@dashboard" || !strings.Contains(mail[0].Body, "target phrase") {
		t.Fatalf("unexpected reserved-sender mail: %+v", mail[0])
	}
	var budgetRows int
	if err := srv.stateStore.DB().QueryRow(`SELECT COUNT(*) FROM turn_budget WHERE agent_id = ?`, target.AgentID).Scan(&budgetRows); err != nil || budgetRows != 0 {
		t.Fatalf("reserved user mail budget rows = %d, %v; want none", budgetRows, err)
	}
	events, err := transcript.ReadFile(srv.configStore.Home(), source.AgentID, transcript.ReadOptions{})
	if err != nil || len(events) != 1 || events[0].Type != runtime.EvAnnotation {
		t.Fatalf("source annotation event = %#v, %v", events, err)
	}
	var indexed string
	if err := srv.stateStore.DB().QueryRow(`SELECT content FROM sessions_fts WHERE agent_id = ?`, source.AgentID).Scan(&indexed); err != nil || !strings.Contains(indexed, "review this branch") {
		t.Fatalf("annotation index = %q, %v", indexed, err)
	}
}

// FS-13.R5 and invariant §15: the durable source event precedes delivery. A
// failed append must leave nothing delivered, because the tray is preserved on
// failure (FS-13.R3) and the natural retry would otherwise insert a second copy
// of the same mail for the recipient to act on twice.
func TestAnnotationAppendFailureDeliversNoMailAndRetrySendsOnce(t *testing.T) {
	srv := testServer(t, true)
	source, target := writeAnnotationPair(t, srv)
	// transcript.Open must MkdirAll the per-agent session directory; a regular
	// file at that path fails it the way a full disk or a permission error would.
	blocked := filepath.Join(srv.configStore.Home(), "sessions", source.AgentID)
	if err := os.MkdirAll(filepath.Dir(blocked), 0o700); err != nil {
		t.Fatalf("MkdirAll sessions: %v", err)
	}
	if err := os.WriteFile(blocked, nil, 0o600); err != nil {
		t.Fatalf("block transcript dir: %v", err)
	}

	body := runtime.AnnotationData{Annotations: []runtime.Annotation{{Seq: 4, Excerpt: "target phrase", Instruction: "review this branch"}}, Target: runtime.AnnotationTarget{Kind: "agent", AgentID: target.AgentID}}
	raw, _ := json.Marshal(body)
	send := func() *httptest.ResponseRecorder {
		req := newLocalRequest(http.MethodPost, "/api/sessions/"+source.AgentID+"/annotations", bytes.NewReader(raw))
		rec := httptest.NewRecorder()
		srv.routes().ServeHTTP(rec, req)
		return rec
	}
	mailCount := func() int {
		t.Helper()
		mail, err := srv.stateStore.ListMessages(target.AgentID, false, 10)
		if err != nil {
			t.Fatalf("ListMessages: %v", err)
		}
		return len(mail)
	}

	if rec := send(); rec.Code != http.StatusInternalServerError {
		t.Fatalf("blocked annotation status = %d: %s", rec.Code, rec.Body.String())
	}
	if got := mailCount(); got != 0 {
		t.Fatalf("mail after a failed append = %d; want 0", got)
	}

	// The preserved tray is re-sent once the transcript is writable again.
	if err := os.Remove(blocked); err != nil {
		t.Fatalf("unblock transcript dir: %v", err)
	}
	if rec := send(); rec.Code != http.StatusAccepted {
		t.Fatalf("retried annotation status = %d: %s", rec.Code, rec.Body.String())
	}
	if got := mailCount(); got != 1 {
		t.Fatalf("mail after the retry = %d; want exactly 1", got)
	}
}
