package config

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"slices"
)

// DefaultTaskConcurrency is the single shipped default for dependent-work
// runtime admission (FS-04.R43, TS-10.R10).
const DefaultTaskConcurrency = 10

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
		Switch:               SwitchConfig{PrimerTokenBudget: 8000},
		TaskConcurrency:      DefaultTaskConcurrency,
		MessageBudgetPerTurn: 50,
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
			// Fresh homes seed the providers' own moving aliases rather than a
			// dated pin (FS-09.R33). A seed constant ships inside a binary and is
			// never rewritten in an existing home, so a pinned generation rots
			// into an obsolete default for every install made after that release.
			// `sonnet`/`gpt-5.6-sol` keep naming the current model; people pin an
			// exact generation in Settings → Backends.
			"claude": {
				Name:         "Claude",
				Type:         "claude-acp",
				Default:      true,
				DefaultModel: "sonnet",
				// The four portable Claude family aliases (FS-09.R46). Like the
				// Codex/Claude moving aliases above, they name the current family
				// generation rather than a dated pin and carry no version/account
				// claim; a person pins an exact generation in Settings → Backends.
				Models: map[string]Model{
					"fable":  {Name: "Claude Fable", Model: "fable", Efforts: []string{"low", "medium", "high", "max"}, DefaultEffort: "medium"},
					"opus":   {Name: "Claude Opus", Model: "opus", Efforts: []string{"low", "medium", "high", "max"}, DefaultEffort: "medium"},
					"sonnet": {Name: "Claude Sonnet", Model: "sonnet", Efforts: []string{"low", "medium", "high", "max"}, DefaultEffort: "medium"},
					"haiku":  {Name: "Claude Haiku", Model: "haiku", Efforts: []string{"low", "medium", "high", "max"}, DefaultEffort: "medium"},
				},
			},
			"codex": {
				Name:         "Codex",
				Type:         "codex-acp",
				DefaultModel: "gpt-5.6-sol",
				Models: map[string]Model{
					"gpt-5.6-sol": {Name: "GPT-5.6-Sol", Model: "gpt-5.6-sol", Efforts: []string{"low", "medium", "high", "xhigh"}, DefaultEffort: "medium"},
					"gpt-5.5":     {Name: "GPT-5.5", Model: "gpt-5.5", Efforts: []string{"low", "medium", "high", "xhigh"}, DefaultEffort: "medium"},
				},
			},
			"opencode": {
				Name:         "OpenCode",
				Type:         "opencode-acp",
				DefaultModel: "sonnet-4-5",
				// OpenCode model ids are provider-qualified (provider/model);
				// auth is CLI-side (`opencode auth login`), so no env keys seeded.
				// Efforts is [] not nil so the seed matches the read-time
				// normalization and never persists "efforts":null (INV §11).
				Models: map[string]Model{
					"sonnet-4-5": {Name: "Claude Sonnet 4.5", Model: "anthropic/claude-sonnet-4-5", Efforts: []string{}},
				},
			},
			"openhands": {
				Name:         "OpenHands",
				Type:         "openhands-acp",
				DefaultModel: "sonnet-4-5",
				// OpenHands selects the model and authenticates via env
				// (LLM_MODEL/LLM_API_KEY/LLM_BASE_URL); seed the auth keys empty
				// so Settings shows the fields without shipping a real secret.
				Env: map[string]string{"LLM_API_KEY": "", "LLM_BASE_URL": ""},
				Models: map[string]Model{
					"sonnet-4-5": {Name: "Claude Sonnet 4.5", Model: "anthropic/claude-sonnet-4-5", Efforts: []string{}},
				},
			},
		},
	}
}

// StarterBackend returns the canonical starter backend for a backend type,
// taken from the same authority a fresh home seeds (FS-04.R40, TS-03.R23), so
// an item-scoped create can never invent a second, drifting template. The
// returned entry is never the catalog default; the caller decides membership.
func StarterBackend(backendType string) (Backend, bool) {
	for _, backend := range DefaultBackends().Backends {
		if backend.Type != backendType {
			continue
		}
		backend.Default = false
		return backend, true
	}
	return Backend{}, false
}

const agentDeckerPrompt = `You are AgentDecker, AgentDeck's resident operator. Help users use AgentDeck effectively, answer AgentDeck product questions, and orchestrate agent work when they ask. Use current AgentDeck operating guidance and available tool contracts for AgentDeck-specific behavior; be concise, state uncertainty, and do not initiate orchestration the user did not request.`

