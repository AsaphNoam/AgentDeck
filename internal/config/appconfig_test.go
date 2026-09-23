package config

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
)

func TestAppearanceSkinConfigPersistence(t *testing.T) {
	s := newTestStore(t)

	// FS-04.A18: an older version-1 file without the additive field is Core
	// and is not rewritten merely by reading it.
	older := []byte("{\"version\":1}\n")
	if err := os.WriteFile(s.configPath(), older, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := s.ReadConfig()
	if err != nil {
		t.Fatalf("ReadConfig older file: %v", err)
	}
	if cfg.AppearanceSkin != "" {
		t.Fatalf("older config appearance skin = %q, want Core", cfg.AppearanceSkin)
	}
	if raw, err := os.ReadFile(s.configPath()); err != nil {
		t.Fatal(err)
	} else if string(raw) != string(older) {
		t.Fatalf("reading older config rewrote it: got %q want %q", raw, older)
	}

	// The selected skin survives rebuilding the store, which is the config
	// persistence boundary crossed by a server restart.
	cfg = DefaultConfig()
	cfg.AppearanceSkin = AppearanceSkinSkyGrove
	if err := s.WriteConfig(cfg); err != nil {
		t.Fatalf("WriteConfig Sky & Grove: %v", err)
	}
	restarted := NewWithHome(s.Home())
	got, err := restarted.ReadConfig()
	if err != nil {
		t.Fatalf("ReadConfig after restart: %v", err)
	}
	if got.AppearanceSkin != AppearanceSkinSkyGrove {
		t.Fatalf("appearance skin after restart = %q, want %q", got.AppearanceSkin, AppearanceSkinSkyGrove)
	}

	// Core is represented by omission, not an empty persisted manifest value.
	got.AppearanceSkin = ""
	if err := restarted.WriteConfig(got); err != nil {
		t.Fatalf("WriteConfig Core: %v", err)
	}
	raw, err := os.ReadFile(s.configPath())
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if _, ok := doc["appearance_skin"]; ok {
		t.Fatalf("Core rewrite retained appearance_skin: %s", raw)
	}
}

func TestAppearanceSkinValidationAllowsOnlyBundledChoices(t *testing.T) {
	for skin, want := range map[string]bool{
		"":                     true,
		AppearanceSkinSkyGrove: true,
		AppearanceSkinStudio:   true,
		"core":                 false,
		"forest-night":         false,
	} {
		if got := ValidAppearanceSkin(skin); got != want {
			t.Errorf("ValidAppearanceSkin(%q) = %t, want %t", skin, got, want)
		}
	}
}

// The Go write set, the frontend allowlist, and the presentation manifest move
// in lockstep (TS-03.R45); the manifest checker ties the frontend to the
// manifest, and this ties Go to the same manifest.
func TestAppearanceSkinsMatchPresentationContract(t *testing.T) {
	raw, err := os.ReadFile("../../ui/src/presentation/contract.json")
	if err != nil {
		t.Fatalf("read presentation contract: %v", err)
	}
	var contract struct {
		Skins []string `json:"skins"`
	}
	if err := json.Unmarshal(raw, &contract); err != nil {
		t.Fatalf("decode presentation contract: %v", err)
	}
	if !slices.Equal(contract.Skins, BuiltInAppearanceSkins) {
		t.Fatalf("contract skins = %v, Go built-in skins = %v", contract.Skins, BuiltInAppearanceSkins)
	}
}
