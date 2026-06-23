package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// writeJSON encodes v as JSON with the standard content type and the given
// status code. Encode errors are logged but cannot change an already-sent header.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("writeJSON: encode failed", "err", err)
	}
}

// errorBody is the consistent error envelope: {"error":"<message>"}.
type errorBody struct {
	Error string `json:"error"`
}

// writeError sends the error envelope with the given status code.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorBody{Error: msg})
}
