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
	}
}

// DefaultLayout is the seeded/fallback layout.json.
func DefaultLayout() Layout {
	return Layout{
		Order:   []string{},
		Density: Density{CardsPerRow: 3, Gap: 16},
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

// seedRoles is the 4 default roles (tech spec §5.4). SkipPermissions is nil
// (null on disk) so each role inherits the global config by default.
func seedRoles() map[string]Role {
	return map[string]Role{
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
