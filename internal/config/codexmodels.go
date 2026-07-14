package config

// Codex model-catalog autosync (FS-09.R28). A codex-acp backend flagged
// AutoSyncModels gains newly available models from the Codex CLI's local cache
// on dashboard startup. The merge is strictly add-only: it never edits or
// removes an existing model entry and never changes default_model.

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// codexModelsCache is the subset of models_cache.json we consume; the file has
// many more fields per model that are irrelevant to the AgentDeck catalog.
type codexModelsCache struct {
	Models []codexModelEntry `json:"models"`
}

type codexModelEntry struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
	Visibility  string `json:"visibility"` // "list" = user-selectable; "hide" = internal
}

// CodexModelCatalogPath returns the Codex CLI model cache path, honoring
// CODEX_HOME (matching the federation resolver) and falling back to ~/.codex.
func CodexModelCatalogPath() string {
	home := os.Getenv("CODEX_HOME")
	if home == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		home = filepath.Join(userHome, ".codex")
	}
	return filepath.Join(home, "models_cache.json")
}

// ReadCodexModelCatalog reads the Codex model cache at path and returns the
// user-selectable models (visibility "list") as AgentDeck Model entries keyed by
// the Codex slug — the slug is both the AgentDeck model id and the provider
// string. A missing or unparseable file returns an error; callers treat that as
// a non-fatal skip.
func ReadCodexModelCatalog(path string) (map[string]Model, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cache codexModelsCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}
	out := make(map[string]Model, len(cache.Models))
	for _, m := range cache.Models {
		if m.Slug == "" || m.Visibility != "list" {
			continue
		}
		name := m.DisplayName
		if name == "" {
			name = m.Slug
		}
		out[m.Slug] = Model{Name: name, Model: m.Slug}
	}
	return out, nil
}

// syncCodexModels merges catalog into every codex-acp backend that opted in via
// AutoSyncModels. Add-only: an existing model id is left exactly as the user had
// it, and default_model is never touched. Returns true if any backend gained at
// least one model (so the caller knows whether to persist).
func syncCodexModels(bc *BackendsConfig, catalog map[string]Model) bool {
	changed := false
	for id, bk := range bc.Backends {
		if bk.Type != "codex-acp" || !bk.AutoSyncModels {
			continue
		}
		if bk.Models == nil {
			bk.Models = map[string]Model{}
		}
		added := false
		for slug, model := range catalog {
			if _, exists := bk.Models[slug]; exists {
				continue // never overwrite a user-owned entry
			}
			bk.Models[slug] = model
			added = true
		}
		if added {
			bc.Backends[id] = bk
			changed = true
		}
	}
	return changed
}

// AutoSyncBackends refreshes opted-in codex-acp backends from the Codex CLI model
// cache (FS-09.R28) and persists backends.json only when a model was added. It is
// best-effort: a missing/unreadable/unparseable cache, or no opted-in backend, is
// a silent no-op that never blocks startup.
func (s *Store) AutoSyncBackends() error {
	bc, err := s.ReadBackends()
	if err != nil {
		return nil // corrupt/absent catalog is handled by seeding/fallback elsewhere
	}
	optedIn := false
	for _, bk := range bc.Backends {
		if bk.Type == "codex-acp" && bk.AutoSyncModels {
			optedIn = true
			break
		}
	}
	if !optedIn {
		return nil
	}
	catalog, err := ReadCodexModelCatalog(CodexModelCatalogPath())
	if err != nil {
		return nil // best-effort: no cache to sync from
	}
	if syncCodexModels(&bc, catalog) {
		return s.WriteBackends(bc)
	}
	return nil
}
