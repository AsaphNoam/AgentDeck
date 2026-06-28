// Package runtime implements the agent runtime abstraction (master PRD §4.1):
// the Runtime interface the server programs against, a Registry that dispatches
// by agent.interface, the normalized transcript Event types, and the chat
// runtime over the ACP stdio protocol. Phase 1.1 lays down the pure-data and
// interface scaffolding; the process work lands in later subphases.
package runtime

import (
	"context"

	"github.com/agentdeck/agentdeck/internal/state"
)

// LaunchSpec is the fully-composed input to Start. The launch flow (techspec §6)
// builds this from agent identity + project + role + backend/model so the Runtime
// needs no further lookups; a running agent's spec is frozen.
type LaunchSpec struct {
	Agent        state.Agent     // stable identity (agent_id, role, project, backend, model, interface)
	Cwd          string          // resolved absolute working dir (project.cwd, ~-expanded)
	AddDirs      []string        // project.add_dirs, ~-expanded
	SystemPrompt string          // composed: context_prompt + role.system_prompt
	BackendType  string          // "claude-acp" | "codex-acp"
	ModelID      string          // provider model id, e.g. "claude-sonnet-4-6"
	Env          []string        // composed env layering (backend then per-model override), "K=V"
	SkipPerms    bool            // effective skip_permissions after role/global resolution
	HookToken    string          // per-launch one-time token passed to the agent's hooks
	MCPServers   []MCPServerSpec // messaging MCP server registration; one entry this phase
	ExtraArgs    []string        // reserved (e.g. extra adapter flags) — empty this phase
}

// MCPServerSpec is one stdio MCP server the agent should connect to. This phase
// carries exactly one: the in-process Go messaging server (techspec §6.4).
type MCPServerSpec struct {
	Name    string   // "agentdeck-messaging"
	Command string   // path to invoke; re-execs the binary in a hidden mcp-stdio mode
	Args    []string // includes the hook token / agent_id so the server scopes to this agent
	Env     []string // "K=V"
}

// Handle is the live, in-memory representation of a started runtime. Returned by
// Start and held by the Registry keyed by agent_id. Not persisted.
type Handle struct {
	AgentID   string
	Pid       int    // == pgid; written to the running row in state.db
	SessionID string // ephemeral CLI session id, written to the running row in state.db

	// Internal process/stream plumbing (stdin writer, event hub, pending-permission
	// map, cancel fn, …) is added in later subphases.
}

// Runtime is the interface the server programs against (master PRD §4.1). One
// real implementation this phase: ChatRuntime (claude-acp). TerminalRuntime and
// the codex backend are stubs returning ErrNotImplemented (techspec §3.3).
type Runtime interface {
	// Start spawns the CLI, performs the ACP initialize handshake, records the
	// ephemeral session id, inserts the running + initial status rows in state.db,
	// and returns a Handle. The Registry guards against a duplicate Handle.
	Start(ctx context.Context, spec LaunchSpec) (*Handle, error)

	// SendPrompt submits one user turn. Non-blocking: it writes the prompt frame
	// and returns; transcript events stream asynchronously via the agent's hub.
	SendPrompt(ctx context.Context, agentID, text string) error

	// Cancel interrupts the in-progress turn (ACP cancel). Safe to call when idle:
	// it is then a no-op and reports false. The bool is true when a turn or a
	// pending permission was actually interrupted. Does not stop the process.
	Cancel(ctx context.Context, agentID string) (bool, error)

	// Stop terminates the process group, removes the running row from state.db,
	// and sets the status row's state. Idempotent.
	Stop(ctx context.Context, agentID string) error

	// Resume re-attaches to a persisted session. STUB this phase: returns
	// ErrNotImplemented. Signature fixed now for Phase 4.
	Resume(ctx context.Context, spec LaunchSpec, sessionID string) (*Handle, error)

	// CheckMessages wakes an idle agent to drain its mailbox. STUB this phase:
	// returns ErrNotImplemented. Signature fixed now for Phase 5.
	CheckMessages(ctx context.Context, pid int) error

	// Permission relays an approve/deny decision back over ACP for a pending
	// permission request. Errors if no such pending request.
	Permission(ctx context.Context, agentID, toolCallID, decision string) error

	// Subscribe returns a channel of normalized events for an agent and an
	// unsubscribe func. Buffered, drop-oldest.
	Subscribe(agentID string) (<-chan Event, func(), error)
}
