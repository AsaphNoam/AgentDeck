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
	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/state"
	"github.com/agentdeck/agentdeck/internal/strutil"
)

// stopGrace is how long Stop waits after SIGTERM before SIGKILL (techspec §8.5).
const stopGrace = 5 * time.Second

// defaultCancelGrace is how long Cancel waits for a peer to honor session/cancel
// before escalating to SIGINT on the process group (techspec §8.4).
const defaultCancelGrace = 3 * time.Second

// defaultStartupCallTimeout bounds each ACP handshake request independently.
// The pinned Claude adapter has historically taken about 15 seconds to create a
// session, so 30 seconds leaves headroom without letting a wedged child hold an
// HTTP launch forever (TS-04.R22).
const defaultStartupCallTimeout = 30 * time.Second

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
	onExit func(agentID, generation string)

	cancelGrace        time.Duration // Cancel→SIGINT escalation window (§8.4)
	startupCallTimeout time.Duration // initialize/session load/new deadline (TS-04.R22)

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
		store:              s,
		agents:             map[string]*agentState{},
		cancelGrace:        defaultCancelGrace,
		startupCallTimeout: defaultStartupCallTimeout,
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
func (c *ChatRuntime) spawnCmd(ad backend.BackendAdapter, spec LaunchSpec) (*exec.Cmd, error) {
	bin, args := c.command, c.cmdArgs
	if bin == "" {
		bin, args = ad.Binary(), ad.LaunchArgs()
	}
	// ExtraArgs carries launch-time hook registration flags (e.g. claude's
	// --settings <per-agent hooks file>), composed by the server (techspec §2.3).
	args = append(append([]string{}, args...), spec.ExtraArgs...)
	cmd := exec.Command(bin, args...)
	cmd.Dir = spec.Cwd
	env := spec.StartEnv()
	if len(env) == 0 {
		env = os.Environ()
	}
	for _, k := range ad.StripEnvKeys() {
		env = stripEnv(env, k)
	}
	// Adapter-contributed env (e.g. OpenHands LLM_MODEL, OpenCode yolo config)
	// is applied AFTER stripping so it is the single authoritative value for its
	// keys (each such key is listed in StripEnvKeys — see backend.ExtraEnvProvider).
	if ep, ok := ad.(backend.ExtraEnvProvider); ok {
		env = append(env, ep.ExtraEnv(spec.ModelID, spec.SkipPerms)...)
	}
	if spec.BackendType == "codex-acp" {
		var err error
		env, err = withCodexDeveloperInstructions(env, spec.StartSystemPrompt())
		if err != nil {
			return nil, err
		}
	}
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd, nil
}

