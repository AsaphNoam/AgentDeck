package server

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/index"
	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
)

// readFileResponse issues the file read and returns the recorder, so a test can
// assert either the success payload or the typed refusal (TS-03.R40).
func readFileResponse(t *testing.T, h http.Handler, agentID, path string) *httptest.ResponseRecorder {
	t.Helper()
	return doGET(t, h, "/api/sessions/"+agentID+"/file?path="+url.QueryEscape(path))
}

// readFileOK asserts a 200 and returns the decoded payload.
func readFileOK(t *testing.T, h http.Handler, agentID, path string) fileContent {
	t.Helper()
	rec := readFileResponse(t, h, agentID, path)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", rec.Code, rec.Body.String())
	}
	var out fileContent
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return out
}

// readFileRefused asserts the refusal code and status for a path that must not
// be served (FS-03.A37).
func readFileRefused(t *testing.T, h http.Handler, agentID, path, wantCode string, wantStatus int) {
	t.Helper()
	rec := readFileResponse(t, h, agentID, path)
	if rec.Code != wantStatus {
		t.Fatalf("path %q: status = %d body=%s, want %d", path, rec.Code, rec.Body.String(), wantStatus)
	}
	if got := apiErrorCode(t, rec.Body.Bytes()); got != wantCode {
		t.Fatalf("path %q: code = %q, want %q (body=%s)", path, got, wantCode, rec.Body.String())
	}
}

