package state

import "errors"

// ErrNotFound is returned by Read* methods when no matching row exists.
var ErrNotFound = errors.New("state: not found")

// ErrInvalidHook is returned when a hook payload is malformed.
var ErrInvalidHook = errors.New("state: invalid hook")

// ErrTokenMismatch is returned when a hook token does not match the live launch.
var ErrTokenMismatch = errors.New("state: token mismatch")
