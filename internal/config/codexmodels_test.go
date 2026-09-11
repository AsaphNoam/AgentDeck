package config

import (
	"os"
	"path/filepath"
	"testing"
)

// FS-09.A8 (R28) — Codex model-catalog autosync.

const codexCacheFixture = `{
  "fetched_at": "2026-07-14T00:00:00Z",
  "etag": "abc",
  "client_version": "0.153.4",
  "models": [
    {"slug": "gpt-5.6-sol",  "display_name": "GPT-5.6-Sol",  "visibility": "list"},
    {"slug": "gpt-5.5",      "display_name": "GPT-5.5",       "visibility": "list"},
    {"slug": "codex-auto-review", "display_name": "Codex Auto Review", "visibility": "hide"},
    {"slug": "", "display_name": "Nameless", "visibility": "list"}
  ]
}`

func writeCache(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "models_cache.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

// FS-09.A29: a packaged runtime must not import models advertised by a cache
// produced by a different Codex version.
func TestReadCodexModelCatalogRequiresPackagedRuntimeVersion(t *testing.T) {
	t.Setenv("AGENTDECK_CODEX_VERSION", "0.153.4")
	if _, err := ReadCodexModelCatalog(writeCache(t, codexCacheFixture)); err != nil {
		t.Fatalf("matching cache rejected: %v", err)
	}
	t.Setenv("AGENTDECK_CODEX_VERSION", "0.144.0")
	if _, err := ReadCodexModelCatalog(writeCache(t, codexCacheFixture)); err == nil {
		t.Fatal("newer personal cache must not be imported by the older packaged runtime")
	}
}

func TestCurrentCodexRuntimeReportsMismatch(t *testing.T) {
	cache := writeCache(t, codexCacheFixture)
	t.Setenv("CODEX_HOME", filepath.Dir(cache))
	t.Setenv("CODEX_PATH", "/private/runtime/codex")
	t.Setenv("AGENTDECK_CODEX_VERSION", "0.144.0")
	got := CurrentCodexRuntime()
	if got.Path != "/private/runtime/codex" || got.Version != "0.144.0" || got.CacheVersion != "0.153.4" || got.CatalogStatus != "mismatch" {
		t.Fatalf("runtime info = %+v", got)
	}
}

func TestReadCodexModelCatalog(t *testing.T) {
	cat, err := ReadCodexModelCatalog(writeCache(t, codexCacheFixture))
	if err != nil {
		t.Fatalf("read catalog: %v", err)
	}
	// Only the two visibility:"list" entries with a non-empty slug survive.
	if len(cat) != 2 {
		t.Fatalf("catalog size = %d, want 2 (got %v)", len(cat), cat)
	}
	if m, ok := cat["gpt-5.6-sol"]; !ok || m.Name != "GPT-5.6-Sol" || m.Model != "gpt-5.6-sol" {
		t.Fatalf("gpt-5.6-sol entry wrong: %+v (ok=%v)", cat["gpt-5.6-sol"], ok)
	}
	if _, ok := cat["codex-auto-review"]; ok {
		t.Error("hidden model must be excluded")
	}

	// A missing file is a non-fatal error (caller skips).
	if _, err := ReadCodexModelCatalog(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Error("expected error for missing cache")
	}
}

// TestReadCodexModelCatalogImportsEfforts guards FS-09.R38/A14: a synced model's
// effort levels and default come from the cache's supported_reasoning_levels /
// default_reasoning_level, an entry without levels contributes none, and a
// default outside the declared levels is dropped rather than imported invalid.
func TestReadCodexModelCatalogImportsEfforts(t *testing.T) {
	const fixture = `{
	  "models": [
	    {"slug": "gpt-eff", "display_name": "GPT Eff", "visibility": "list",
	     "supported_reasoning_levels": [{"effort": "low"}, {"effort": "high"}],
	     "default_reasoning_level": "high", "additional_speed_tiers": ["fast"]},
	    {"slug": "gpt-none", "display_name": "GPT None", "visibility": "list"},
	    {"slug": "gpt-baddefault", "display_name": "GPT Bad", "visibility": "list",
	     "supported_reasoning_levels": [{"effort": "low"}],
	     "default_reasoning_level": "extreme"}
	  ]
	}`
	cat, err := ReadCodexModelCatalog(writeCache(t, fixture))
	if err != nil {
		t.Fatalf("read catalog: %v", err)
	}
	if got := cat["gpt-eff"]; len(got.Efforts) != 2 || got.Efforts[0] != "low" || got.Efforts[1] != "high" || got.DefaultEffort != "high" || !got.Fast {
		t.Fatalf("gpt-eff capabilities = %+v, want [low high], default high, and fast", got)
	}
	if got := cat["gpt-none"]; len(got.Efforts) != 0 || got.DefaultEffort != "" || got.Fast {
		t.Fatalf("gpt-none efforts = %+v, want none", got)
	}
	if got := cat["gpt-baddefault"]; got.DefaultEffort != "" {
		t.Fatalf("gpt-baddefault default_effort = %q, want dropped (not in levels)", got.DefaultEffort)
	}
}

func TestSyncCodexModelsAddsVisibleModels(t *testing.T) {
	bc := BackendsConfig{Version: 2, Backends: map[string]Backend{
		"codex": {
			Type: "codex-acp", AutoSyncModels: true, DefaultModel: "gpt-4o",
			Models: map[string]Model{"gpt-4o": {Name: "GPT-4o", Model: "gpt-4o"}},
		},
	}}
	catalog := map[string]Model{
		"gpt-5.6-sol": {Name: "GPT-5.6-Sol", Model: "gpt-5.6-sol"},
		"gpt-5.5":     {Name: "GPT-5.5", Model: "gpt-5.5"},
	}
	if !syncCodexModels(&bc, catalog) {
		t.Fatal("expected changed=true")
	}
	got := bc.Backends["codex"].Models
	for _, want := range []string{"gpt-4o", "gpt-5.6-sol", "gpt-5.5"} {
		if _, ok := got[want]; !ok {
			t.Errorf("model %q missing after sync: %v", want, got)
		}
	}
}

func TestSyncCodexModelsPreservesExistingAndDefault(t *testing.T) {
	bc := BackendsConfig{Version: 2, Backends: map[string]Backend{
		"codex": {
			Type: "codex-acp", AutoSyncModels: true, DefaultModel: "gpt-5.5",
			// User hand-edited gpt-5.5's label; sync must not clobber it.
			Models: map[string]Model{"gpt-5.5": {Name: "My Custom Name", Model: "gpt-5.5"}},
		},
	}}
	catalog := map[string]Model{"gpt-5.5": {Name: "GPT-5.5", Model: "gpt-5.5"}}
	if syncCodexModels(&bc, catalog) {
		t.Fatal("expected changed=false: nothing new to add")
	}
	bk := bc.Backends["codex"]
	if bk.Models["gpt-5.5"].Name != "My Custom Name" {
		t.Errorf("existing entry overwritten: %+v", bk.Models["gpt-5.5"])
	}
	if bk.DefaultModel != "gpt-5.5" {
		t.Errorf("default_model changed to %q", bk.DefaultModel)
	}
}

func TestSyncCodexModelsRespectsFlagAndType(t *testing.T) {
	catalog := map[string]Model{"gpt-5.5": {Name: "GPT-5.5", Model: "gpt-5.5"}}

	// Flag off → untouched.
	off := BackendsConfig{Version: 2, Backends: map[string]Backend{
		"codex": {Type: "codex-acp", AutoSyncModels: false, Models: map[string]Model{}},
	}}
	if syncCodexModels(&off, catalog) || len(off.Backends["codex"].Models) != 0 {
		t.Error("flag-off codex backend must be untouched")
	}

	// Wrong type with flag on → untouched (flag is codex-only).
	claude := BackendsConfig{Version: 2, Backends: map[string]Backend{
		"claude": {Type: "claude-acp", AutoSyncModels: true, Models: map[string]Model{}},
	}}
	if syncCodexModels(&claude, catalog) || len(claude.Backends["claude"].Models) != 0 {
		t.Error("non-codex backend must be untouched")
	}
}

func TestAutoSyncBackendsPersistsOnlyWhenChanged(t *testing.T) {
	store := newTestStore(t)
	bc := BackendsConfig{Version: 2, Backends: map[string]Backend{
		"codex": {
			Type: "codex-acp", AutoSyncModels: true, DefaultModel: "gpt-4o",
			Models: map[string]Model{"gpt-4o": {Name: "GPT-4o", Model: "gpt-4o"}},
		},
	}}
	if err := store.WriteBackends(bc); err != nil {
		t.Fatalf("seed backends: %v", err)
	}
	t.Setenv("CODEX_HOME", filepath.Dir(writeCache(t, codexCacheFixture)))

	if err := store.AutoSyncBackends(); err != nil {
		t.Fatalf("AutoSyncBackends: %v", err)
	}
	after, err := store.ReadBackends()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if _, ok := after.Backends["codex"].Models["gpt-5.6-sol"]; !ok {
		t.Errorf("expected synced model persisted, got %v", after.Backends["codex"].Models)
	}
}

func TestAutoSyncBackendsSkipsNewerCacheForPackagedRuntime(t *testing.T) {
	store := newTestStore(t)
	bc := BackendsConfig{Version: 2, Backends: map[string]Backend{
		"codex": {Type: "codex-acp", AutoSyncModels: true, DefaultModel: "gpt-4o", Models: map[string]Model{"gpt-4o": {Name: "GPT-4o", Model: "gpt-4o"}}},
	}}
	if err := store.WriteBackends(bc); err != nil {
		t.Fatalf("seed backends: %v", err)
	}
	t.Setenv("CODEX_HOME", filepath.Dir(writeCache(t, codexCacheFixture)))
	t.Setenv("AGENTDECK_CODEX_VERSION", "0.144.0")
	if err := store.AutoSyncBackends(); err != nil {
		t.Fatalf("mismatch must remain non-blocking: %v", err)
	}
	after, err := store.ReadBackends()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := after.Backends["codex"].Models["gpt-5.6-sol"]; ok {
		t.Fatal("newer-cache model was imported for the older packaged runtime")
	}
}
