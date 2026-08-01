package config

import (
	"os"
	"path/filepath"
	"testing"
)

// FS-09.A18 (R45) / A19 (R46) — Claude configured-model autosync and seed.

func writeClaudeSettings(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write settings: %v", err)
	}
	return path
}

func TestReadClaudeConfiguredModels(t *testing.T) {
	const fixture = `{
	  "model": "sonnet",
	  "availableModels": ["opus", "claude-sonnet-4-6", "sonnet"],
	  "fallbackModel": ["haiku"],
	  "permissions": {"allow": ["Bash"]},
	  "unrelated": "ignored"
	}`
	cat, err := ReadClaudeConfiguredModels(writeClaudeSettings(t, fixture))
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	// sonnet (deduped across model + availableModels), opus, claude-sonnet-4-6, haiku.
	if len(cat) != 4 {
		t.Fatalf("catalog size = %d, want 4 (got %+v)", len(cat), cat)
	}
	// Alias labels are friendly; the exact selector is both key and provider string.
	if m := cat["sonnet"]; m.Name != "Claude Sonnet" || m.Model != "sonnet" {
		t.Errorf("sonnet entry = %+v, want Claude Sonnet/sonnet", m)
	}
	if m := cat["haiku"]; m.Name != "Claude Haiku" || m.Model != "haiku" {
		t.Errorf("haiku entry = %+v, want Claude Haiku/haiku", m)
	}
	// A non-alias selector labels itself.
	if m := cat["claude-sonnet-4-6"]; m.Name != "claude-sonnet-4-6" || m.Model != "claude-sonnet-4-6" {
		t.Errorf("pinned entry = %+v, want self-labelled", m)
	}
	// No inferred effort levels, and [] not nil (INV §11).
	if m := cat["opus"]; m.Efforts == nil || len(m.Efforts) != 0 || m.DefaultEffort != "" {
		t.Errorf("opus efforts = %+v, want non-nil empty with no default", m.Efforts)
	}
	// Unrelated source content never becomes a model.
	if _, ok := cat["ignored"]; ok {
		t.Error("unrelated field leaked into catalog")
	}
}

func TestReadClaudeConfiguredModelsLegacyFallbackAndFiltering(t *testing.T) {
	// Older singular-string fallbackModel; blank and bracketed selectors dropped.
	const fixture = `{
	  "model": "",
	  "availableModels": ["opus", "sonnet[high]", ""],
	  "fallbackModel": "haiku"
	}`
	cat, err := ReadClaudeConfiguredModels(writeClaudeSettings(t, fixture))
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	if len(cat) != 2 {
		t.Fatalf("catalog size = %d, want 2 (opus, haiku); got %+v", len(cat), cat)
	}
	if _, ok := cat["opus"]; !ok {
		t.Error("opus missing")
	}
	if _, ok := cat["haiku"]; !ok {
		t.Error("legacy singular fallbackModel not imported")
	}
	if _, ok := cat["sonnet[high]"]; ok {
		t.Error("bracketed selector must be rejected")
	}
}

func TestReadClaudeConfiguredModelsSkipsBadFileAndShape(t *testing.T) {
	// Missing file.
	if _, err := ReadClaudeConfiguredModels(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Error("expected error for missing settings")
	}
	// Malformed JSON.
	if _, err := ReadClaudeConfiguredModels(writeClaudeSettings(t, "{ not json")); err == nil {
		t.Error("expected error for malformed settings")
	}
	// Wrong shape: numeric model.
	if _, err := ReadClaudeConfiguredModels(writeClaudeSettings(t, `{"model": 42}`)); err == nil {
		t.Error("expected error for numeric model")
	}
	// Wrong shape: scalar availableModels.
	if _, err := ReadClaudeConfiguredModels(writeClaudeSettings(t, `{"availableModels": "sonnet"}`)); err == nil {
		t.Error("expected error for scalar availableModels")
	}
	// Wrong shape: numeric fallbackModel.
	if _, err := ReadClaudeConfiguredModels(writeClaudeSettings(t, `{"fallbackModel": 3}`)); err == nil {
		t.Error("expected error for numeric fallbackModel")
	}
}

