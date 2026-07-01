package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/agentdeck/agentdeck/internal/state"
)

// ErrAlreadyStarted: a live handle already exists for the agent (techspec §8.6).
var ErrAlreadyStarted = errors.New("runtime: agent already started")

// Registry dispatches launch/control calls to the right Runtime by
// agent.interface (techspec §3.2). Backend selection (claude vs codex) is handled
// inside ChatRuntime.Start via spec.BackendType, not here. The runtimes own their
// live handles; the registry just remembers which runtime owns each agent so
// control ops route correctly.
type Registry struct {
	mu        sync.Mutex
	byIface   map[string]Runtime // "chat" -> ChatRuntime, "terminal" -> terminal runtime/stub
	rtByAgent map[string]Runtime // agent_id -> owning runtime
	chat      *ChatRuntime
	term      Runtime // the registered terminal runtime (nil until SetTerminalRuntime)
	store     *state.Store

	// onExitExtra runs after forget on an unsolicited agent exit (crash). The
	// server wires its teardownAgentRegistration here so a crash tears down the
	// hook token / MCP session / hook files, not just registry ownership.
	onExitExtra func(string)
}

// exitNotifier is implemented by runtimes that can tell the Registry to drop
// ownership when an agent's process disappears outside a Stop (crash teardown).
type exitNotifier interface{ SetOnExit(func(string)) }

// stopAller is implemented by runtimes that can stop all their live agents on
// server shutdown.
type stopAller interface{ StopAll(ctx context.Context) }

// NewRegistry builds a Registry wired with the chat runtime and the terminal
// stub. The terminal runtime is a not-implemented stub until Phase 6.
func NewRegistry(s *state.Store) *Registry {
	chat := NewChatRuntime(s)
	r := &Registry{
		byIface:   map[string]Runtime{},
		rtByAgent: map[string]Runtime{},
		chat:      chat,
		store:     s,
	}
	r.byIface["chat"] = chat
	r.byIface["terminal"] = notImplementedRuntime{name: "terminal"}
	// When a runtime tears an agent down unsolicited (crash), it must clear our
	// ownership record too — otherwise rtByAgent keeps claiming the agent and
	// blocks relaunch/resume with ErrAlreadyStarted (techspec §8.2). It must also
	// run the server's registration teardown (SetExitHook) so the hook token / MCP
	// session / hook files don't leak on a crash.
	chat.onExit = r.handleAgentExit
	return r
}

// forget drops the ownership record for an agent. Called by an owning runtime
// when its live handle disappears outside a Stop (crash teardown).
func (r *Registry) forget(agentID string) {
	r.mu.Lock()
	delete(r.rtByAgent, agentID)
	r.mu.Unlock()
}

// SetExitHook registers a callback run (after ownership forget) whenever an agent
// exits unsolicited. The server uses it to tear down per-agent registration
// artifacts on the crash path, mirroring a solicited stop.
func (r *Registry) SetExitHook(fn func(string)) {
	r.mu.Lock()
	r.onExitExtra = fn
	r.mu.Unlock()
}

// handleAgentExit is the runtimes' onExit callback: it drops registry ownership
// and then runs the server's registration teardown, if wired.
func (r *Registry) handleAgentExit(agentID string) {
	r.forget(agentID)
	r.mu.Lock()
	extra := r.onExitExtra
	r.mu.Unlock()
	if extra != nil {
		extra(agentID)
	}
}

// Chat returns the chat runtime (e.g. to point it at a pinned adapter binary or,
// in tests, the fake CLI).
func (r *Registry) Chat() *ChatRuntime { return r.chat }

// SetTerminalRuntime registers the real terminal runtime under interface
// "terminal", replacing the not-implemented stub. It lives in a subpackage
// (internal/runtime/terminal) that imports this package, so it can't be
// constructed here without an import cycle — the server wires it in via this
// setter. If the runtime supports onExit notification, it is connected to the
// Registry's ownership-forget path (same crash-teardown contract as chat).
func (r *Registry) SetTerminalRuntime(rt Runtime) {
	if rt == nil {
		return
	}
	r.mu.Lock()
	r.byIface["terminal"] = rt
	r.term = rt
	r.mu.Unlock()
	if en, ok := rt.(exitNotifier); ok {
		en.SetOnExit(r.handleAgentExit)
	}
}

