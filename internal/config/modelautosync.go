package config

// Native model-catalog autosync (FS-09.R28/R45, TS-01.R14). One bounded startup
// import reads the backends snapshot once, invokes only the pure local catalog
// readers of the provider types that opted in, merges every successful candidate
// set add-only through one shared helper, and rewrites backends.json at most once.

// syncModels adds catalog entries to every backend of backendType that opted
// into AutoSyncModels, keyed by the catalog key. Add-only: it never overwrites an
// existing entry or changes default_model. When matchProvider is true a candidate
// whose key equals an existing entry's provider `model` string is also treated as
// already represented (Claude keys by selector, so a selector already used as a
// provider string must not become a duplicate entry). Returns true if any backend
// gained at least one model.
func syncModels(bc *BackendsConfig, backendType string, catalog map[string]Model, matchProvider bool) bool {
	changed := false
	for id, bk := range bc.Backends {
		if bk.Type != backendType || !bk.AutoSyncModels {
			continue
		}
		if addModels(&bk, catalog, matchProvider) > 0 {
			bc.Backends[id] = bk
			changed = true
		}
	}
	return changed
}

// addModels merges a catalog into one backend add-only and returns how many
// entries it actually added. It is the single merge rule shared by whole-catalog
// startup sync and the target-scoped import an enabled source bind performs.
func addModels(bk *Backend, catalog map[string]Model, matchProvider bool) int {
	if bk.Models == nil {
		bk.Models = map[string]Model{}
	}
	represented := make(map[string]bool, len(bk.Models))
	for key, m := range bk.Models {
		represented[key] = true
		if matchProvider && m.Model != "" {
			represented[m.Model] = true
		}
	}
	added := 0
	for key, model := range catalog {
		if represented[key] {
			continue // never overwrite a user-owned entry
		}
		bk.Models[key] = model
		represented[key] = true
		added++
	}
	return added
}

// ImportConfiguredModels enables autosync on exactly one backend and immediately
// performs that provider's add-only import into it (FS-09.R47, TS-07.R17). It
// reuses the same local readers and merge rules as startup sync rather than
// parsing native files a second way, changes no other backend, no default, and
// no existing entry, and reports a missing/unreadable/invalid local catalog as a
// zero-model success. It returns how many models were added.
func ImportConfiguredModels(bc *BackendsConfig, backendID string) int {
	backend, ok := bc.Backends[backendID]
	if !ok {
		return 0
	}
	if backend.Type != "codex-acp" && backend.Type != "claude-acp" {
		return 0
	}
	catalog, matchProvider, _ := readProviderCatalog(backend.Type)
	backend.AutoSyncModels = true
	added := addModels(&backend, catalog, matchProvider)
	bc.Backends[backendID] = backend
	return added
}

// readProviderCatalog is the one local candidate reader per provider type:
// Codex's personal model cache, or Claude's configured user-level selectors
// (which key by selector, so matchProvider). It never queries a provider or
// the network.
func readProviderCatalog(backendType string) (catalog map[string]Model, matchProvider bool, err error) {
	switch backendType {
	case "codex-acp":
		catalog, err = ReadCodexModelCatalog(CodexModelCatalogPath())
		return catalog, false, err
	case "claude-acp":
		catalog, err = ReadClaudeConfiguredModels(ClaudeSettingsPath())
		return catalog, true, err
	}
	return nil, false, nil
}

// RefreshBackendModels runs one opted-in backend's add-only import for an
// explicit Refresh provider (FS-09.R72, TS-04.R73). It never enables autosync,
// changes defaults or edits existing entries. enabled reports opt-in;
// available reports whether the local candidate source could be read.
func RefreshBackendModels(bc *BackendsConfig, backendID string) (enabled, available bool, added int) {
	backend, ok := bc.Backends[backendID]
	if !ok || !backend.AutoSyncModels {
		return false, false, 0
	}
	catalog, matchProvider, err := readProviderCatalog(backend.Type)
	if err != nil {
		return true, false, 0
	}
	added = addModels(&backend, catalog, matchProvider)
	bc.Backends[backendID] = backend
	return true, true, added
}

// AutoSyncBackends imports configured provider models into opted-in backends on
// dashboard startup and persists backends.json only when a model was added. It is
// best-effort: a missing/unreadable/unparseable source, or no opted-in backend of
// a type, is a silent no-op that never blocks startup, and one provider's failed
// read never suppresses another provider's valid additions.
func (s *Store) AutoSyncBackends() error {
	bc, err := s.ReadBackends()
	if err != nil {
		return nil // corrupt/absent catalog is handled by seeding/fallback elsewhere
	}

	codexOptedIn, claudeOptedIn := false, false
	for _, bk := range bc.Backends {
		if !bk.AutoSyncModels {
			continue
		}
		switch bk.Type {
		case "codex-acp":
			codexOptedIn = true
		case "claude-acp":
			claudeOptedIn = true
		}
	}

	changed := false
	if codexOptedIn {
		if catalog, err := ReadCodexModelCatalog(CodexModelCatalogPath()); err == nil {
			if syncModels(&bc, "codex-acp", catalog, false) {
				changed = true
			}
		}
	}
	if claudeOptedIn {
		if catalog, err := ReadClaudeConfiguredModels(ClaudeSettingsPath()); err == nil {
			if syncModels(&bc, "claude-acp", catalog, true) {
				changed = true
			}
		}
	}

	if changed {
		return s.WriteBackends(bc)
	}
	return nil
}
