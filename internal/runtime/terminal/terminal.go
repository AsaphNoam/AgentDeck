package terminal

import (
	"context"
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"

	rt "github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
)

// stopGrace is how long Stop waits after SIGTERM before SIGKILL (techspec §8.5,
// matching the chat runtime).
const stopGrace = 5 * time.Second

// defaultInitialIdleDelay is the §3.1 step-7 race guard: the runtime writes the
// initial idle status only if a SessionStart hook hasn't already produced one.
const defaultInitialIdleDelay = 500 * time.Millisecond

// Runtime is the terminal runtime.Runtime (§3): it launches the CLI under a
// TerminalDriver and lets hooks drive status (§3.3). It writes no status during
// a turn — only the initial idle (race-guarded) and a terminal done on Stop.
type Runtime struct {
	store  *state.Store
	driver TerminalDriver

	command string   // launch binary OVERRIDE (tests); empty → interactiveBinary()
	cmdArgs []string // override args

	initialIdleDelay time.Duration

	mu     sync.Mutex
	agents map[string]*termAgent
	touch  func(string)
	onExit func(string)
}

// termAgent is the live state for one terminal agent.
type termAgent struct {
	agentID string
	tab     *Tab
	hub     *rt.Hub

	mu      sync.Mutex
	stopped bool
}

func (a *termAgent) isStopped() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.stopped
}

// New builds the terminal runtime bound to the state store, defaulting to the
// xterm/PTY driver.
func New(s *state.Store) *Runtime {
	return &Runtime{
		store:            s,
		driver:           xtermDriver{},
		initialIdleDelay: defaultInitialIdleDelay,
		agents:           map[string]*termAgent{},
	}
}

// SetDriver overrides the terminal driver (e.g. tmux, or a fake in tests).
func (r *Runtime) SetDriver(d TerminalDriver) { r.driver = d }

// SetCommand overrides the launch binary + args (tests point at a harmless
// process; production resolves the interactive CLI per backend).
func (r *Runtime) SetCommand(bin string, args ...string) {
	r.command = bin
	r.cmdArgs = args
}

// SetInitialIdleDelay tunes the §3.1 race-guard window (tests shorten it).
func (r *Runtime) SetInitialIdleDelay(d time.Duration) { r.initialIdleDelay = d }

// SetStateTouch wires runtime state writes to the dashboard state manager.
func (r *Runtime) SetStateTouch(touch func(string)) { r.touch = touch }

// SetOnExit lets the Registry drop ownership when a terminal agent's process
// disappears outside a Stop (crash teardown), mirroring the chat runtime.
func (r *Runtime) SetOnExit(fn func(string)) { r.onExit = fn }

// Start launches the CLI under the driver, records the running row (tty, driver,
// driver_ids), and schedules the race-guarded initial idle (§3.1).
func (r *Runtime) Start(ctx context.Context, spec rt.LaunchSpec) (*rt.Handle, error) {
	tab, err := r.driver.StartTab(r.tabSpec(spec, false, ""))
	if err != nil {
		return nil, err
	}
	a := &termAgent{agentID: spec.Agent.AgentID, tab: tab, hub: rt.NewHub()}

	if err := r.store.WriteRunning(state.RunningEntry{
		AgentID: a.agentID, PID: tab.PGID, SessionID: "", Interface: "terminal",
		TTY: tab.TTY, Driver: tab.Driver, DriverIDs: tab.IDs,
		HookToken: spec.HookToken, StartedAt: time.Now().UTC(),
	}); err != nil {
		_ = r.driver.CloseTab(tab)
		return nil, fmt.Errorf("terminal: write running: %w", err)
	}

	r.mu.Lock()
	r.agents[a.agentID] = a
	r.mu.Unlock()

	r.startWatcher(a)
	r.scheduleInitialIdle(a)
	return &rt.Handle{AgentID: a.agentID, Pid: tab.PGID, SessionID: ""}, nil
}

