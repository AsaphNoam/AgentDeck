package config

// Claude configured-model autosync (FS-09.R45). A claude-acp backend flagged
// AutoSyncModels gains the model selectors the user explicitly named in their
// user-level Claude settings on dashboard startup. Unlike Codex (R28), Claude
// exposes no local full-catalog cache, so this imports only configured
// selectors and never discovers effort capability. The merge is strictly
// add-only, sharing modelautosync.go's helper.

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// claudeSettings is the subset of ~/.claude/settings.json we consume: the three
// user-level model-selector sources. The file carries many more fields, all of
// which we ignore. Decoding into typed fields also rejects a wrong shape (e.g. a
// numeric `model` or a scalar `availableModels`) before any candidate returns.
type claudeSettings struct {
	Model           string          `json:"model"`
	AvailableModels []string        `json:"availableModels"`
	FallbackModel   json.RawMessage `json:"fallbackModel"`
}

// claudeAliasLabels give the four portable family aliases friendly labels; any
// other selector uses itself as its label because the settings file carries no
// display metadata (FS-09.R45).
var claudeAliasLabels = map[string]string{
	"fable":  "Claude Fable",
	"opus":   "Claude Opus",
	"sonnet": "Claude Sonnet",
	"haiku":  "Claude Haiku",
}

// ClaudeSettingsPath returns the fixed user-level Claude settings path. The sync
// deliberately reads only this file, never project/local/managed settings, the
// executable, private caches, the environment, or the network (FS-09 §6).
func ClaudeSettingsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "settings.json")
}

// ReadClaudeConfiguredModels reads the user-level Claude settings at path and
// returns its configured model selectors as AgentDeck Model entries, each keyed
// by and carrying the exact selector. It collects `model`, every `availableModels`
// entry, and every `fallbackModel` entry (tolerating the older singular string
// shape). Only distinct, non-empty selectors that pass the shared model-string
// validation survive, and none gain inferred effort levels. A missing, unreadable,
// malformed, or shape-invalid file returns an error; callers treat that as a
// non-fatal skip.
func ReadClaudeConfiguredModels(path string) (map[string]Model, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s claudeSettings
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	fallbacks, err := decodeClaudeFallback(s.FallbackModel)
	if err != nil {
		return nil, err
	}
	selectors := make([]string, 0, 1+len(s.AvailableModels)+len(fallbacks))
	selectors = append(selectors, s.Model)
	selectors = append(selectors, s.AvailableModels...)
	selectors = append(selectors, fallbacks...)

	out := make(map[string]Model)
	for _, sel := range selectors {
		if !validModelString(sel) {
			continue
		}
		if _, seen := out[sel]; seen {
			continue // distinct selectors only
		}
		out[sel] = Model{Name: claudeModelLabel(sel), Model: sel, Efforts: []string{}}
	}
	return out, nil
}

// decodeClaudeFallback accepts the array-of-strings `fallbackModel` shape and
// the older singular string shape, and returns an error for any other shape so
// the caller skips a wrong-shaped file rather than importing a partial catalog.
func decodeClaudeFallback(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, nil
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return []string{single}, nil
	}
	return nil, errors.New("config: fallbackModel must be a string array or string")
}

// syncClaudeModels merges catalog into every claude-acp backend that opted in
// via AutoSyncModels. Add-only, matching provider strings as well as map keys:
// Claude keys models by the exact selector, so a selector already carried by an
// existing entry's provider `model` string must not become a duplicate entry
// (FS-09.R45). Returns true if any backend gained at least one model.
func syncClaudeModels(bc *BackendsConfig, catalog map[string]Model) bool {
	return syncModels(bc, "claude-acp", catalog, true)
}

// claudeModelLabel returns the friendly label for a family alias, or the
// selector itself for any other value.
func claudeModelLabel(selector string) string {
	if label, ok := claudeAliasLabels[selector]; ok {
		return label
	}
	return selector
}
