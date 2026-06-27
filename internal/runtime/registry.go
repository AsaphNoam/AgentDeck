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
	byIface   map[string]Runtime // "chat" -> ChatRuntime, "terminal" -> stub
	rtByAgent map[string]Runtime // agent_id -> owning runtime
	chat      *ChatRuntime
	store     *state.Store
}

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
	return r
}

// Chat returns the chat runtime (e.g. to point it at a pinned adapter binary or,
// in tests, the fake CLI).
func (r *Registry) Chat() *ChatRuntime { return r.chat }

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
	r.mu.Lock()
	if _, ok := r.rtByAgent[spec.Agent.AgentID]; ok {
		r.mu.Unlock()
		return nil, ErrAlreadyStarted
	}
	r.mu.Unlock()

	h, err := rt.Start(ctx, spec)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.rtByAgent[spec.Agent.AgentID] = rt
	r.mu.Unlock()
	return h, nil
}

// ownerFor returns the runtime that owns an agent, or ErrNoHandle.
func (r *Registry) ownerFor(agentID string) (Runtime, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rt, ok := r.rtByAgent[agentID]
	if !ok {
		return nil, ErrNoHandle
	}
	return rt, nil
}

// SendPrompt routes a prompt to the owning runtime.
func (r *Registry) SendPrompt(ctx context.Context, agentID, text string) error {
	rt, err := r.ownerFor(agentID)
	if err != nil {
		return err
	}
	return rt.SendPrompt(ctx, agentID, text)
}

// Cancel routes a cancel to the owning runtime.
func (r *Registry) Cancel(ctx context.Context, agentID string) error {
	rt, err := r.ownerFor(agentID)
	if err != nil {
		return err
	}
	return rt.Cancel(ctx, agentID)
}

// Stop routes a stop to the owning runtime and forgets the agent.
func (r *Registry) Stop(ctx context.Context, agentID string) error {
	rt, err := r.ownerFor(agentID)
	if err != nil {
		return ErrNoHandle
	}
	err = rt.Stop(ctx, agentID)
	r.mu.Lock()
	delete(r.rtByAgent, agentID)
	r.mu.Unlock()
	return err
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

// Shutdown stops every live agent (server shutdown, techspec §8.5).
func (r *Registry) Shutdown(ctx context.Context) {
	r.chat.StopAll(ctx)
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

func (n notImplementedRuntime) Cancel(context.Context, string) error {
	return fmt.Errorf("%w: %s runtime", ErrNotImplemented, n.name)
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

// compile-time assertions that the stub and chat runtime satisfy Runtime.
var (
	_ Runtime = notImplementedRuntime{}
	_ Runtime = (*ChatRuntime)(nil)
)
