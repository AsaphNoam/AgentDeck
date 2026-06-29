package runtime

import (
	"errors"
	"net/http"
)

// ErrNotImplemented is the sentinel returned by stubbed runtimes and dispatch
// paths (terminal interface, codex backend, Resume, CheckMessages). The API
// layer maps it to HTTP 501 / code "not_implemented" (techspec §3.2, §7.7).
var ErrNotImplemented = errors.New("not implemented")

// Runtime-state sentinels. The API layer maps these to 404/409 (techspec §7).
var (
	// ErrNoHandle: no live handle for the agent (not started / already stopped).
	ErrNoHandle = errors.New("runtime: agent not started")
	// ErrTurnInFlight: a turn is already running for the agent (no queue, §12.3).
	ErrTurnInFlight = errors.New("runtime: a turn is already in flight")
	// ErrNoPendingPermission: no pending permission for the given tool_call_id.
	ErrNoPendingPermission = errors.New("runtime: no pending permission request")
	// ErrInvalidDecision: a permission decision other than approve/deny.
	ErrInvalidDecision = errors.New("runtime: invalid permission decision")
	// ErrProtocolVersion: the adapter negotiated an ACP protocol version outside
	// the pinned [minACPVersion, maxACPVersion] range (techspec §12.1).
	ErrProtocolVersion = errors.New("runtime: incompatible ACP protocol version")
)

// Error code vocabulary (techspec §7.7). These are the project-wide error codes
// surfaced in the API error envelope; each maps to a fixed HTTP status.
const (
	CodeValidation         = "validation"           // 422
	CodeNotFound           = "not_found"            // 404
	CodeConflict           = "conflict"             // 409
	CodeNotImplemented     = "not_implemented"      // 501
	CodeRuntimeStartFailed = "runtime_start_failed" // 502
	CodeInternal           = "internal"             // 500
)

// APIError is the normalized error payload. It serializes to the §7.7 envelope:
//
//	{ "error": { "code": "...", "message": "...", "details": {} } }
//
// HTTPStatus returns the status code the API layer should respond with.
type APIError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// Error implements the error interface.
func (e *APIError) Error() string { return e.Code + ": " + e.Message }

// HTTPStatus maps the error code to its HTTP status (techspec §7.7).
func (e *APIError) HTTPStatus() int {
	return statusForCode(e.Code)
}

// statusForCode maps an error code to its HTTP status. Unknown codes map to 500.
func statusForCode(code string) int {
	switch code {
	case CodeValidation:
		return http.StatusUnprocessableEntity // 422
	case CodeNotFound:
		return http.StatusNotFound // 404
	case CodeConflict:
		return http.StatusConflict // 409
	case CodeNotImplemented:
		return http.StatusNotImplemented // 501
	case CodeRuntimeStartFailed:
		return http.StatusBadGateway // 502
	default:
		return http.StatusInternalServerError // 500
	}
}

// NewAPIError builds an APIError with the given code and message.
func NewAPIError(code, message string) *APIError {
	return &APIError{Code: code, Message: message}
}

// APIErrorFor classifies an arbitrary error into an APIError. ErrNotImplemented
// becomes a not_implemented (501) error; everything else becomes internal (500).
// Callers that already know the precise classification should build the APIError
// directly with NewAPIError.
func APIErrorFor(err error) *APIError {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrNotImplemented) {
		return NewAPIError(CodeNotImplemented, err.Error())
	}
	return NewAPIError(CodeInternal, err.Error())
}
