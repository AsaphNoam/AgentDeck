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
	StartedAt time.Time `json:"started_at"`
}

// Status is the live, frequently-updated state of an agent.
type Status struct {
	AgentID    string     `json:"agent_id"`
	State      string     `json:"state"`
	Detail     string     `json:"detail,omitempty"`
	LastTrace  string     `json:"last_trace,omitempty"`
	BusySince  *time.Time `json:"busy_since,omitempty"`
	ContextPct float64    `json:"context_pct"`
}

// Message is one agent-to-agent mailbox entry.
type Message struct {
	ID        int64      `json:"id"`
	FromAgent string     `json:"from_agent"`
	ToAgent   string     `json:"to_agent"`
	Body      string     `json:"body"`
	CreatedAt time.Time  `json:"created_at"`
	ReadAt    *time.Time `json:"read_at,omitempty"`
}
