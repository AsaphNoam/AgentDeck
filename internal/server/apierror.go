package server

import (
	"net/http"

	"github.com/agentdeck/agentdeck/internal/runtime"
)

// writeAPIError emits the techspec §7.7 nested error envelope:
//
//	{ "error": { "code": "...", "message": "...", "details": {} } }
//
// at the status mapped from the code. The Phase-1 session routes use this shape;
// the older Phase-0 GET routes keep the flat {"error":"msg"} envelope.
func writeAPIError(w http.ResponseWriter, ae *runtime.APIError) {
	writeJSON(w, ae.HTTPStatus(), struct {
		Error *runtime.APIError `json:"error"`
	}{Error: ae})
}

// apiError is a terse constructor for an APIError at a known code.
func apiError(code, msg string) *runtime.APIError { return runtime.NewAPIError(code, msg) }
