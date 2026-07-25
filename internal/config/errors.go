package config

import "errors"

// Sentinel errors returned by the config store. Callers compare with errors.Is.
var (
	// ErrNotFound is returned by Read* when the backing file does not exist.
	ErrNotFound = errors.New("config: not found")
	// ErrCorrupt is wrapped by Read* when the backing file exists but cannot
	// be parsed as JSON or fails its structural validation. List* never returns
	// this — it skips corrupt files.
	ErrCorrupt = errors.New("config: corrupt file")
	// ErrTooLarge is wrapped when a bounded config reader refuses a file before
	// decoding it. Pipeline templates use this to keep hand-edited JSON bounded.
	ErrTooLarge = errors.New("config: file too large")
)
