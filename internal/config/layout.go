package config

// Layout single-file object: layout.json.

// ReadLayout reads layout.json. Returns ErrNotFound if absent, ErrCorrupt if
// unparseable; callers fall back to DefaultLayout.
func (s *Store) ReadLayout() (Layout, error) {
	var l Layout
	err := readJSON(s.layoutPath(), &l)
	return l, err
}

// WriteLayout atomically writes layout.json.
func (s *Store) WriteLayout(l Layout) error {
	return writeJSONAtomic(s.layoutPath(), l)
}
