package config

import "encoding/json"

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
	var disk struct {
		Version              int                 `json:"version"`
		Port                 int                 `json:"port"`
		DefaultProject       string              `json:"default_project"`
		DefaultRole          string              `json:"default_role"`
		AppearanceSkin       string              `json:"appearance_skin,omitempty"`
		SkipPermissions      bool                `json:"skip_permissions"`
		OnboardingComplete   bool                `json:"onboarding_complete"`
		Notifications        NotificationsConfig `json:"notifications"`
		Switch               SwitchConfig        `json:"switch"`
		TaskConcurrency      int                 `json:"task_concurrency"`
		MessageBudgetPerTurn json.RawMessage     `json:"message_budget_per_turn"`
	}
	if err := readJSON(s.configPath(), &disk); err != nil {
		return Config{}, err
	}
	c := Config{
		Version: disk.Version, Port: disk.Port, DefaultProject: disk.DefaultProject,
		DefaultRole: disk.DefaultRole, AppearanceSkin: disk.AppearanceSkin,
		SkipPermissions: disk.SkipPermissions, OnboardingComplete: disk.OnboardingComplete,
		Notifications: disk.Notifications, Switch: disk.Switch, TaskConcurrency: disk.TaskConcurrency,
	}
	_ = json.Unmarshal(disk.MessageBudgetPerTurn, &c.MessageBudgetPerTurn)
	return c, nil
}

// WriteConfig atomically writes config.json.
func (s *Store) WriteConfig(c Config) error {
	return writeJSONAtomic(s.configPath(), c)
}