func TestSyncClaudeModelsPreservesExistingKeyAndProvider(t *testing.T) {
	bc := BackendsConfig{Version: 2, Backends: map[string]Backend{
		"claude": {
			Type: "claude-acp", AutoSyncModels: true, DefaultModel: "sonnet",
			Models: map[string]Model{
				// Existing entry keyed differently from its provider string.
				"my-pin": {Name: "My Pin", Model: "sonnet"},
			},
		},
	}}
	catalog := map[string]Model{
		"sonnet": {Name: "Claude Sonnet", Model: "sonnet"}, // already represented by provider string
		"opus":   {Name: "Claude Opus", Model: "opus"},     // genuinely new
	}
	if !syncClaudeModels(&bc, catalog) {
		t.Fatal("expected changed=true (opus is new)")
	}
	got := bc.Backends["claude"].Models
	if _, ok := got["sonnet"]; ok {
		t.Error("selector already used as a provider string must not become a duplicate entry")
	}
	if _, ok := got["opus"]; !ok {
		t.Error("new selector not added")
	}
	if got["my-pin"].Name != "My Pin" {
		t.Errorf("existing entry overwritten: %+v", got["my-pin"])
	}
	if bc.Backends["claude"].DefaultModel != "sonnet" {
		t.Error("default_model changed")
	}
}

func TestSyncClaudeModelsRespectsFlagAndType(t *testing.T) {
	catalog := map[string]Model{"opus": {Name: "Claude Opus", Model: "opus"}}

	off := BackendsConfig{Version: 2, Backends: map[string]Backend{
		"claude": {Type: "claude-acp", AutoSyncModels: false, Models: map[string]Model{}},
	}}
	if syncClaudeModels(&off, catalog) || len(off.Backends["claude"].Models) != 0 {
		t.Error("flag-off claude backend must be untouched")
	}

	codex := BackendsConfig{Version: 2, Backends: map[string]Backend{
		"codex": {Type: "codex-acp", AutoSyncModels: true, Models: map[string]Model{}},
	}}
	if syncClaudeModels(&codex, catalog) || len(codex.Backends["codex"].Models) != 0 {
		t.Error("non-claude backend must be untouched by the Claude source")
	}
}

func TestAutoSyncBackendsCombinesProviders(t *testing.T) {
	// Point both providers' sources at a private fake home; one rewrite persists
	// additions from both (FS-09.A18 combined-provider persistence).
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o700); err != nil {
		t.Fatalf("mkdir codex: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o700); err != nil {
		t.Fatalf("mkdir claude: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".codex", "models_cache.json"), []byte(codexCacheFixture), 0o600); err != nil {
		t.Fatalf("write codex cache: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(`{"availableModels":["opus"]}`), 0o600); err != nil {
		t.Fatalf("write claude settings: %v", err)
	}

	store := newTestStore(t)
	bc := BackendsConfig{Version: 2, Backends: map[string]Backend{
		"codex":  {Type: "codex-acp", AutoSyncModels: true, DefaultModel: "gpt-4o", Models: map[string]Model{"gpt-4o": {Name: "GPT-4o", Model: "gpt-4o"}}},
		"claude": {Type: "claude-acp", AutoSyncModels: true, DefaultModel: "sonnet", Models: map[string]Model{"sonnet": {Name: "Claude Sonnet", Model: "sonnet"}}},
	}}
	if err := store.WriteBackends(bc); err != nil {
		t.Fatalf("seed backends: %v", err)
	}
	if err := store.AutoSyncBackends(); err != nil {
		t.Fatalf("AutoSyncBackends: %v", err)
	}
	after, err := store.ReadBackends()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if _, ok := after.Backends["codex"].Models["gpt-5.6-sol"]; !ok {
		t.Errorf("codex model not synced: %v", after.Backends["codex"].Models)
	}
	if _, ok := after.Backends["claude"].Models["opus"]; !ok {
		t.Errorf("claude model not imported: %v", after.Backends["claude"].Models)
	}
}

func TestSeedClaudeFamilyAliases(t *testing.T) {
	s := newTestStore(t)
	if err := s.SeedIfAbsent(); err != nil {
		t.Fatalf("SeedIfAbsent: %v", err)
	}
	fresh, err := s.ReadBackends()
	if err != nil {
		t.Fatalf("ReadBackends: %v", err)
	}
	claude := fresh.Backends["claude"]
	if claude.DefaultModel != "sonnet" {
		t.Errorf("claude default = %q, want sonnet", claude.DefaultModel)
	}
	want := map[string]string{"fable": "Claude Fable", "opus": "Claude Opus", "sonnet": "Claude Sonnet", "haiku": "Claude Haiku"}
	if len(claude.Models) != len(want) {
		t.Fatalf("claude has %d models, want exactly 4: %+v", len(claude.Models), claude.Models)
	}
	for id, label := range want {
		m, ok := claude.Models[id]
		if !ok {
			t.Errorf("missing family alias %q", id)
			continue
		}
		if m.Name != label || m.Model != id {
			t.Errorf("alias %q = %+v, want %s/%s", id, m, label, id)
		}
	}
}