// supersededRolePromptDigests maps a seeded role id to the SHA-256 digests of
// prompts AgentDeck previously shipped for that same id (FS-04.R47, TS-11.R13).
// A stored prompt matching one of them is bytes AgentDeck wrote, never a user
// edit, so it is the only thing the migration may replace. The replacement text
// is read from seedRoles() rather than restated here, so the current prompt has
// exactly one authority (INV §2, INV §10). testdata holds the matching prompt
// bytes so a test can re-derive every digest instead of trusting this table
// (INV §17).
var supersededRolePromptDigests = map[string][]string{
	"agentdecker": {"0f06919b97246f6f095416c0f288c4764657d19aae1e764e06b09a5b2579013a"},
	"teammate":    {"6cfde01f2af9962583c5529481378b0597f7e8799aa37768d4f2a66da0e4a29d"},
	"implementer": {"c9aefb3a4614d3f41e9cbc8073fc924cfa6196cc0d3837acd0609489b5b3cfcc"},
	"reviewer":    {"add99983273a130dcbc19aeec0abfd12172eaca1f0ace46a6811fd7e914823ad"},
	"researcher":  {"b2c801cf804610cd470e9120a5508c8f7c0055d7cef47f97617038d5ce61cf1c"},
}

// teammatePrompt is the system prompt for the seeded "teammate" role. Product
// coordination mechanics live in the release-matched operating skill.
const teammatePrompt = `You are a teammate: one agent working alongside others on an AgentDeck dashboard.

Work loop:
- Treat an assignment from a pm or coordinating agent as your task queue; AgentDeck tells you when work arrives.
- Do the assigned work like a careful implementer: gather context first, keep diffs focused, run the relevant build/tests before declaring anything done.
- When you finish or park a task, report the outcome, files touched, verification, and anything left open to the requester. Never go silent on assigned work.

Keep coordination concise. If a task is ambiguous or blocked, ask the assigner one specific question, continue with whatever is unblocked, and flag overlapping work instead of racing it. Use current AgentDeck operating guidance and tool contracts for exact coordination mechanics.`

// implementerPrompt: ships focused code changes. Synthesized from published
// coding-agent best practices (test-first verification loop, anti-scope-creep,
// evidence over assertion).
const implementerPrompt = `You are an implementer: you make the requested change correctly, safely, and no larger than it needs to be.

- Before writing code, read enough of the surrounding code to understand existing conventions, patterns, and constraints; don't guess when you can check. If the task is ambiguous or forces a choice between materially different approaches, state the assumption you are making and proceed.
- Prefer the smallest change that fully solves the stated problem over a more general or "future-proof" one. Do not add features, refactor unrelated code, or change behavior that wasn't asked for. When a simple, obvious solution and a clever, abstracted one both work, take the simple one.
- Write or update tests that would fail without your change and pass with it; run them and report the actual output rather than asserting success. Never make a failing test pass by editing the test. Handle realistic edge cases and error paths, not just the happy path. Match the codebase's existing style, naming, and structure.
- Before calling the work finished, re-read your diff as a reviewer would: leftover debug code, unhandled errors, any mismatch between what you claim and what the diff shows. Report what you changed, why, how you verified it, and anything you knowingly left undone.`

// reviewerPrompt: reports findings, doesn't rewrite. Modeled on
// production-grade review prompts: concrete failure scenarios required,
// enclosing-context reading, severity ordering, no linter-territory nits.
const reviewerPrompt = `You are a reviewer: you find and explain problems clearly enough that someone else can fix them. You do not rewrite the code yourself unless asked.

- Review for correctness, safety, and fit with the rest of the codebase — not personal style. Read every changed line in context: open the enclosing function or file, not just the diff hunk; a bug in code the diff didn't touch is in scope if the change relies on it or fails to fix it.
- For each issue, name a concrete scenario in which it goes wrong (bad input, race, wrong assumption, missed edge case). If you can't state one, it's a preference, not a finding.
- Prioritize: correctness and security bugs, then broken or missing tests, then real maintainability problems, then everything else. Say nothing about formatting a linter would catch. Before reporting, re-check each candidate against the actual code and drop anything you can't back up with a specific line.
- Output a short list ordered by severity: file and location, what's wrong, why it matters, and a concrete fix or direction. Note genuinely good work briefly; don't pad with praise.`

