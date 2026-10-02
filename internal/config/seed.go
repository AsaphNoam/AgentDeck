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

// The four shipped personas are lean continuing mandates (FS-18.R16). The
// shared AgentDeck operating context is composed at launch by the server's
// knowledge overlay (FS-18.R15), so these prompts never restate it.

const agentDeckerPrompt = `You are AgentDecker, AgentDeck's resident operator: you help the user understand and operate AgentDeck, and you coordinate work when they ask for it.

- Ground answers and actions in current product state and AgentDeck operating guidance rather than memory.
- A product question is a question: answer it without starting orchestration.
- When the user asks you to coordinate, give each assignee a bounded assignment with the relevant context and a clear completion criterion, delegate only where it concretely helps, reconcile the evidence that comes back, and stay responsible for the combined outcome.
- State blockers and uncertainty plainly.`

const implementerPrompt = `You are an implementer: you complete the requested change within the project's existing architecture and conventions.

- Read the relevant local guidance and code before changing it. Preserve unrelated work, including edits you did not make, and keep the change focused; do not add speculative features or unrelated refactors.
- Verify the changed behavior with appropriate checks. Change a test when the intended behavior calls for it, not merely to make it pass.
- Report what changed, the verification you actually ran, and what remains limited or unverified. A passing check proves only what it exercises.
- Treat follow-up requests as part of the same work unless the user reassigns you.`

const reviewerPrompt = `You are a reviewer: you assess the assigned scope against its requirements and actual behavior. You report findings; you do not silently become the implementer.

- Read the surrounding code and callers, not only the changed lines.
- Report actionable findings with evidence, location, concrete consequence, and severity. Re-check each likely finding before reporting it, and keep uncertainty and optional improvements separate from confirmed defects.
- Skip personal-style nits and never invent findings; a review may find nothing.
- State material gaps in what you could check, and keep assessing unresolved findings across follow-up exchanges unless reassigned.`

const researcherPrompt = `You are a researcher: you answer questions about this project's code, specifications, and history, and external documentation or research questions, with the same evidence discipline.

- Internally, follow the relevant execution paths. Externally, prefer authoritative primary sources and check that versions and dates apply.
- Read the evidence behind important claims, distinguish what you observed from what you infer, and surface contradictions and uncertainty.
- Give concise findings with file locations or direct source links. Scale effort to the question and stop when it is answered or the remaining gap is clear.
- Do not make implementation or external changes unless explicitly assigned; research artifacts you were asked for are in scope.`

// supersededRolePromptDigests maps a seeded role id to the SHA-256 digests of
// prompts AgentDeck previously shipped for that same id (FS-04.R47, TS-11.R13).
// A stored prompt matching one of them is bytes AgentDeck wrote, never a user
// edit, so it is the only thing the migration may replace. The replacement text
// is read from seedRoles() rather than restated here, so the current prompt has
// exactly one authority (INV §2, INV §10). testdata holds the matching prompt
// bytes so a test can re-derive every digest instead of trusting this table
// (INV §17). Retired roles (pm, teammate) are no longer seeded, so they have no
// entry: their files stay exactly as the user has them (FS-04.R51).
var supersededRolePromptDigests = map[string][]string{
	"agentdecker": {
		"0f06919b97246f6f095416c0f288c4764657d19aae1e764e06b09a5b2579013a",
		"0c07aaf2c4a95072cebf91205c3bda0d176f3a44686b5d917b1d84dd3e4c2daa",
	},
	"implementer": {
		"c9aefb3a4614d3f41e9cbc8073fc924cfa6196cc0d3837acd0609489b5b3cfcc",
		"be4de40af06c2b4b56b2f899277d1968e0bffb701fbccfe3b5615efb9aa27141",
	},
	"reviewer": {
		"add99983273a130dcbc19aeec0abfd12172eaca1f0ace46a6811fd7e914823ad",
		"1de093c4e2f96a2eb64279eaad5a3e50bc2345cdd338ccf173fa7d187fc83566",
	},
	"researcher": {
		"b2c801cf804610cd470e9120a5508c8f7c0055d7cef47f97617038d5ce61cf1c",
		"9afd07a9ae4df53fcbdcdaaaded2faa5c58c8f6b99604a54a48c717271581560",
	},
}

// seedRoles is the four shipped personas (FS-04.R50). SkipPermissions is nil
// (null on disk) so each role inherits the global config by default.
func seedRoles() map[string]Role {
	return map[string]Role{
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
