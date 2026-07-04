package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/agentdeck/agentdeck/internal/backend"
	"github.com/agentdeck/agentdeck/internal/state"
	"github.com/agentdeck/agentdeck/internal/strutil"
)

// stopGrace is how long Stop waits after SIGTERM before SIGKILL (techspec §8.5).
const stopGrace = 5 * time.Second

// defaultCancelGrace is how long Cancel waits for a peer to honor session/cancel
// before escalating to SIGINT on the process group (techspec §8.4).
const defaultCancelGrace = 3 * time.Second

// ChatRuntime drives ACP agents (claude-acp, codex-acp) over the stdio protocol.
// It owns one agentState per live agent. ALL ACP wire decoding is isolated in
// acpmap.go; per-backend differences (binary, env strip, resume) live in the
// backend.BackendAdapter; this file orchestrates process lifecycle, the hub, and
// status writes.
type ChatRuntime struct {
	store   *state.Store
	command string   // adapter binary OVERRIDE (injectable for tests); empty → adapter default
	cmdArgs []string // adapter args override

	// onExit notifies the owner (the Registry) that an agent's live handle is
	// gone after an unsolicited teardown (crash). Without it, Registry.rtByAgent
	// keeps stale ownership and blocks relaunch/resume until a manual Stop. Nil
	// when the runtime is constructed standalone (tests). See registry.go.
	onExit func(agentID string)

	cancelGrace time.Duration // Cancel→SIGINT escalation window (§8.4)

	mu     sync.Mutex
	agents map[string]*agentState
	sink   func(Event)
	touch  func(string)

	transcriptHome string
	openTranscript TranscriptOpener
	indexer        PersistenceIndexer
}

// NewChatRuntime constructs the chat runtime bound to the state store. The launch
// binary is resolved per agent from the backend.BackendAdapter unless overridden
// via SetCommand (or c.command) — e.g. tests pointing at the fake ACP CLI.
func NewChatRuntime(s *state.Store) *ChatRuntime {
	return &ChatRuntime{
		store:       s,
		agents:      map[string]*agentState{},
		cancelGrace: defaultCancelGrace,
	}
}

// SetCancelGrace overrides the Cancel→SIGINT escalation window (tests use a short
// value; a non-positive value disables escalation).
func (c *ChatRuntime) SetCancelGrace(d time.Duration) { c.cancelGrace = d }

// SetCommand overrides the adapter binary + args for every backend. Used to
// point at a pinned adapter path (1.6) or, in tests, the fake ACP CLI.
func (c *ChatRuntime) SetCommand(bin string, args ...string) {
	c.command = bin
	c.cmdArgs = args
}

// adapterFor resolves the per-backend adapter, or ErrNotImplemented for an
// unknown backend type (mapped to 501 by the API layer).
func (c *ChatRuntime) adapterFor(backendType string) (backend.BackendAdapter, error) {
	ad, ok := backend.For(backendType)
	if !ok {
		return nil, fmt.Errorf("%w: backend %q", ErrNotImplemented, backendType)
	}
	return ad, nil
}

// spawnCmd builds the *exec.Cmd for a launch/resume: the adapter supplies the
// default binary/args (unless overridden) and the env keys to strip; the process
// runs in its own group so the runtime can signal the whole tree.
func (c *ChatRuntime) spawnCmd(ad backend.BackendAdapter, spec LaunchSpec) *exec.Cmd {
	bin, args := c.command, c.cmdArgs
	if bin == "" {
		bin, args = ad.Binary(), ad.LaunchArgs()
	}
	// ExtraArgs carries launch-time hook registration flags (e.g. claude's
	// --settings <per-agent hooks file>), composed by the server (techspec §2.3).
	args = append(append([]string{}, args...), spec.ExtraArgs...)
	cmd := exec.Command(bin, args...)
	cmd.Dir = spec.Cwd
	env := spec.Env
	if len(env) == 0 {
		env = os.Environ()
	}
	for _, k := range ad.StripEnvKeys() {
		env = stripEnv(env, k)
	}
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd
}

// SetEventSink mirrors normalized runtime events into the Phase 2 bus.
func (c *ChatRuntime) SetEventSink(sink func(Event)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sink = sink
}

