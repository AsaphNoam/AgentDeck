package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
)

func writeSessionTranscript(t *testing.T, srv *Server, agentID string, body string) {
	t.Helper()
	path := filepath.Join(srv.configStore.Home(), "sessions", agentID, "transcript.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll transcript dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile transcript: %v", err)
	}
}

// transcriptEvent marshals a normalized runtime.Event into one NDJSON line.
func transcriptEvent(t *testing.T, agentID string, seq int, typ string, data any) string {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal event data: %v", err)
	}
	ev := runtime.Event{AgentID: agentID, Seq: int64(seq), Type: typ, Ts: "2026-07-02T00:00:00Z", Data: raw}
	line, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	return string(line) + "\n"
}

func TestReconcileSessionsOnceAppliesStaleCorrection(t *testing.T) {
	srv := testServer(t, true)
	agentID := seedHookAgent(t, srv)
	old := time.Now().Add(-time.Hour).UnixMilli()
	if err := srv.stateStore.WriteStatus(state.Status{
		AgentID: agentID, State: "idle", Detail: "old detail", LastTrace: "Stop",
		UpdatedAt: old,
	}); err != nil {
		t.Fatalf("WriteStatus: %v", err)
	}
	body := transcriptEvent(t, agentID, 1, runtime.EvAssistantText, runtime.AssistantTextData{Delta: "the latest assistant "}) +
		transcriptEvent(t, agentID, 2, runtime.EvAssistantText, runtime.AssistantTextData{Delta: "output line"}) +
		transcriptEvent(t, agentID, 3, runtime.EvTurnEnd, runtime.TurnEndData{StopReason: "end_turn"})

	writeSessionTranscript(t, srv, agentID, body)

	if got := srv.reconcileSessionsOnce(time.Now()); got != 1 {
		t.Fatalf("reconcile applied = %d, want 1", got)
	}
	status, err := srv.stateStore.ReadStatus(agentID)
	if err != nil {
		t.Fatalf("ReadStatus: %v", err)
	}
	if status.Detail != "the latest assistant output line" {
		t.Fatalf("detail after reconcile = %q, want the joined assistant text", status.Detail)
	}
	// last_trace must stay within the §4.4 vocabulary — the sweep preserves the
	// prior trace rather than inventing an out-of-vocabulary "ReconcileSweep".
	if status.LastTrace != "Stop" {
		t.Fatalf("last_trace after reconcile = %q, want preserved %q", status.LastTrace, "Stop")
	}
}

// Regression for the BLOCKING finding: a live idle agent whose transcript ends
// in a raw turn_end envelope must NOT have that raw JSON written into its
// status detail (which the card renders as its preview).
func TestReconcileSessionsOnceDoesNotWriteRawJSON(t *testing.T) {
	srv := testServer(t, true)
	agentID := seedHookAgent(t, srv)
	old := time.Now().Add(-time.Hour).UnixMilli()
	if err := srv.stateStore.WriteStatus(state.Status{
		AgentID: agentID, State: "idle", Detail: "human readable", LastTrace: "Stop",
		UpdatedAt: old,
	}); err != nil {
		t.Fatalf("WriteStatus: %v", err)
	}
	body := transcriptEvent(t, agentID, 1, runtime.EvAssistantText, runtime.AssistantTextData{Delta: "done thinking"}) +
		transcriptEvent(t, agentID, 2, runtime.EvTurnEnd, runtime.TurnEndData{StopReason: "end_turn"})
	writeSessionTranscript(t, srv, agentID, body)

	srv.reconcileSessionsOnce(time.Now())
	status, err := srv.stateStore.ReadStatus(agentID)
	if err != nil {
		t.Fatalf("ReadStatus: %v", err)
	}
	if len(status.Detail) == 0 || status.Detail[0] == '{' {
		t.Fatalf("detail is raw JSON envelope: %q", status.Detail)
	}
	if status.Detail != "done thinking" {
		t.Fatalf("detail = %q, want the assistant text preview", status.Detail)
	}
}

// A transcript with no assistant text at all leaves the existing detail intact
// instead of clobbering it with a raw non-text envelope.
func TestReconcileSessionsOncePreservesDetailWithoutAssistantText(t *testing.T) {
	srv := testServer(t, true)
	agentID := seedHookAgent(t, srv)
	old := time.Now().Add(-time.Hour).UnixMilli()
	if err := srv.stateStore.WriteStatus(state.Status{
		AgentID: agentID, State: "idle", Detail: "keep me", LastTrace: "Stop",
		UpdatedAt: old,
	}); err != nil {
		t.Fatalf("WriteStatus: %v", err)
	}
	body := transcriptEvent(t, agentID, 1, runtime.EvToolCall, runtime.ToolCallData{ToolCallID: "t1", Name: "Bash"}) +
		transcriptEvent(t, agentID, 2, runtime.EvTurnEnd, runtime.TurnEndData{StopReason: "end_turn"})
	writeSessionTranscript(t, srv, agentID, body)

	srv.reconcileSessionsOnce(time.Now())
	status, err := srv.stateStore.ReadStatus(agentID)
	if err != nil {
		t.Fatalf("ReadStatus: %v", err)
	}
	if status.Detail != "keep me" {
		t.Fatalf("detail = %q, want preserved %q", status.Detail, "keep me")
	}
}

func TestReconcileSessionsOnceDoesNotOverrideFreshHook(t *testing.T) {
	srv := testServer(t, true)
	agentID := seedHookAgent(t, srv)
	fresh := time.Now().Add(time.Hour).UnixMilli()
	if err := srv.stateStore.WriteStatus(state.Status{
		AgentID: agentID, State: "busy", Detail: "fresh hook detail", LastTrace: "PostToolUse: Edit",
		UpdatedAt: fresh,
	}); err != nil {
		t.Fatalf("WriteStatus: %v", err)
	}
	writeSessionTranscript(t, srv, agentID,
		transcriptEvent(t, agentID, 1, runtime.EvAssistantText, runtime.AssistantTextData{Delta: "stale transcript line"}))

	if got := srv.reconcileSessionsOnce(time.Now()); got != 0 {
		t.Fatalf("reconcile applied = %d, want 0", got)
	}
	status, err := srv.stateStore.ReadStatus(agentID)
	if err != nil {
		t.Fatalf("ReadStatus: %v", err)
	}
	if status.Detail != "fresh hook detail" || status.LastTrace != "PostToolUse: Edit" {
		t.Fatalf("fresh status was overwritten: %+v", status)
	}
}