// Resume re-launches under the driver in resume form on the same agent_id. Used
// by switch-runtime (6.4) and Phase 4 archive resume when the target is terminal.
func (r *Runtime) Resume(ctx context.Context, spec rt.LaunchSpec, sessionID string) (*rt.Handle, error) {
	tab, err := r.driver.StartTab(r.tabSpec(spec, true, sessionID))
	if err != nil {
		return nil, err
	}
	a := &termAgent{agentID: spec.Agent.AgentID, tab: tab, hub: rt.NewHub()}

	if err := r.store.WriteRunning(state.RunningEntry{
		AgentID: a.agentID, PID: tab.PGID, SessionID: sessionID, Interface: "terminal",
		TTY: tab.TTY, Driver: tab.Driver, DriverIDs: tab.IDs,
		HookToken: spec.HookToken, StartedAt: time.Now().UTC(),
	}); err != nil {
		_ = r.driver.CloseTab(tab)
		return nil, fmt.Errorf("terminal: write running: %w", err)
	}

	r.mu.Lock()
	r.agents[a.agentID] = a
	r.mu.Unlock()

	r.startWatcher(a)
	r.scheduleInitialIdle(a)
	return &rt.Handle{AgentID: a.agentID, Pid: tab.PGID, SessionID: sessionID}, nil
}

// SendPrompt delivers a prompt over the driver's WriteText path; status flows
// from hooks, so the runtime writes no status here (§3.1, §3.3).
func (r *Runtime) SendPrompt(ctx context.Context, agentID, text string) error {
	a, err := r.lookup(agentID)
	if err != nil {
		return err
	}
	return r.driver.WriteText(a.tab, text)
}

// Cancel sends SIGINT to the agent's process group (§3.1). Reports interrupted
// = true when there is a process to signal (terminal has no turn tracking).
func (r *Runtime) Cancel(ctx context.Context, agentID string) (bool, error) {
	a, err := r.lookup(agentID)
	if err != nil {
		return false, err
	}
	if a.tab.PGID > 0 {
		_ = syscall.Kill(-a.tab.PGID, syscall.SIGINT)
		return true, nil
	}
	return false, nil
}

// Stop terminates the process group, closes the emulator tab, removes the
// running row, and sets the status row to done (kept for the archive/UI). The
// liveness watcher self-suppresses once stopped is set (§3.1).
func (r *Runtime) Stop(ctx context.Context, agentID string) error {
	r.mu.Lock()
	a, ok := r.agents[agentID]
	if ok {
		delete(r.agents, agentID)
	}
	r.mu.Unlock()
	if !ok {
		_ = r.store.DeleteRunning(agentID)
		r.touchState(agentID)
		return nil
	}

	a.mu.Lock()
	already := a.stopped
	a.stopped = true
	a.mu.Unlock()
	if !already {
		r.terminate(a)
	}
	_ = r.driver.CloseTab(a.tab)
	_ = r.store.DeleteRunning(agentID)
	r.setDone(agentID)
	r.touchState(agentID)
	a.hub.Close()
	return nil
}

// CheckMessages wakes an idle terminal agent by writing a nudge prompt (Phase 5
// nudger support, §3.1).
func (r *Runtime) CheckMessages(ctx context.Context, pid int) error {
	a, err := r.lookupByPID(pid)
	if err != nil {
		return err
	}
	return r.driver.WriteText(a.tab, "You have new messages. Call the check_messages tool and handle them.")
}

// Permission has no terminal analogue: an approval prompt surfaces as
// waiting_input (via hooks) and the user answers it in the terminal itself, so
// there is no ACP channel to relay a decision over.
func (r *Runtime) Permission(ctx context.Context, agentID, toolCallID, decision string) error {
	return fmt.Errorf("%w: terminal permission relay", rt.ErrNotImplemented)
}

// Subscribe returns the agent's event hub. Terminal content flows over the PTY
// WebSocket, not as normalized events, so the hub stays empty until Stop closes
// it; subscribers simply observe the close.
func (r *Runtime) Subscribe(agentID string) (<-chan rt.Event, func(), error) {
	a, err := r.lookup(agentID)
	if err != nil {
		return nil, nil, err
	}
	ch, cancel := a.hub.Subscribe()
	return ch, cancel, nil
}

// Transcript has no in-memory normalized transcript for terminal agents (content
// lives in the terminal / persisted log).
func (r *Runtime) Transcript(agentID string) ([]rt.Event, error) { return nil, nil }