// SetEventSink mirrors runtime transcript events into an external bus.
func (r *Registry) SetEventSink(sink func(Event)) {
	if r == nil || r.chat == nil {
		return
	}
	r.chat.SetEventSink(sink)
}

// SetStateTouch wires runtime state writes to the dashboard state manager.
func (r *Registry) SetStateTouch(touch func(string)) {
	if r == nil || r.chat == nil {
		return
	}
	r.chat.SetStateTouch(touch)
}

// SetPersistence wires durable transcript/index sinks into the chat runtime.
func (r *Registry) SetPersistence(home string, open TranscriptOpener, ix PersistenceIndexer) {
	if r == nil || r.chat == nil {
		return
	}
	r.chat.SetPersistence(home, open, ix)
}

// runtimeFor dispatches by agent.interface. An unknown interface yields
// ErrNotImplemented (techspec §3.2), which the API layer maps to 501.
func (r *Registry) runtimeFor(iface string) (Runtime, error) {
	rt, ok := r.byIface[iface]
	if !ok {
		return nil, fmt.Errorf("%w: interface %q", ErrNotImplemented, iface)
	}
	return rt, nil
}

// Launch dispatches by the agent's interface, guards against a double-start, runs
// Start, and records the owning runtime for later control ops (techspec §6.1 step 7).
func (r *Registry) Launch(ctx context.Context, spec LaunchSpec) (*Handle, error) {
	rt, err := r.runtimeFor(spec.Agent.Interface)
	if err != nil {
		return nil, err
	}
	// Atomically claim the slot with a nil sentinel before releasing the lock so
	// no concurrent Launch for the same agent can also pass the existence check.
	r.mu.Lock()
	if _, ok := r.rtByAgent[spec.Agent.AgentID]; ok {
		r.mu.Unlock()
		return nil, ErrAlreadyStarted
	}
	r.rtByAgent[spec.Agent.AgentID] = nil // sentinel: "launching in progress"
	r.mu.Unlock()

	h, err := rt.Start(ctx, spec)
	if err != nil {
		r.mu.Lock()
		delete(r.rtByAgent, spec.Agent.AgentID)
		r.mu.Unlock()
		return nil, err
	}
	r.mu.Lock()
	r.rtByAgent[spec.Agent.AgentID] = rt
	r.mu.Unlock()
	return h, nil
}

// ownerFor returns the runtime that owns an agent, or ErrNoHandle.
// A nil entry means a Launch is in progress; treat as ErrNoHandle until committed.
func (r *Registry) ownerFor(agentID string) (Runtime, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rt, ok := r.rtByAgent[agentID]
	if !ok || rt == nil {
		return nil, ErrNoHandle
	}
	return rt, nil
}

// Resume re-attaches to a persisted inactive agent. The spec carries LastSessionID
// and LastContextPct from the sessions snapshot. Guards against double-resume with
// the same nil-sentinel pattern used by Launch.
func (r *Registry) Resume(ctx context.Context, spec LaunchSpec) (*Handle, error) {
	rt, err := r.runtimeFor(spec.Agent.Interface)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	if _, ok := r.rtByAgent[spec.Agent.AgentID]; ok {
		r.mu.Unlock()
		return nil, ErrAlreadyStarted
	}
	r.rtByAgent[spec.Agent.AgentID] = nil // sentinel: "resuming in progress"
	r.mu.Unlock()

	h, err := rt.Resume(ctx, spec, spec.LastSessionID)
	if err != nil {
		r.mu.Lock()
		delete(r.rtByAgent, spec.Agent.AgentID)
		r.mu.Unlock()
		return nil, err
	}
	r.mu.Lock()
	r.rtByAgent[spec.Agent.AgentID] = rt
	r.mu.Unlock()
	return h, nil
}

// SendPrompt routes a prompt to the owning runtime.
func (r *Registry) SendPrompt(ctx context.Context, agentID, text string) error {
	rt, err := r.ownerFor(agentID)
	if err != nil {
		return err
	}
	return rt.SendPrompt(ctx, agentID, text)
}

// Cancel routes a cancel to the owning runtime. The bool reports whether a turn
// or pending permission was actually interrupted (false = idle no-op).
func (r *Registry) Cancel(ctx context.Context, agentID string) (bool, error) {
	rt, err := r.ownerFor(agentID)
	if err != nil {
		return false, err
	}
	return rt.Cancel(ctx, agentID)
}

