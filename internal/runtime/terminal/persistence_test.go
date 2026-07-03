package terminal

import (
	"context"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/index"
	rt "github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
	"github.com/agentdeck/agentdeck/internal/transcript"
)

// withPersistence wires the terminal runtime to a real transcript writer + indexer
// the same way the server layer does, so a launched agent gets a sessions row.
func withPersistence(t *testing.T, r *Runtime, store *state.Store) {
	t.Helper()
	ix := index.New(store.DB())
	r.SetPersistence(t.TempDir(), func(home, agentID string, meta *rt.SessionMetaData) (rt.TranscriptWriter, error) {
		return transcript.Open(home, agentID, meta)
	}, ix)
}

// Finding 7 regression: a terminal-origin agent must be a first-class citizen of
// the archive/resume contracts. Before the fix Start/Resume never created a
// sessions row, so the agent was invisible to the archive and unresumable (422
// "no persisted session"). After the fix: Start writes a sessions row; Stop keeps
// it; resume of the stopped agent finds the persisted session.
func TestTerminalStartCreatesSessionRowSurvivingStop(t *testing.T) {
	store, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	r := New(store)
	r.SetCommand("cat")
	r.SetInitialIdleDelay(10 * time.Millisecond)
	withPersistence(t, r, store)

	const id, token = "a_termpersist", "tok-persist"
	agent := state.Agent{
		AgentID: id, Name: "Atlas", Role: "dev", Project: "demo",
		Backend: "claude", Model: "sonnet", Interface: "terminal",
		CreatedAt: time.Now().UTC(),
	}
	if err := store.WriteAgent(agent); err != nil {
		t.Fatalf("WriteAgent: %v", err)
	}

	if _, err := r.Start(context.Background(), rt.LaunchSpec{
		Agent: agent, Cwd: t.TempDir(), BackendType: "claude-acp", HookToken: token,
	}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// (a) a launched terminal agent produces a queryable sessions row.
	snap, err := store.ReadSession(id)
	if err != nil {
		t.Fatalf("ReadSession after Start: %v (terminal agent got no sessions row)", err)
	}
	if snap.Interface != "terminal" {
		t.Fatalf("sessions.interface = %q, want terminal", snap.Interface)
	}

	// (b) Stop keeps the sessions row (archive can still list it).
	if err := r.Stop(context.Background(), id); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if _, err := store.ReadRunning(id); err == nil {
		t.Fatal("running row should be gone after Stop")
	}
	if _, err := store.ReadSession(id); err != nil {
		t.Fatalf("ReadSession after Stop: %v (session row must survive Stop for the archive)", err)
	}

	// (c) resume finds the persisted session (no 422) and re-writes a running row.
	if _, err := r.Resume(context.Background(), rt.LaunchSpec{
		Agent: agent, Cwd: t.TempDir(), BackendType: "claude-acp", HookToken: token,
	}, "sess-resumed-1"); err != nil {
		t.Fatalf("Resume: %v", err)
	}
	t.Cleanup(func() { r.Stop(context.Background(), id) })
	if _, err := store.ReadSession(id); err != nil {
		t.Fatalf("ReadSession after Resume: %v", err)
	}
	run, err := store.ReadRunning(id)
	if err != nil {
		t.Fatalf("ReadRunning after Resume: %v", err)
	}
	if run.SessionID != "sess-resumed-1" {
		t.Fatalf("running.session_id = %q, want sess-resumed-1", run.SessionID)
	}
}

// Finding 5 regression: Stop on an agent the in-memory registry does not own (e.g.
// after a dashboard restart, where reconcile never re-adopts a live PID) must not
// silently succeed while a live child keeps running — it must kill the recorded
// process group first. Here we WriteRunning a row pointing at a live child that is
// NOT in the runtime's agents map, then Stop, and assert the child is dead.
func TestTerminalStopKillsOrphanedLiveProcess(t *testing.T) {
	store, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	// Spawn a harmless long-lived child in its own process group (pgid == pid).
	cmd := exec.Command("sleep", "60")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("spawn sleep: %v", err)
	}
	pgid := cmd.Process.Pid
	t.Cleanup(func() { _ = syscall.Kill(-pgid, syscall.SIGKILL); _, _ = cmd.Process.Wait() })
	// Reap the child when it exits so it doesn't linger as a zombie (which would
	// still answer signal 0 and fail the liveness assertion below).
	go func() { _, _ = cmd.Process.Wait() }()

	const id = "a_orphan"
	if err := store.WriteAgent(state.Agent{
		AgentID: id, Name: "Ghost", Role: "dev", Project: "demo",
		Backend: "claude", Model: "sonnet", Interface: "terminal",
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("WriteAgent: %v", err)
	}
	if err := store.WriteRunning(state.RunningEntry{
		AgentID: id, PID: pgid, SessionID: "", Interface: "terminal",
		Driver: "xterm", StartedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("WriteRunning: %v", err)
	}

	// The runtime does NOT own this agent (empty agents map) → the !ok branch.
	r := New(store)
	if err := r.Stop(context.Background(), id); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	// The running row is cleared AND the orphaned process group is dead.
	if _, err := store.ReadRunning(id); err == nil {
		t.Fatal("running row should be gone after orphan Stop")
	}
	if pidStillAlive(t, pgid) {
		t.Fatal("orphaned live process survived Stop (silent success bug)")
	}
}

// pidStillAlive polls signal 0, allowing a brief window for the SIGTERM-then-grace
// teardown to complete.
func pidStillAlive(t *testing.T, pid int) bool {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if syscall.Kill(pid, 0) != nil {
			return false
		}
		time.Sleep(20 * time.Millisecond)
	}
	return syscall.Kill(pid, 0) == nil
}