// Bridge returns the PTY backing a terminal agent for the WebSocket handler
// (§3.4). Returns an error for drivers without a server-side PTY (e.g. tmux,
// where the user attaches directly).
func (r *Runtime) Bridge(agentID string) (PTYConn, error) {
	a, err := r.lookup(agentID)
	if err != nil {
		return nil, err
	}
	if a.tab.ptmx == nil {
		return nil, fmt.Errorf("terminal: agent %s has no PTY bridge (driver %q)", agentID, a.tab.Driver)
	}
	// Hand the WebSocket a dup() of the master, never the master itself. Bridge
	// closes its PTYConn on every WS teardown — and the browser closes the WS on
	// any unmount (tab switch, navigate away), not just an intentional stop. If
	// that closed the agent's live master, the CLI would get SIGHUP and die. A dup
	// shares the same open file description, so closing the WS's fd leaves the
	// agent's master open; only Stop/CloseTab closes the real master. It also lets
	// a reconnect (or a second viewer) get its own fd instead of racing to close
	// the one master.
	dup, err := dupPTYMaster(a.tab.ptmx, agentID)
	if err != nil {
		return nil, err
	}
	return &ptyMaster{dup}, nil
}

// dupPTYMaster returns a pollable dup of the PTY master. It dups via
// SyscallConn().Control rather than (*os.File).Fd()+syscall.Dup: Fd() forces the
// shared open file description into BLOCKING mode, which would leave the returned
// File's Read uninterruptible by Close and hang the WS pump on teardown. Reading
// the fd through Control leaves the description non-blocking, so os.NewFile wraps
// a pollable File whose Close cleanly unblocks the pump — and closing it does not
// touch the agent's own master fd.
func dupPTYMaster(f *os.File, agentID string) (*os.File, error) {
	rc, err := f.SyscallConn()
	if err != nil {
		return nil, fmt.Errorf("terminal: pty syscallconn for %s: %w", agentID, err)
	}
	var dupFD int
	var dupErr error
	if cerr := rc.Control(func(fd uintptr) {
		dupFD, dupErr = syscall.Dup(int(fd))
		if dupErr == nil {
			syscall.CloseOnExec(dupFD)
			// The dup can come back in blocking mode; force non-blocking so
			// os.NewFile registers it with the runtime poller and Close can
			// interrupt a pending pump Read on WS teardown (otherwise it hangs).
			dupErr = syscall.SetNonblock(dupFD, true)
		}
	}); cerr != nil {
		return nil, fmt.Errorf("terminal: pty control for %s: %w", agentID, cerr)
	}
	if dupErr != nil {
		return nil, fmt.Errorf("terminal: dup pty master for %s: %w", agentID, dupErr)
	}
	return os.NewFile(uintptr(dupFD), f.Name()+"-ws"), nil
}

// StopAll stops every live terminal agent (server shutdown, §8.5).
func (r *Runtime) StopAll(ctx context.Context) {
	r.mu.Lock()
	ids := make([]string, 0, len(r.agents))
	for id := range r.agents {
		ids = append(ids, id)
	}
	r.mu.Unlock()
	for _, id := range ids {
		_ = r.Stop(ctx, id)
	}
}

// --- internals ---

func (r *Runtime) lookup(agentID string) (*termAgent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.agents[agentID]
	if !ok {
		return nil, rt.ErrNoHandle
	}
	return a, nil
}

func (r *Runtime) lookupByPID(pid int) (*termAgent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, a := range r.agents {
		if a.tab.PGID == pid {
			return a, nil
		}
	}
	return nil, rt.ErrNoHandle
}

func (r *Runtime) removeAgent(agentID string) {
	r.mu.Lock()
	delete(r.agents, agentID)
	r.mu.Unlock()
}

// startWatcher reaps an unexpected process exit (xterm has a waitable process;
// tmux liveness is handled by the 6.6 sweep). On exit outside Stop it clears the
// running row, marks the agent done, and tells the Registry to drop ownership.
func (r *Runtime) startWatcher(a *termAgent) {
	if a.tab.exited == nil {
		return
	}
	go func() {
		<-a.tab.exited
		a.mu.Lock()
		if a.stopped {
			a.mu.Unlock()
			return // Stop owns the cleanup
		}
		a.stopped = true
		a.mu.Unlock()

		_ = r.store.DeleteRunning(a.agentID)
		r.setDone(a.agentID)
		r.touchState(a.agentID)
		r.removeAgent(a.agentID)
		if r.onExit != nil {
			r.onExit(a.agentID)
		}
		a.hub.Close()
	}()
}