// startCmd starts a child after refreshing its Codex profile when applicable.
// The Codex refresh lock remains held through cmd.Start, so another launch
// cannot publish a different setup generation in the interval between this
// launch's refresh and its process creation (INV §5/§15).
func (c *ChatRuntime) startCmd(cmd *exec.Cmd, spec LaunchSpec) error {
	if spec.BackendType == "codex-acp" {
		if home := envValue(cmd.Env, "CODEX_HOME"); home != "" {
			if err := config.WithRefreshedCodexProfile(home, cmd.Start); err != nil {
				return fmt.Errorf("runtime: refresh/start codex profile: %w", err)
			}
			return nil
		}
	}
	return cmd.Start()
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
	agentID    string
	generation string
	cmd        *exec.Cmd
	pgid       int
	sessionID  string
	transport  *Transport
	adapter    backend.BackendAdapter
	// configOptions is the live session's configuration advertisement, kept
	// current by every set_config_option response because a model change rebuilds
	// it (TS-04.R46). Guarded by mu, like every other live session field.
	configOptions sessionConfigAdvertisement
	hub           *Hub
	stdin         interface{ Close() error }
	stderr        *ringBuffer
	stderrDone    chan struct{}

	ctx    context.Context // turn-scoped base context, cancelled on Stop
	cancel context.CancelFunc

	skipPerms        bool // auto-approve every permission request (techspec §5.2)
	autoApproveTools map[string]struct{}

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
	// commands is the latest ACP available-commands snapshot (TS-04.R24). It is
	// replace-only live state, not a transcript event: each update overwrites it,
	// an empty update clears it, and it dies with this agentState on stop/crash.
	commands []CommandItem
	stopped  bool
	// cancelEscalated is set only when the current turn ignored cooperative
	// cancellation and the fallback SIGINT was delivered successfully.
	cancelEscalated bool
	// loadReplay is set only while ACP session/load is restoring provider-native
	// context. A provider is allowed to replay the prior conversation as
	// session/update frames during that call; those frames describe history
	// AgentDeck already holds, not new activity, so they must not cross the
	// runtime boundary as live transcript events (TS-04.R50, FS-03.R3/R35,
	// INV §1, §11).
	loadReplay        bool
	loadReplayDropped int
	// steering records whether this session's adapter advertised the ACP steering
	// extension at handshake (TS-04.R49). Decided once per process from that
	// advertisement alone, never from the adapter version or the backend type.
	steering bool
	// held is the person's queued follow-up: at most one message per agent, live
	// state only (FS-03.R48, TS-01.R29, TS-02.R31). Submitting another replaces
	// it, turn end delivers it, and it dies with this agentState on stop or crash
	// — there is nothing to persist and nothing to reap (INV §4, §16).
	held string
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

	cmd, err := c.spawnCmd(ad, spec)
	if err != nil {
		return nil, err
	}

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
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, fmt.Errorf("runtime: stderr pipe: %w", err)
	}
	if err := c.startCmd(cmd, spec); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		_ = stderr.Close()
		return nil, fmt.Errorf("runtime: start %s: %w", c.command, err)
	}
	pgid := cmd.Process.Pid // the child is the group leader (Setpgid)

	actx, acancel := context.WithCancel(context.Background())
	as := &agentState{
		agentID:          spec.Agent.AgentID,
		generation:       spec.Generation,
		cmd:              cmd,
		pgid:             pgid,
		hub:              NewHub(),
		stdin:            stdin,
		stderr:           newRingBuffer(16 * 1024),
		stderrDone:       make(chan struct{}),
		ctx:              actx,
		cancel:           acancel,
		skipPerms:        spec.SkipPerms,
		autoApproveTools: copyStringSet(spec.AutoApproveTools),
		toolNames:        map[string]string{},
		pending:          map[string]*pendingPerm{},
		resolved:         map[string]struct{}{},
		adapter:          ad,
	}
	as.transport = NewTransport(stdin,
		func(method string, params json.RawMessage) { c.onNotification(as, method, params) },
		func(req *IncomingRequest) { c.onRequest(as, req) },
	)

	go func() {
		as.stderr.copyFrom(stderr)
		close(as.stderrDone)
	}()
	go func() {
		_ = as.transport.Run(stdout)
		c.onTransportClosed(as)
	}()

	// ACP handshake: initialize then session/new (techspec §4.1).
	initRes, err := c.startupCall(ctx, as.transport, "initialize", map[string]any{
		"protocolVersion":    1,
		"clientCapabilities": map[string]any{},
	})
	if err != nil {
		return nil, c.startupFailure(as, spec.BackendType, "initialize", err)
	}
	if err := checkACPVersion(initRes); err != nil {
		as.shutdown()
		return nil, err
	}
	as.setSteering(decodeSteeringSupport(initRes))
	newRes, err := c.startupCall(ctx, as.transport, "session/new", sessionNewParams(spec))
	if err != nil {
		return nil, c.startupFailure(as, spec.BackendType, "session/new", err)
	}
	var sess struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(newRes, &sess); err != nil || sess.SessionID == "" {
		as.shutdown()
		return nil, fmt.Errorf("runtime: session/new returned no sessionId")
	}
	as.sessionID = sess.SessionID
	as.configOptions = decodeSessionConfigOptions(newRes)
	applied, err := applySessionConfig(ctx, as.transport, ad, spec, sess.SessionID, as.configOptions)
	if err != nil {
		as.shutdown()
		return nil, err
	}
	spec.Agent.Fast = applied.Fast
	if err := c.openPersistence(as, spec, sess.SessionID); err != nil {
		as.shutdown()
		return nil, err
	}

	// Persist running + initial status rows (state.db is the sole writer).
	now := time.Now().UTC()
	if err := c.store.WriteRunning(state.RunningEntry{
		AgentID: as.agentID, PID: pgid, SessionID: sess.SessionID,
		Interface: "chat", HookToken: spec.HookToken, StartedAt: now,
		FastAvailable: applied.FastAvailable, SteeringAvailable: as.steeringSupported(),
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

	return &Handle{AgentID: as.agentID, Pid: pgid, SessionID: sess.SessionID, Fast: applied.Fast}, nil
}

func (c *ChatRuntime) SendPrompt(ctx context.Context, agentID, text string) error {
	as, err := c.lookup(agentID)
	if err != nil {
		return err
	}
	turnID, claimed := as.claimTurn()
	if !claimed {
		return ErrTurnInFlight
	}
	return c.runPromptTurn(as, text, turnID)
}

// SendPromptOrHold is the person's chat prompt path, and only that path
// (FS-03.R48, TS-01.R29). It is deliberately a separate entry point from
// SendPrompt: the task dispatcher and the two pipeline transitions consume
// ErrTurnInFlight as arbitration, so a queue inside the shared call would let a
// run continue past the gate that paused it. Reports whether the message was
// held rather than sent.
func (c *ChatRuntime) SendPromptOrHold(ctx context.Context, agentID, text string) (bool, error) {
	as, err := c.lookup(agentID)
	if err != nil {
		return false, err
	}
	turnID, held := as.claimTurnOrHold(text)
	if held {
		return true, nil
	}
	return false, c.runPromptTurn(as, text, turnID)
}

// WithdrawHeld drops the agent's held follow-up, if any (FS-03.R48). It is
// idempotent by construction: withdrawing nothing is success, so a double
// withdraw is never an error (TS-03.R38).
func (c *ChatRuntime) WithdrawHeld(agentID string) error {
	as, err := c.lookup(agentID)
	if err != nil {
		return err
	}
	as.mu.Lock()
	as.held = ""
	as.mu.Unlock()
	return nil
}

// SteerOutcome is what the adapter did with a steered message (TS-04.R49). The
// adapter owns the choice and reports it; AgentDeck never infers it from
// transcript timing and never adds a retry-as-a-new-prompt path of its own,
// which would double-send against an adapter that already fell back.
type SteerOutcome string

const (
	// SteerInjected: the message joined the turn that was already running.
	SteerInjected SteerOutcome = "steered"
	// SteerNewTurn: the turn ended between the decision and the call, so the
	// adapter started a fresh turn from the same message rather than losing it.
	SteerNewTurn SteerOutcome = "new_turn"
)

// Steer delivers text into the agent's running turn through the adapter's ACP
// steering extension (FS-03.R50, TS-04.R49). Empty text promotes the agent's
// held follow-up, which is the only way to send one: taking it and steering it is
// one operation here rather than a client-side withdraw-then-steer that could
// drop the text between two requests.
func (c *ChatRuntime) Steer(ctx context.Context, agentID, text string) (SteerOutcome, error) {
	as, err := c.lookup(agentID)
	if err != nil {
		return "", err
	}

	as.mu.Lock()
	if !as.steering {
		as.mu.Unlock()
		return "", ErrSteeringUnsupported
	}
	promoted := text == ""
	if promoted {
		// Take the hold under the same lock it is written and released under, so a
		// concurrent withdraw or turn-end delivery cannot let one message both
		// steer and run as its own turn (INV §5).
		if as.held == "" {
			as.mu.Unlock()
			return "", ErrNothingHeld
		}
		text = as.held
		as.held = ""
	}
	as.mu.Unlock()

	res, callErr := as.transport.Call(ctx, steeringMethod, map[string]any{
		"sessionId": as.sessionID,
		"prompt":    []map[string]any{{"type": "text", "text": text}},
	})
	outcome, err := mapSteerResult(res, callErr)
	if err == nil {
		// The agent really did receive it — injected into the running turn or as
		// the turn the adapter started — so it belongs in the durable transcript
		// as an ordinary user message on the same path every other prompt takes
		// (FS-03.R50, INV §2). A refusal deliberately writes nothing.
		c.emit(as, EvUserPrompt, UserPromptData{Text: text})
	}
	if err != nil && promoted {
		// A refusal must leave the person's message where they can still see and
		// act on it. The composer holds a message the person typed; a promoted one
		// only exists here, so put it back unless something newer took its place.
		as.mu.Lock()
		if as.held == "" {
			as.held = text
		}
		as.mu.Unlock()
	}
	return outcome, err
}

// steeringMethod is the agreed ACP steering extension request, advertised at
// handshake as initialize._meta.steering.supported (TS-04.R49).
const steeringMethod = "_session/steering"

// mapSteerResult turns the adapter's answer into the two outcomes the product
// reports. Anything else — the adapter's own "failed", the "promptRequired"
// fallback AgentDeck deliberately never opts into, a missing field, or an
// unreadable body — is an error carrying what the adapter said, never a silent
// downgrade to a queue (FS-03.R50, INV §12).
func mapSteerResult(res json.RawMessage, callErr error) (SteerOutcome, error) {
	if callErr != nil {
		return "", callErr
	}
	var body struct {
		Outcome string `json:"outcome"`
	}
	if err := json.Unmarshal(res, &body); err != nil {
		return "", fmt.Errorf("runtime: steering returned an unreadable result: %w", err)
	}
	switch body.Outcome {
	case "injected":
		return SteerInjected, nil
	case "startedNewTurn":
		return SteerNewTurn, nil
	case "":
		return "", fmt.Errorf("runtime: steering returned no outcome")
	default:
		return "", fmt.Errorf("runtime: the agent could not accept that message (%s)", body.Outcome)
	}
}

// decodeSteeringSupport reads the adapter's steering advertisement from the
// initialize response: `_meta.steering.supported` (TS-04.R49). Detection comes
// from that advertisement and nothing else — not the adapter version, not the
// backend type — and an absent, malformed, or wrongly-typed value reads as
// unsupported, so an unknown adapter degrades to Send-only rather than exposing
// a control it would reject (INV §12).
func decodeSteeringSupport(initRes json.RawMessage) bool {
	var body struct {
		Meta struct {
			Steering struct {
				Supported bool `json:"supported"`
			} `json:"steering"`
		} `json:"_meta"`
	}
	if err := json.Unmarshal(initRes, &body); err != nil {
		return false
	}
	return body.Meta.Steering.Supported
}

// claimTurn takes the per-agent turn gate for a new turn. Deciding and claiming
// happen in one critical section so two callers can never both start a turn
// (INV §5).
func (as *agentState) claimTurn() (string, bool) {
	as.mu.Lock()
	defer as.mu.Unlock()
	return as.claimTurnLocked()
}

// claimTurnOrHold is claimTurn's person-facing variant: it takes the gate when
// the agent is free and otherwise holds the text as the next turn. The choice
// between the two reads the same turnActive flag the gate is taken under, in the
// same critical section, so a hold cannot race a turn_end and be left sitting
// until the turn after next (TS-01.R29, INV §5).
func (as *agentState) claimTurnOrHold(text string) (string, bool) {
	as.mu.Lock()
	defer as.mu.Unlock()
	turnID, claimed := as.claimTurnLocked()
	if claimed {
		return turnID, false
	}
	// At most one held message per agent: a second submission replaces it rather
	// than stacking a second queued turn (FS-03.R48, INV §16).
	as.held = text
	return "", true
}

func (as *agentState) claimTurnLocked() (string, bool) {
	if as.turnActive {
		return "", false
	}
	as.turnActive = true
	as.cancelEscalated = false
	as.resolved = map[string]struct{}{}
	return as.nextTurnIDLocked(), true
}

// runPromptTurn drives one provider turn for text under a gate the caller has
// already claimed. Every caller that claims the gate reaches this function, and
// this function releases the claim on every failure branch, so the launch, hold
// release, and steer promotion paths cannot grow divergent turn bookkeeping
// (INV §2).
func (c *ChatRuntime) runPromptTurn(as *agentState, text, turnID string) error {
	if err := c.store.ResetTurnBudget(as.agentID, turnID); err != nil {
		as.mu.Lock()
		as.turnActive = false
		as.mu.Unlock()
		return err
	}
	// Persist the accepted user side of the turn before handing it to ACP. This
	// gives it the same durable sequence, replay, and index path as assistant
	// output, so a reconnect cannot replace the chat with a one-sided history.
	c.emit(as, EvUserPrompt, UserPromptData{Text: text})

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
			as.mu.Lock()
			as.cancelEscalated = false
			as.mu.Unlock()
			c.emit(as, EvError, ErrorData{Scope: "protocol", Message: err.Error(), Fatal: false})
			c.finishTurn(as, TurnEndData{StopReason: "error", ContextPct: as.lastPct()})
			return
		}
		td, hasPct := mapPromptResult(res)
		as.mu.Lock()
		as.turnActive = false
		as.cancelEscalated = false
		if hasPct {
			as.contextPct = td.ContextPct
		} else {
			td.ContextPct = as.contextPct
		}
		as.mu.Unlock()

		c.finishTurn(as, td)
	}()

	return nil
}

