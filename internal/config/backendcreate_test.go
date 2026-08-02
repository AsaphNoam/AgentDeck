package config

import (
	"os"
	"path/filepath"
	"testing"
)

// FS-04.A20 (R40) / FS-09.A20 (R47) — the canonical per-type starter backend an
// item-scoped create builds from, and the target-scoped add-only model import an
// enabled source connection performs.

func TestStarterBackendMatchesSeedAuthority(t *testing.T) {
	seed := DefaultBackends()
	for _, backendType := range []string{"claude-acp", "codex-acp", "opencode-acp", "openhands-acp"} {
		starter, ok := StarterBackend(backendType)
		if !ok {
			t.Fatalf("no starter for %s", backendType)
		}
		// The starter is the seed entry, so a created backend can never drift
		// from a fresh home's template.
		var seeded Backend
		for _, backend := range seed.Backends {
			if backend.Type == backendType {
				seeded = backend
			}
		}
		if starter.Name != seeded.Name || starter.DefaultModel != seeded.DefaultModel {
			t.Errorf("%s starter = %s/%s, want %s/%s", backendType,
				starter.Name, starter.DefaultModel, seeded.Name, seeded.DefaultModel)
		}
		if len(starter.Models) == 0 {
			t.Errorf("%s starter has no model", backendType)
		}
		if _, ok := starter.Models[starter.DefaultModel]; !ok {
			t.Errorf("%s starter default_model %q is not in its model map", backendType, starter.DefaultModel)
		}
		// Membership is the caller's decision, never the template's.
		if starter.Default {
			t.Errorf("%s starter must not claim catalog default", backendType)
		}
	}
	if _, ok := StarterBackend("not-a-backend"); ok {
		t.Error("unknown type produced a starter")
	}
}

func TestStarterBackendModelsAreNotShared(t *testing.T) {
	first, _ := StarterBackend("claude-acp")
	first.Models["injected"] = Model{Name: "Injected", Model: "injected"}
	second, _ := StarterBackend("claude-acp")
	if _, leaked := second.Models["injected"]; leaked {
		t.Error("starter model maps are shared between calls")
	}
}

// claudeHomeWithSettings points HOME at a private tree carrying user-level
// Claude settings, so the import reads a fixture rather than the real home.
func claudeHomeWithSettings(t *testing.T, body string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o700); err != nil {
		t.Fatalf("mkdir claude: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(body), 0o600); err != nil {
		t.Fatalf("write settings: %v", err)
	}
}

func TestImportConfiguredModelsIsTargetScopedAndAddOnly(t *testing.T) {
	claudeHomeWithSettings(t, `{"availableModels":["opus","sonnet"]}`)

	bc := BackendsConfig{Version: 2, Backends: map[string]Backend{
		"claude": {Type: "claude-acp", DefaultModel: "sonnet", Models: map[string]Model{
			"sonnet": {Name: "Mine", Model: "sonnet"},
		}},
		// A second opted-in Claude backend must NOT be touched: the import is
		// scoped to the connected target, never whole-catalog startup sync.
		"claude-other": {Type: "claude-acp", AutoSyncModels: true, DefaultModel: "sonnet", Models: map[string]Model{
			"sonnet": {Name: "Other", Model: "sonnet"},
		}},
	}}

	added := ImportConfiguredModels(&bc, "claude")
	if added != 1 {
		t.Fatalf("added = %d, want 1 (opus only; sonnet already represented)", added)
	}
	target := bc.Backends["claude"]
	if !target.AutoSyncModels {
		t.Error("connecting must enable continuing sync on the target")
	}
	if _, ok := target.Models["opus"]; !ok {
		t.Errorf("opus not imported: %+v", target.Models)
	}
	if target.Models["sonnet"].Name != "Mine" {
		t.Error("existing entry was overwritten")
	}
	if target.DefaultModel != "sonnet" {
		t.Error("default_model changed")
	}
	if other := bc.Backends["claude-other"]; len(other.Models) != 1 {
		t.Errorf("another opted-in backend was modified: %+v", other.Models)
	}
}

func TestImportConfiguredModelsMissingSourceIsZeroModelSuccess(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home) // no ~/.claude/settings.json at all
	t.Setenv("CODEX_HOME", filepath.Join(home, "absent-codex"))

	bc := BackendsConfig{Version: 2, Backends: map[string]Backend{
		"claude": {Type: "claude-acp", DefaultModel: "sonnet", Models: map[string]Model{"sonnet": {Name: "S", Model: "sonnet"}}},
		"codex":  {Type: "codex-acp", DefaultModel: "gpt-5.5", Models: map[string]Model{"gpt-5.5": {Name: "G", Model: "gpt-5.5"}}},
	}}
	for _, id := range []string{"claude", "codex"} {
		if added := ImportConfiguredModels(&bc, id); added != 0 {
			t.Errorf("%s added = %d, want 0", id, added)
		}
		// The connection still succeeds and still enables continuing sync; the
		// backend keeps its valid starter model.
		if !bc.Backends[id].AutoSyncModels {
			t.Errorf("%s sync not enabled", id)
		}
		if len(bc.Backends[id].Models) != 1 {
			t.Errorf("%s models changed: %+v", id, bc.Backends[id].Models)
		}
	}
}

func TestImportConfiguredModelsIgnoresUnfederatedTypeAndUnknownBackend(t *testing.T) {
	bc := BackendsConfig{Version: 2, Backends: map[string]Backend{
		"opencode": {Type: "opencode-acp", DefaultModel: "sonnet-4-5", Models: map[string]Model{"sonnet-4-5": {Name: "S", Model: "a/b"}}},
	}}
	if added := ImportConfiguredModels(&bc, "opencode"); added != 0 {
		t.Errorf("added = %d, want 0", added)
	}
	if bc.Backends["opencode"].AutoSyncModels {
		t.Error("a non-federated type must not gain autosync")
	}
	if added := ImportConfiguredModels(&bc, "absent"); added != 0 {
		t.Errorf("unknown backend added = %d, want 0", added)
	}
}
