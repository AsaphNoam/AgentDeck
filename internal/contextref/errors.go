package contextref

import "fmt"

// Stable outcome codes shared by the service and its MCP tools (TS-04.R28).
const (
	// CodeNotFound covers both an unknown and an unauthorized reference or
	// grant id. The two are deliberately indistinguishable so ids cannot
	// enumerate another agent's context metadata (TS-05.R16).
	CodeNotFound = "context_not_found"
	// CodeSourceGone — an authorized, already-canonical reference now points at
	// a deleted or unreadable source (FS-15.R13).
	CodeSourceGone = "context_source_unavailable"
	// CodeSourceUnavailable — a share-time friendly selector has no eligible
	// current source; no row was created (FS-15.R15).
	CodeSourceUnavailable = "source_unavailable"
	CodeRecipientNotFound = "recipient_not_found"
	CodeAmbiguous         = "ambiguous_recipient"
	CodeInvalidCursor     = "invalid_cursor"
	CodeValidation        = "validation"
	// CodeUnavailable — the underlying store could not answer.
	CodeUnavailable = "store_unavailable"
)

// Error is a typed context outcome. Message is product-safe: it never carries
// transcript or report bytes (TS-05.R16).
type Error struct {
	Code       string
	Message    string
	Candidates any
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

func failf(code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// notFound is the single safe answer for a missing or unauthorized id.
func notFound() *Error {
	return &Error{Code: CodeNotFound, Message: "No context reference or grant is available to you with that id."}
}

func unavailable(err error) *Error {
	return &Error{Code: CodeUnavailable, Message: err.Error()}
}