// scheduleInitialIdle writes the initial idle status after the race-guard delay,
// but only if no hook has already produced a status row (§3.1 step 7).
func (r *Runtime) scheduleInitialIdle(a *termAgent) {
	go func() {
		time.Sleep(r.initialIdleDelay)
		if a.isStopped() {
			return
		}
		if _, err := r.store.ReadStatus(a.agentID); err == nil {
			return // a SessionStart hook already wrote one
		}
		if err := r.store.WriteStatus(state.Status{
			AgentID: a.agentID, State: "idle", Detail: "ready", LastTrace: "SessionStart",
		}); err != nil {
			return
		}
		r.touchState(a.agentID)
	}()
}

// terminate signals SIGTERM→(grace)→SIGKILL to the process group, waiting on the
// driver's exited channel where available.
func (r *Runtime) terminate(a *termAgent) {
	pgid := a.tab.PGID
	if pgid > 0 {
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
	}
	switch {
	case a.tab.exited != nil:
		select {
		case <-a.tab.exited:
		case <-time.After(stopGrace):
			if pgid > 0 {
				_ = syscall.Kill(-pgid, syscall.SIGKILL)
			}
			<-a.tab.exited
		}
	case pgid > 0:
		// Driver without a waitable process (tmux): brief grace, then SIGKILL;
		// CloseTab does the authoritative teardown (kill-session).
		time.Sleep(stopGrace)
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	}
}

// setDone sets the status row to done, keeping it so the archive/UI can show a
// final state (mirrors the chat runtime's Stop).
func (r *Runtime) setDone(agentID string) {
	st, err := r.store.ReadStatus(agentID)
	if err != nil {
		st = state.Status{AgentID: agentID}
	}
	st.State = "done"
	st.BusySince = nil
	_ = r.store.WriteStatus(st)
}

func (r *Runtime) touchState(agentID string) {
	if r.touch != nil {
		r.touch(agentID)
	}
}

func (r *Runtime) tabSpec(spec rt.LaunchSpec, resume bool, sessionID string) TabSpec {
	env := spec.Env
	if len(env) == 0 {
		env = os.Environ()
	}
	return TabSpec{
		Command: r.launchArgv(spec, resume, sessionID),
		Cwd:     spec.Cwd,
		Env:     env,
		Title:   tabTitle(spec.Agent),
	}
}

// launchArgv builds the launch command. Tests override the binary; production
// resolves the per-backend interactive CLI and appends the hook-registration
// args (spec.ExtraArgs, e.g. claude `--settings <path>`).
func (r *Runtime) launchArgv(spec rt.LaunchSpec, resume bool, sessionID string) []string {
	if r.command != "" {
		return append(append([]string{r.command}, r.cmdArgs...), spec.ExtraArgs...)
	}
	argv := []string{interactiveBinary(spec.BackendType)}
	argv = append(argv, spec.ExtraArgs...)
	if resume && sessionID != "" {
		// GATED: claude's interactive resume flag; unverified against a live CLI,
		// same class as the other Phase 6 credentialed gates. Codex's resume form
		// differs (CODEX_HOME) — refined when the live CLI surface is known.
		argv = append(argv, "--resume", sessionID)
	}
	return argv
}

// interactiveBinary maps a backend type to its interactive CLI binary (the
// terminal runtime runs the *real* CLI, not the ACP adapter). GATED: unverified
// against live CLIs, same class as the other Phase 6 gates.
func interactiveBinary(backendType string) string {
	switch backendType {
	case "codex-acp":
		return "codex"
	default:
		return "claude"
	}
}

func tabTitle(a state.Agent) string {
	return fmt.Sprintf("%s · %s@%s", a.Name, a.Role, a.Project)
}

// compile-time assertion that Runtime satisfies the runtime.Runtime interface.
var _ rt.Runtime = (*Runtime)(nil)