// Stop routes a stop to the owning runtime and forgets the agent.
// The agent is removed from rtByAgent before rt.Stop() so that concurrent
// SendPrompt/Permission calls get ErrNoHandle immediately rather than racing
// for the full stopGrace window.
func (r *Registry) Stop(ctx context.Context, agentID string) error {
	r.mu.Lock()
	rt, ok := r.rtByAgent[agentID]
	delete(r.rtByAgent, agentID)
	r.mu.Unlock()
	if !ok || rt == nil {
		return ErrNoHandle
	}
	return rt.Stop(ctx, agentID)
}

// CheckMessages routes a nudger wake-up by process id. The nudger reads pids
// from the running registry; this method maps that pid back to the owning agent
// and runtime.
func (r *Registry) CheckMessages(ctx context.Context, pid int) error {
	running, err := r.store.ListRunning()
	if err != nil {
		return err
	}
	for _, row := range running {
		if row.PID != pid {
			continue
		}
		rt, err := r.ownerFor(row.AgentID)
		if err != nil {
			return err
		}
		return rt.CheckMessages(ctx, pid)
	}
	return ErrNoHandle
}

// Permission routes a permission decision to the owning runtime.
func (r *Registry) Permission(ctx context.Context, agentID, toolCallID, decision string) error {
	rt, err := r.ownerFor(agentID)
	if err != nil {
		return err
	}
	return rt.Permission(ctx, agentID, toolCallID, decision)
}

// Subscribe routes a subscription to the owning runtime.
func (r *Registry) Subscribe(agentID string) (<-chan Event, func(), error) {
	rt, err := r.ownerFor(agentID)
	if err != nil {
		return nil, nil, err
	}
	return rt.Subscribe(agentID)
}

func (r *Registry) Transcript(agentID string) ([]Event, error) {
	rt, err := r.ownerFor(agentID)
	if err != nil {
		return nil, err
	}
	return rt.Transcript(agentID)
}

// Shutdown stops every live agent (server shutdown, techspec §8.5).
func (r *Registry) Shutdown(ctx context.Context) {
	r.chat.StopAll(ctx)
	r.mu.Lock()
	term := r.term
	r.mu.Unlock()
	if sa, ok := term.(stopAller); ok {
		sa.StopAll(ctx)
	}
	r.mu.Lock()
	r.rtByAgent = map[string]Runtime{}
	r.mu.Unlock()
}

// notImplementedRuntime is the stub for interfaces not implemented this phase
// (terminal). Every method returns ErrNotImplemented. Real chat methods live on
// ChatRuntime; this exists so dispatch wires up without nil entries.
type notImplementedRuntime struct {
	name string
}

func (n notImplementedRuntime) Start(context.Context, LaunchSpec) (*Handle, error) {
	return nil, fmt.Errorf("%w: %s runtime", ErrNotImplemented, n.name)
}

func (n notImplementedRuntime) SendPrompt(context.Context, string, string) error {
	return fmt.Errorf("%w: %s runtime", ErrNotImplemented, n.name)
}

func (n notImplementedRuntime) Cancel(context.Context, string) (bool, error) {
	return false, fmt.Errorf("%w: %s runtime", ErrNotImplemented, n.name)
}

func (n notImplementedRuntime) Stop(context.Context, string) error {
	return fmt.Errorf("%w: %s runtime", ErrNotImplemented, n.name)
}

func (n notImplementedRuntime) Resume(context.Context, LaunchSpec, string) (*Handle, error) {
	return nil, fmt.Errorf("%w: %s runtime", ErrNotImplemented, n.name)
}

func (n notImplementedRuntime) CheckMessages(context.Context, int) error {
	return fmt.Errorf("%w: %s runtime", ErrNotImplemented, n.name)
}

func (n notImplementedRuntime) Permission(context.Context, string, string, string) error {
	return fmt.Errorf("%w: %s runtime", ErrNotImplemented, n.name)
}

func (n notImplementedRuntime) Subscribe(string) (<-chan Event, func(), error) {
	return nil, nil, fmt.Errorf("%w: %s runtime", ErrNotImplemented, n.name)
}

func (n notImplementedRuntime) Transcript(string) ([]Event, error) {
	return nil, fmt.Errorf("%w: %s runtime", ErrNotImplemented, n.name)
}

// compile-time assertions that the stub and chat runtime satisfy Runtime.
var (
	_ Runtime = notImplementedRuntime{}
	_ Runtime = (*ChatRuntime)(nil)
)
