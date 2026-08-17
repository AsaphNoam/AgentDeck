package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/runtime"
)

// doPicker posts the directory-picker action against the real mux, so every case
// below also passes through the localOnly guard and the API middleware.
func doPicker(t *testing.T, h http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, newLocalRequest(http.MethodPost, "/api/directory-picker", nil))
	return rec
}

// pickerErrorCode extracts the structured error code, failing if the body is not
// the TS-03.R3 envelope.
func pickerErrorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error runtime.APIError `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error envelope %q: %v", rec.Body.String(), err)
	}
	return body.Error.Code
}

// TestDirectoryPickerReturnsSelectedDirectory: a selection answers 200 with the
// one absolute path and nothing else (TS-03.R26).
func TestDirectoryPickerReturnsSelectedDirectory(t *testing.T) {
	srv := testServer(t, true)
	picked := t.TempDir()
	srv.pickDirectory = func(context.Context) (string, bool, error) { return picked, false, nil }

	rec := doPicker(t, srv.routes())
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["path"] != picked {
		t.Errorf("path = %v, want %q", body["path"], picked)
	}
	if len(body) != 1 {
		t.Errorf("response carries extra fields: %v", body)
	}
}

// TestDirectoryPickerCancelIsEmptyNoContent: dismissing the panel is not an
// error. It answers 204 with no path and no error envelope, so the form can
// leave its current value untouched (FS-04.R42, TS-03.R26).
func TestDirectoryPickerCancelIsEmptyNoContent(t *testing.T) {
	srv := testServer(t, true)
	srv.pickDirectory = func(context.Context) (string, bool, error) { return "", true, nil }

	rec := doPicker(t, srv.routes())
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("cancel body = %q, want empty", rec.Body.String())
	}
}

// TestDirectoryPickerFailuresAreSafeAndOpaque: a launch failure and unusable
// panel output both answer the one safe 500 code, and neither the selected path
// nor any script diagnostic reaches the response (TS-03.R26, TS-05.R15).
func TestDirectoryPickerFailuresAreSafeAndOpaque(t *testing.T) {
	// Text a real osascript failure could contain: the user's home path and the
	// script's own error output.
	leaky := errors.New("/Users/someone/Secret Project: execution error: osascript is not allowed assistive access (-1728)")
	cases := map[string]error{
		"launch failure": leaky,
		"invalid output": errPickerBadOutput,
		"unavailable":    errPickerUnavailable,
	}
	for name, runErr := range cases {
		t.Run(name, func(t *testing.T) {
			srv := testServer(t, true)
			srv.pickDirectory = func(context.Context) (string, bool, error) { return "", false, runErr }

			rec := doPicker(t, srv.routes())
			if rec.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want 500", rec.Code)
			}
			if got := pickerErrorCode(t, rec); got != runtime.CodeDirectoryPickerFailed {
				t.Errorf("code = %q, want %q", got, runtime.CodeDirectoryPickerFailed)
			}
			body := rec.Body.String()
			for _, leak := range []string{"/Users/", "execution error", "osascript", "-1728"} {
				if strings.Contains(body, leak) {
					t.Errorf("response leaks %q: %s", leak, body)
				}
			}
		})
	}
}

// TestDirectoryPickerIsSingleFlight: one process-wide claim allows at most one
// open panel. A request arriving while a panel is open is refused with the typed
// 409 rather than queued or given a second panel, and the claim is released on
// every outcome so the next request still works (TS-03.R26).
func TestDirectoryPickerIsSingleFlight(t *testing.T) {
	srv := testServer(t, true)
	h := srv.routes()
	opened := make(chan struct{})
	release := make(chan struct{})
	first := t.TempDir()
	srv.pickDirectory = func(context.Context) (string, bool, error) {
		close(opened)
		<-release
		return first, false, nil
	}

	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, newLocalRequest(http.MethodPost, "/api/directory-picker", nil))
		done <- rec
	}()

	select {
	case <-opened:
	case <-time.After(5 * time.Second):
		t.Fatal("first picker never opened")
	}

	busy := doPicker(t, h)
	if busy.Code != http.StatusConflict {
		t.Fatalf("concurrent status = %d, want 409 (body %q)", busy.Code, busy.Body.String())
	}
	if got := pickerErrorCode(t, busy); got != runtime.CodeDirectoryPickerBusy {
		t.Errorf("code = %q, want %q", got, runtime.CodeDirectoryPickerBusy)
	}

	close(release)
	if rec := <-done; rec.Code != http.StatusOK {
		t.Fatalf("first picker status = %d, want 200", rec.Code)
	}

	// The claim is released, so a later request opens a panel normally.
	srv.pickDirectory = func(context.Context) (string, bool, error) { return "", true, nil }
	if rec := doPicker(t, h); rec.Code != http.StatusNoContent {
		t.Fatalf("status after release = %d, want 204", rec.Code)
	}
}

// TestDirectoryPickerRequestCancellationClosesPanelAndClaim: the handler holds
// the panel only for its own request. When the caller disconnects (or the server
// shuts down, which cancels the same context) the picker context is cancelled so
// the subprocess dies, nothing is written back, and the claim is freed
// (TS-03.R26).
func TestDirectoryPickerRequestCancellationClosesPanelAndClaim(t *testing.T) {
	srv := testServer(t, true)
	h := srv.routes()
	opened := make(chan struct{})
	pickerCtxDone := make(chan struct{})
	srv.pickDirectory = func(ctx context.Context) (string, bool, error) {
		close(opened)
		<-ctx.Done() // the real runner's exec.CommandContext kills osascript here
		close(pickerCtxDone)
		return "", false, ctx.Err()
	}

	ctx, cancel := context.WithCancel(context.Background())
	req := newLocalRequest(http.MethodPost, "/api/directory-picker", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		h.ServeHTTP(rec, req)
		close(done)
	}()

	select {
	case <-opened:
	case <-time.After(5 * time.Second):
		t.Fatal("picker never opened")
	}
	cancel()

	select {
	case <-pickerCtxDone:
	case <-time.After(5 * time.Second):
		t.Fatal("picker context was not cancelled with the request")
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("handler did not return after cancellation")
	}
	if rec.Body.Len() != 0 {
		t.Errorf("abandoned request wrote a body: %q", rec.Body.String())
	}

	srv.pickDirectory = func(context.Context) (string, bool, error) { return "", true, nil }
	if next := doPicker(t, h); next.Code != http.StatusNoContent {
		t.Fatalf("status after cancellation = %d, want 204 — the claim leaked", next.Code)
	}
}

// TestDirectoryPickerRejectsNonLocalHost: the action is a same-machine authority
// like the rest of the local API, so a DNS-rebinding page cannot make the
// victim's browser open a folder panel (TS-05.R15, INV §14).
func TestDirectoryPickerRejectsNonLocalHost(t *testing.T) {
	srv := testServer(t, true)
	srv.pickDirectory = func(context.Context) (string, bool, error) {
		t.Fatal("picker ran for a non-local request")
		return "", false, nil
	}
	h := srv.routes()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/directory-picker", nil)) // Host: example.com
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-local Host status = %d, want 403", rec.Code)
	}

	rec = httptest.NewRecorder()
	hostile := newLocalRequest(http.MethodPost, "/api/directory-picker", nil)
	hostile.Header.Set("Origin", "http://attacker.example")
	h.ServeHTTP(rec, hostile)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("hostile Origin status = %d, want 403", rec.Code)
	}
}

// TestVerifyPickedDirectory pins the panel-output contract. The trailing
// separator and newline are what a real `POSIX path of (choose folder …)` emits,
// verified against /usr/bin/osascript on macOS 15.
func TestVerifyPickedDirectory(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a-file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	cases := []struct {
		name string
		out  string
		want string
		ok   bool
	}{
		{"folder alias with trailing separator", dir + "/\n", dir, true},
		{"plain absolute path", dir + "\n", dir, true},
		{"root stays root", "/\n", "/", true},
		{"empty output", "\n", "", false},
		{"relative path", "Projects/app\n", "", false},
		{"missing directory", filepath.Join(dir, "gone") + "/\n", "", false},
		{"a file is not a directory", file + "\n", "", false},
	}
	for _, c := range cases {
		got, ok := verifyPickedDirectory(c.out)
		if ok != c.ok || got != c.want {
			t.Errorf("%s: verifyPickedDirectory(%q) = (%q, %v), want (%q, %v)", c.name, c.out, got, ok, c.want, c.ok)
		}
	}
}

// TestIsPickerCancel: only AppleScript's stable numeric userCanceledErr decides
// cancellation, because the surrounding message text is localized and has
// changed across macOS releases (INV §12). The -128 line is the real stderr of
// `osascript -e 'error number -128'`.
func TestIsPickerCancel(t *testing.T) {
	cases := map[string]bool{
		"0:17: execution error: User canceled. (-128)":                    true,
		"0:0: execution error: Der Benutzer hat abgebrochen. (-128)":      true,
		"0:0: execution error: osascript needs assistive access. (-1728)": false,
		"execution error: Can’t make folder into type alias. (-1700)":     false,
		"": false,
		"0:0: execution error: The operation couldn’t be completed. (-600)": false,
	}
	for stderr, want := range cases {
		if got := isPickerCancel(stderr); got != want {
			t.Errorf("isPickerCancel(%q) = %v, want %v", stderr, got, want)
		}
	}
}

// TestPickDirectoryViaOSAScriptHonorsCancelledContext exercises the real command
// boundary without opening a panel: an already-cancelled context makes
// exec.CommandContext refuse to start osascript, and the runner reports the
// context error rather than a cancellation or a bogus path.
func TestPickDirectoryViaOSAScriptHonorsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	path, cancelled, err := pickDirectoryViaOSAScript(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if path != "" || cancelled {
		t.Errorf("got (%q, %v), want no path and no user cancellation", path, cancelled)
	}
}
