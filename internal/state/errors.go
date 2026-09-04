package state

import "errors"

// ErrNotFound is returned by Read* methods when no matching row exists.
var ErrNotFound = errors.New("state: not found")

// ErrInvalidHook is returned when a hook payload is malformed.
var ErrInvalidHook = errors.New("state: invalid hook")

// ErrTokenMismatch is returned when a hook token does not match the live launch.
var ErrTokenMismatch = errors.New("state: token mismatch")

// ErrPipelineConflict is returned when a stale run revision loses a control or
// transition race.
var ErrPipelineConflict = errors.New("state: pipeline revision conflict")

// ErrPipelineRequestConflict is returned when a caller reuses a start request
// id with content different from the original request.
var ErrPipelineRequestConflict = errors.New("state: pipeline request id conflict")

// ErrPipelineActive is returned when deletion targets a non-terminal run.
var ErrPipelineActive = errors.New("state: pipeline run is active")

// The three proposal-claim losses. Each names the state the row is actually in
// so a refused Reject or Delete can say what happened instead of retrying blind
// (TS-02.R29, FS-14.R57). A record that is simply gone loses with ErrNotFound.
var (
	// ErrPipelineProposalConsumed is returned when an approval already committed
	// the mutation the record offered.
	ErrPipelineProposalConsumed = errors.New("state: pipeline proposal is consumed")
	// ErrPipelineProposalDeclined is returned when the record is already declined.
	ErrPipelineProposalDeclined = errors.New("state: pipeline proposal is already declined")
	// ErrPipelineProposalNotDeclined is returned when a delete targets a record
	// that has not been rejected first.
	ErrPipelineProposalNotDeclined = errors.New("state: pipeline proposal is not declined")
)
