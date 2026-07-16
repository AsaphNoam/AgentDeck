package config

import "fmt"

// Backends single-file object: backends.json.

// ReadBackends reads and validates backends.json. Returns ErrNotFound if absent
// and ErrCorrupt if it cannot be parsed or is structurally incomplete; callers
// (handlers, dashboard start) fall back to defaults.
func (s *Store) ReadBackends() (BackendsConfig, error) {
	var b BackendsConfig
	path := s.backendsPath()
	if err := readJSON(path, &b); err != nil {
		return b, err
	}
	// A missing `backends` field decodes to nil, which is distinct from an
	// intentionally empty map. Treat it as corrupt rather than serializing it as
	// JSON null for the UI to iterate (INV §11).
	if b.Backends == nil {
		return b, fmt.Errorf("%s: %w: missing backends collection", path, ErrCorrupt)
	}
	if err := ValidateBackendsConfig(&b); err != nil {
		return b, fmt.Errorf("%s: %w: %v", path, ErrCorrupt, err)
	}
	return b, nil
}

// WriteBackends atomically writes backends.json.
func (s *Store) WriteBackends(b BackendsConfig) error {
	return writeJSONAtomic(s.backendsPath(), b)
}
