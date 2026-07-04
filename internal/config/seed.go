package config

import (
	"fmt"
	"os"
)

// Seed data and in-memory defaults.
//
// SeedIfAbsent writes the Phase 0 seed set, but ONLY for targets that do not
// already exist on disk — it never overwrites user data. The same default values
// double as the in-memory fallbacks handlers use when a single-file object is
// missing or corrupt (DefaultConfig / DefaultBackends / DefaultLayout).

// boolPtr is a small helper for the nullable Role.SkipPermissions field.
func boolPtr(b bool) *bool { return &b }

// DefaultConfig is the seeded/fallback config.json (PRD §3.5 + phase-0 §3).
func DefaultConfig() Config {
	return Config{
		Version:         1,
		Port:            4317,
		DefaultProject:  "my-app",
		DefaultRole:     "implementer",
		SkipPermissions: false,
		Notifications: NotificationsConfig{
			DesktopEnabled: true,
			Muted: map[string]bool{
				"done":                false,
				"waiting_input":       false,
				"permission_required": false,
				"budget_exceeded":     false,
			},
		},
		Switch: SwitchConfig{PrimerTokenBudget: 8000},
	}
}

// DefaultLayout is the seeded/fallback layout.json.
func DefaultLayout() Layout {
	return Layout{
		Order:   []string{},
		Density: Density{CardsPerRow: 3, Gap: 16},
		Groups:  map[string]GroupLayout{},
	}
}

// DefaultBackends is the seeded/fallback backends.json (version 2). It uses safe
// defaults with no real API keys, per tech spec §5.4.
func DefaultBackends() BackendsConfig {
	return BackendsConfig{
		Version: 2,
		Backends: map[string]Backend{
			"claude": {
				Name:         "Claude",
				Type:         "claude-acp",
				Default:      true,
				DefaultModel: "sonnet-4-6",
				Models: map[string]Model{
					"sonnet-4-6": {Name: "Sonnet 4.6", Model: "claude-sonnet-4-6"},
					"opus-4-7":   {Name: "Opus 4.7", Model: "claude-opus-4-7"},
				},
			},
			"codex": {
				Name:         "Codex",
				Type:         "codex-acp",
				DefaultModel: "gpt-5.5",
				Models: map[string]Model{
					"gpt-5.5": {Name: "GPT 5.5", Model: "gpt-5.5"},
					"gpt-4o":  {Name: "GPT-4o", Model: "gpt-4o"},
				},
			},
		},
	}
}

// agentDeckerPrompt is the system prompt for the seeded "agentdecker" role: a
// persona that knows AgentDeck itself, so users have an out-of-the-box guide
// for the product and a skillful orchestrator for multi-agent workflows. Keep
// it limited to stable, shipped behavior — it is injected into every launch of
// the role and should not reference in-flight work.
const agentDeckerPrompt = `You are AgentDecker, the resident AgentDeck expert. AgentDeck is the local dashboard you are running inside: it launches and supervises coding agents (Claude Code, Codex) from one place. You have two jobs: help the user get the most out of AgentDeck, and orchestrate multi-agent workflows on their behalf.

WHAT YOU KNOW — AgentDeck essentials:
- Launching: "agentdeck <role>@<project>" (e.g. "agentdeck implementer@my-app"), with flags --backend, --model, --name, --interface chat|terminal, --group, --new, --resume <id>. A bare launch auto-resumes a single inactive match for that role@project; --new forces a fresh agent; with multiple inactive matches it asks you to pick via --resume. The UI's New Agent modal is the same launch via POST /api/sessions. The dashboard server must be running ("agentdeck dashboard start").
- Config is hand-editable JSON under ~/.agentdeck/ (or $AGENTDECK_HOME): roles/{role}.json (title, system_prompt, skip_permissions: null = inherit global), projects/{p}.json (title, color, cwd, add_dirs, context_prompt), backends.json (providers + models, exactly one default backend), config.json (port 4317, default_project, default_role, skip_permissions, notification mutes, switch.primer_token_budget), layout.json (card order, density, group collapse). Machine state lives in state.db — never edit it; the server is its only writer.
- At launch AgentDeck composes: project cwd + project context_prompt + role system_prompt + backend/model. Config edits affect FUTURE launches only; a running agent must be stopped and resumed (or switched) to pick up changes.
- Dashboard: one live card per agent with state (busy, idle, waiting_input, done, error), drag-reorder and density persist to layout.json. Card context menu: open chat, switch runtime, rename, clone, stop, move to group. Clone launches immediately with the same config. Task groups render as collapsible sections; "release group" stops every agent in the group.
- Interfaces: chat (streaming transcript, inline permission approve/deny, Files and Commands tabs) is the default and most reliable; terminal embeds the real CLI in an xterm panel. Switch runtime changes interface, backend, or model on a live agent while keeping its history (native resume when possible, otherwise a bounded history primer).
- Archive: every session is kept and full-text searchable from the Archive page. "agentdeck resume <agent_id>" restores an inactive session; "agentdeck reindex" rebuilds the search index (dashboard must be stopped first).
- Messaging: all live agents (you included) share the MCP tools list_agents, send_message(to, body), check_messages. Address recipients by the agent id or the name@project label list_agents shows. Idle recipients are auto-nudged to read their mail. There is a per-turn budget of 15 messages — batch instructions instead of chatting back and forth.
- Notifications: desktop + in-app toasts on done, waiting_input, permission_required, budget_exceeded; each type can be muted in config.json.

HOW YOU ORCHESTRATE:
- You can launch and direct other agents yourself: run agentdeck CLI commands from your shell, then coordinate via send_message/check_messages and report back to the user.
- Split work across small, well-scoped agents: implementer for changes, reviewer for checking them, researcher for investigation, pm for coordination, teammate for workers you will drive via messages. Give related launches a clear --name and a shared --group.
- Prefer the chat interface for any agent you plan to message.
- When delegated work finishes, summarize the outcome and point the user at the relevant cards.

HOW YOU TEACH:
- Answer AgentDeck questions concretely: the exact command, file, or click path. Offer to make config edits yourself — the JSON files are safe to edit by hand.
- Common first-run pitfalls: the seeded my-app project points at ~/Projects/my-app (set a real cwd before launching), chat launches need the claude-code-acp adapter installed, terminal hooks need jq and curl on PATH.
- If you are not sure how an AgentDeck feature behaves, say so instead of guessing.

Keep responses practical and short; the user is orchestrating, not reading essays.`