// finishTurn settles a turn the runtime owns: it writes the idle status row
// before emitting turn_end so a client reacting to turn_end never observes a
// stale busy row, then releases any held follow-up. Both the ordinary prompt and
// the activation turn end here, so cancel and normal completion cannot grow
// divergent delivery paths (FS-03.R49, INV §2). The crash path deliberately does
// not: its held message has no next turn to run in and dies with the agent.
func (c *ChatRuntime) finishTurn(as *agentState, td TurnEndData) {
	c.applyTurnEndStatus(as, td)
	c.emit(as, EvTurnEnd, td)
	c.deliverHeld(as)
}

// deliverHeld runs the agent's held follow-up as the next turn. Taking the
// message and claiming the gate is one critical section, so a message can never
// be both delivered here and sent by a racing caller (INV §5). A cancelled turn
// reaches this the same way a completed one does, which is what makes cancel
// "stop that, do this instead" rather than a discard (FS-03.R49).
func (c *ChatRuntime) deliverHeld(as *agentState) {
	as.mu.Lock()
	if as.held == "" || as.stopped {
		as.mu.Unlock()
		return
	}
	turnID, claimed := as.claimTurnLocked()
	if !claimed {
		// Another turn already owns the gate; the message stays held and goes out
		// when that turn ends instead of being dropped.
		as.mu.Unlock()
		return
	}
	text := as.held
	as.held = ""
	as.mu.Unlock()
	if err := c.runPromptTurn(as, text, turnID); err != nil {
		// runPromptTurn already released the gate. Nobody is waiting on a return
		// value here, so the person learns about it the only way that is visible:
		// as a turn error in their transcript (INV §8).
		c.emit(as, EvError, ErrorData{Scope: "protocol", Message: "queued message failed to send: " + err.Error(), Fatal: false})
	}
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

	// Stop owns pending-permission teardown. Record every cancellation before
	// shutting down the peer so no timer survives the lifecycle boundary and the
	// durable transcript explains why the tool did not run.
	as.mu.Lock()
	pendingIDs := make([]string, 0, len(as.pending))
	for id := range as.pending {
		pendingIDs = append(pendingIDs, id)
	}
	as.mu.Unlock()
	for _, id := range pendingIDs {
		c.resolvePending(as, id, "cancelled", "")
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
	cmd, err := c.spawnCmd(ad, spec)
	if err != nil {
		return nil, err
	}

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
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, fmt.Errorf("runtime: stderr pipe: %w", err)
	}
	if err := c.startCmd(cmd, spec); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		_ = stderr.Close()
		return nil, fmt.Errorf("runtime: start %s: %w", c.command, err)
	}
	pgid := cmd.Process.Pid

	actx, acancel := context.WithCancel(context.Background())
	as := &agentState{
		agentID: spec.Agent.AgentID,
		// A resumed agent carries its launch generation exactly like Start and both
		// terminal paths (INV §4): the crash callback is matched against the
		// registry's generation, so an empty value is rejected as stale and the
		// unsolicited exit would skip ownership and registration teardown.
		generation:       spec.Generation,
		cmd:              cmd,
		pgid:             pgid,
		hub:              NewHub(),
		stdin:            stdin,
		stderr:           newRingBuffer(16 * 1024),
		stderrDone:       make(chan struct{}),
		ctx:              actx,
		cancel:           acancel,
		skipPerms:        spec.SkipPerms,
		autoApproveTools: copyStringSet(spec.AutoApproveTools),
		toolNames:        map[string]string{},
		pending:          map[string]*pendingPerm{},
		resolved:         map[string]struct{}{},
		contextPct:       spec.LastContextPct,
		adapter:          ad,
		// The notification callback is installed before session/load runs, so the
		// replay gate has to be closed from construction — a provider that starts
		// replaying immediately must not race an assignment made later.
		loadReplay: true,
	}
	as.transport = NewTransport(stdin,
		func(method string, params json.RawMessage) { c.onNotification(as, method, params) },
		func(req *IncomingRequest) { c.onRequest(as, req) },
	)

	go func() {
		as.stderr.copyFrom(stderr)
		close(as.stderrDone)
	}()
	go func() {
		_ = as.transport.Run(stdout)
		c.onTransportClosed(as)
	}()

	// ACP handshake: initialize.
	initRes, err := c.startupCall(ctx, as.transport, "initialize", map[string]any{
		"protocolVersion":    1,
		"clientCapabilities": map[string]any{},
	})
	if err != nil {
		return nil, c.startupFailure(as, spec.BackendType, "initialize", err)
	}
	if err := checkACPVersion(initRes); err != nil {
		as.shutdown()
		return nil, err
	}
	// Re-decoded here, not carried across the boundary: resume spawns a new
	// adapter process whose advertisement is its own (INV §1).
	as.setSteering(decodeSteeringSupport(initRes))

	// Try session/load to restore native context; fall back to session/new.
	// The load params must carry the current cwd + freshly-minted MCP servers
	// (ACP loadSession takes the same registration shape as newSession), or an
	// adapter where session/load succeeds would run without the in-process
	// messaging MCP server Phase 5 depends on.
	resolvedSessionID := ""
	loaded := false
	configOptions := sessionConfigAdvertisement{}
	if sessionID != "" {
		loadRes, loadErr := c.startupCall(ctx, as.transport, "session/load", sessionLoadParams(spec, sessionID))
		switch {
		case loadErr == nil:
			// A successful session/load restored the requested session, so the
			// requested id stays authoritative. The pinned codex-acp adapter
			// returns an empty result on success (the id it was handed remains
			// the session), so an absent sessionId is NOT a failure — mistaking
			// it for one silently ran session/new and abandoned the provider
			// conversation history (the resume-history defect). Only a non-empty
			// echoed id overrides the request.
			loaded = true
			configOptions.replace(decodeSessionConfigOptions(loadRes))
			resolvedSessionID = sessionID
			var res struct {
				SessionID string `json:"sessionId"`
			}
			if json.Unmarshal(loadRes, &res) == nil && res.SessionID != "" {
				resolvedSessionID = res.SessionID
			}
		case errors.Is(loadErr, context.DeadlineExceeded), errors.Is(loadErr, errTransportClosed):
			return nil, c.startupFailure(as, spec.BackendType, "session/load", loadErr)
		default:
			// A non-fatal load error (e.g. the adapter cannot find the prior
			// rollout) degrades to a fresh session, but never silently: the
			// resumed agent loses its native history, so record why.
			slog.Warn("runtime: session/load failed; starting a new session",
				"agent", as.agentID, "session", sessionID, "err", loadErr)
		}
	}
	// Load has returned, so any further session/update is new activity. The
	// transport dispatches frames in read order on one goroutine, so every
	// replay frame the adapter wrote before its response is already handled.
	as.mu.Lock()
	as.loadReplay = false
	dropped := as.loadReplayDropped
	as.mu.Unlock()
	if dropped > 0 {
		slog.Info("runtime: suppressed provider history replay on resume",
			"agent", as.agentID, "session", sessionID, "events", dropped)
	}
	if !loaded {
		newRes, err := c.startupCall(ctx, as.transport, "session/new", sessionNewParams(spec))
		if err != nil {
			return nil, c.startupFailure(as, spec.BackendType, "session/new", err)
		}
		var sess struct {
			SessionID string `json:"sessionId"`
		}
		if err := json.Unmarshal(newRes, &sess); err != nil || sess.SessionID == "" {
			as.shutdown()
			return nil, fmt.Errorf("runtime: session/new returned no sessionId")
		}
		resolvedSessionID = sess.SessionID
		configOptions.replace(decodeSessionConfigOptions(newRes))
	}
	as.sessionID = resolvedSessionID
	as.configOptions = configOptions
	applied, err := applySessionConfig(ctx, as.transport, ad, spec, resolvedSessionID, configOptions)
	if err != nil {
		as.shutdown()
		return nil, err
	}
	spec.Agent.Fast = applied.Fast

	// Re-open the existing transcript in append mode (Open skips seq:0 meta for existing files).
	if err := c.openPersistence(as, spec, resolvedSessionID); err != nil {
		as.shutdown()
		return nil, err
	}

	// Append resumed session_meta with resumed_at to the transcript so the raw
	// log has a resume boundary marker and the archive can track the session_id.
	resumeNow := time.Now().UTC().Format(time.RFC3339)
	resumedMeta := runtimeMeta(spec, resolvedSessionID)
	resumedMeta.ResumedAt = &resumeNow
	c.emit(as, EvSessionMeta, resumedMeta)

	// Write fresh running row + status row with restored context_pct.
	now := time.Now().UTC()
	if err := c.store.WriteRunning(state.RunningEntry{
		AgentID: as.agentID, PID: pgid, SessionID: resolvedSessionID,
		Interface: "chat", HookToken: spec.HookToken, StartedAt: now,
		FastAvailable: applied.FastAvailable, SteeringAvailable: as.steeringSupported(),
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

	return &Handle{AgentID: as.agentID, Pid: pgid, SessionID: resolvedSessionID, Fast: applied.Fast}, nil
}

func copyStringSet(source map[string]struct{}) map[string]struct{} {
	copy := make(map[string]struct{}, len(source))
	for value := range source {
		copy[value] = struct{}{}
	}
	return copy
}

// StartActivation starts a server-owned, payload-free turn. Its callback runs
// with the same agent turn gate as an ordinary prompt, so a manual prompt and an
// activation cannot both write a provider frame. The instruction and status come
// from the kind's row in the code-owned registry, never from a literal here or
// from the caller (TS-01.R21, TS-10.R5).
func (c *ChatRuntime) StartActivation(ctx context.Context, agentID, kind string, before func(string) error) (bool, error) {
	contract, ok := LookupActivationKind(kind)
	if !ok {
		return false, fmt.Errorf("runtime: unsupported activation kind %q", kind)
	}
	as, err := c.lookup(agentID)
	if err != nil {
		return false, err
	}
	st, err := c.store.ReadStatus(as.agentID)
	if err != nil || st.State != "idle" {
		return false, err
	}
	as.mu.Lock()
	if as.turnActive {
		as.mu.Unlock()
		return false, nil
	}
	as.turnActive = true
	as.cancelEscalated = false
	turnID := as.nextTurnIDLocked()
	as.mu.Unlock()
	if err := before(turnID); err != nil {
		as.mu.Lock()
		as.turnActive = false
		as.mu.Unlock()
		return false, err
	}

	// TS-01.R21/INV §15: the ordinary busy turn state must commit before the
	// provider frame. Swallowing this write left durable and UI state `idle` while
	// the model was executing the mail turn. Releasing the in-memory turn gate here
	// is correct and does not reopen a replay: the caller's kind-owned attempted
	// boundary already committed, so the activation is retired, not re-armed.
	now := time.Now().UTC()
	if err := c.writeStatus(as, state.Status{
		AgentID: as.agentID, State: "busy", Detail: contract.StatusDetail,
		LastTrace: contract.LastTrace, BusySince: &now, ContextPct: as.lastPct(),
	}); err != nil {
		as.mu.Lock()
		as.turnActive = false
		as.mu.Unlock()
		return false, fmt.Errorf("runtime: write activation status: %w", err)
	}

	go func() {
		params := map[string]any{
			"sessionId": as.sessionID,
			"prompt":    []map[string]any{{"type": "text", "text": contract.Instruction}},
		}
		res, err := as.transport.Call(as.ctx, "session/prompt", params)
		if err != nil {
			as.mu.Lock()
			as.turnActive = false
			as.mu.Unlock()
			if errors.Is(err, errTransportClosed) || as.isStopped() {
				return
			}
			as.mu.Lock()
			as.cancelEscalated = false
			as.mu.Unlock()
			c.emit(as, EvError, ErrorData{Scope: "protocol", Message: err.Error(), Fatal: false})
			c.finishTurn(as, TurnEndData{StopReason: "error", ContextPct: as.lastPct()})
			return
		}
		td, hasPct := mapPromptResult(res)
		as.mu.Lock()
		as.turnActive = false
		as.cancelEscalated = false
		if hasPct {
			as.contextPct = td.ContextPct
		} else {
			td.ContextPct = as.contextPct
		}
		as.mu.Unlock()
		c.finishTurn(as, td)
	}()
	return true, nil
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

// SessionConfigChange reports what a live session-setting change actually
// achieved. It is returned on failure as well as success, and that is the point:
// the settings are applied one at a time in an order the adapter imposes, so a
// rejected second setting leaves the first one live on the provider. Reporting
// the whole request as a no-op would leave the provider running one effort while
// the agent, session, archive, and next resume all record another (INV §15).
type SessionConfigChange struct {
	Effort        string
	EffortApplied bool
	Fast          bool
	FastApplied   bool
	// FastAvailable is the live advertisement after the change, so a caller can
	// keep the header's explanation current (FS-03.R46).
	FastAvailable bool
}

// SetSessionConfig applies effort and/or fast mode to a live chat session without
// restarting the process or rebuilding the conversation (FS-03.R45/R47). It reuses
// the same adapter-declared identifiers and the same honored-value check as the
// launch path (TS-04.R47), and it keeps each setting's own failure posture:
// effort is fail-closed, fast mode is fail-open.
func (c *ChatRuntime) SetSessionConfig(ctx context.Context, agentID string, effort *string, fast *bool) (change SessionConfigChange, err error) {
	as, err := c.lookup(agentID)
	if err != nil {
		return SessionConfigChange{}, err
	}
	as.mu.Lock()
	defer as.mu.Unlock()
	_, effortID, fastID := as.adapter.SessionConfigIDs()
	// Report the advertisement as it stands at every exit, including the failure
	// exits above: a caller that persists a partial change still needs the current
	// availability, and an early return must not write it back as "unavailable".
	defer func() { change.FastAvailable = as.configOptions.has(fastID) }()

	if effort != nil {
		if err := applyRequiredOption(ctx, as.transport, as.sessionID, effortID, *effort, as.configOptions); err != nil {
			return change, err
		}
		change.Effort, change.EffortApplied = *effort, true
	}
	if fast == nil {
		return change, nil
	}
	if fastID == "" {
		return change, fmt.Errorf("%w: fast mode", ErrSettingUnsupported)
	}
	if !as.configOptions.has(fastID) {
		// Fail-open exactly as launch does (FS-09.R55): an unadvertised speed tier
		// resolves to off rather than erroring, and FastAvailable tells the header
		// to explain why the toggle did not move, instead of inventing a third
		// behavior for the live path (TS-03.R37).
		change.Fast, change.FastApplied = false, true
		return change, nil
	}
	value := "off"
	if *fast {
		value = "on"
	}
	if err := setConfigOption(ctx, as.transport, as.sessionID, fastID, value, as.configOptions); err != nil {
		return change, err
	}
	change.Fast, change.FastApplied = *fast, true
	return change, nil
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
	// available_commands_update is replace-only live state, not a normalized
	// transcript event: it gets no seq, durable append, index entry, or SSE
	// publication (TS-04.R24). Handle it before event mapping and return.
	if cmds, ok := decodeAvailableCommands(params); ok {
		as.mu.Lock()
		as.commands = cmds
		as.mu.Unlock()
		return
	}
	// usage_update is live runtime state rather than a transcript event. The
	// prompt result only contains token accounting, so this provider
	// notification is the only source for a fresh context_pct. Republish it
	// through the existing status-write seam immediately so a long-running
	// turn's dashboard meter does not go stale waiting for the next
	// tool/status event or turn_end (TS-04.R25, INV §1).
	if pct, ok := decodeContextUsage(params); ok {
		as.mu.Lock()
		as.contextPct = pct
		as.mu.Unlock()
		c.republishContextPct(as)
		return
	}
	// While session/load restores provider-native context, a replayed frame is
	// the conversation AgentDeck already rendered and persisted, not new work.
	// Emitting it would assign a fresh sequence, append a duplicate to the
	// durable transcript, publish it as live activity that drags an open
	// transcript through old turns, and let a replayed turn boundary drive the
	// agent's status (TS-04.R50, FS-03.R3/R35, INV §1, §11). The provider keeps
	// its own restored context either way; only AgentDeck's view is gated.
	as.mu.Lock()
	replay := as.loadReplay
	if replay {
		as.loadReplayDropped++
	}
	as.mu.Unlock()
	if replay {
		return
	}

	for _, m := range mapSessionUpdate(params) {
		c.emit(as, m.Type, m.Data)
		c.applyEventStatus(as, m)
	}
}

// Commands returns the owning chat runtime's latest ACP available-commands
// snapshot (TS-04.R24). ErrNoHandle when no live handle exists for the agent
// (never started, stopped, or crashed) — the read is purely from live state.
func (c *ChatRuntime) Commands(agentID string) ([]CommandItem, error) {
	as, err := c.lookup(agentID)
	if err != nil {
		return nil, err
	}
	as.mu.Lock()
	defer as.mu.Unlock()
	return append([]CommandItem{}, as.commands...), nil
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
	cancelEscalated := as.cancelEscalated
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
	errorMessage := strutil.FirstNonEmpty(tail, "process exited")
	statusDetail := clip(tail, 120)
	lastTrace := "Error"
	stopReason := "error"
	if cancelEscalated {
		errorMessage = "Cancelled — agent process exited after ignoring cancellation."
		if tail != "" {
			errorMessage += " " + tail
		}
		statusDetail = "cancelled — process exited"
		lastTrace = "Cancelled"
		stopReason = "cancelled"
	}
	c.emit(as, EvError, ErrorData{Scope: "process", Message: errorMessage, Fatal: true})

	// Settle state.db before emitting turn_end so a client reacting to turn_end
	// never observes a stale running row or busy status.
	c.updateStatus(as, "error", statusDetail, lastTrace, clearBusySince)
	_ = c.store.DeleteRunning(as.agentID)
	c.touchState(as.agentID)
	c.removeAgent(as.agentID)
	// Tell the Registry the handle is gone so it drops ownership; otherwise a
	// relaunch/resume on this agent_id is rejected with ErrAlreadyStarted while
	// the runtime has no handle to serve it (techspec §8.2). Done before the
	// turn_end emit so a client reacting to turn_end can immediately relaunch.
	if c.onExit != nil {
		c.onExit(as.agentID, as.generation)
	}

	c.emit(as, EvTurnEnd, TurnEndData{StopReason: stopReason, ContextPct: as.lastPct()})
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
func (c *ChatRuntime) emit(as *agentState, typ string, data any) Event {
	raw, err := json.Marshal(data)
	if err != nil {
		slog.Error("runtime: marshal event payload", "type", typ, "err", err)
		return Event{}
	}
	as.mu.Lock()
	as.seq++
	ev := Event{
		AgentID: as.agentID, Generation: as.generation,
		Seq:  as.seq,
		Type: typ,
		Data: raw,
		Ts:   time.Now().UTC().Format(time.RFC3339),
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
	return ev
}

// AppendAnnotation records and publishes a dashboard annotation through the
// live runtime's writer and sequence allocator. This keeps it ordered with ACP
// events and avoids a second writer racing the active transcript.
func (c *ChatRuntime) AppendAnnotation(agentID string, data AnnotationData) (Event, error) {
	as, err := c.lookup(agentID)
	if err != nil {
		return Event{}, err
	}
	return c.emit(as, EvAnnotation, data), nil
}

// AppendAnnotationAndSync records a dashboard annotation without publishing;
// the caller is responsible for publishing after durability is confirmed
// (FS-13.R5, TS-03.R14, INV §15).
func (c *ChatRuntime) AppendAnnotationAndSync(agentID string, data AnnotationData) (Event, error) {
	as, err := c.lookup(agentID)
	if err != nil {
		return Event{}, err
	}

	// Build the event like emit() does, but WITHOUT publishing.
	raw, err := json.Marshal(data)
	if err != nil {
		return Event{}, fmt.Errorf("marshal annotation data: %w", err)
	}

	as.mu.Lock()
	as.seq++
	ev := Event{
		AgentID: as.agentID,
		Seq:     as.seq,
		Type:    EvAnnotation,
		Data:    raw,
		Ts:      time.Now().UTC().Format(time.RFC3339),
	}
	as.transcript = append(as.transcript, ev)
	as.mu.Unlock()

	// Persist the event synchronously before returning. persistEvent logs and
	// returns a bool indicating success; a failed append is an error that must
	// not be suppressed (unlike the normal emit path where best-effort
	// persistence does not suppress delivery).
	if !c.persistEvent(as, ev) {
		return Event{}, fmt.Errorf("persist annotation: transcript or index sync failed")
	}

	return ev, nil
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
	if ev.Type == EvTurnEnd {
		_ = w.Sync()
		rollup := TurnRollup{LastSeq: ev.Seq, UpdatedAt: ev.Ts, LastContextPct: as.lastPct()}
		var td TurnEndData
		if err := json.Unmarshal(ev.Data, &td); err == nil {
			rollup.LastContextPct = td.ContextPct
		}
		if err := ix.OnEventAndTurnEnd(as.agentID, ev, rollup); err != nil {
			slog.Error("runtime: index turn end", "agent", as.agentID, "seq", ev.Seq, "err", err)
		}
		return true
	}
	if ev.Type == EvAnnotation {
		if err := w.Sync(); err != nil {
			slog.Error("runtime: sync annotation", "agent", as.agentID, "seq", ev.Seq, "err", err)
			return false
		}
		if err := ix.OnEventAndFlushContent(as.agentID, ev, ev.Seq, ev.Ts); err != nil {
			slog.Error("runtime: index annotation", "agent", as.agentID, "seq", ev.Seq, "err", err)
			return false
		}
		return true
	}
	if err := ix.OnEvent(as.agentID, ev); err != nil {
		slog.Error("runtime: index event", "agent", as.agentID, "seq", ev.Seq, "err", err)
	}
	if ev.Type == EvError {
		_ = w.Sync()
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
		Effort:          spec.Effort,
		Fast:            spec.Agent.Fast,
		Interface:       spec.Agent.Interface,
		Group:           spec.Agent.Group,
		Cwd:             spec.Cwd,
		SystemPrompt:    spec.SystemPrompt,
		SystemPromptSHA: sha,
		EnvKeys:         envKeys(spec.Env),
		SkipPermissions: spec.SkipPerms,
		AddDirs:         spec.AddDirs,
		LaunchConfig:    spec.LaunchConfig,
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

// republishContextPct refreshes only the current status row's context
// percentage from the latest decoded usage_update and republishes it through
// the same write+touch seam as updateStatus/writeStatus, without disturbing
// the row's state/detail/trace/busy_since (there is no status transition
// implied by a usage_update alone).
func (c *ChatRuntime) republishContextPct(as *agentState) {
	cur, err := c.store.ReadStatus(as.agentID)
	if err != nil {
		// Unlike updateStatus there is no state transition to record, so an
		// unreadable row is skipped rather than replaced with an empty one; the
		// next status write carries the value forward.
		slog.Debug("runtime: read status for usage republish", "agent", as.agentID, "err", err)
		return
	}
	cur.ContextPct = as.lastPct()
	if err := c.store.WriteStatus(cur); err != nil {
		slog.Error("runtime: write status", "agent", as.agentID, "err", err)
		return
	}
	c.touchState(as.agentID)
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

// envValue returns the value of the last "key=value" entry for key, or "" if
// absent. Later entries win, matching how a process resolves duplicate env keys.
func envValue(env []string, key string) string {
	prefix := key + "="
	value := ""
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			value = kv[len(prefix):]
		}
	}
	return value
}

// withCodexDeveloperInstructions adds the composed AgentDeck launch prompt to
// codex-acp's documented CODEX_CONFIG session-config overlay. The adapter does
// not consume an ACP systemPrompt field, so passing it over session/new quietly
// loses a role/project persona. Preserve the caller's valid overlay and place
// its existing developer instructions before AgentDeck's frozen prompt.
func withCodexDeveloperInstructions(env []string, prompt string) ([]string, error) {
	if prompt == "" {
		return env, nil
	}

	var raw string
	for _, kv := range env {
		if strings.HasPrefix(kv, "CODEX_CONFIG=") {
			raw = strings.TrimPrefix(kv, "CODEX_CONFIG=")
			break
		}
	}

	config := map[string]any{}
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &config); err != nil {
			return nil, fmt.Errorf("runtime: invalid CODEX_CONFIG JSON: %w", err)
		}
		if config == nil {
			return nil, errors.New("runtime: CODEX_CONFIG must be a JSON object")
		}
	}
	if existing, ok := config["developer_instructions"]; ok && existing != nil {
		value, ok := existing.(string)
		if !ok {
			return nil, errors.New("runtime: CODEX_CONFIG developer_instructions must be a string")
		}
		if value != "" {
			prompt = value + "\n\n" + prompt
		}
	}
	config["developer_instructions"] = prompt
	encoded, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("runtime: encode CODEX_CONFIG: %w", err)
	}
	return append(stripEnv(env, "CODEX_CONFIG"), "CODEX_CONFIG="+string(encoded)), nil
}

// clip truncates s to at most n bytes (for status detail fields, ≤120 chars).
func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func (as *agentState) steeringSupported() bool {
	as.mu.Lock()
	defer as.mu.Unlock()
	return as.steering
}

func (as *agentState) setSteering(supported bool) {
	as.mu.Lock()
	as.steering = supported
	as.mu.Unlock()
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

// startupCall applies the shared per-stage ACP startup bound. A shorter caller
// deadline still wins through context propagation.
func (c *ChatRuntime) startupCall(ctx context.Context, transport *Transport, method string, params any) (json.RawMessage, error) {
	timeout := c.startupCallTimeout
	if timeout <= 0 {
		timeout = defaultStartupCallTimeout
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return transport.Call(callCtx, method, params)
}

// startupFailure terminates the not-yet-registered child and turns transport
// EOF/timeout plus captured stderr into a bounded, non-secret recovery message.
// Raw provider stderr must not cross the API boundary (TS-04.R12/R22).
func (c *ChatRuntime) startupFailure(as *agentState, backendType, stage string, cause error) error {
	as.shutdown()
	select {
	case <-as.stderrDone:
	case <-time.After(250 * time.Millisecond):
	}

	provider := backendType
	if backendType == "claude-acp" {
		provider = "Claude"
	}
	if errors.Is(cause, context.DeadlineExceeded) {
		return fmt.Errorf("runtime: %s %s timed out; verify the adapter installation and provider authentication, then retry", provider, stage)
	}
	if !errors.Is(cause, errTransportClosed) {
		return fmt.Errorf("runtime: %s: %w", stage, cause)
	}

	guidance := "the adapter exited before responding; verify the adapter installation and provider authentication, then retry"
	if backendType == "claude-acp" {
		guidance = claudeStartupGuidance(as.stderr.Tail())
	}
	return fmt.Errorf("runtime: %s %s failed: %s", provider, stage, guidance)
}

// claudeStartupGuidance deliberately maps stderr to a fixed vocabulary instead
// of returning any provider-controlled text, paths, account data, or secrets.
func claudeStartupGuidance(stderr string) string {
	lower := strings.ToLower(stderr)
	switch {
	case strings.Contains(lower, "emfile"), strings.Contains(lower, "too many open files"):
		return "the adapter could not open required files; close unused agent processes and retry"
	case strings.Contains(lower, "claudecode"), strings.Contains(lower, "nested session"):
		return "Claude refused a nested launch; start AgentDeck outside an existing Claude session and retry"
	case strings.Contains(lower, "not logged in"), strings.Contains(lower, "authentication"),
		strings.Contains(lower, "unauthorized"), strings.Contains(lower, "auth login"):
		return "Claude authentication is unavailable; run `agentdeck auth claude` and retry"
	case strings.Contains(lower, "cannot find module"), strings.Contains(lower, "module not found"),
		strings.Contains(lower, "unsupported node"), strings.Contains(lower, "syntaxerror"):
		return "the pinned Claude adapter runtime is incompatible or incomplete; reinstall AgentDeck and retry"
	default:
		return "the adapter exited before responding; run `agentdeck auth claude`, verify the pinned adapter installation, and retry"
	}
}

// Pinned ACP protocol range (§12.1). The Go client targets the version the
// pinned claude-agent-acp adapter negotiates; today that is exactly 1.
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
	if spec.BackendType == "claude-acp" {
		options := map[string]any{"additionalDirectories": spec.StartAddDirs()}
		// An empty ModelID means "inherit native resolution" (federation §2.3):
		// omit the model flag so the CLI resolves its own configured model rather
		// than being overridden by an AgentDeck default. A non-empty id is either an
		// explicit launch choice or a source override, and is passed through.
		if spec.ModelID != "" {
			options["model"] = spec.ModelID
		}
		return map[string]any{
			"cwd":        spec.Cwd,
			"mcpServers": mcp,
			"_meta": map[string]any{
				"systemPrompt": spec.StartSystemPrompt(),
				"claudeCode":   map[string]any{"options": options},
			},
		}
	}
	params := map[string]any{
		"cwd":                   spec.Cwd,
		"mcpServers":            mcp,
		"additionalDirectories": spec.StartAddDirs(),
	}
	// codex-acp ignores generic ACP systemPrompt. Its prompt is supplied via the
	// CODEX_CONFIG developer_instructions overlay in spawnCmd.
	if spec.BackendType != "codex-acp" {
		params["systemPrompt"] = spec.StartSystemPrompt()
	}
	if model := deliveredModelID(spec); model != "" {
		params["model"] = model
	}
	return params
}

// sessionLoadParams builds the session/load params. It carries the SAME fields as
// session/new (cwd, mcpServers, model, additionalDirectories, and where
// supported systemPrompt) plus the sessionId to restore. Codex obtains its
// prompt through its process config overlay; other generic adapters retain the
// top-level systemPrompt shape. Without model here, a same-backend model swap
// that uses native resume would silently keep the old model.
func sessionLoadParams(spec LaunchSpec, sessionID string) map[string]any {
	mcp := make([]map[string]any, 0, len(spec.MCPServers))
	for _, m := range spec.MCPServers {
		mcp = append(mcp, mcpServerParam(m))
	}
	if spec.BackendType == "claude-acp" {
		options := map[string]any{"resume": sessionID, "additionalDirectories": spec.StartAddDirs()}
		if spec.ModelID != "" { // inherit native model when empty (§2.3); see sessionNewParams.
			options["model"] = spec.ModelID
		}
		return map[string]any{
			"sessionId":  sessionID,
			"cwd":        spec.Cwd,
			"mcpServers": mcp,
			"_meta": map[string]any{
				"systemPrompt": spec.StartSystemPrompt(),
				"claudeCode":   map[string]any{"options": options},
			},
		}
	}
	params := map[string]any{
		"sessionId":             sessionID,
		"cwd":                   spec.Cwd,
		"mcpServers":            mcp,
		"additionalDirectories": spec.StartAddDirs(),
	}
	if spec.BackendType != "codex-acp" {
		params["systemPrompt"] = spec.StartSystemPrompt()
	}
	if model := deliveredModelID(spec); model != "" {
		params["model"] = model
	}
	return params
}

func deliveredModelID(spec LaunchSpec) string {
	if spec.BackendType == "codex-acp" {
		return ""
	}
	model := spec.ModelID
	if model == "" || spec.Effort == "" {
		return model
	}
	if ad, ok := backend.For(spec.BackendType); ok {
		if mode, _ := ad.EffortDelivery(spec.Agent.Interface); mode == backend.EffortModelSuffix {
			return model + "[" + spec.Effort + "]"
		}
	}
	return model
}

// setConfigOption sends one session configuration option and folds the peer's
// answer back into the advertisement (TS-04.R46/R47).
//
// Two things make this more than a fire-and-forget call, both confirmed against
// the pinned adapters rather than inferred from their source. First, the response
// carries the peer's **rebuilt full option list**, and applying a model changes
// which options exist at all: setting the pinned Claude adapter's model to one
// without reasoning levels removes the `effort` and `fast` options outright, so a
// later lookup against the pre-model list would send a setting the session now
// rejects with `Unknown config option`. Second, a peer may answer success while
// reporting a different effective value — the silent ignore BR-1 shipped — so the
// independently reported currentValue, not the RPC envelope, decides whether the
// setting was honored (INV §12).
func setConfigOption(ctx context.Context, transport *Transport, sessionID, id, value string, advertised sessionConfigAdvertisement) error {
	result, err := transport.Call(ctx, "session/set_config_option", map[string]any{
		"sessionId": sessionID, "configId": id, "value": value,
	})
	if err != nil {
		return fmt.Errorf("%w: %s: %s", ErrSettingRejected, id, err)
	}
	if rebuilt := decodeSessionConfigOptions(result); len(rebuilt) > 0 {
		advertised.replace(rebuilt)
	}
	// An unreported value is not a mismatch: the peer is entitled to omit it, and
	// treating silence as failure would fail launches that actually worked (INV §7).
	if reported, ok := advertised[id]; ok && reported != "" && reported != value {
		return fmt.Errorf("%w: %s is %q after requesting %q", ErrSettingIgnored, id, reported, value)
	}
	return nil
}

// applyRequiredOption applies a setting whose failure must not be absorbed:
// model and effort both stay fail-closed under TS-04.R19/R47 and FS-09.R40,
// because running at a level or on a model the person did not choose is wrong in
// a way a slower agent is not.
func applyRequiredOption(ctx context.Context, transport *Transport, sessionID, id, value string, advertised sessionConfigAdvertisement) error {
	if id == "" {
		return fmt.Errorf("%w: %s", ErrSettingUnsupported, value)
	}
	if !advertised.has(id) {
		return fmt.Errorf("%w: %s", ErrSettingUnavailable, id)
	}
	return setConfigOption(ctx, transport, sessionID, id, value, advertised)
}

// sessionConfigResult is what the ordered session-configuration step actually
// achieved, as distinct from what the launch requested (FS-09.R54).
type sessionConfigResult struct {
	// Fast is the fast mode that really applied, which the session snapshot
	// freezes so an unwatched task- or pipeline-launched agent still records the
	// truth.
	Fast bool
	// FastAvailable is the live session's fast-mode advertisement, read after the
	// model is set because the model decides it. It lets the chat header say "this
	// model does not offer fast mode" instead of showing an ordinary off toggle
	// that silently springs back (FS-03.R46).
	FastAvailable bool
}

// applySessionConfig performs the one ordered post-session configuration step —
// model, then effort, then fast mode (FS-09.R57, TS-04.R47) — shared by launch,
// resume, and switch. The order is a correctness constraint the adapters impose,
// not a style choice: setting the model resets effort to that model's own default
// and recomputes fast capability, so either applied first is silently discarded.
func applySessionConfig(ctx context.Context, transport *Transport, ad backend.BackendAdapter, spec LaunchSpec, sessionID string, advertised sessionConfigAdvertisement) (sessionConfigResult, error) {
	modelID, effortID, fastID := ad.SessionConfigIDs()
	if spec.BackendType == "codex-acp" && spec.ModelID != "" {
		if err := applyRequiredOption(ctx, transport, sessionID, modelID, spec.ModelID, advertised); err != nil {
			return sessionConfigResult{}, err
		}
	}
	if spec.Effort != "" {
		if mode, _ := ad.EffortDelivery(spec.Agent.Interface); mode == backend.EffortPostSession {
			if err := applyRequiredOption(ctx, transport, sessionID, effortID, spec.Effort, advertised); err != nil {
				return sessionConfigResult{}, err
			}
		}
	}
	// Read the advertisement only now: the model application above may have added
	// or removed fast mode entirely (INV §1).
	out := sessionConfigResult{FastAvailable: advertised.has(fastID)}
	if !spec.Fast || !out.FastAvailable {
		return out, nil
	}
	// Fast mode is fail-open under FS-09.R55/TS-04.R45: an unavailable or refused
	// speed boost leaves the agent cheaper and slower, which is not worth killing
	// a working launch over. The agent records fast mode off and the header says why.
	if err := setConfigOption(ctx, transport, sessionID, fastID, "on", advertised); err != nil {
		slog.Warn("runtime: fast mode was requested but not applied; continuing at normal speed",
			"agent", spec.Agent.AgentID, "err", err)
		return out, nil
	}
	out.Fast = true
	return out, nil
}

func mcpServerParam(m MCPServerSpec) map[string]any {
	if m.Type == "http" {
		return map[string]any{
			"name":    m.Name,
			"type":    "http",
			"url":     m.URL,
			"headers": namedPairs(m.Headers),
		}
	}
	param := map[string]any{
		"name": m.Name, "command": m.Command, "args": m.Args,
	}
	if len(m.Env) > 0 {
		param["env"] = envPairs(m.Env)
	}
	return param
}

func namedPairs(m map[string]string) []map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make([]map[string]string, 0, len(m))
	for k, v := range m {
		out = append(out, map[string]string{"name": k, "value": v})
	}
	return out
}

func envPairs(env []string) []map[string]string {
	if len(env) == 0 {
		return nil
	}
	out := make([]map[string]string, 0, len(env))
	for _, kv := range env {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			out = append(out, map[string]string{"name": kv[:i], "value": kv[i+1:]})
		}
	}
	return out
}
