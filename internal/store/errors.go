package store

import "errors"

// Sentinel errors returned by the store. Callers compare with errors.Is.
var (
	// ErrNotFound is returned by Read* when the backing file does not exist.
	ErrNotFound = errors.New("store: not found")
	// ErrCorrupt is returned by Read* when the backing file exists but cannot
	// be parsed as JSON. List* never returns this — it skips corrupt files.
	ErrCorrupt = errors.New("store: corrupt file")
)
