package runtime

import (
	"context"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/state"
)

// Finding 5 regression (chat runtime): Stop on an agent this runtime does not own
// in memory (e.g. after a dashboard restart, where ReconcileStale never re-adopts
// a live PID) must not silently succeed while the CLI keeps running — it must kill
// the recorded process group first. We WriteRunning a row pointing at a live child
// that is NOT in the runtime's agents map, then Stop, and assert the child is dead.
func TestChatStopKillsOrphanedLiveProcess(t *testing.T) {
	store, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	cmd := exec.Command("sleep", "60")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("spawn sleep: %v", err)
	}
	pgid := cmd.Process.Pid
	t.Cleanup(func() { _ = syscall.Kill(-pgid, syscall.SIGKILL); _, _ = cmd.Process.Wait() })
	go func() { _, _ = cmd.Process.Wait() }() // reap so it doesn't linger as a zombie

	const id = "a_chat_orphan"
	if err := store.WriteAgent(state.Agent{
		AgentID: id, Name: "Ghost", Role: "dev", Project: "demo",
		Backend: "claude", Model: "sonnet", Interface: "chat",
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("WriteAgent: %v", err)
	}
	if err := store.WriteRunning(state.RunningEntry{
		AgentID: id, PID: pgid, SessionID: "sess-x", Interface: "chat",
		StartedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("WriteRunning: %v", err)
	}

	c := NewChatRuntime(store) // empty agents map → the !ok branch
	if err := c.Stop(context.Background(), id); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	if _, err := store.ReadRunning(id); err == nil {
		t.Fatal("running row should be gone after orphan Stop")
	}
	if orphanAlive(t, pgid) {
		t.Fatal("orphaned live process survived Stop (silent success bug)")
	}
}

func orphanAlive(t *testing.T, pid int) bool {
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
