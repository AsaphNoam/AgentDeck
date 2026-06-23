package store

// Backends single-file object: backends.json.

// ReadBackends reads backends.json. Returns ErrNotFound if absent, ErrCorrupt
// if unparseable; callers (handlers, dashboard start) fall back to defaults.
func (s *Store) ReadBackends() (BackendsConfig, error) {
	var b BackendsConfig
	err := readJSON(s.backendsPath(), &b)
	return b, err
}

// WriteBackends atomically writes backends.json.
func (s *Store) WriteBackends(b BackendsConfig) error {
	return writeJSONAtomic(s.backendsPath(), b)
}
