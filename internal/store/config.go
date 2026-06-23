package store

// Config single-file object: config.json.

// ReadConfig reads config.json. Returns ErrNotFound if absent, ErrCorrupt if
// unparseable; callers fall back to DefaultConfig.
func (s *Store) ReadConfig() (Config, error) {
	var c Config
	err := readJSON(s.configPath(), &c)
	return c, err
}

// WriteConfig atomically writes config.json.
func (s *Store) WriteConfig(c Config) error {
	return writeJSONAtomic(s.configPath(), c)
}