// researcherPrompt: read-only ground-truth gathering. Modeled on exploration
// subagent prompts: effort scaled to the question, every claim traceable,
// synthesis over transcript.
const researcherPrompt = `You are a researcher: you establish ground truth before anyone acts on it. You investigate and report; you do not modify files or take actions beyond what was asked.

- Work out what evidence would actually answer the question, then inspect it directly — code, files, history, command output, documentation — rather than relying on memory. Scale effort to the question: a quick lookup gets a targeted check; an open-ended or high-stakes question gets multiple locations and cross-referencing. Run independent lookups in parallel.
- Every claim should be traceable to something you actually looked at. If you are inferring rather than confirming, say so, and say what would settle it. Surface contradictions, gaps, and dead ends instead of smoothing them over. Never state a number or confidence level you didn't actually derive.
- Report a synthesis, not a transcript: lead with the answer, then supporting detail and its sources (file paths, line numbers, commands). Flag anything material you could not verify.`

// pmPrompt: plans, assigns, and tracks — the coordinator counterpart to the
// teammate role. The AgentDeck section teaches the MCP messaging workflow
// (self-contained assignments, status via mail, budget awareness).
const pmPrompt = `You are a pm: you turn a goal into a concrete, sequenced plan and keep an honest, current picture of progress. You do not write the implementation yourself unless separately asked.

- Ground plans in the actual project: read the relevant code, docs, and similar past work first, so the plan reflects real constraints and conventions rather than a generic template.
- Break work into specific, actionable units, each with a clear definition of done and stated dependencies. Order by what must happen first; schedule risky or uncertain pieces early so problems surface while there is time to adapt. Call out assumptions, open questions, and decisions that belong to the human instead of quietly picking an answer.
- Report status plainly: done, in progress, blocked (and why), next. Don't round up, paper over slippage, or invent numbers you can't measure. Keep the plan current as reality diverges from it — a stale plan is worse than none.
- Use current AgentDeck operating guidance and tool contracts when assigning or coordinating work. Give each assignee a self-contained goal, scope boundary, dependencies, and definition of done.`

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
			SystemPrompt:    implementerPrompt,
			SkipPermissions: nil,
		},
		"reviewer": {
			Title:           "Reviewer",
			SystemPrompt:    reviewerPrompt,
			SkipPermissions: nil,
		},
		"researcher": {
			Title:           "Researcher",
			SystemPrompt:    researcherPrompt,
			SkipPermissions: nil,
		},
		"pm": {
			Title:           "PM",
			SystemPrompt:    pmPrompt,
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

// MigrateSupersededRolePrompts replaces only exact previously shipped seed
// prompts, for every seeded role. Callers gate this on verified skill
// availability (FS-04.R47, FS-18.R13).
func (s *Store) MigrateSupersededRolePrompts() (int, error) {
	return s.migrateSupersededRolePrompts(supersededRolePromptDigests)
}

// migrateSupersededRolePrompts treats each role as independently failable: one
// unreadable, undecodable, or unwritable role is reported and skipped rather
// than aborting the pass (INV §7, TS-11.R13). Roles are visited in sorted order
// so the reported errors and the write order are deterministic.
func (s *Store) migrateSupersededRolePrompts(digests map[string][]string) (int, error) {
	current := seedRoles()
	ids := make([]string, 0, len(digests))
	for id := range digests {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	migrated := 0
	var errs []error
	for _, id := range ids {
		seeded, ok := current[id]
		if !ok {
			errs = append(errs, fmt.Errorf("config: superseded prompt digest for unseeded role %q", id))
			continue
		}
		replaced, err := s.migrateRolePrompt(id, digests[id], seeded.SystemPrompt)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if replaced {
			migrated++
		}
	}
	return migrated, errors.Join(errs...)
}

func (s *Store) migrateRolePrompt(id string, supersededDigests []string, replacement string) (bool, error) {
	role, err := s.ReadRole(id)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	sum := sha256.Sum256([]byte(role.SystemPrompt))
	stored := hex.EncodeToString(sum[:])
	if !slices.Contains(supersededDigests, stored) {
		return false, nil
	}
	role.SystemPrompt = replacement
	if err := s.WriteRole(id, role); err != nil {
		return false, err
	}
	return true, nil
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
