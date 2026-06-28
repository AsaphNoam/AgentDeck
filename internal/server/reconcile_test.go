package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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

func TestReconcileSessionsOnceAppliesStaleCorrection(t *testing.T) {
	srv := testServer(t, true)
	agentID := seedHookAgent(t, srv)
	old := time.Now().Add(-time.Hour).UnixMilli()
	if err := srv.stateStore.WriteStatus(state.Status{
		AgentID: agentID, State: "busy", Detail: "old detail", LastTrace: "UserPromptSubmit",
		UpdatedAt: old,
	}); err != nil {
		t.Fatalf("WriteStatus: %v", err)
	}
	writeSessionTranscript(t, srv, agentID, "first line\nlatest transcript line\n")

	if got := srv.reconcileSessionsOnce(time.Now()); got != 1 {
		t.Fatalf("reconcile applied = %d, want 1", got)
	}
	status, err := srv.stateStore.ReadStatus(agentID)
	if err != nil {
		t.Fatalf("ReadStatus: %v", err)
	}
	if status.Detail != "latest transcript line" || status.LastTrace != "ReconcileSweep" {
		t.Fatalf("status after reconcile = %+v", status)
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
	writeSessionTranscript(t, srv, agentID, "stale transcript line\n")

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
