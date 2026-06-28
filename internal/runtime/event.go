package runtime

import "encoding/json"

// EventType constants name the normalized transcript event kinds (techspec §4.2).
// These are AgentDeck's own vocabulary, independent of the ACP wire shape, so a
// new backend never changes anything downstream. New types may be added; existing
// payload fields are append-only (techspec §11).
const (
	EvAssistantText     = "assistant_text"
	EvToolCall          = "tool_call"
	EvToolResult        = "tool_result"
	EvDiff              = "diff"
	EvPermissionRequest = "permission_request"
	EvTurnEnd           = "turn_end"
	EvError             = "error"
)

// Event is the normalized transcript event emitted to subscribers (techspec §3.1).
// The interim per-agent SSE streams this struct verbatim as its data object, and
// Phase 2 wraps the identical object as a multiplexed new_message payload — so
// agent_id, seq, type, ts, and data are permanent fields (techspec §11).
type Event struct {
	AgentID string          `json:"agent_id"`
	Seq     int64           `json:"seq"`  // monotonic per agent, starts at 1
	Type    string          `json:"type"` // one of the EventType constants
	Data    json.RawMessage `json:"data"` // type-specific payload (below)
	Ts      string          `json:"ts"`   // RFC3339 UTC
}

// AssistantTextData — a streamed markdown delta. NOT cumulative; the client
// appends successive deltas.
type AssistantTextData struct {
	Delta string `json:"delta"`
}

// ToolCallData — the agent intends to / begins to run a tool.
type ToolCallData struct {
	ToolCallID string          `json:"tool_call_id"` // correlates result + permission
	Name       string          `json:"name"`         // e.g. "Edit", "Bash", "Read"
	Title      string          `json:"title"`        // human label from ACP if present, else Name
	Args       json.RawMessage `json:"args"`         // raw tool arguments object
	Status     string          `json:"status"`       // "pending" | "in_progress"
}

// ToolResultData — outcome of a tool call.
type ToolResultData struct {
	ToolCallID string          `json:"tool_call_id"`
	Status     string          `json:"status"` // "completed" | "failed"
	Content    json.RawMessage `json:"content"`
	Error      string          `json:"error,omitempty"`
}

// DiffData — a file edit expressed as a patch (often arrives within a tool call).
type DiffData struct {
	ToolCallID string `json:"tool_call_id"`
	Path       string `json:"path"`     // absolute or cwd-relative file path
	OldText    string `json:"old_text"` // may be empty for new files
	NewText    string `json:"new_text"`
	Patch      string `json:"patch"` // unified diff if the adapter provides one, else derived
}

// PermissionRequestData — execution is PAUSED awaiting approve/deny (techspec §5).
type PermissionRequestData struct {
	ToolCallID   string          `json:"tool_call_id"`
	Name         string          `json:"name"`   // tool requiring permission
	Reason       string          `json:"reason"` // why permission is needed
	Args         json.RawMessage `json:"args"`
	Options      []PermOption    `json:"options"`       // ACP-offered options
	AutoApproved bool            `json:"auto_approved"` // true when skip_permissions bypassed the gate
	ExpiresAt    string          `json:"expires_at"`    // RFC3339; after this we auto-deny
}

// PermOption is one ACP-offered permission option.
type PermOption struct {
	OptionID string `json:"option_id"`
	Label    string `json:"label"`
	Kind     string `json:"kind"` // "allow_once" | "allow_always" | "reject_once" | "reject_always"
}

// TurnEndData — the assistant turn completed (success or stopped).
type TurnEndData struct {
	StopReason string  `json:"stop_reason"` // "end_turn" | "cancelled" | "max_tokens" | "error"
	ContextPct float64 `json:"context_pct"` // 0..1 if reported, else last-known
}

// ErrorData — runtime/protocol/process error surfaced to the client.
type ErrorData struct {
	Scope   string `json:"scope"` // "protocol" | "process" | "tool" | "internal"
	Message string `json:"message"`
	Fatal   bool   `json:"fatal"` // true => session is dead, Stop has been performed
}
