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
	"github.com/creack/pty"
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
	driver TerminalDriver // explicit override (SetDriver/tests); nil → resolve per-spec.Driver

	command string   // launch binary OVERRIDE (tests); empty → interactiveBinary()
	cmdArgs []string // override args

	initialIdleDelay time.Duration

	mu     sync.Mutex
	agents map[string]*termAgent
	touch  func(string)
	onExit func(string)

	// Persistence (mirrors the chat runtime): opening a per-agent transcript and
	// upserting the sessions row on Start/Resume makes a terminal-origin agent a
	// first-class citizen of the archive/resume contracts (Finding 7). Nil until
	// SetPersistence wires them (tests without persistence leave them unset).
	transcriptHome string
	openTranscript rt.TranscriptOpener
	indexer        rt.PersistenceIndexer
}

// termAgent is the live state for one terminal agent.
type termAgent struct {
	agentID string
	tab     *Tab
	driver  TerminalDriver // the driver that launched this tab; WriteText/CloseTab dispatch here
	hub     *rt.Hub
	ptyHub  *ptyHub             // per-agent PTY broadcast hub (nil for non-PTY drivers, e.g. tmux)
	writer  rt.TranscriptWriter // durable transcript handle; nil when persistence is off

	mu      sync.Mutex
	stopped bool
}

// startPTYHub creates the per-agent PTY broadcast hub over the tab's master and
// starts its always-on reader. No-op for drivers without a server-side PTY
// (tmux), where the user attaches directly and there is no master to drain.
func (a *termAgent) startPTYHub() {
	if a.tab == nil || a.tab.ptmx == nil {
		return
	}
	master := a.tab.ptmx
	a.ptyHub = newPTYHub(master, func(rows, cols uint16) error {
		return pty.Setsize(master, &pty.Winsize{Rows: rows, Cols: cols})
	})
}

// closePTYHub stops the always-on PTY reader and closes every subscriber (and the
// master). No-op for non-PTY drivers. Safe to call multiple times.
func (a *termAgent) closePTYHub() {
	if a.ptyHub != nil {
		a.ptyHub.Close()
	}
}

func (a *termAgent) closePersistence() {
	a.mu.Lock()
	w := a.writer
	a.writer = nil
	a.mu.Unlock()
	if w != nil {
		_ = w.Close()
	}
}

func (a *termAgent) isStopped() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.stopped
}

// New builds the terminal runtime bound to the state store. It resolves the
// per-launch driver from spec.Driver (defaulting to the cross-platform xterm/PTY
// driver) unless an explicit override is installed via SetDriver.
func New(s *state.Store) *Runtime {
	return &Runtime{
		store:            s,
		initialIdleDelay: defaultInitialIdleDelay,
		agents:           map[string]*termAgent{},
	}
}

// SetDriver installs an explicit driver override used for EVERY launch regardless
// of spec.Driver (a fake in tests, or forcing tmux). Leave it unset in production
// so each launch resolves its driver from spec.Driver via driverFor.
func (r *Runtime) SetDriver(d TerminalDriver) { r.driver = d }

// driverFor resolves the TerminalDriver for a launch: the explicit SetDriver
// override wins; otherwise dispatch by name. The server validates availability
// against the capability probe (§3.5) before launch, so an unavailable optional
// driver never reaches here — an unknown name is a programming error, surfaced as
// a launch failure rather than a silent xterm fallback.
func (r *Runtime) driverFor(name string) (TerminalDriver, error) {
	if r.driver != nil {
		return r.driver, nil
	}
	switch name {
	case "", "xterm":
		return xtermDriver{}, nil
	case "tmux":
		return tmuxDriver{}, nil
	case "iterm2":
		return iterm2Driver{}, nil
	default:
		return nil, fmt.Errorf("terminal: unknown driver %q", name)
	}
}

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

// SetPersistence enables durable transcript writes and sessions-row indexing for
// terminal agents, wired identically to the chat runtime (server layer). Without
// it a terminal agent produces no sessions row and is invisible to the archive /
// unresumable (Finding 7).
func (r *Runtime) SetPersistence(home string, open rt.TranscriptOpener, ix rt.PersistenceIndexer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.transcriptHome = home
	r.openTranscript = open
	r.indexer = ix
}