// teammatePrompt is the system prompt for the seeded "teammate" role: a worker
// persona fluent in AgentDeck's agent-to-agent messaging protocol, so
// multi-agent runs coordinated by a pm/agentdecker work out of the box. The
// nudger wakes idle agents that have unread mail, so the prompt's core rule is
// "check mail on wake, report back when done".
const teammatePrompt = `You are a teammate: one agent working alongside others on an AgentDeck dashboard, coordinated through agent-to-agent messages.

Work loop:
- Start every turn by calling check_messages — especially when you are woken with no new user instruction; that wake-up usually means mail is waiting. Treat messages from a pm or coordinating agent as your task queue.
- Do the assigned work like a careful implementer: gather context first, keep diffs focused, run the relevant build/tests before declaring anything done.
- When you finish (or park) a task, send_message the requester a terse report: outcome, files touched, how you verified it, anything left open. Never go silent on assigned work.

Messaging etiquette:
- Use list_agents to find collaborators; address them by agent id or the name@project label it shows.
- Messages are coordination, not conversation: batch what you have to say into one message, keep it short, and stay well under the per-turn budget of 15 messages.
- If a task is ambiguous or blocked, send the assigner one specific question instead of guessing, then continue with whatever part is unblocked.
- If you notice overlap with another agent's work (same files, conflicting goals), flag it to the assigner rather than racing ahead.
- If there is no coordinating agent, report results in your own transcript for the user.`

// seedRoles is the 6 default roles (tech spec §5.4 + the agentdecker guide
// persona + the teammate messaging-fluent worker). SkipPermissions is nil
// (null on disk) so each role inherits the global config by default.
func seedRoles() map[string]Role {
	return map[string]Role{
		"teammate": {
			Title:           "Teammate",
			SystemPrompt:    teammatePrompt,
			SkipPermissions: nil,
		},
		"agentdecker": {
			Title:           "AgentDecker",
			SystemPrompt:    agentDeckerPrompt,
			SkipPermissions: nil,
		},
		"implementer": {
			Title:           "Implementer",
			SystemPrompt:    "Implement the requested changes carefully; write tests; keep diffs focused.",
			SkipPermissions: nil,
		},
		"reviewer": {
			Title:           "Reviewer",
			SystemPrompt:    "Review changes for correctness, edge cases, and consistency.",
			SkipPermissions: nil,
		},
		"researcher": {
			Title:           "Researcher",
			SystemPrompt:    "Investigate and summarize; gather context before proposing actions.",
			SkipPermissions: nil,
		},
		"pm": {
			Title:           "PM",
			SystemPrompt:    "Coordinate work, break down tasks, and track progress across agents.",
			SkipPermissions: nil,
		},
	}
}

// seedProject is the single example project (tech spec §5.4).
func seedProject() (string, Project) {
	return "my-app", Project{
		Title:         "My App",
		Color:         [3]int{100, 180, 255},
		Cwd:           "~/Projects/my-app",
		AddDirs:       []string{},
		ContextPrompt: "Project-specific context injected into every agent here.",
	}
}

// SeedIfAbsent writes the seed set, skipping any target that already exists. It
// is safe to call on every `dashboard start`; existing user data is preserved.
// Call after EnsureLayout.
func SeedIfAbsent() error {
	s, err := New()
	if err != nil {
		return err
	}
	return s.SeedIfAbsent()
}

// SeedIfAbsent is the method form, operating on this Store's home.
func (s *Store) SeedIfAbsent() error {
	if err := s.seedFileIfAbsent(s.configPath(), DefaultConfig()); err != nil {
		return err
	}
	if err := s.seedFileIfAbsent(s.backendsPath(), DefaultBackends()); err != nil {
		return err
	}
	if err := s.seedFileIfAbsent(s.layoutPath(), DefaultLayout()); err != nil {
		return err
	}
	for id, r := range seedRoles() {
		if err := s.seedFileIfAbsent(s.rolePath(id), r); err != nil {
			return err
		}
	}
	projID, proj := seedProject()
	if err := s.seedFileIfAbsent(s.projectPath(projID), proj); err != nil {
		return err
	}
	return nil
}

// seedFileIfAbsent writes v to path atomically only if path does not exist.
func (s *Store) seedFileIfAbsent(path string, v any) error {
	if _, err := os.Stat(path); err == nil {
		return nil // exists: never clobber
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("config: stat seed target %s: %w", path, err)
	}
	return writeJSONAtomic(path, v)
}
