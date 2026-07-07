package terminal

import (
	"context"
	"strings"
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

// TestLaunchArgvHonorsComposedSpec guards the BLOCKING §6 finding that the
// terminal runtime built its CLI invocation from argv/env only, silently
// dropping the composed model, add_dirs, and system prompt / switch primer.
// The claude interactive CLI must receive them as flags.
func TestLaunchArgvHonorsComposedSpec(t *testing.T) {
	r := New(nil) // no store needed: launchArgv touches only the spec
	spec := rt.LaunchSpec{
		Agent:               state.Agent{Backend: "claude", Interface: "terminal"},
		BackendType:         "claude-acp",
		ModelID:             "claude-sonnet-4-6",
		AddDirs:             []string{"/work/extra-a", "/work/extra-b"},
		SystemPrompt:        "be a careful engineer",
		RuntimeSystemPrompt: "PRIMER: prior context summary",
		ExtraArgs:           []string{"--settings", "/tmp/settings.json"},
	}
	argv := r.launchArgv(spec, true, "sess-42")
	joined := strings.Join(argv, " ")

	wantPairs := [][2]string{
		{"--model", "claude-sonnet-4-6"},
		{"--add-dir", "/work/extra-a"},
		{"--add-dir", "/work/extra-b"},
		// StartSystemPrompt prefers the one-shot RuntimeSystemPrompt (the primer).
		{"--append-system-prompt", "PRIMER: prior context summary"},
		{"--settings", "/tmp/settings.json"},
		{"--resume", "sess-42"},
	}
	for _, p := range wantPairs {
		if !argvHasFlagValue(argv, p[0], p[1]) {
			t.Errorf("argv missing %s %q; got: %s", p[0], p[1], joined)
		}
	}
	if argv[0] != "claude" {
		t.Errorf("argv[0] = %q, want claude", argv[0])
	}
}

// argvHasFlagValue reports whether argv contains flag immediately followed by val.
func argvHasFlagValue(argv []string, flag, val string) bool {
	for i := 0; i+1 < len(argv); i++ {
		if argv[i] == flag && argv[i+1] == val {
			return true
		}
	}
	return false
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

// Regression (review fix): a WebSocket teardown (tab switch, navigate away — the
// browser closes the WS on any unmount) must NOT kill the agent. Bridge closes
// its PTYConn on every teardown; before the fix that closed the agent's live PTY
// master and SIGHUP'd the CLI. Now Bridge hands out a hub SUBSCRIBER whose Close
// only unsubscribes, so after a full bridge-to-EOF the child is still alive and a
// second Bridge streams.
func TestBridgeTeardownKeepsPTYAndAgentAlive(t *testing.T) {
	store, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	r := New(store)
	r.SetCommand("cat") // echoes its input → proves the PTY is still live
	r.SetInitialIdleDelay(10 * time.Millisecond)

	const id, token = "a_bridge", "tok-bridge"
	agent := state.Agent{
		AgentID: id, Name: "Nova", Role: "dev", Project: "demo",
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
	t.Cleanup(func() { r.Stop(context.Background(), id) })

	// First bridge: the client disconnects immediately (fakeWS has no frames, so
	// read returns io.EOF), tearing the bridge down and closing its PTYConn.
	conn1, err := r.Bridge(id)
	if err != nil {
		t.Fatalf("first Bridge: %v", err)
	}
	_ = Bridge(context.Background(), &fakeWS{}, conn1) // returns io.EOF; not fatal

	// The agent must survive a mere WS teardown.
	if _, err := r.lookup(id); err != nil {
		t.Fatalf("agent removed after a WS teardown: %v", err)
	}

	// A second bridge must still reach a live PTY: writing to the master reaches
	// cat, which echoes it back — impossible if the first teardown killed the CLI.
	conn2, err := r.Bridge(id)
	if err != nil {
		t.Fatalf("second Bridge after teardown: %v", err)
	}
	defer conn2.Close()

	got := make(chan int, 1)
	go func() {
		buf := make([]byte, 64)
		n, _ := conn2.Read(buf)
		got <- n
	}()
	if _, err := conn2.Write([]byte("ping\n")); err != nil {
		t.Fatalf("write to the live PTY master: %v", err)
	}
	select {
	case n := <-got:
		if n <= 0 {
			t.Fatal("no bytes from the live PTY; the CLI appears dead after WS teardown")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out reading from the PTY; the CLI appears dead after WS teardown")
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
