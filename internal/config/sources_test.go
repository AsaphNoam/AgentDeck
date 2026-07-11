package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func validConfigSources(t *testing.T) ConfigSources {
	t.Helper()
	root := filepath.Join(t.TempDir(), "codex")
	claudeRoot := filepath.Join(t.TempDir(), "claude")
	return ConfigSources{
		Version: 1,
		Sources: map[string]SourceBinding{
			"claude": {
				Provider: "claude-code",
				Mode:     "mirrored",
				Root:     claudeRoot,
				Claims:   []string{"launch_defaults", "setup"},
				Approved: []string{claudeRoot},
			},
			"codex": {
				Provider: "codex",
				Mode:     "linked",
				Root:     root,
				Profile:  "work_profile-1",
				Claims:   []string{"launch_defaults", "model_catalog", "setup"},
				Approved: []string{root, filepath.Dir(root)},
			},
		},
	}
}

func validBackendsForSources() BackendsConfig {
	return BackendsConfig{Version: 2, Backends: map[string]Backend{
		"claude":   {Type: "claude-acp"},
		"codex":    {Type: "codex-acp"},
		"opencode": {Type: "opencode-acp"},
	}}
}

func TestConfigSourcesRoundTripAndOwnerOnlyAtomicWrite(t *testing.T) {
	s := NewWithHome(t.TempDir())
	c := validConfigSources(t)
	if err := s.WriteConfigSources(c); err != nil {
		t.Fatalf("WriteConfigSources: %v", err)
	}

	got, err := s.ReadConfigSources()
	if err != nil {
		t.Fatalf("ReadConfigSources: %v", err)
	}
	if ve := ValidateConfigSources(&got, validBackendsForSources()); ve != nil {
		t.Fatalf("round-tripped config invalid: %v", ve.Errors)
	}
	if got.Sources["codex"].Profile != "work_profile-1" {
		t.Fatalf("profile = %q, want work_profile-1", got.Sources["codex"].Profile)
	}

	info, err := os.Stat(s.configSourcesPath())
	if err != nil {
		t.Fatalf("stat config-sources.json: %v", err)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		t.Errorf("config-sources.json permissions = %04o, want owner-only", perm)
	}
	entries, err := os.ReadDir(s.Home())
	if err != nil {
		t.Fatalf("read config home: %v", err)
	}
	for _, entry := range entries {
		if len(entry.Name()) >= 5 && entry.Name()[:5] == ".tmp-" {
			t.Errorf("leftover atomic-write temp file: %s", entry.Name())
		}
	}
}

func TestConfigSourcesEmptyCollectionsMarshalNonNull(t *testing.T) {
	s := NewWithHome(t.TempDir())
	if err := s.WriteConfigSources(ConfigSources{Version: 1}); err != nil {
		t.Fatalf("WriteConfigSources: %v", err)
	}
	data, err := os.ReadFile(s.configSourcesPath())
	if err != nil {
		t.Fatalf("read config-sources.json: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal config-sources.json: %v", err)
	}
	if string(raw["sources"]) != "{}" {
		t.Errorf("sources = %s, want {}", raw["sources"])
	}

	withBinding := ConfigSources{Version: 1, Sources: map[string]SourceBinding{
		"claude": {Provider: "claude-code", Mode: "linked", Root: filepath.Join(t.TempDir(), "claude")},
	}}
	if err := s.WriteConfigSources(withBinding); err != nil {
		t.Fatalf("WriteConfigSources binding: %v", err)
	}
	got, err := s.ReadConfigSources()
	if err != nil {
		t.Fatalf("ReadConfigSources binding: %v", err)
	}
	binding := got.Sources["claude"]
	if binding.Claims == nil || binding.Approved == nil {
		t.Fatalf("nil collections after round trip: claims=%v approved=%v", binding.Claims, binding.Approved)
	}
}

