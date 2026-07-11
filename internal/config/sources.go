package config

import (
	"fmt"
	"path/filepath"
	"regexp"
)

// ConfigSources is the versioned ownership manifest for native CLI
// configuration. Effective values and resolver health are derived runtime
// state and intentionally do not belong in this document.
type ConfigSources struct {
	Version int                      `json:"version"`
	Sources map[string]SourceBinding `json:"sources"`
}

// SourceBinding links one AgentDeck backend to a native provider root.
type SourceBinding struct {
	Provider  string          `json:"provider"`
	Mode      string          `json:"mode"`
	Root      string          `json:"root"`
	Profile   string          `json:"profile,omitempty"`
	Claims    []string        `json:"claims"`
	Overrides SourceOverrides `json:"overrides,omitempty"`
	Approved  []string        `json:"approved_roots"`
}

// SourceOverrides contains explicit AgentDeck-owned values layered over a
// native source. A non-nil Model pointing at the empty string means to use the
// native/default model.
type SourceOverrides struct {
	Model  *string `json:"model,omitempty"`
	Effort *string `json:"effort,omitempty"`
}

var sourceProfileRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

var sourceProviderByBackendType = map[string]string{
	"claude-acp": "claude-code",
	"codex-acp":  "codex",
}

var knownSourceClaims = map[string]bool{
	"launch_defaults": true,
	"model_catalog":   true,
	"setup":           true,
}

// ProviderForBackendType maps a backend type to its federation provider, and
// reports whether the type supports configuration federation at all. Only
// claude-acp and codex-acp participate (techspec §2.2).
func ProviderForBackendType(backendType string) (string, bool) {
	p, ok := sourceProviderByBackendType[backendType]
	return p, ok
}

// ReadConfigSources reads config-sources.json. A missing file yields
// ErrNotFound and an unparseable file yields ErrCorrupt. Nil collections are
// normalized for callers so an empty document can be safely amended and
// written back.
func (s *Store) ReadConfigSources() (ConfigSources, error) {
	var c ConfigSources
	if err := readJSON(s.configSourcesPath(), &c); err != nil {
		return c, err
	}
	normalizeConfigSources(&c)
	return c, nil
}

// WriteConfigSources atomically writes config-sources.json. Validation is kept
// explicit, matching the other typed config stores; callers validate before
// persistence.
func (s *Store) WriteConfigSources(c ConfigSources) error {
	normalizeConfigSources(&c)
	return writeJSONAtomic(s.configSourcesPath(), c)
}

// ValidateConfigSources validates the persisted v1 schema against the current
// backend instances. Paths are required to be absolute and lexically
// canonical; symlink resolution and approval are performed by preview before a
// binding reaches this store.
func ValidateConfigSources(c *ConfigSources, backends BackendsConfig) *ValidationErrors {
	var errs []FieldError
	if c == nil {
		return &ValidationErrors{Errors: []FieldError{{
			Field: "config_sources", Code: "required", Message: "config sources are required",
		}}}
	}

	if c.Version != 1 {
		return &ValidationErrors{Errors: []FieldError{{
			Field:   "version",
			Code:    "unsupported_version",
			Message: fmt.Sprintf("version must be 1, got %d", c.Version),
		}}}
	}

	for backendID, binding := range c.Sources {
		prefix := fmt.Sprintf("sources.%s", backendID)
		backend, backendExists := backends.Backends[backendID]
		wantProvider, supportedBackend := sourceProviderByBackendType[backend.Type]
		if !backendExists {
			errs = append(errs, FieldError{
				Field: prefix, Code: "unknown_backend", Message: fmt.Sprintf("backend %q does not exist", backendID),
			})
		} else if !supportedBackend || binding.Provider != wantProvider {
			message := fmt.Sprintf("backend %q of type %q does not support configuration federation", backendID, backend.Type)
			if supportedBackend {
				message = fmt.Sprintf("backend %q of type %q requires provider %q", backendID, backend.Type, wantProvider)
			}
			errs = append(errs, FieldError{Field: prefix + ".provider", Code: "provider_backend_mismatch", Message: message})
		}

		if binding.Mode != "linked" && binding.Mode != "mirrored" {
			errs = append(errs, FieldError{
				Field: prefix + ".mode", Code: "invalid_mode", Message: `mode must be "linked" or "mirrored"`,
			})
		}
		if !canonicalAbsolutePath(binding.Root) {
			errs = append(errs, FieldError{
				Field: prefix + ".root", Code: "invalid_path", Message: "root must be a canonical absolute path",
			})
		}
		if binding.Profile != "" && !sourceProfileRE.MatchString(binding.Profile) {
			errs = append(errs, FieldError{
				Field: prefix + ".profile", Code: "invalid_profile", Message: "profile must be 1-64 letters, digits, dots, underscores, or hyphens and begin with a letter or digit",
			})
		}

		seenClaims := make(map[string]bool, len(binding.Claims))
		for i, claim := range binding.Claims {
			field := fmt.Sprintf("%s.claims.%d", prefix, i)
			if !knownSourceClaims[claim] {
				errs = append(errs, FieldError{Field: field, Code: "unknown_claim", Message: fmt.Sprintf("unknown claim %q", claim)})
			} else if seenClaims[claim] {
				errs = append(errs, FieldError{Field: field, Code: "duplicate_claim", Message: fmt.Sprintf("claim %q appears more than once", claim)})
			}
			seenClaims[claim] = true
		}

		seenApproved := make(map[string]bool, len(binding.Approved))
		rootApproved := false
		for i, root := range binding.Approved {
			field := fmt.Sprintf("%s.approved_roots.%d", prefix, i)
			if !canonicalAbsolutePath(root) {
				errs = append(errs, FieldError{Field: field, Code: "invalid_path", Message: "approved root must be a canonical absolute path"})
			} else if seenApproved[root] {
				errs = append(errs, FieldError{Field: field, Code: "duplicate_path", Message: fmt.Sprintf("approved root %q appears more than once", root)})
			}
			seenApproved[root] = true
			if root == binding.Root {
				rootApproved = true
			}
		}
		if canonicalAbsolutePath(binding.Root) && !rootApproved {
			errs = append(errs, FieldError{
				Field: prefix + ".approved_roots", Code: "root_not_approved", Message: "approved_roots must include the binding root",
			})
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return &ValidationErrors{Errors: errs}
}

func normalizeConfigSources(c *ConfigSources) {
	if c.Sources == nil {
		c.Sources = map[string]SourceBinding{}
	}
	for id, binding := range c.Sources {
		if binding.Claims == nil {
			binding.Claims = []string{}
		}
		if binding.Approved == nil {
			binding.Approved = []string{}
		}
		c.Sources[id] = binding
	}
}

func canonicalAbsolutePath(path string) bool {
	return path != "" && filepath.IsAbs(path) && filepath.Clean(path) == path
}
