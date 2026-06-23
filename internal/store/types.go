package store

import "time"

// All data structures persisted under ~/.agentdeck/. JSON tags match master
// PRD §3 exactly. Pointers are used for nullable/override fields so that a JSON
// null (inherit) is distinguishable from an explicit value.

// ---- Agent identity: agents/{agent_id}.json (PRD §3.1) ----

// Agent is the stable identity of an agent. The AgentID never changes for the
// life of the agent; everything that "switches" (model/backend/interface)
// re-launches against the same AgentID.
type Agent struct {
	AgentID   string    `json:"agent_id"`        // stable, never changes ("a_8f3c12")
	Name      string    `json:"name"`            // human display name, user-editable
	Role      string    `json:"role"`            // references roles/{role}.json
	Project   string    `json:"project"`         // references projects/{project}.json
	Backend   string    `json:"backend"`         // references a backend key in backends.json
	Model     string    `json:"model"`           // model key within the backend
	Interface string    `json:"interface"`       // "chat" | "terminal"
	CreatedAt time.Time `json:"created_at"`      // RFC3339
	Group     string    `json:"group,omitempty"` // optional task-group label
}

// ---- Active session registry: running/{agent_id}.json (PRD §3.1) ----

// RunningEntry records an active session for an agent. SessionID is ephemeral
// and changes on fork/resume; PID is the process group id of the CLI.
type RunningEntry struct {
	AgentID   string    `json:"agent_id"`
	PID       int       `json:"pid"`           // process group id of the CLI
	SessionID string    `json:"session_id"`    // ephemeral, changes on fork/resume
	Interface string    `json:"interface"`     // "chat" | "terminal"
	TTY       string    `json:"tty,omitempty"` // only for terminal interface
	StartedAt time.Time `json:"started_at"`    // RFC3339
}

// ---- Live state: status/{agent_id}.json (PRD §3.1) ----

// Status is the live, frequently-updated state of an agent.
type Status struct {
	AgentID    string     `json:"agent_id"`
	State      string     `json:"state"`            // "busy"|"idle"|"waiting_input"|"done"|"error"
	Detail     string     `json:"detail,omitempty"` // "Editing src/auth.ts"
	LastTrace  string     `json:"last_trace,omitempty"`
	BusySince  *time.Time `json:"busy_since,omitempty"`
	ContextPct float64    `json:"context_pct"` // 0..1
}

// ---- Role: roles/{role}.json (PRD §3.2) ----

// Role is a reusable persona. SkipPermissions is a pointer so null (inherit the
// global config), true, and false are all distinguishable on disk and over the API.
type Role struct {
	Title           string `json:"title"`
	SystemPrompt    string `json:"system_prompt"`
	SkipPermissions *bool  `json:"skip_permissions"` // null = inherit global; true/false = override
}

// ---- Project: projects/{project}.json (PRD §3.3) ----

// Project is a workspace definition. Cwd may contain a leading "~" which callers
// expand via ExpandTilde; the store itself stores paths verbatim.
type Project struct {
	Title         string   `json:"title"`
	Color         [3]int   `json:"color"`   // RGB display accent, e.g. [100,180,255]
	Cwd           string   `json:"cwd"`     // "~/Projects/my-app"
	AddDirs       []string `json:"add_dirs"` // extra accessible directories
	ContextPrompt string   `json:"context_prompt"`
}

// ---- Backend config: backends.json (PRD §3.4), version 2 ----

// BackendsConfig is the single backends.json file. Version is the schema
// version (== 2 in Phase 0).
type BackendsConfig struct {
	Version  int                `json:"version"`  // == 2
	Backends map[string]Backend `json:"backends"` // keyed by backend id ("claude","codex")
}

// Backend is one provider entry. Exactly one backend should have Default == true.
type Backend struct {
	Name         string            `json:"name"`
	Type         string            `json:"type"`              // "claude-acp" | "codex-acp"
	Default      bool              `json:"default,omitempty"` // exactly one backend default
	DefaultModel string            `json:"default_model"`
	Models       map[string]Model  `json:"models"`        // keyed by model id
	Env          map[string]string `json:"env,omitempty"` // backend-level env, applies to all models
}

// Model is one model under a backend. Per-model Env overrides backend-level Env.
type Model struct {
	Name  string            `json:"name"`
	Model string            `json:"model"`         // provider model string ("claude-sonnet-4-6")
	Env   map[string]string `json:"env,omitempty"` // per-model env; overrides backend env
}

// ---- Layout: layout.json (PRD §3.5) ----

// Layout is the dashboard card arrangement.
type Layout struct {
	Order   []string `json:"order"` // agent_id card order
	Density Density  `json:"density"`
}

// Density controls card grid spacing.
type Density struct {
	CardsPerRow int `json:"cards_per_row"`
	Gap         int `json:"gap"` // px
}

// ---- Config: config.json (PRD §3.5 + phase-0 §3) ----

// Config is the top-level config.json. Version is the schema version (== 1).
type Config struct {
	Version         int    `json:"version"`          // == 1
	Port            int    `json:"port"`             // 4317
	DefaultProject  string `json:"default_project"`  // "my-app"
	DefaultRole     string `json:"default_role"`     // "implementer"
	SkipPermissions bool   `json:"skip_permissions"` // false
}