// SetStateTouch is called after runtime-owned state.db writes so the dashboard
// manager can recompute and publish state_update.
func (c *ChatRuntime) SetStateTouch(touch func(string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.touch = touch
}

// SetPersistence enables durable transcript writes and state.db indexing.
func (c *ChatRuntime) SetPersistence(home string, open TranscriptOpener, ix PersistenceIndexer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.transcriptHome = home
	c.openTranscript = open
	c.indexer = ix
}

// agentState is the live, in-memory state for one running agent.
type agentState struct {
	agentID   string
	cmd       *exec.Cmd
	pgid      int
	sessionID string
	transport *Transport
	hub       *Hub
	stdin     interface{ Close() error }
	stderr    *ringBuffer

	ctx    context.Context // turn-scoped base context, cancelled on Stop
	cancel context.CancelFunc

	skipPerms bool // auto-approve every permission request (techspec §5.2)

	mu         sync.Mutex
	seq        int64
	turnSeq    int64
	contextPct float64
	turnActive bool
	toolNames  map[string]string       // toolCallID -> normalized name (for status detail)
	pending    map[string]*pendingPerm // toolCallID -> withheld permission request
	resolved   map[string]struct{}     // toolCallIDs already settled this turn
	transcript []Event
	writer     TranscriptWriter
	stopped    bool
}

// pendingPerm is a withheld session/request_permission awaiting a decision.
type pendingPerm struct {
	req       *IncomingRequest
	name      string
	optByKind map[string]string // kind -> optionId
	timer     *time.Timer
}

func (c *ChatRuntime) Start(ctx context.Context, spec LaunchSpec) (*Handle, error) {
	ad, err := c.adapterFor(spec.BackendType)
	if err != nil {
		return nil, err
	}

	cmd := c.spawnCmd(ad, spec)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("runtime: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("runtime: stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("runtime: stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("runtime: start %s: %w", c.command, err)
	}
	pgid := cmd.Process.Pid // the child is the group leader (Setpgid)

	actx, acancel := context.WithCancel(context.Background())
	as := &agentState{
		agentID:   spec.Agent.AgentID,
		cmd:       cmd,
		pgid:      pgid,
		hub:       NewHub(),
		stdin:     stdin,
		stderr:    newRingBuffer(16 * 1024),
		ctx:       actx,
		cancel:    acancel,
		skipPerms: spec.SkipPerms,
		toolNames: map[string]string{},
		pending:   map[string]*pendingPerm{},
		resolved:  map[string]struct{}{},
	}
	as.transport = NewTransport(stdin,
		func(method string, params json.RawMessage) { c.onNotification(as, method, params) },
		func(req *IncomingRequest) { c.onRequest(as, req) },
	)

	go as.stderr.copyFrom(stderr)
	go func() {
		_ = as.transport.Run(stdout)
		c.onTransportClosed(as)
	}()

	// ACP handshake: initialize then session/new (techspec §4.1).
	initRes, err := as.transport.Call(ctx, "initialize", map[string]any{
		"protocolVersion":    1,
		"clientCapabilities": map[string]any{},
	})
	if err != nil {
		as.shutdown()
		return nil, fmt.Errorf("runtime: initialize: %w", err)
	}
	if err := checkACPVersion(initRes); err != nil {
		as.shutdown()
		return nil, err
	}
	newRes, err := as.transport.Call(ctx, "session/new", sessionNewParams(spec))
	if err != nil {
		as.shutdown()
		return nil, fmt.Errorf("runtime: session/new: %w", err)
	}
	var sess struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(newRes, &sess); err != nil || sess.SessionID == "" {
		as.shutdown()
		return nil, fmt.Errorf("runtime: session/new returned no sessionId")
	}
	as.sessionID = sess.SessionID
	if err := c.openPersistence(as, spec, sess.SessionID); err != nil {
		as.shutdown()
		return nil, err
	}

	// Persist running + initial status rows (state.db is the sole writer).
	now := time.Now().UTC()
	if err := c.store.WriteRunning(state.RunningEntry{
		AgentID: as.agentID, PID: pgid, SessionID: sess.SessionID,
		Interface: "chat", HookToken: spec.HookToken, StartedAt: now,
	}); err != nil {
		as.shutdown()
		return nil, fmt.Errorf("runtime: write running: %w", err)
	}
	if err := c.writeStatus(as, state.Status{
		AgentID: as.agentID, State: "idle", Detail: "ready",
		LastTrace: "SessionStart", ContextPct: 0,
	}); err != nil {
		as.shutdown()
		_ = c.store.DeleteRunning(as.agentID)
		return nil, fmt.Errorf("runtime: write status: %w", err)
	}

	c.mu.Lock()
	c.agents[as.agentID] = as
	c.mu.Unlock()

	return &Handle{AgentID: as.agentID, Pid: pgid, SessionID: sess.SessionID}, nil
}

func (c *ChatRuntime) SendPrompt(ctx context.Context, agentID, text string) error {
	as, err := c.lookup(agentID)
	if err != nil {
		return err
	}

	as.mu.Lock()
	if as.turnActive {
		as.mu.Unlock()
		return ErrTurnInFlight
	}
	as.turnActive = true
	as.resolved = map[string]struct{}{}
	turnID := as.nextTurnIDLocked()
	as.mu.Unlock()
	if err := c.store.ResetTurnBudget(as.agentID, turnID); err != nil {
		as.mu.Lock()
		as.turnActive = false
		as.mu.Unlock()
		return err
	}

	// busy / thinking (techspec §4.4).
	now := time.Now().UTC()
	_ = c.writeStatus(as, state.Status{
		AgentID: as.agentID, State: "busy", Detail: "thinking",
		LastTrace: "UserPromptSubmit", BusySince: &now, ContextPct: as.lastPct(),
	})

	// Drive the turn asynchronously: notifications stream over the hub while the
	// prompt Call blocks for the result. SendPrompt itself returns immediately.
	go func() {
		params := map[string]any{
			"sessionId": as.sessionID,
			"prompt":    []map[string]any{{"type": "text", "text": text}},
		}
		res, err := as.transport.Call(as.ctx, "session/prompt", params)
		if err != nil {
			as.mu.Lock()
			as.turnActive = false
			as.mu.Unlock()
			// Transport closed (crash/stop) is owned by onTransportClosed / Stop.
			// A genuine RPC error while the process lives surfaces here.
			if errors.Is(err, errTransportClosed) || as.isStopped() {
				return
			}
			c.emit(as, EvError, ErrorData{Scope: "protocol", Message: err.Error(), Fatal: false})
			td := TurnEndData{StopReason: "error", ContextPct: as.lastPct()}
			c.applyTurnEndStatus(as, td)
			c.emit(as, EvTurnEnd, td)
			return
		}
		td, hasPct := mapPromptResult(res)
		as.mu.Lock()
		as.turnActive = false
		if hasPct {
			as.contextPct = td.ContextPct
		} else {
			td.ContextPct = as.contextPct
		}
		as.mu.Unlock()

		// Write the idle status row before emitting turn_end so a client that
		// reacts to turn_end never observes a stale busy row.
		c.applyTurnEndStatus(as, td)
		c.emit(as, EvTurnEnd, td)
	}()

	return nil
}

func (c *ChatRuntime) Stop(ctx context.Context, agentID string) error {
	c.mu.Lock()
	as, ok := c.agents[agentID]
	if ok {
		delete(c.agents, agentID)
	}
	c.mu.Unlock()
	if !ok {
		// The runtime doesn't own this agent — typically after a dashboard restart,
		// where ReconcileStale intentionally never re-adopts a still-live PID. Don't
		// silently succeed: if the recorded process group is alive, kill it before
		// clearing the row, otherwise the CLI keeps running invisible and unkillable
		// from the UI (Finding 5).
		c.reconcileOrphanStop(agentID)
		_ = c.store.DeleteRunning(agentID)
		c.touchState(agentID)
		return nil
	}

	as.shutdown()
	// closePersistence is called in onTransportClosed's early-return path after
	// the transport goroutine exits, so all in-flight emit() calls complete first.
	_ = c.store.DeleteRunning(agentID)
	// Keep the status row so the archive/UI can show a final state (§7.5).
	st, err := c.store.ReadStatus(agentID)
	if err != nil {
		st = state.Status{AgentID: agentID, ContextPct: as.lastPct()}
	}
	st.State = "done"
	st.BusySince = nil
	_ = c.store.WriteStatus(st)
	c.touchState(agentID)
	as.hub.Close()
	return nil
}

// reconcileOrphanStop handles a Stop on an agent this runtime no longer owns in
// memory (e.g. post-restart, where reconcile leaves the live PID un-adopted). If
// the recorded process group is still alive it is SIGTERM'd, then SIGKILL'd after
// a short grace, so Stop never reports success while a live child keeps running
// (Finding 5). No live PID → nothing to kill.
func (c *ChatRuntime) reconcileOrphanStop(agentID string) {
	row, err := c.store.ReadRunning(agentID)
	if err != nil {
		return // no running row → nothing to reconcile
	}
	if row.PID <= 0 || !pidAlive(row.PID) {
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
			if !pidAlive(row.PID) {
				return
			}
		}
	}
}

func (c *ChatRuntime) Subscribe(agentID string) (<-chan Event, func(), error) {
	as, err := c.lookup(agentID)
	if err != nil {
		return nil, nil, err
	}
	ch, cancel := as.hub.Subscribe()
	return ch, cancel, nil
}

func (c *ChatRuntime) Transcript(agentID string) ([]Event, error) {
	as, err := c.lookup(agentID)
	if err != nil {
		return nil, err
	}
	as.mu.Lock()
	defer as.mu.Unlock()
	out := make([]Event, len(as.transcript))
	copy(out, as.transcript)
	return out, nil
}

// Cancel and Permission live in permission.go.

// --- still-stubbed methods (later phases) ---

func (c *ChatRuntime) Resume(ctx context.Context, spec LaunchSpec, sessionID string) (*Handle, error) {
	ad, err := c.adapterFor(spec.BackendType)
	if err != nil {
		return nil, err
	}

	// Spawn process (identical to Start).
	cmd := c.spawnCmd(ad, spec)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("runtime: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("runtime: stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("runtime: stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("runtime: start %s: %w", c.command, err)
	}
	pgid := cmd.Process.Pid

	actx, acancel := context.WithCancel(context.Background())
	as := &agentState{
		agentID:    spec.Agent.AgentID,
		cmd:        cmd,
		pgid:       pgid,
		hub:        NewHub(),
		stdin:      stdin,
		stderr:     newRingBuffer(16 * 1024),
		ctx:        actx,
		cancel:     acancel,
		skipPerms:  spec.SkipPerms,
		toolNames:  map[string]string{},
		pending:    map[string]*pendingPerm{},
		resolved:   map[string]struct{}{},
		contextPct: spec.LastContextPct,
	}
	as.transport = NewTransport(stdin,
		func(method string, params json.RawMessage) { c.onNotification(as, method, params) },
		func(req *IncomingRequest) { c.onRequest(as, req) },
	)

	go as.stderr.copyFrom(stderr)
	go func() {
		_ = as.transport.Run(stdout)
		c.onTransportClosed(as)
	}()

	// ACP handshake: initialize.
	initRes, err := as.transport.Call(ctx, "initialize", map[string]any{
		"protocolVersion":    1,
		"clientCapabilities": map[string]any{},
	})
	if err != nil {
		as.shutdown()
		return nil, fmt.Errorf("runtime: initialize: %w", err)
	}
	if err := checkACPVersion(initRes); err != nil {
		as.shutdown()
		return nil, err
	}

	// Try session/load to restore native context; fall back to session/new.
	// The load params must carry the current cwd + freshly-minted MCP servers
	// (ACP loadSession takes the same registration shape as newSession), or an
	// adapter where session/load succeeds would run without the in-process
	// messaging MCP server Phase 5 depends on.
	newSessionID := ""
	if sessionID != "" {
		if loadRes, loadErr := as.transport.Call(ctx, "session/load", sessionLoadParams(spec, sessionID)); loadErr == nil {
			var loaded struct {
				SessionID string `json:"sessionId"`
			}
			if json.Unmarshal(loadRes, &loaded) == nil && loaded.SessionID != "" {
				newSessionID = loaded.SessionID
			}
		}
	}
	if newSessionID == "" {
		newRes, err := as.transport.Call(ctx, "session/new", sessionNewParams(spec))
		if err != nil {
			as.shutdown()
			return nil, fmt.Errorf("runtime: session/new: %w", err)
		}
		var sess struct {
			SessionID string `json:"sessionId"`
		}
		if err := json.Unmarshal(newRes, &sess); err != nil || sess.SessionID == "" {
			as.shutdown()
			return nil, fmt.Errorf("runtime: session/new returned no sessionId")
		}
		newSessionID = sess.SessionID
	}
	as.sessionID = newSessionID

	// Re-open the existing transcript in append mode (Open skips seq:0 meta for existing files).
	if err := c.openPersistence(as, spec, newSessionID); err != nil {
		as.shutdown()
		return nil, err
	}

	// Append resumed session_meta with resumed_at to the transcript so the raw
	// log has a resume boundary marker and the archive can track the new session_id.
	resumeNow := time.Now().UTC().Format(time.RFC3339)
	resumedMeta := runtimeMeta(spec, newSessionID)
	resumedMeta.ResumedAt = &resumeNow
	c.emit(as, EvSessionMeta, resumedMeta)

	// Write fresh running row + status row with restored context_pct.
	now := time.Now().UTC()
	if err := c.store.WriteRunning(state.RunningEntry{
		AgentID: as.agentID, PID: pgid, SessionID: newSessionID,
		Interface: "chat", HookToken: spec.HookToken, StartedAt: now,
	}); err != nil {
		as.shutdown()
		return nil, fmt.Errorf("runtime: write running: %w", err)
	}
	if err := c.writeStatus(as, state.Status{
		AgentID: as.agentID, State: "idle", Detail: "resumed",
		LastTrace: "SessionStart", ContextPct: spec.LastContextPct,
	}); err != nil {
		as.shutdown()
		_ = c.store.DeleteRunning(as.agentID)
		return nil, fmt.Errorf("runtime: write status: %w", err)
	}

	c.mu.Lock()
	c.agents[as.agentID] = as
	c.mu.Unlock()

	return &Handle{AgentID: as.agentID, Pid: pgid, SessionID: newSessionID}, nil
}

func (c *ChatRuntime) CheckMessages(ctx context.Context, pid int) error {
	as, err := c.lookupByPID(pid)
	if err != nil {
		return err
	}
	st, err := c.store.ReadStatus(as.agentID)
	if err != nil || st.State != "idle" {
		return nil
	}
	as.mu.Lock()
	if as.turnActive {
		as.mu.Unlock()
		return nil
	}
	as.turnActive = true
	turnID := as.nextTurnIDLocked()
	as.mu.Unlock()
	if err := c.store.ResetTurnBudget(as.agentID, turnID); err != nil {
		as.mu.Lock()
		as.turnActive = false
		as.mu.Unlock()
		return err
	}

	now := time.Now().UTC()
	_ = c.writeStatus(as, state.Status{
		AgentID: as.agentID, State: "busy", Detail: "checking messages",
		LastTrace: "MessageNudge", BusySince: &now, ContextPct: as.lastPct(),
	})

	go func() {
		params := map[string]any{
			"sessionId": as.sessionID,
			"prompt": []map[string]any{{"type": "text",
				"text": "You have new messages. Call the check_messages tool and handle them."}},
		}
		res, err := as.transport.Call(as.ctx, "session/prompt", params)
		if err != nil {
			as.mu.Lock()
			as.turnActive = false
			as.mu.Unlock()
			if errors.Is(err, errTransportClosed) || as.isStopped() {
				return
			}
			c.emit(as, EvError, ErrorData{Scope: "protocol", Message: err.Error(), Fatal: false})
			td := TurnEndData{StopReason: "error", ContextPct: as.lastPct()}
			c.applyTurnEndStatus(as, td)
			c.emit(as, EvTurnEnd, td)
			return
		}
		td, hasPct := mapPromptResult(res)
		as.mu.Lock()
		as.turnActive = false
		if hasPct {
			as.contextPct = td.ContextPct
		} else {
			td.ContextPct = as.contextPct
		}
		as.mu.Unlock()
		c.applyTurnEndStatus(as, td)
		c.emit(as, EvTurnEnd, td)
	}()
	return nil
}

// --- internals ---

func (c *ChatRuntime) lookup(agentID string) (*agentState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	as, ok := c.agents[agentID]
	if !ok {
		return nil, ErrNoHandle
	}
	return as, nil
}

func (c *ChatRuntime) lookupByPID(pid int) (*agentState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, as := range c.agents {
		if as.pgid == pid {
			return as, nil
		}
	}
	return nil, ErrNoHandle
}

// onNotification dispatches a server→client notification. session/update frames
// are mapped to normalized events; everything else is ignored this phase.
func (c *ChatRuntime) onNotification(as *agentState, method string, params json.RawMessage) {
	if method != "session/update" {
		return
	}
	for _, m := range mapSessionUpdate(params) {
		c.emit(as, m.Type, m.Data)
		c.applyEventStatus(as, m)
	}
}

// onTransportClosed handles the read loop ending (EOF or scanner error). If we
// initiated shutdown (Stop), the rows are already handled. Otherwise the process
// crashed mid-session: emit error{fatal:true} + turn_end{error}, set the status
// row to error, delete the running row, and tear down (techspec §8.2).
func (c *ChatRuntime) onTransportClosed(as *agentState) {
	as.mu.Lock()
	if as.stopped {
		as.turnActive = false
		as.mu.Unlock()
		// Stop() already handled state.db cleanup; close the transcript writer now
		// that the transport goroutine (and any in-flight emit calls) have exited.
		as.closePersistence()
		return
	}
	as.stopped = true
	as.turnActive = false
	pend := as.pending
	as.pending = map[string]*pendingPerm{}
	as.mu.Unlock()

	// Abandon any held permission (the process is gone; the request can't be answered).
	for _, p := range pend {
		if p.timer != nil {
			p.timer.Stop()
		}
	}

	_ = as.cmd.Wait() // reap the exited process

	tail := as.stderr.Tail()
	c.emit(as, EvError, ErrorData{Scope: "process", Message: strutil.FirstNonEmpty(tail, "process exited"), Fatal: true})

	// Settle state.db before emitting turn_end so a client reacting to turn_end
	// never observes a stale running row or busy status.
	c.updateStatus(as, "error", clip(tail, 120), "Error", clearBusySince)
	_ = c.store.DeleteRunning(as.agentID)
	c.touchState(as.agentID)
	c.removeAgent(as.agentID)
	// Tell the Registry the handle is gone so it drops ownership; otherwise a
	// relaunch/resume on this agent_id is rejected with ErrAlreadyStarted while
	// the runtime has no handle to serve it (techspec §8.2). Done before the
	// turn_end emit so a client reacting to turn_end can immediately relaunch.
	if c.onExit != nil {
		c.onExit(as.agentID)
	}

	c.emit(as, EvTurnEnd, TurnEndData{StopReason: "error", ContextPct: as.lastPct()})
	as.closePersistence()
	as.cancel()
	as.hub.Close()
}

func (c *ChatRuntime) removeAgent(agentID string) {
	c.mu.Lock()
	delete(c.agents, agentID)
	c.mu.Unlock()
}

// emit stamps seq/agent_id/ts, marshals the payload, and publishes to the hub.
// seq increment and transcript append are done under a single as.mu acquisition
// so concurrent emitters cannot interleave their events in the in-memory log.
func (c *ChatRuntime) emit(as *agentState, typ string, data any) {
	raw, err := json.Marshal(data)
	if err != nil {
		slog.Error("runtime: marshal event payload", "type", typ, "err", err)
		return
	}
	as.mu.Lock()
	as.seq++
	ev := Event{
		AgentID: as.agentID,
		Seq:     as.seq,
		Type:    typ,
		Data:    raw,
		Ts:      time.Now().UTC().Format(time.RFC3339),
	}
	as.transcript = append(as.transcript, ev)
	as.mu.Unlock()
	// Durability is best-effort: a transcript write failure is logged inside
	// persistEvent but must NOT suppress live delivery, or subscribers' in-memory
	// view would silently diverge from Transcript() on a disk error.
	c.persistEvent(as, ev)
	as.hub.Publish(ev)
	c.mu.Lock()
	sink := c.sink
	c.mu.Unlock()
	if sink != nil {
		sink(ev)
	}
}

func (c *ChatRuntime) openPersistence(as *agentState, spec LaunchSpec, sessionID string) error {
	c.mu.Lock()
	home := c.transcriptHome
	open := c.openTranscript
	ix := c.indexer
	c.mu.Unlock()
	if home == "" || open == nil || ix == nil {
		return nil
	}
	meta := runtimeMeta(spec, sessionID)
	w, err := open(home, spec.Agent.AgentID, &meta)
	if err != nil {
		return fmt.Errorf("runtime: open transcript: %w", err)
	}
	if err := ix.UpsertSessionMeta(spec.Agent.AgentID, meta); err != nil {
		_ = w.Close()
		return fmt.Errorf("runtime: index session meta: %w", err)
	}
	as.writer = w
	as.seq = w.NextSeq() - 1
	return nil
}

func (c *ChatRuntime) persistEvent(as *agentState, ev Event) bool {
	c.mu.Lock()
	ix := c.indexer
	c.mu.Unlock()
	as.mu.Lock()
	w := as.writer
	as.mu.Unlock()
	if w == nil || ix == nil {
		return true
	}
	if err := w.Append(ev); err != nil {
		slog.Error("runtime: append transcript", "agent", as.agentID, "seq", ev.Seq, "err", err)
		return false
	}
	if err := ix.OnEvent(as.agentID, ev); err != nil {
		slog.Error("runtime: index event", "agent", as.agentID, "seq", ev.Seq, "err", err)
	}
	if ev.Type == EvError {
		_ = w.Sync()
	}
	if ev.Type == EvTurnEnd {
		_ = w.Sync()
		rollup := TurnRollup{LastSeq: ev.Seq, UpdatedAt: ev.Ts, LastContextPct: as.lastPct()}
		var td TurnEndData
		if err := json.Unmarshal(ev.Data, &td); err == nil {
			rollup.LastContextPct = td.ContextPct
		}
		if err := ix.OnTurnEnd(as.agentID, rollup); err != nil {
			slog.Error("runtime: index turn end", "agent", as.agentID, "seq", ev.Seq, "err", err)
		}
	}
	return true
}

func (as *agentState) closePersistence() {
	as.mu.Lock()
	w := as.writer
	as.writer = nil
	as.mu.Unlock()
	if w != nil {
		_ = w.Close()
	}
}

// NewSessionMeta builds the SessionMetaData for a launch/resume. Exported so the
// terminal runtime (a subpackage) can create the same session_meta the chat
// runtime does, keeping session-row creation interface-agnostic.
func NewSessionMeta(spec LaunchSpec, sessionID string) SessionMetaData {
	return runtimeMeta(spec, sessionID)
}

func runtimeMeta(spec LaunchSpec, sessionID string) SessionMetaData {
	var sha string
	if spec.SystemPrompt != "" {
		sum := sha256sum(spec.SystemPrompt)
		sha = fmt.Sprintf("%x", sum[:])
	}
	return SessionMetaData{
		Name:            spec.Agent.Name,
		Role:            spec.Agent.Role,
		Project:         spec.Agent.Project,
		Backend:         spec.Agent.Backend,
		Model:           spec.Agent.Model,
		Interface:       spec.Agent.Interface,
		Group:           spec.Agent.Group,
		Cwd:             spec.Cwd,
		SystemPrompt:    spec.SystemPrompt,
		SystemPromptSHA: sha,
		EnvKeys:         envKeys(spec.Env),
		CreatedAt:       spec.Agent.CreatedAt.UTC().Format(time.RFC3339),
		SessionID:       sessionID,
	}
}

func envKeys(env []string) []string {
	keys := make([]string, 0, len(env))
	for _, kv := range env {
		if i := strings.Index(kv, "="); i > 0 {
			keys = append(keys, kv[:i])
		}
	}
	return keys
}

// applyEventStatus writes the §4.4 status transition implied by a streamed event.
func (c *ChatRuntime) applyEventStatus(as *agentState, m mappedEvent) {
	switch m.Type {
	case EvToolCall:
		d := m.Data.(ToolCallData)
		as.mu.Lock()
		as.toolNames[d.ToolCallID] = d.Name
		as.mu.Unlock()
		c.updateStatus(as, "busy", "Running "+d.Name, "PreToolUse: "+d.Name, keepBusySince)
	case EvToolResult:
		d := m.Data.(ToolResultData)
		name := as.toolNameFor(d.ToolCallID)
		c.updateStatus(as, "busy", name+" done", "PostToolUse: "+name, keepBusySince)
	}
	// assistant_text / diff carry no status transition (agent stays busy).
}

func (c *ChatRuntime) applyTurnEndStatus(as *agentState, td TurnEndData) {
	switch td.StopReason {
	case "cancelled":
		c.updateStatus(as, "idle", "cancelled", "Cancelled", clearBusySince)
	case "error":
		c.updateStatus(as, "error", "turn failed", "Error", clearBusySince)
	default:
		c.updateStatus(as, "idle", "", "Stop", clearBusySince)
	}
}

type busySinceMode int

const (
	keepBusySince busySinceMode = iota
	clearBusySince
)

// updateStatus reads the current row, applies the transition, and writes it back.
func (c *ChatRuntime) updateStatus(as *agentState, st, detail, trace string, mode busySinceMode) {
	cur, err := c.store.ReadStatus(as.agentID)
	if err != nil {
		cur = state.Status{AgentID: as.agentID}
	}
	cur.State = st
	cur.Detail = detail
	cur.LastTrace = trace
	cur.ContextPct = as.lastPct()
	if mode == clearBusySince {
		cur.BusySince = nil
	}
	if err := c.store.WriteStatus(cur); err != nil {
		slog.Error("runtime: write status", "agent", as.agentID, "err", err)
	}
	c.touchState(as.agentID)
}

// writeStatus writes a fully-specified status row.
func (c *ChatRuntime) writeStatus(as *agentState, st state.Status) error {
	if err := c.store.WriteStatus(st); err != nil {
		return err
	}
	c.touchState(as.agentID)
	return nil
}

func (c *ChatRuntime) touchState(agentID string) {
	c.mu.Lock()
	touch := c.touch
	c.mu.Unlock()
	if touch != nil {
		touch(agentID)
	}
}

// stripEnv returns env without any "KEY=..." entries for the given key.
func stripEnv(env []string, key string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// clip truncates s to at most n bytes (for status detail fields, ≤120 chars).
func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func (as *agentState) lastPct() float64 {
	as.mu.Lock()
	defer as.mu.Unlock()
	return as.contextPct
}

func (as *agentState) nextTurnIDLocked() string {
	as.turnSeq++
	return fmt.Sprintf("t_%012d", as.turnSeq)
}

func (as *agentState) toolNameFor(id string) string {
	as.mu.Lock()
	defer as.mu.Unlock()
	if n, ok := as.toolNames[id]; ok && n != "" {
		return n
	}
	return "tool"
}

func (as *agentState) isStopped() bool {
	as.mu.Lock()
	defer as.mu.Unlock()
	return as.stopped
}

func sha256sum(s string) []byte {
	h := sha256.Sum256([]byte(s))
	return h[:]
}

// shutdown terminates the process group (SIGTERM→grace→SIGKILL) and cancels the
// turn context. Idempotent.
func (as *agentState) shutdown() {
	as.mu.Lock()
	if as.stopped {
		as.mu.Unlock()
		return
	}
	as.stopped = true
	as.mu.Unlock()

	as.cancel()
	_ = as.stdin.Close()

	if as.cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-as.pgid, syscall.SIGTERM)

	waited := make(chan struct{})
	go func() { _ = as.cmd.Wait(); close(waited) }()
	select {
	case <-waited:
	case <-time.After(stopGrace):
		_ = syscall.Kill(-as.pgid, syscall.SIGKILL)
		<-waited
	}
}

// Pinned ACP protocol range (§12.1). The Go client targets the version the
// pinned claude-code-acp adapter negotiates; today that is exactly 1.
const (
	minACPVersion = 1
	maxACPVersion = 1
)

// checkACPVersion enforces the pinned ACP protocol range on the initialize
// result (§12.1). A reported version outside [minACPVersion, maxACPVersion]
// fails the handshake with ErrProtocolVersion. A missing/unparseable version
// (0) is tolerated — there is nothing to negotiate against, so we proceed
// best-effort rather than refusing adapters that omit the field.
func checkACPVersion(initRes json.RawMessage) error {
	var initResp struct {
		ProtocolVersion int `json:"protocolVersion"`
	}
	if err := json.Unmarshal(initRes, &initResp); err != nil {
		return nil
	}
	if initResp.ProtocolVersion == 0 {
		return nil
	}
	if initResp.ProtocolVersion < minACPVersion || initResp.ProtocolVersion > maxACPVersion {
		return fmt.Errorf("%w: adapter negotiated %d, supported range [%d,%d]",
			ErrProtocolVersion, initResp.ProtocolVersion, minACPVersion, maxACPVersion)
	}
	return nil
}

// sessionNewParams builds the session/new params from the launch spec (§4.1).
func sessionNewParams(spec LaunchSpec) map[string]any {
	mcp := make([]map[string]any, 0, len(spec.MCPServers))
	for _, m := range spec.MCPServers {
		mcp = append(mcp, mcpServerParam(m))
	}
	return map[string]any{
		"cwd":                   spec.Cwd,
		"mcpServers":            mcp,
		"model":                 spec.ModelID,
		"systemPrompt":          spec.StartSystemPrompt(),
		"additionalDirectories": spec.AddDirs,
	}
}

// sessionLoadParams builds the session/load params. It carries the SAME fields as
// session/new (cwd, mcpServers, model, systemPrompt, additionalDirectories) plus
// the sessionId to restore — so resuming applies the freshly-minted messaging MCP
// server AND the current model/system-prompt on the native-resume path, not only
// on the session/new fallback. Without model here, a same-backend model swap that
// uses native resume (CanSwitchModelOnResume) would silently keep the old model.
func sessionLoadParams(spec LaunchSpec, sessionID string) map[string]any {
	mcp := make([]map[string]any, 0, len(spec.MCPServers))
	for _, m := range spec.MCPServers {
		mcp = append(mcp, mcpServerParam(m))
	}
	return map[string]any{
		"sessionId":             sessionID,
		"cwd":                   spec.Cwd,
		"mcpServers":            mcp,
		"model":                 spec.ModelID,
		"systemPrompt":          spec.StartSystemPrompt(),
		"additionalDirectories": spec.AddDirs,
	}
}

func mcpServerParam(m MCPServerSpec) map[string]any {
	if m.Type == "http" {
		return map[string]any{
			"name":    m.Name,
			"type":    "http",
			"url":     m.URL,
			"headers": m.Headers,
		}
	}
	return map[string]any{
		"name": m.Name, "command": m.Command, "args": m.Args, "env": m.Env,
	}
}
