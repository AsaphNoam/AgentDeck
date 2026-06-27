package state

import (
	"reflect"
	"testing"
	"time"
)

type capturePublisher struct {
	updates []AgentStateUpdate
}

func (p *capturePublisher) PublishStateUpdate(update AgentStateUpdate) {
	p.updates = append(p.updates, update)
}

func withFixedNow(t *testing.T, now time.Time) {
	t.Helper()
	original := timeNow
	timeNow = func() time.Time { return now.UTC() }
	t.Cleanup(func() { timeNow = original })
}

func TestManagerRecomputesEffectiveAgentState(t *testing.T) {
	withFixedNow(t, mustTime(t, "2026-06-22T10:01:00Z"))
	st, _ := newTestStore(t)
	pub := &capturePublisher{}
	mgr := NewManager(st, pub)

	created := mustTime(t, "2026-06-22T10:00:00Z")
	started := mustTime(t, "2026-06-22T10:00:01Z")
	busy := mustTime(t, "2026-06-22T10:00:05Z")
	agent := testAgent("a_8f3c12", created)
	if err := st.WriteAgent(agent); err != nil {
		t.Fatalf("WriteAgent: %v", err)
	}
	if err := st.WriteRunning(RunningEntry{
		AgentID: agent.AgentID, PID: 48213, SessionID: "claude-sess-xyz",
		Interface: "chat", StartedAt: started,
	}); err != nil {
		t.Fatalf("WriteRunning: %v", err)
	}
	if err := st.WriteStatus(Status{
		AgentID: agent.AgentID, State: "busy", Detail: "Editing src/auth.ts",
		LastTrace: "PostToolUse: Edit", BusySince: &busy, ContextPct: 0.42,
	}); err != nil {
		t.Fatalf("WriteStatus: %v", err)
	}

	update, err := mgr.Touch(agent.AgentID)
	if err != nil {
		t.Fatalf("Touch: %v", err)
	}
	want := AgentStateUpdate{AgentState: AgentState{
		AgentID: agent.AgentID, Name: "Atlas", Role: "implementer", Project: "my-app",
		Backend: "claude", Model: "sonnet-4-6", Interface: "chat", Group: "auth-migration",
		CreatedAt: formatTime(created), Running: true, PID: 48213, SessionID: "claude-sess-xyz",
		StartedAt: formatTime(started), State: "busy", Detail: "Editing src/auth.ts",
		LastTrace: "PostToolUse: Edit", BusySince: formatTime(busy), ContextPct: 0.42,
		UpdatedAt: mustTime(t, "2026-06-22T10:01:00Z").UnixMilli(),
	}}
	if !reflect.DeepEqual(update, want) {
		t.Fatalf("update = %+v, want %+v", update, want)
	}
	if len(pub.updates) != 1 || !reflect.DeepEqual(pub.updates[0], want) {
		t.Fatalf("published = %+v, want one %+v", pub.updates, want)
	}
}

func TestManagerRecomputeRunningFalseAndRemovalTombstone(t *testing.T) {
	withFixedNow(t, mustTime(t, "2026-06-22T10:01:00Z"))
	st, _ := newTestStore(t)
	pub := &capturePublisher{}
	mgr := NewManager(st, pub)

	agent := testAgent("a_8f3c12", mustTime(t, "2026-06-22T10:00:00Z"))
	if err := st.WriteAgent(agent); err != nil {
		t.Fatalf("WriteAgent: %v", err)
	}
	if err := st.WriteRunning(RunningEntry{
		AgentID: agent.AgentID, PID: 1, SessionID: "s", Interface: "chat",
		StartedAt: mustTime(t, "2026-06-22T10:00:01Z"),
	}); err != nil {
		t.Fatalf("WriteRunning: %v", err)
	}
	if _, err := mgr.Touch(agent.AgentID); err != nil {
		t.Fatalf("initial Touch: %v", err)
	}
	if err := st.DeleteRunning(agent.AgentID); err != nil {
		t.Fatalf("DeleteRunning: %v", err)
	}
	stopped, err := mgr.Touch(agent.AgentID)
	if err != nil {
		t.Fatalf("Touch after DeleteRunning: %v", err)
	}
	if stopped.Running {
		t.Fatalf("Running = true after DeleteRunning, want false")
	}
	if stopped.State != "unknown" {
		t.Fatalf("State = %q, want unknown without status row", stopped.State)
	}

	if err := st.DeleteAgent(agent.AgentID); err != nil {
		t.Fatalf("DeleteAgent: %v", err)
	}
	tombstone, err := mgr.Touch(agent.AgentID)
	if err != nil {
		t.Fatalf("Touch after DeleteAgent: %v", err)
	}
	if !tombstone.Removed || tombstone.AgentID != agent.AgentID {
		t.Fatalf("tombstone = %+v, want removed %s", tombstone, agent.AgentID)
	}
	if !pub.updates[len(pub.updates)-1].Removed {
		t.Fatalf("last published update = %+v, want removed tombstone", pub.updates[len(pub.updates)-1])
	}
}

func TestManagerStartPublishesExistingAgents(t *testing.T) {
	withFixedNow(t, mustTime(t, "2026-06-22T10:01:00Z"))
	st, _ := newTestStore(t)
	pub := &capturePublisher{}
	mgr := NewManager(st, pub)

	if err := st.WriteAgent(testAgent("a_8f3c12", mustTime(t, "2026-06-22T10:00:00Z"))); err != nil {
		t.Fatalf("WriteAgent first: %v", err)
	}
	if err := st.WriteAgent(testAgent("a_123abc", mustTime(t, "2026-06-22T10:00:01Z"))); err != nil {
		t.Fatalf("WriteAgent second: %v", err)
	}

	if err := mgr.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(pub.updates) != 2 {
		t.Fatalf("published %d updates, want 2: %+v", len(pub.updates), pub.updates)
	}
	if pub.updates[0].AgentID != "a_8f3c12" || pub.updates[1].AgentID != "a_123abc" {
		t.Fatalf("published order = %+v, want created_at order", pub.updates)
	}
}
