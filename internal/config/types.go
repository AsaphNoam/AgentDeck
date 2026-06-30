package config

// Config file data structures persisted under ~/.agentdeck/. JSON tags match
// master PRD §3 exactly. Pointers are used for nullable/override fields so JSON
// null (inherit) is distinguishable from an explicit value.

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
	Color         [3]int   `json:"color"`    // RGB display accent, e.g. [100,180,255]
	Cwd           string   `json:"cwd"`      // "~/Projects/my-app"
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
	Order   []string               `json:"order"` // agent_id card order
	Density Density                `json:"density"`
	Groups  map[string]GroupLayout `json:"groups,omitempty"`
}

// Density controls card grid spacing.
type Density struct {
	CardsPerRow int `json:"cards_per_row"`
	Gap         int `json:"gap"` // px
}

type GroupLayout struct {
	Collapsed bool `json:"collapsed"`
}

// ---- Config: config.json (PRD §3.5 + phase-0 §3) ----

// Config is the top-level config.json. Version is the schema version (== 1).
type Config struct {
	Version            int                 `json:"version"`             // == 1
	Port               int                 `json:"port"`                // 4317
	DefaultProject     string              `json:"default_project"`     // "my-app"
	DefaultRole        string              `json:"default_role"`        // "implementer"
	SkipPermissions    bool                `json:"skip_permissions"`    // false
	OnboardingComplete bool                `json:"onboarding_complete"` // set after first launch
	Notifications      NotificationsConfig `json:"notifications"`
	Switch             SwitchConfig        `json:"switch"`
}

type NotificationsConfig struct {
	DesktopEnabled bool            `json:"desktop_enabled"`
	Muted          map[string]bool `json:"muted"`
}

type SwitchConfig struct {
	PrimerTokenBudget int `json:"primer_token_budget"`
}
