package terminal

import (
	"context"
	"testing"
	"time"

	rt "github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
)

func TestProbeXtermAlwaysAvailable(t *testing.T) {
	caps := Probe()
	if !caps.Terminal.Available {
		t.Fatal("terminal must always be available (xterm default)")
	}
	if !caps.Terminal.Drivers.Xterm {
		t.Fatal("xterm driver must always be available")
	}
	if caps.Terminal.DefaultDriver != "xterm" {
		t.Fatalf("default_driver = %q, want xterm", caps.Terminal.DefaultDriver)
	}
	if !caps.DriverAvailable("xterm") {
		t.Fatal("DriverAvailable(xterm) = false")
	}
	if caps.DriverAvailable("bogus") {
		t.Fatal("DriverAvailable(bogus) = true")
	}
	// iTerm2 driver is not wired until 6.7, so it must report unavailable with a
	// reason for the UI tooltip.
	if caps.Terminal.Drivers.ITerm2.Available {
		t.Fatal("iterm2 must report unavailable in 6.3")
	}
	if caps.Terminal.Drivers.ITerm2.Reason == "" {
		t.Fatal("unavailable iterm2 must carry a reason")
	}
}

// A terminal agent launches under the xterm/PTY driver, records its tty + driver
// in the running row, lands on idle, and then transitions idle→busy→idle purely
// through hook POSTs (the manager's ApplyHook ingest path, §3.3/§4).
func TestTerminalLaunchRecordsTTYAndHookStatusFlow(t *testing.T) {
	store, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	r := New(store)
	r.SetCommand("cat") // harmless long-running process under the PTY
	r.SetInitialIdleDelay(10 * time.Millisecond)

	const id, token = "a_term01", "tok-term-1"
	agent := state.Agent{
		AgentID: id, Name: "Atlas", Role: "dev", Project: "demo",
		Backend: "claude", Model: "sonnet", Interface: "terminal",
		CreatedAt: time.Now().UTC(),
	}
	if err := store.WriteAgent(agent); err != nil {
		t.Fatalf("WriteAgent: %v", err)
	}

	h, err := r.Start(context.Background(), rt.LaunchSpec{
		Agent: agent, Cwd: t.TempDir(), BackendType: "claude-acp", HookToken: token,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if h.Pid <= 0 {
		t.Fatalf("handle pid = %d, want > 0", h.Pid)
	}

	run, err := store.ReadRunning(id)
	if err != nil {
		t.Fatalf("ReadRunning: %v", err)
	}
	if run.Interface != "terminal" {
		t.Fatalf("interface = %q, want terminal", run.Interface)
	}
	if run.TTY == "" {
		t.Fatal("running row tty is empty; expected a recorded pty path")
	}
	if run.Driver != "xterm" {
		t.Fatalf("driver = %q, want xterm", run.Driver)
	}

	// The race-guarded initial idle lands once no hook beat it (§3.1 step 7).
	waitState(t, store, id, "idle")

	// idle → busy → idle, driven only by hook POSTs through the manager.
	mgr := state.NewManager(store, nil)
	if _, err := mgr.ApplyHook(token, state.HookPayload{
		AgentID: id, Event: "PreToolUse", State: "busy", Detail: "Edit", LastTrace: "PreToolUse: Edit",
	}); err != nil {
		t.Fatalf("ApplyHook busy: %v", err)
	}
	if st, _ := store.ReadStatus(id); st.State != "busy" {
		t.Fatalf("after busy hook, state = %q, want busy", st.State)
	}
	if _, err := mgr.ApplyHook(token, state.HookPayload{
		AgentID: id, Event: "Stop", State: "idle", Detail: "turn complete", LastTrace: "Stop",
	}); err != nil {
		t.Fatalf("ApplyHook idle: %v", err)
	}
	if st, _ := store.ReadStatus(id); st.State != "idle" {
		t.Fatalf("after stop hook, state = %q, want idle", st.State)
	}

	// Stop tears down the process group, removes the running row, marks done.
	if err := r.Stop(context.Background(), id); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if _, err := store.ReadRunning(id); err == nil {
		t.Fatal("running row should be gone after Stop")
	}
	if st, _ := store.ReadStatus(id); st.State != "done" {
		t.Fatalf("after Stop, state = %q, want done", st.State)
	}
}

func waitState(t *testing.T, store *state.Store, id, want string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if st, err := store.ReadStatus(id); err == nil && st.State == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	st, _ := store.ReadStatus(id)
	t.Fatalf("status never reached %q (last = %q)", want, st.State)
}