// openPersistence opens the per-agent transcript and upserts the sessions row
// (interface-agnostic, driven off the launch spec). No-op when persistence is
// unwired. Mirrors ChatRuntime.openPersistence.
func (r *Runtime) openPersistence(a *termAgent, spec rt.LaunchSpec, sessionID string) error {
	r.mu.Lock()
	home := r.transcriptHome
	open := r.openTranscript
	ix := r.indexer
	r.mu.Unlock()
	if home == "" || open == nil || ix == nil {
		return nil
	}
	meta := rt.NewSessionMeta(spec, sessionID)
	w, err := open(home, spec.Agent.AgentID, &meta)
	if err != nil {
		return fmt.Errorf("terminal: open transcript: %w", err)
	}
	if err := ix.UpsertSessionMeta(spec.Agent.AgentID, meta); err != nil {
		_ = w.Close()
		return fmt.Errorf("terminal: index session meta: %w", err)
	}
	a.writer = w
	return nil
}

// Start launches the CLI under the driver, records the running row (tty, driver,
// driver_ids), and schedules the race-guarded initial idle (§3.1).
func (r *Runtime) Start(ctx context.Context, spec rt.LaunchSpec) (*rt.Handle, error) {
	drv, err := r.driverFor(spec.Driver)
	if err != nil {
		return nil, err
	}
	tab, err := drv.StartTab(r.tabSpec(spec, false, ""))
	if err != nil {
		return nil, err
	}
	a := &termAgent{agentID: spec.Agent.AgentID, tab: tab, driver: drv, hub: rt.NewHub()}

	// Open the transcript + upsert the sessions row BEFORE the running row so a
	// launched terminal agent is a first-class archive/resume citizen (Finding 7).
	if err := r.openPersistence(a, spec, ""); err != nil {
		_ = drv.CloseTab(tab)
		return nil, err
	}

	// Start the always-on PTY hub (ptmx drivers only) so the master is drained
	// from launch — no WS need attach — preventing a full-tty-buffer stall
	// (Finding 9), and so every viewer sees identical bytes (Finding 8).
	a.startPTYHub()

	if err := r.store.WriteRunning(state.RunningEntry{
		AgentID: a.agentID, PID: tab.PGID, SessionID: "", Interface: "terminal",
		TTY: tab.TTY, Driver: tab.Driver, DriverIDs: tab.IDs,
		HookToken: spec.HookToken, StartedAt: time.Now().UTC(),
	}); err != nil {
		a.closePTYHub()
		a.closePersistence()
		_ = drv.CloseTab(tab)
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
	drv, err := r.driverFor(spec.Driver)
	if err != nil {
		return nil, err
	}
	tab, err := drv.StartTab(r.tabSpec(spec, true, sessionID))
	if err != nil {
		return nil, err
	}
	a := &termAgent{agentID: spec.Agent.AgentID, tab: tab, driver: drv, hub: rt.NewHub()}

	// Re-open the transcript in append mode and re-upsert the sessions row with the
	// resumed session_id, mirroring the chat runtime so archive resume of a terminal
	// agent stays a first-class citizen (Finding 7).
	if err := r.openPersistence(a, spec, sessionID); err != nil {
		_ = drv.CloseTab(tab)
		return nil, err
	}

	// Start the always-on PTY hub (ptmx drivers only): drains from launch and
	// broadcasts to all viewers (Findings 8+9), same as Start.
	a.startPTYHub()

	if err := r.store.WriteRunning(state.RunningEntry{
		AgentID: a.agentID, PID: tab.PGID, SessionID: sessionID, Interface: "terminal",
		TTY: tab.TTY, Driver: tab.Driver, DriverIDs: tab.IDs,
		HookToken: spec.HookToken, StartedAt: time.Now().UTC(),
	}); err != nil {
		a.closePTYHub()
		a.closePersistence()
		_ = drv.CloseTab(tab)
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
	return a.driver.WriteText(a.tab, text)
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
		// The registry doesn't own this agent — typically after a dashboard restart,
		// where ReconcileStale intentionally never re-adopts a still-live PID. Do NOT
		// silently succeed: if the recorded process group is alive, kill it before
		// clearing the row, otherwise the PTY/CLI keeps running invisible and
		// unkillable from the UI (Finding 5).
		r.reconcileOrphanStop(agentID)
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
	// Close the PTY hub (subscribers + master) so the always-on reader exits, then
	// CloseTab (idempotent on the same master fd). The reader goroutine terminates
	// on the master close — closePTYHub blocks nothing, but readLoop unblocks and
	// tears down its subscribers with no leak.
	a.closePTYHub()
	_ = a.driver.CloseTab(a.tab)
	a.closePersistence()
	_ = r.store.DeleteRunning(agentID)
	r.setDone(agentID)
	r.touchState(agentID)
	a.hub.Close()
	return nil
}

// reconcileOrphanStop handles a Stop on an agent the in-memory registry no longer
// owns (e.g. post-restart, where reconcile leaves the live PID un-adopted). If the
// recorded process group is still alive it is SIGTERM'd, then SIGKILL'd after a
// short grace, so Stop never reports success while a live child keeps running
// (Finding 5). No live PID → nothing to kill.
func (r *Runtime) reconcileOrphanStop(agentID string) {
	row, err := r.store.ReadRunning(agentID)
	if err != nil {
		return // no running row → nothing to reconcile
	}
	if row.PID <= 0 || !rt.PidAlive(row.PID) {
		return
	}
	_ = syscall.Kill(-row.PID, syscall.SIGTERM)
	deadline := time.After(stopGrace)
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-deadline:
			_ = syscall.Kill(-row.PID, syscall.SIGKILL)
			return
		case <-tick.C:
			if !rt.PidAlive(row.PID) {
				return
			}
		}
	}
}

