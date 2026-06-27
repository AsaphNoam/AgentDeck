package state

import "errors"

// ErrNotFound is returned by Read* methods when no matching row exists.
var ErrNotFound = errors.New("state: not found")
