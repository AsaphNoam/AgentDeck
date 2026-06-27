package runtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"
)

// TestStatusForCode locks the code→HTTP-status mapping (techspec §7.7).
func TestStatusForCode(t *testing.T) {
	cases := map[string]int{
		CodeValidation:         http.StatusUnprocessableEntity,
		CodeNotFound:           http.StatusNotFound,
		CodeConflict:           http.StatusConflict,
		CodeNotImplemented:     http.StatusNotImplemented,
		CodeRuntimeStartFailed: http.StatusBadGateway,
		CodeInternal:           http.StatusInternalServerError,
		"unknown":              http.StatusInternalServerError,
	}
	for code, want := range cases {
		if got := statusForCode(code); got != want {
			t.Errorf("statusForCode(%q) = %d, want %d", code, got, want)
		}
		if want != http.StatusInternalServerError || code == CodeInternal {
			ae := NewAPIError(code, "msg")
			if got := ae.HTTPStatus(); got != want {
				t.Errorf("APIError{%q}.HTTPStatus() = %d, want %d", code, got, want)
			}
		}
	}
}

// TestAPIErrorFor classifies sentinel and arbitrary errors.
func TestAPIErrorFor(t *testing.T) {
	if APIErrorFor(nil) != nil {
		t.Fatal("APIErrorFor(nil) should be nil")
	}

	wrapped := fmt.Errorf("dispatch: %w", ErrNotImplemented)
	ae := APIErrorFor(wrapped)
	if ae.Code != CodeNotImplemented {
		t.Errorf("wrapped ErrNotImplemented code = %q, want %q", ae.Code, CodeNotImplemented)
	}
	if ae.HTTPStatus() != http.StatusNotImplemented {
		t.Errorf("status = %d, want 501", ae.HTTPStatus())
	}

	other := APIErrorFor(errors.New("disk full"))
	if other.Code != CodeInternal {
		t.Errorf("generic error code = %q, want %q", other.Code, CodeInternal)
	}
}

// TestAPIErrorEnvelope asserts the §7.7 JSON envelope shape.
func TestAPIErrorEnvelope(t *testing.T) {
	ae := &APIError{Code: CodeValidation, Message: "bad role", Details: map[string]any{"field": "role"}}
	b, err := json.Marshal(struct {
		Error *APIError `json:"error"`
	}{Error: ae})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got struct {
		Error struct {
			Code    string         `json:"code"`
			Message string         `json:"message"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Error.Code != CodeValidation || got.Error.Message != "bad role" {
		t.Errorf("envelope mismatch: %s", b)
	}
	if got.Error.Details["field"] != "role" {
		t.Errorf("details lost: %s", b)
	}
}
