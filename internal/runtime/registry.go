package runtime

import (
	"context"
	"fmt"
	"sync"

	"github.com/agentdeck/agentdeck/internal/state"
)

// Registry dispatches launch/control calls to the right Runtime by
// agent.interface and holds live handles keyed by agent_id (techspec §3.2).
// Backend selection (claude vs codex) is handled inside ChatRuntime.Start via
// spec.BackendType, not here.
type Registry struct {
	mu      sync.Mutex
	handles map[string]*Handle // agent_id -> live handle
	byIface map[string]Runtime // "chat" -> ChatRuntime, "terminal" -> stub
	store   *state.Store       // config file objects + state.db
}

// NewRegistry builds a Registry wired with the chat runtime and the terminal
// stub. The terminal runtime is a not-implemented stub until Phase 6.
func NewRegistry(s *state.Store) *Registry {
	r := &Registry{
		handles: map[string]*Handle{},
		byIface: map[string]Runtime{},
		store:   s,
	}
	r.byIface["chat"] = NewChatRuntime(s)
	r.byIface["terminal"] = notImplementedRuntime{name: "terminal"}
	return r
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
