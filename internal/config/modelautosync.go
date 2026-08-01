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
		added := false
		for key, model := range catalog {
			if represented[key] {
				continue // never overwrite a user-owned entry
			}
			bk.Models[key] = model
			represented[key] = true
			added = true
		}
		if added {
			bc.Backends[id] = bk
			changed = true
		}
	}
	return changed
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
