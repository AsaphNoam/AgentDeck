package runtime

import (
	"context"
	"fmt"

	"github.com/agentdeck/agentdeck/internal/state"
)

// ChatRuntime drives one Claude Code agent over the ACP stdio protocol. Phase
// 1.1 lands the type and the Runtime-interface surface as stubs returning
// ErrNotImplemented; the real spawn/handshake/stream/gating logic arrives in
// subphases 1.3–1.4. ChatRuntime.Start checks spec.BackendType — only
// "claude-acp" proceeds; anything else (e.g. "codex-acp") returns
// ErrNotImplemented (techspec §3.3).
type ChatRuntime struct {
	store *state.Store
}

// NewChatRuntime constructs the chat runtime bound to the state store.
func NewChatRuntime(s *state.Store) *ChatRuntime {
	return &ChatRuntime{store: s}
}

func (c *ChatRuntime) Start(ctx context.Context, spec LaunchSpec) (*Handle, error) {
	if spec.BackendType != "claude-acp" {
		return nil, fmt.Errorf("%w: backend %q", ErrNotImplemented, spec.BackendType)
	}
	return nil, fmt.Errorf("%w: ChatRuntime.Start (subphase 1.3)", ErrNotImplemented)
}

func (c *ChatRuntime) SendPrompt(ctx context.Context, agentID, text string) error {
	return fmt.Errorf("%w: ChatRuntime.SendPrompt (subphase 1.3)", ErrNotImplemented)
}

func (c *ChatRuntime) Cancel(ctx context.Context, agentID string) error {
	return fmt.Errorf("%w: ChatRuntime.Cancel (subphase 1.4)", ErrNotImplemented)
}

func (c *ChatRuntime) Stop(ctx context.Context, agentID string) error {
	return fmt.Errorf("%w: ChatRuntime.Stop (subphase 1.4)", ErrNotImplemented)
}

func (c *ChatRuntime) Resume(ctx context.Context, spec LaunchSpec, sessionID string) (*Handle, error) {
	return nil, fmt.Errorf("%w: Resume (Phase 4)", ErrNotImplemented)
}

func (c *ChatRuntime) CheckMessages(ctx context.Context, pid int) error {
	return fmt.Errorf("%w: CheckMessages (Phase 5)", ErrNotImplemented)
}

func (c *ChatRuntime) Permission(ctx context.Context, agentID, toolCallID, decision string) error {
	return fmt.Errorf("%w: ChatRuntime.Permission (subphase 1.4)", ErrNotImplemented)
}

func (c *ChatRuntime) Subscribe(agentID string) (<-chan Event, func(), error) {
	return nil, nil, fmt.Errorf("%w: ChatRuntime.Subscribe (subphase 1.3)", ErrNotImplemented)
}
