package state

import "time"

// Agent is the stable identity of an agent. AgentID never changes.
type Agent struct {
	AgentID   string    `json:"agent_id"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	Project   string    `json:"project"`
	Backend   string    `json:"backend"`
	Model     string    `json:"model"`
	Interface string    `json:"interface"`
	CreatedAt time.Time `json:"created_at"`
	Group     string    `json:"group,omitempty"`
}

// RunningEntry records an active session for an agent.
type RunningEntry struct {
	AgentID   string    `json:"agent_id"`
	PID       int       `json:"pid"`
	SessionID string    `json:"session_id"`
	Interface string    `json:"interface"`
	TTY       string    `json:"tty,omitempty"`
	// Driver is the TerminalDriver backing a terminal agent (xterm/tmux/iterm2);
	// empty for chat agents. DriverIDs carries driver-specific identifiers (e.g.
	// the tmux session name, iTerm2 window/tab/session ids). (Phase 6 techspec §3.1.)
	Driver    string            `json:"driver,omitempty"`
	DriverIDs map[string]string `json:"driver_ids,omitempty"`
	HookToken string            `json:"-"`
	StartedAt time.Time         `json:"started_at"`
}

// Status is the live, frequently-updated state of an agent.
type Status struct {
	AgentID    string     `json:"agent_id"`
	State      string     `json:"state"`
	Detail     string     `json:"detail,omitempty"`
	LastTrace  string     `json:"last_trace,omitempty"`
	BusySince  *time.Time `json:"busy_since,omitempty"`
	ContextPct float64    `json:"context_pct"`
	UpdatedAt  int64      `json:"updated_at"`
}

// AgentState is the dashboard-ready merge of agent identity, running state, and
// latest status. Timestamps are strings because this shape is sent directly to
// the browser over SSE.
type AgentState struct {
	AgentID   string `json:"agent_id"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	Project   string `json:"project"`
	Backend   string `json:"backend"`
	Model     string `json:"model"`
	Interface string `json:"interface"`
	Group     string `json:"group,omitempty"`
	CreatedAt string `json:"created_at"`

	Running   bool   `json:"running"`
	PID       int    `json:"pid,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	TTY       string `json:"tty,omitempty"`
	Driver    string `json:"driver,omitempty"`
	StartedAt string `json:"started_at,omitempty"`

	State      string  `json:"state"`
	Detail     string  `json:"detail"`
	LastTrace  string  `json:"last_trace,omitempty"`
	BusySince  string  `json:"busy_since,omitempty"`
	ContextPct float64 `json:"context_pct"`

	UnreadMessages int    `json:"unread_messages,omitempty"`
	LastSentAt     string `json:"last_sent_at,omitempty"`
	UpdatedAt      int64  `json:"updated_at"`
}

// AgentStateUpdate is the payload published after Manager recomputes an agent.
// Normal updates embed AgentState fields; hard deletes publish Removed=true.
type AgentStateUpdate struct {
	AgentState
	Removed bool `json:"removed,omitempty"`
}

// HookPayload is the POST /api/hook body after token extraction.
type HookPayload struct {
	AgentID    string   `json:"agent_id"`
	Event      string   `json:"event"`
	State      string   `json:"state,omitempty"`
	Detail     string   `json:"detail,omitempty"`
	LastTrace  string   `json:"last_trace,omitempty"`
	ContextPct *float64 `json:"context_pct,omitempty"`
	PID        int      `json:"pid,omitempty"`
	SessionID  string   `json:"session_id,omitempty"`
	TTY        string   `json:"tty,omitempty"`
	TS         int64    `json:"ts,omitempty"`
	// Fields for file_edit / command hook events (Phase 4 file/command tracking).
	Path       string `json:"path,omitempty"`
	Command    string `json:"command,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
	Seq        int64  `json:"seq,omitempty"`
	Timestamp  string `json:"timestamp,omitempty"` // RFC3339; fallback if TS is absent
}

// Message is one agent-to-agent mailbox entry (Phase 5 techspec §4.1). The
// dashboard server is the sole writer. from_agent is always the sender's
// session-bound agent_id (never a spoofable argument).
type Message struct {
	MessageID    string     `json:"message_id"`
	FromAgent    string     `json:"from_agent"`
	FromAddress  string     `json:"from_address"`
	FromName     string     `json:"from_name"`
	ToAgent      string     `json:"to_agent"`
	Subject      string     `json:"subject"`
	Body         string     `json:"body"`
	CreatedAt    time.Time  `json:"created_at"`
	Read         bool       `json:"read"`
	ReadAt       *time.Time `json:"read_at,omitempty"`
	DeliveredVia string     `json:"delivered_via"`
	InReplyTo    string     `json:"in_reply_to,omitempty"`
}

// LiveAgent is a currently-running agent (a row in the running registry) joined
// with identity and latest status — the unit list_agents returns and
// ResolveRecipient matches against (techspec §3.2, §3.3).
type LiveAgent struct {
	AgentID    string  `json:"agent_id"`
	Name       string  `json:"name"`
	Role       string  `json:"role"`
	Project    string  `json:"project"`
	Address    string  `json:"address"` // canonical "role@project"
	State      string  `json:"state"`   // latest status state, or "unknown"
	Detail     string  `json:"detail"`
	ContextPct float64 `json:"context_pct"`
}

// AgentRef is a compact reference returned in ambiguous-recipient errors so the
// sender can re-address by agent_id (techspec §9).
type AgentRef struct {
	AgentID string `json:"agent_id"`
	Name    string `json:"name"`
	Address string `json:"address"`
}
