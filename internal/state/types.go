package state

import (
	"encoding/json"
	"time"
)

// Agent is the stable identity of an agent. AgentID never changes.
type Agent struct {
	AgentID   string    `json:"agent_id"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	Project   string    `json:"project"`
	Backend   string    `json:"backend"`
	Model     string    `json:"model"`
	Effort    string    `json:"effort"`
	Interface string    `json:"interface"`
	CreatedAt time.Time `json:"created_at"`
	Group     string    `json:"group,omitempty"`
	Archived  bool      `json:"archived"`
}

// RunningEntry records an active session for an agent.
type RunningEntry struct {
	AgentID   string `json:"agent_id"`
	PID       int    `json:"pid"`
	SessionID string `json:"session_id"`
	Interface string `json:"interface"`
	TTY       string `json:"tty,omitempty"`
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
	Effort    string `json:"effort"`
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

	UnreadMessages int                  `json:"unread_messages,omitempty"`
	LastSentAt     string               `json:"last_sent_at,omitempty"`
	UpdatedAt      int64                `json:"updated_at"`
	Archived       bool                 `json:"archived"`
	Pipeline       *PipelineAssociation `json:"pipeline,omitempty"`
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

// Availability values for LiveAgent: an agent that is running right now, or a
// stopped chat agent a message wakes before delivery (FS-06.R22).
const (
	AvailabilityRunning         = "running"
	AvailabilityStoppedWakeable = "stopped_wakeable"
	// AvailabilityStopped marks a durable stopped identity in the context
	// recipient directory. It is deliberately distinct from stopped_wakeable:
	// a context grant starts no process, so this value must never be read as a
	// promise that mail would wake the agent (FS-15.R17).
	AvailabilityStopped = "stopped"
)

// LiveAgent is an addressable agent — running, or stopped and wakeable — joined
// with identity and latest status: the unit list_agents returns and
// ResolveRecipient matches against (techspec §3.2, §3.3). State always carries
// the agent's latest durable status, so a stopped agent still reports the state
// it ended in; Availability is what tells a sender that delivery implies a wake.
type LiveAgent struct {
	AgentID      string  `json:"agent_id"`
	Name         string  `json:"name"`
	Role         string  `json:"role"`
	Project      string  `json:"project"`
	Interface    string  `json:"interface"`    // "chat" | "terminal"
	Address      string  `json:"address"`      // canonical "role@project"
	State        string  `json:"state"`        // latest status state, or "unknown"
	Availability string  `json:"availability"` // "running" | "stopped_wakeable"
	Detail       string  `json:"detail"`
	ContextPct   float64 `json:"context_pct"`
}

// AgentRef is a compact reference returned in ambiguous-recipient errors so the
// sender can re-address by agent_id (techspec §9).
type AgentRef struct {
	AgentID string `json:"agent_id"`
	Name    string `json:"name"`
	Address string `json:"address"`
}

// PipelineRunRecord is the durable machine-state row. JSON blobs contain the
// immutable model-neutral template/run snapshot and are decoded by
// internal/pipeline, keeping internal/state independent of the orchestrator.
type PipelineRunRecord struct {
	RunID            string          `json:"run_id"`
	TemplateID       string          `json:"template_id"`
	TemplateSnapshot json.RawMessage `json:"template_snapshot"`
	DisplayName      string          `json:"display_name"`
	Project          string          `json:"project"`
	Goal             string          `json:"goal"`
	Inputs           json.RawMessage `json:"inputs"`
	Assignments      json.RawMessage `json:"assignments"`
	State            string          `json:"state"`
	Revision         int64           `json:"revision"`
	PendingAction    string          `json:"pending_action"`
	CurrentStageID   string          `json:"current_stage_id"`
	CurrentAttemptID string          `json:"current_attempt_id"`
	CurrentAgentID   string          `json:"current_agent_id"`
	AttentionReason  string          `json:"attention_reason"`
	FinalOutcome     string          `json:"final_outcome"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

type PipelineAttemptRecord struct {
	AttemptID         string          `json:"attempt_id"`
	RunID             string          `json:"run_id"`
	StageID           string          `json:"stage_id"`
	AttemptNo         int             `json:"attempt_no"`
	VisitNo           int             `json:"visit_no"`
	ParentAttemptID   string          `json:"parent_attempt_id,omitempty"`
	AgentID           string          `json:"agent_id,omitempty"`
	AgentGeneration   string          `json:"agent_generation,omitempty"`
	Backend           string          `json:"backend"`
	Model             string          `json:"model"`
	Effort            string          `json:"effort"`
	State             string          `json:"state"`
	AssignmentText    string          `json:"assignment_text"`
	AssignmentHash    string          `json:"assignment_hash"`
	AssignmentVersion int             `json:"assignment_version"`
	ReportOutcome     string          `json:"report_outcome,omitempty"`
	ReportSummary     string          `json:"report_summary,omitempty"`
	ReportDetails     string          `json:"report_details,omitempty"`
	ReportChecks      string          `json:"report_checks,omitempty"`
	ReportOutputs     json.RawMessage `json:"report_outputs"`
	ReportedAt        *time.Time      `json:"reported_at,omitempty"`
	QuiescentAt       *time.Time      `json:"quiescent_at,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

type PipelineValueRecord struct {
	RunID           string    `json:"run_id"`
	Name            string    `json:"name"`
	Value           string    `json:"value"`
	SourceKind      string    `json:"source_kind"`
	SourceAttemptID string    `json:"source_attempt_id,omitempty"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// PipelineProposalRecord is the durable, canonical result of an AgentDecker
// proposal tool call. Its payload stays opaque here so state remains independent
// of the pipeline package's template and run request types.
type PipelineProposalRecord struct {
	ProposalID string          `json:"proposal_id"`
	Kind       string          `json:"kind"`
	Digest     string          `json:"digest"`
	Payload    json.RawMessage `json:"payload"`
	CreatedAt  time.Time       `json:"created_at"`
}

type CreatePipelineRunParams struct {
	Run            PipelineRunRecord
	RequestID      string
	RequestHash    string
	Values         []PipelineValueRecord
	InitialAttempt *PipelineAttemptRecord
}

type PipelineRunUpdate struct {
	State            string
	PendingAction    string
	CurrentStageID   string
	CurrentAttemptID string
	CurrentAgentID   string
	AttentionReason  string
	FinalOutcome     string
	UpdatedAt        time.Time
}

type PipelineReportInput struct {
	RunID            string
	AttemptID        string
	AgentID          string
	AgentGeneration  string
	ExpectedRevision int64
	Outcome          string
	Summary          string
	Details          string
	Checks           string
	Outputs          []PipelineValueRecord
	ReportedAt       time.Time
}

type PipelineAssociation struct {
	RunID     string `json:"run_id"`
	RunName   string `json:"run_name"`
	StageID   string `json:"stage_id"`
	AttemptID string `json:"attempt_id"`
	AttemptNo int    `json:"attempt_no"`
}