// CheckMessages wakes an idle terminal agent by writing a nudge prompt (Phase 5
// nudger support, §3.1).
func (r *Runtime) CheckMessages(ctx context.Context, pid int) error {
	a, err := r.lookupByPID(pid)
	if err != nil {
		return err
	}
	return a.driver.WriteText(a.tab, "You have new messages. Call the check_messages tool and handle them.")
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

// Bridge returns a PTYConn backing a terminal agent for the WebSocket handler
// (§3.4). It is a SUBSCRIBER of the agent's per-agent PTY hub: its Read replays
// the current scrollback then the live stream, Write/Resize go to the single
// shared master, and Close only unsubscribes (never closes the master). Multiple
// concurrent viewers therefore see identical bytes (Finding 8), and the always-on
// hub reader keeps draining the master even with no WS attached (Finding 9).
// Returns an error for drivers without a server-side PTY (e.g. tmux, where the
// user attaches directly).
func (r *Runtime) Bridge(agentID string) (PTYConn, error) {
	a, err := r.lookup(agentID)
	if err != nil {
		return nil, err
	}
	if a.tab.ptmx == nil || a.ptyHub == nil {
		return nil, fmt.Errorf("terminal: agent %s has no PTY bridge (driver %q)", agentID, a.tab.Driver)
	}
	// Subscribe is safe to race with teardown: after Close it returns the final
	// scrollback and an already-closed channel, so a late Bridge yields a
	// short-lived read-only conn rather than panicking.
	snapshot, ch, unsub := a.ptyHub.subscribe()
	return &hubConn{hub: a.ptyHub, ch: ch, unsub: unsub, pending: snapshot}, nil
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

		a.closePTYHub()
		a.closePersistence()
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
// resolves the per-backend interactive CLI, honors the composed LaunchSpec
// (model, add_dirs, system prompt/primer), and appends the hook-registration
// args (spec.ExtraArgs, e.g. claude `--settings <path>`).
func (r *Runtime) launchArgv(spec rt.LaunchSpec, resume bool, sessionID string) []string {
	if r.command != "" {
		return append(append([]string{r.command}, r.cmdArgs...), spec.ExtraArgs...)
	}
	bin := interactiveBinary(spec.BackendType)
	argv := []string{bin}
	argv = append(argv, spec.ExtraArgs...)
	// §6 contract: the composed model, add_dirs, and system prompt / switch primer
	// MUST reach the interactive CLI — silently dropping them launched a
	// default-model agent with no project dirs or persona. These are the
	// documented Claude Code CLI flags; GATED like interactiveBinary/--resume/
	// --settings (unverified against a live login). Codex terminal is rejected
	// upstream (422 terminal_unavailable), so only the claude CLI reaches here.
	if bin == "claude" {
		if spec.ModelID != "" {
			argv = append(argv, "--model", spec.ModelID)
		}
		for _, d := range spec.AddDirs {
			argv = append(argv, "--add-dir", d)
		}
		if sp := spec.StartSystemPrompt(); sp != "" {
			argv = append(argv, "--append-system-prompt", sp)
		}
	}
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
