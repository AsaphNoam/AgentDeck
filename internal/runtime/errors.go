package runtime

import (
	"errors"
	"net/http"
)

// ErrNotImplemented is the sentinel returned by stubbed runtimes and dispatch
// paths (terminal interface, codex backend, Resume, StartActivation). The API
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
	// ErrPermissionAlreadyResolved: a concurrent approve/deny/cancel/timeout
	// already settled this permission request before the caller won the race.
	ErrPermissionAlreadyResolved = errors.New("runtime: permission request already resolved")
	// ErrInvalidDecision: a permission decision other than approve/deny.
	ErrInvalidDecision = errors.New("runtime: invalid permission decision")
	// ErrProtocolVersion: the adapter negotiated an ACP protocol version outside
	// the pinned [minACPVersion, maxACPVersion] range (techspec §12.1).
	ErrProtocolVersion = errors.New("runtime: incompatible ACP protocol version")
	// ErrSteeringUnsupported: this live session's adapter did not advertise the
	// ACP steering extension, so there is no Steer control to reach this path
	// (FS-03.R50, TS-04.R49).
	ErrSteeringUnsupported = errors.New("runtime: this agent's adapter does not support steering")
	// ErrNothingHeld: a steer with no text asked to promote the agent's held
	// follow-up and there is none — typically because the turn ended and it was
	// already delivered (FS-03.R50).
	ErrNothingHeld = errors.New("runtime: no message is held for this agent")
)

// Session-configuration sentinels (TS-03.R37). A live setting change fails for
// four reasons a person can act on differently, so each is typed and the API
// layer maps it to its own envelope code instead of collapsing all four into one
// upstream-failure status (INV §8, INV §11).
var (
	// ErrSettingUnsupported: the adapter declares no delivery mechanism for the
	// setting at all, so no session could ever accept it.
	ErrSettingUnsupported = errors.New("runtime: setting is not supported by this backend")
	// ErrSettingUnavailable: the adapter supports the setting but this live
	// session does not advertise it — typically because its current model does
	// not offer it. Live-confirmed against the pinned Claude adapter, which drops
	// the effort and fast options entirely once the model is set to one that has
	// neither.
	ErrSettingUnavailable = errors.New("runtime: setting is not available on this session")
	// ErrSettingRejected: the provider refused the requested value.
	ErrSettingRejected = errors.New("runtime: provider rejected the setting")
	// ErrSettingIgnored: the provider answered success but its own rebuilt option
	// list reports a different effective value. This is the silent-ignore class
	// BR-1 shipped, so accepting the RPC envelope as success is not enough
	// (INV §12).
	ErrSettingIgnored = errors.New("runtime: provider ignored the setting")
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

	// switch-runtime error codes (Phase 6 techspec §8.1). Distinct code strings
	// the UI branches on, each with its own HTTP status.
	CodeNoChange               = "no_change"                 // 400
	CodeInvalidField           = "invalid_field"             // 400
	CodeAgentNotRunning        = "agent_not_running"         // 409
	CodeSwitchInProgress       = "switch_in_progress"        // 409
	CodeTerminalUnavailable    = "terminal_unavailable"      // 422
	CodeSwitchFailed           = "switch_failed"             // 500
	CodeSwitchFailedRolledBack = "switch_failed_rolled_back" // 500

	// identity/group edit error codes (Phase 6 techspec §8.2–8.4).
	CodeEmptyName        = "empty_name"         // 400
	CodeInvalidGroupName = "invalid_group_name" // 400
	CodeGroupNotFound    = "group_not_found"    // 404

	// configuration-federation error codes (Phase 7 techspec §2.7).
	CodeSourceNotFound   = "source_not_found"  // 404
	CodeSourceChanged    = "source_changed"    // 409
	CodeSourceConflict   = "source_conflict"   // 409
	CodeApprovalRequired = "approval_required" // 409
	CodeSourceInvalid    = "source_invalid"    // 422
	CodeAgentArchived    = "agent_archived"    // 409
	CodeAgentArchiving   = "agent_archiving"   // 409
	CodeProjectArchived  = "project_archived"  // 409
	CodeProjectArchiving = "project_archiving" // 409

	// CodeBackendExists rejects an item-scoped backend create that reuses an id
	// with a different name or type (TS-03.R23). An exact replay is idempotent
	// and never reaches this code.
	CodeBackendExists = "backend_exists" // 409
	// CodeBackendCatalogChanged rejects a whole-catalog replacement whose ETag
	// predates a committed catalog mutation in another tab.
	CodeBackendCatalogChanged = "backend_catalog_changed" // 409

	// directory-picker codes (TS-03.R26). Busy is the single-flight refusal: one
	// process-wide claim allows at most one open folder panel, so a concurrent
	// request is rejected rather than queued behind it or given a second panel.
	// Failed is the single safe outcome for a launch failure, unusable output, or
	// a selection that cannot be verified — none of them may carry script
	// diagnostics or a filesystem path into the response (TS-05.R15).
	CodeDirectoryPickerBusy   = "directory_picker_busy"   // 409
	CodeDirectoryPickerFailed = "directory_picker_failed" // 500
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
	case CodeValidation, CodeTerminalUnavailable, CodeSourceInvalid:
		return http.StatusUnprocessableEntity // 422
	case CodeNoChange, CodeInvalidField, CodeEmptyName, CodeInvalidGroupName:
		return http.StatusBadRequest // 400
	case CodeNotFound, CodeGroupNotFound, CodeSourceNotFound:
		return http.StatusNotFound // 404
	case CodeConflict, CodeAgentNotRunning, CodeSwitchInProgress,
		CodeSourceChanged, CodeSourceConflict, CodeApprovalRequired,
		CodeAgentArchived, CodeAgentArchiving, CodeProjectArchived, CodeProjectArchiving,
		CodeBackendExists, CodeBackendCatalogChanged, CodeDirectoryPickerBusy:
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
