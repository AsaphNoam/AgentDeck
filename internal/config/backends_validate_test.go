package config

import "testing"

func baseBackends() BackendsConfig {
	return BackendsConfig{
		Version: 2,
		Backends: map[string]Backend{
			"claude": {
				Name:         "Claude",
				Type:         "claude-acp",
				Default:      true,
				DefaultModel: "default",
				Models: map[string]Model{
					"default": {Name: "Default", Model: "claude-sonnet-4-6"},
				},
			},
		},
	}
}

func TestValidateBackendsConfig_Valid(t *testing.T) {
	b := baseBackends()
	if ve := ValidateBackendsConfig(&b); ve != nil {
		t.Errorf("valid config failed: %v", ve.Errors)
	}
}

func TestValidateBackendsConfig_UnsupportedVersion(t *testing.T) {
	b := baseBackends()
	b.Version = 1
	ve := ValidateBackendsConfig(&b)
	if ve == nil {
		t.Fatal("expected error")
	}
	if !backendsHasCode(ve, "unsupported_version") {
		t.Errorf("expected unsupported_version, got %v", ve.Errors)
	}
}

func TestValidateBackendsConfig_AutoPromoteDefaultBackend(t *testing.T) {
	b := BackendsConfig{
		Version: 2,
		Backends: map[string]Backend{
			"claude": {
				Name:         "Claude",
				Type:         "claude-acp",
				Default:      false, // no default set
				DefaultModel: "default",
				Models: map[string]Model{
					"default": {Name: "D", Model: "m"},
				},
			},
		},
	}
	if ve := ValidateBackendsConfig(&b); ve != nil {
		t.Fatalf("auto-promote failed: %v", ve.Errors)
	}
	if !b.Backends["claude"].Default {
		t.Error("expected claude to be auto-promoted to default")
	}
}

func TestValidateBackendsConfig_MultipleDefaultBackends(t *testing.T) {
	b := BackendsConfig{
		Version: 2,
		Backends: map[string]Backend{
			"a": {Type: "claude-acp", Default: true, DefaultModel: "m", Models: map[string]Model{"m": {Model: "x"}}},
			"b": {Type: "codex-acp", Default: true, DefaultModel: "m", Models: map[string]Model{"m": {Model: "x"}}},
		},
	}
	ve := ValidateBackendsConfig(&b)
	if ve == nil {
		t.Fatal("expected error")
	}
	if !backendsHasCode(ve, "multiple_default_backends") {
		t.Errorf("expected multiple_default_backends, got %v", ve.Errors)
	}
}

func TestValidateBackendsConfig_UnknownDefaultModel(t *testing.T) {
	b := BackendsConfig{
		Version: 2,
		Backends: map[string]Backend{
			"claude": {
				Type:         "claude-acp",
				Default:      true,
				DefaultModel: "nonexistent",
				Models:       map[string]Model{"default": {Model: "m"}},
			},
		},
	}
	ve := ValidateBackendsConfig(&b)
	if ve == nil {
		t.Fatal("expected error")
	}
	if !backendsHasCode(ve, "unknown_default_model") {
		t.Errorf("expected unknown_default_model, got %v", ve.Errors)
	}
}

func TestValidateBackendsConfig_AutoPromoteDefaultModel(t *testing.T) {
	b := BackendsConfig{
		Version: 2,
		Backends: map[string]Backend{
			"claude": {
				Type:         "claude-acp",
				Default:      true,
				DefaultModel: "", // missing
				Models:       map[string]Model{"alpha": {Model: "m"}, "beta": {Model: "n"}},
			},
		},
	}
	if ve := ValidateBackendsConfig(&b); ve != nil {
		t.Fatalf("auto-promote default model failed: %v", ve.Errors)
	}
	// Should be promoted to lexicographically first: "alpha"
	if b.Backends["claude"].DefaultModel != "alpha" {
		t.Errorf("DefaultModel = %q, want alpha", b.Backends["claude"].DefaultModel)
	}
}

func TestValidateBackendsConfig_BackendWithoutModels(t *testing.T) {
	b := BackendsConfig{
		Version: 2,
		Backends: map[string]Backend{
			"claude": {Type: "claude-acp", Default: true, Models: map[string]Model{}},
		},
	}
	ve := ValidateBackendsConfig(&b)
	if ve == nil {
		t.Fatal("expected error")
	}
	if !backendsHasCode(ve, "backend_without_models") {
		t.Errorf("expected backend_without_models, got %v", ve.Errors)
	}
}

func TestValidateBackendsConfig_UnknownBackendType(t *testing.T) {
	b := BackendsConfig{
		Version: 2,
		Backends: map[string]Backend{
			"x": {Type: "openai-direct", Default: true, DefaultModel: "m", Models: map[string]Model{"m": {Model: "gpt"}}},
		},
	}
	ve := ValidateBackendsConfig(&b)
	if ve == nil {
		t.Fatal("expected error")
	}
	if !backendsHasCode(ve, "unknown_backend_type") {
		t.Errorf("expected unknown_backend_type, got %v", ve.Errors)
	}
}

func TestValidateBackendsConfig_NewBackendTypesAccepted(t *testing.T) {
	// The Phase 7 backends are part of the four-value type union; a document using
	// them must validate (so a seeded opencode/openhands survives a Settings save).
	for _, typ := range []string{"opencode-acp", "openhands-acp"} {
		b := BackendsConfig{
			Version: 2,
			Backends: map[string]Backend{
				"x": {Type: typ, Default: true, DefaultModel: "m", Models: map[string]Model{"m": {Model: "prov/model"}}},
			},
		}
		if ve := ValidateBackendsConfig(&b); ve != nil {
			t.Fatalf("%s should validate, got %v", typ, ve.Errors)
		}
	}
}

func TestValidateBackendsConfig_EmptyModelField(t *testing.T) {
	b := BackendsConfig{
		Version: 2,
		Backends: map[string]Backend{
			"claude": {
				Type:         "claude-acp",
				Default:      true,
				DefaultModel: "default",
				Models:       map[string]Model{"default": {Name: "X", Model: ""}}, // empty model field
			},
		},
	}
	ve := ValidateBackendsConfig(&b)
	if ve == nil {
		t.Fatal("expected error")
	}
	if !backendsHasCode(ve, "required") {
		t.Errorf("expected required, got %v", ve.Errors)
	}
}

func backendsHasCode(ve *ValidationErrors, code string) bool {
	for _, e := range ve.Errors {
		if e.Code == code {
			return true
		}
	}
	return false
}