// seedReadableWorkspace builds a working directory with a tracked file, a
// Git-ignored file, a nested file, and a `.git` directory, and seeds an archived
// (not running) chat session pointing at it — the read is deliberately not gated
// on a running record (TS-03.R40).
func seedReadableWorkspace(t *testing.T, srv *Server, agentID string) string {
	t.Helper()
	dir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	writeFile(t, filepath.Join(resolved, "main.go"), "package main\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(resolved, "notes.md"), "# Notes\n\ntext\n")
	writeFile(t, filepath.Join(resolved, "build.log"), "generated output\n")
	writeFile(t, filepath.Join(resolved, ".gitignore"), "build.log\n")
	if err := os.MkdirAll(filepath.Join(resolved, "internal", "state"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, filepath.Join(resolved, "internal", "state", "messages.go"), "package state\n")
	if err := os.MkdirAll(filepath.Join(resolved, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	writeFile(t, filepath.Join(resolved, ".git", "config"), "[core]\n")
	seedArchivedChatSession(t, srv, agentID, resolved)
	return resolved
}

// seedArchivedChatSession creates a chat agent and its sessions row with no
// running record, which is the archived case FS-03.R55 requires to keep working.
func seedArchivedChatSession(t *testing.T, srv *Server, agentID, cwd string) {
	t.Helper()
	agent := state.Agent{
		AgentID: agentID, Name: "Atlas", Role: "implementer", Project: "my-app",
		Backend: "claude", Model: "sonnet-4-6", Interface: "chat",
		CreatedAt: time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC),
	}
	if err := srv.stateStore.WriteAgent(agent); err != nil {
		t.Fatalf("WriteAgent: %v", err)
	}
	ix := index.New(srv.stateStore.DB())
	meta := runtime.SessionMetaData{
		Name: "Atlas", Role: "implementer", Project: "my-app",
		Backend: "claude", Model: "sonnet-4-6", Interface: "chat",
		Cwd: cwd, CreatedAt: "2026-06-28T10:00:00Z", SessionID: "sess-fr-" + agentID,
	}
	if err := ix.UpsertSessionMeta(agentID, meta); err != nil {
		t.Fatalf("UpsertSessionMeta: %v", err)
	}
}

// TestFileReadServesTextInsideWorkingDirectory covers the success payload for an
// archived session, including the extension-derived language hint and the nested
// relative path (TS-03.R40, FS-03.A37).
func TestFileReadServesTextInsideWorkingDirectory(t *testing.T) {
	srv := testServer(t, false)
	seedReadableWorkspace(t, srv, "a_read")
	h := srv.routes()

	got := readFileOK(t, h, "a_read", "main.go")
	if got.Content != "package main\n\nfunc main() {}\n" {
		t.Fatalf("content = %q", got.Content)
	}
	if got.Path != "main.go" || got.Language != "go" || got.LineCount != 3 {
		t.Fatalf("path/language/lines = %q/%q/%d", got.Path, got.Language, got.LineCount)
	}
	if got.Truncated {
		t.Fatalf("small file reported truncated")
	}
	if got.AgentID != "a_read" {
		t.Fatalf("agent_id = %q", got.AgentID)
	}
	if got.ModTime == "" {
		t.Fatalf("mod_time is empty")
	}

	nested := readFileOK(t, h, "a_read", "internal/state/messages.go")
	if nested.Path != "internal/state/messages.go" || nested.Content != "package state\n" {
		t.Fatalf("nested read = %+v", nested)
	}
}

// TestFileReadAcceptsAbsolutePathInsideRoot proves an absolute path is accepted
// only by resolving to the same relative form (TS-03.R40).
func TestFileReadAcceptsAbsolutePathInsideRoot(t *testing.T) {
	srv := testServer(t, false)
	root := seedReadableWorkspace(t, srv, "a_abs")
	h := srv.routes()

	got := readFileOK(t, h, "a_abs", filepath.Join(root, "main.go"))
	if got.Path != "main.go" {
		t.Fatalf("path = %q, want the relative form", got.Path)
	}
}

// TestFileReadServesGitIgnoredFile records the deliberate product decision that
// Git-ignored files inside the working directory are readable (FS-03.R55).
func TestFileReadServesGitIgnoredFile(t *testing.T) {
	srv := testServer(t, false)
	seedReadableWorkspace(t, srv, "a_ignored")
	h := srv.routes()

	got := readFileOK(t, h, "a_ignored", "build.log")
	if got.Content != "generated output\n" {
		t.Fatalf("content = %q", got.Content)
	}
}

// TestFileReadRefusesEscapes is the adversarial set TS-05.R11 requires:
// traversal, absolute-path escape, and `.git` are each refused on the path's
// form, before any filesystem access (TS-05.R21, FS-03.A37).
func TestFileReadRefusesEscapes(t *testing.T) {
	srv := testServer(t, false)
	root := seedReadableWorkspace(t, srv, "a_escape")
	outside := filepath.Join(filepath.Dir(root), "outside-secret.txt")
	writeFile(t, outside, "secret\n")
	h := srv.routes()

	for _, path := range []string{
		"../outside-secret.txt",
		"internal/../../outside-secret.txt",
		"..",
		outside,
		"/etc/passwd",
		".git/config",
		"internal/.git/config",
	} {
		readFileRefused(t, h, "a_escape", path, runtime.CodePathRefused, http.StatusUnprocessableEntity)
	}
}

// TestFileReadRefusesEscapeWithoutProbingExistence proves the form-first refusal
// does not distinguish an existing outside file from an absent one, so the route
// cannot be used to test what exists elsewhere on the machine (TS-05.R21).
func TestFileReadRefusesEscapeWithoutProbingExistence(t *testing.T) {
	srv := testServer(t, false)
	root := seedReadableWorkspace(t, srv, "a_probe")
	existing := filepath.Join(filepath.Dir(root), "exists.txt")
	writeFile(t, existing, "here\n")
	absent := filepath.Join(filepath.Dir(root), "absent.txt")
	h := srv.routes()

	present := readFileResponse(t, h, "a_probe", existing)
	missing := readFileResponse(t, h, "a_probe", absent)
	if present.Code != missing.Code || present.Body.String() != missing.Body.String() {
		t.Fatalf("outside-path responses differ: %d %s vs %d %s",
			present.Code, present.Body.String(), missing.Code, missing.Body.String())
	}
}

// TestFileReadRefusesSymlinkEscape proves the resolve-and-recheck containment
// shared with file-search: a symlink that lives inside the root but points out
// of it is refused (TS-05.R21, INV §2).
func TestFileReadRefusesSymlinkEscape(t *testing.T) {
	srv := testServer(t, false)
	root := seedReadableWorkspace(t, srv, "a_symlink")
	outside := filepath.Join(filepath.Dir(root), "outside-target.txt")
	writeFile(t, outside, "secret\n")
	if err := os.Symlink(outside, filepath.Join(root, "escape.txt")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	h := srv.routes()

	readFileRefused(t, h, "a_symlink", "escape.txt", runtime.CodePathRefused, http.StatusUnprocessableEntity)
}

// TestFileReadRefusesTargetReplacedBeforeOpen proves containment is enforced by
// the open itself: replacing an accepted in-root file with an outside symlink
// after the preliminary check cannot return outside bytes (TS-05.R21, INV §17).
func TestFileReadRefusesTargetReplacedBeforeOpen(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "report.txt")
	outside := filepath.Join(t.TempDir(), "secret.txt")
	writeFile(t, target, "safe\n")
	writeFile(t, outside, "secret\n")

	got, apiErr := readWorkspaceFileAfterValidation(root, "report.txt", func() {
		if err := os.Remove(target); err != nil {
			t.Fatalf("remove checked target: %v", err)
		}
		if err := os.Symlink(outside, target); err != nil {
			t.Skipf("symlink unsupported: %v", err)
		}
	})
	if apiErr == nil {
		t.Fatalf("replacement read succeeded with content %q", got.Content)
	}
	if apiErr.Code != runtime.CodePathRefused {
		t.Fatalf("code = %q, want %q", apiErr.Code, runtime.CodePathRefused)
	}
}

// TestFileReadRefusesNonFileAndMissing covers the directory, missing-file, and
// non-regular outcomes (FS-03.A37).
func TestFileReadRefusesNonFileAndMissing(t *testing.T) {
	srv := testServer(t, false)
	root := seedReadableWorkspace(t, srv, "a_kind")
	h := srv.routes()

	readFileRefused(t, h, "a_kind", "internal", runtime.CodeNotAFile, http.StatusUnprocessableEntity)
	readFileRefused(t, h, "a_kind", "gone.go", runtime.CodeNotFound, http.StatusNotFound)
	readFileRefused(t, h, "a_kind", "", runtime.CodeValidation, http.StatusUnprocessableEntity)

	// A Unix socket is a non-regular file that every supported platform can
	// create without a build-tagged syscall.
	ln, err := net.Listen("unix", filepath.Join(root, "sock"))
	if err != nil {
		t.Logf("unix socket unsupported, skipping non-regular case: %v", err)
		return
	}
	defer ln.Close()
	readFileRefused(t, h, "a_kind", "sock", runtime.CodeNotAFile, http.StatusUnprocessableEntity)
}

// TestFileReadRefusesNonText proves invalid UTF-8 is refused rather than
// transcoded or escaped (TS-05.R21).
func TestFileReadRefusesNonText(t *testing.T) {
	srv := testServer(t, false)
	root := seedReadableWorkspace(t, srv, "a_binary")
	if err := os.WriteFile(filepath.Join(root, "logo.png"), []byte{0x89, 'P', 'N', 'G', 0xff, 0xfe, 0x00, 0x01}, 0o644); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	h := srv.routes()

	readFileRefused(t, h, "a_binary", "logo.png", runtime.CodeNotText, http.StatusUnprocessableEntity)
}

// TestFileReadLabelsPartialRead proves the read is bounded by the explicit byte
// limit and says so, rather than returning the whole file (TS-05.R21, INV §16).
func TestFileReadLabelsPartialRead(t *testing.T) {
	srv := testServer(t, false)
	root := seedReadableWorkspace(t, srv, "a_big")
	big := strings.Repeat("abcdefgh\n", (fileReadLimit/9)+512)
	writeFile(t, filepath.Join(root, "big.txt"), big)
	h := srv.routes()

	got := readFileOK(t, h, "a_big", "big.txt")
	if !got.Truncated {
		t.Fatalf("oversized read not labelled truncated")
	}
	if len(got.Content) > fileReadLimit {
		t.Fatalf("content length %d exceeds limit %d", len(got.Content), fileReadLimit)
	}
	if got.Size != int64(len(big)) {
		t.Fatalf("size = %d, want the file's full size %d", got.Size, len(big))
	}
	if !strings.HasPrefix(got.Content, "abcdefgh\n") {
		t.Fatalf("partial read does not start at the file's beginning")
	}
}

// TestFileReadRefusesMissingWorkspace covers a long-archived session whose
// recorded working directory is gone (FS-03.R55).
func TestFileReadRefusesMissingWorkspace(t *testing.T) {
	srv := testServer(t, false)
	gone := filepath.Join(t.TempDir(), "removed")
	seedArchivedChatSession(t, srv, "a_gone", gone)
	h := srv.routes()

	readFileRefused(t, h, "a_gone", "main.go", runtime.CodeWorkspaceUnavailable, http.StatusUnprocessableEntity)

	seedArchivedChatSession(t, srv, "a_nocwd", "")
	readFileRefused(t, h, "a_nocwd", "main.go", runtime.CodeWorkspaceUnavailable, http.StatusUnprocessableEntity)
}

// TestFileReadRefusesUnknownAndNonChatAgent keeps the route's record gating in
// line with file-search: unknown is 404, a non-chat agent is refused, and a
// stopped chat agent is not (TS-03.R40).
func TestFileReadRefusesUnknownAndNonChatAgent(t *testing.T) {
	srv := testServer(t, false)
	h := srv.routes()

	readFileRefused(t, h, "a_missing", "main.go", runtime.CodeNotFound, http.StatusNotFound)

	term := state.Agent{
		AgentID: "a_term", Name: "Term", Role: "implementer", Project: "my-app",
		Backend: "claude", Interface: "terminal",
		CreatedAt: time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC),
	}
	if err := srv.stateStore.WriteAgent(term); err != nil {
		t.Fatalf("WriteAgent: %v", err)
	}
	readFileRefused(t, h, "a_term", "main.go", runtime.CodeConflict, http.StatusConflict)
}

// TestFileReadServesJSONContentType proves no route serves file bytes under a
// caller-influenced content type (TS-03.R40).
func TestFileReadServesJSONContentType(t *testing.T) {
	srv := testServer(t, false)
	seedReadableWorkspace(t, srv, "a_ctype")
	h := srv.routes()

	rec := readFileResponse(t, h, "a_ctype", "notes.md")
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content-type = %q, want application/json", ct)
	}
}
