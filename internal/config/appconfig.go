package config

// Config single-file object: config.json.

// AppearanceSkinSkyGrove is the only version-1 built-in appearance skin id.
// Core is represented by an absent or empty appearance_skin field, not an id.
const AppearanceSkinSkyGrove = "sky-grove"

// ValidAppearanceSkin reports whether an appearance_skin value may be written
// through the configuration API. ReadConfig intentionally does not use this
// check: an unknown hand-edited id remains readable so callers can fall back to
// Core and explain the unsupported value.
func ValidAppearanceSkin(skin string) bool {
	return skin == "" || skin == AppearanceSkinSkyGrove
}

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