func TestConfigSourcesIsNotSeeded(t *testing.T) {
	s := NewWithHome(t.TempDir())
	if err := s.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout: %v", err)
	}
	if err := s.SeedIfAbsent(); err != nil {
		t.Fatalf("SeedIfAbsent: %v", err)
	}
	if _, err := s.ReadConfigSources(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ReadConfigSources after seed = %v, want ErrNotFound", err)
	}
}

func TestValidateConfigSources(t *testing.T) {
	tests := []struct {
		name string
		edit func(*ConfigSources)
		code string
	}{
		{name: "valid", edit: func(*ConfigSources) {}},
		{name: "version", edit: func(c *ConfigSources) { c.Version = 2 }, code: "unsupported_version"},
		{name: "unknown backend", edit: func(c *ConfigSources) {
			binding := c.Sources["codex"]
			delete(c.Sources, "codex")
			c.Sources["custom"] = binding
		}, code: "unknown_backend"},
		{name: "provider pair", edit: func(c *ConfigSources) {
			binding := c.Sources["codex"]
			binding.Provider = "claude-code"
			c.Sources["codex"] = binding
		}, code: "provider_backend_mismatch"},
		{name: "unsupported backend type", edit: func(c *ConfigSources) {
			binding := c.Sources["codex"]
			delete(c.Sources, "codex")
			binding.Provider = "codex"
			c.Sources["opencode"] = binding
		}, code: "provider_backend_mismatch"},
		{name: "detached mode", edit: func(c *ConfigSources) {
			binding := c.Sources["codex"]
			binding.Mode = "detached"
			c.Sources["codex"] = binding
		}, code: "invalid_mode"},
		{name: "relative root", edit: func(c *ConfigSources) {
			binding := c.Sources["codex"]
			binding.Root = "relative/root"
			c.Sources["codex"] = binding
		}, code: "invalid_path"},
		{name: "unclean root", edit: func(c *ConfigSources) {
			binding := c.Sources["codex"]
			binding.Root = filepath.Join(t.TempDir(), "x") + string(filepath.Separator) + ".." + string(filepath.Separator) + "codex"
			c.Sources["codex"] = binding
		}, code: "invalid_path"},
		{name: "profile traversal", edit: func(c *ConfigSources) {
			binding := c.Sources["codex"]
			binding.Profile = "../work"
			c.Sources["codex"] = binding
		}, code: "invalid_profile"},
		{name: "unknown claim", edit: func(c *ConfigSources) {
			binding := c.Sources["codex"]
			binding.Claims = append(binding.Claims, "secrets")
			c.Sources["codex"] = binding
		}, code: "unknown_claim"},
		{name: "duplicate claim", edit: func(c *ConfigSources) {
			binding := c.Sources["codex"]
			binding.Claims = append(binding.Claims, "setup")
			c.Sources["codex"] = binding
		}, code: "duplicate_claim"},
		{name: "relative approved root", edit: func(c *ConfigSources) {
			binding := c.Sources["codex"]
			binding.Approved = append(binding.Approved, "project")
			c.Sources["codex"] = binding
		}, code: "invalid_path"},
		{name: "root not approved", edit: func(c *ConfigSources) {
			binding := c.Sources["codex"]
			binding.Approved = []string{filepath.Dir(binding.Root)}
			c.Sources["codex"] = binding
		}, code: "root_not_approved"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := validConfigSources(t)
			tt.edit(&c)
			ve := ValidateConfigSources(&c, validBackendsForSources())
			if tt.code == "" {
				if ve != nil {
					t.Fatalf("unexpected validation errors: %v", ve.Errors)
				}
				return
			}
			if ve == nil || !sourceValidationHasCode(ve, tt.code) {
				t.Fatalf("validation = %v, want code %q", ve, tt.code)
			}
		})
	}
}

func sourceValidationHasCode(ve *ValidationErrors, code string) bool {
	for _, fieldErr := range ve.Errors {
		if fieldErr.Code == code {
			return true
		}
	}
	return false
}
