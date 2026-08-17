package server

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/agentdeck/agentdeck/internal/runtime"
)

// Directory browsing for project working directories (FS-04.R42, TS-03.R26,
// TS-05.R15).
//
// The browser cannot supply what AgentDeck persists: the File System Access API
// hands back a directory handle, not the absolute host path a project's cwd and
// add_dirs must store. So the panel is shown by the local server instead, as one
// synchronous action with no caller-chosen path anywhere in its input.
//
// Everything here is deliberately narrow: no directory listing, no content read,
// no persistence, no SSE. The route returns exactly the one absolute directory
// the person selected, and its cancel/failure answers carry neither a path nor
// script diagnostics (TS-05.R15).

// osascriptPath is the fixed system executable. It is never resolved through
// PATH and never run through a shell, so nothing on the caller's side can
// redirect the command or inject script text (TS-05.R15).
const osascriptPath = "/usr/bin/osascript"

// chooseFolderPrompt is the panel's fixed prompt. The whole script is these two
// constants; no request data reaches it.
const chooseFolderPrompt = "Choose a directory for AgentDeck"

// Sentinel picker failures. They are intentionally opaque: both the API response
// and the server log see only these fixed strings, never osascript's stderr
// (which names the user's environment) or a rejected path.
var (
	errPickerUnavailable = errors.New("directory picker: the folder panel could not be shown")
	errPickerBadOutput   = errors.New("directory picker: the folder panel did not return a usable directory")
)

// directoryPickerResponse is the success body: {"path":"/abs/dir"}.
type directoryPickerResponse struct {
	Path string `json:"path"`
}

// acquirePicker takes the process-wide picker claim, mirroring acquireSwitch's
// check-then-set under one critical section (INV §5). It is a single flag rather
// than a per-key map because the panel is a per-machine resource: a second panel
// would compete for the same screen and pointer.
func (s *Server) acquirePicker() bool {
	s.pickerMu.Lock()
	defer s.pickerMu.Unlock()
	if s.pickerBusy {
		return false
	}
	s.pickerBusy = true
	return true
}

func (s *Server) releasePicker() {
	s.pickerMu.Lock()
	s.pickerBusy = false
	s.pickerMu.Unlock()
}

// handleDirectoryPicker shows the standard macOS folder panel and returns the
// selected directory. Cancel is 204 with no body, a concurrent request is 409,
// and every other outcome is one safe 500 (TS-03.R26).
func (s *Server) handleDirectoryPicker(w http.ResponseWriter, r *http.Request) {
	if !s.acquirePicker() {
		writeAPIError(w, apiError(runtime.CodeDirectoryPickerBusy, "a folder picker is already open"))
		return
	}
	defer s.releasePicker()

	// r.Context() is the only bound: the person may take as long as they like in
	// the panel, but a closed tab or a server shutdown cancels this context and
	// exec.CommandContext kills the panel with it, so no orphan process survives
	// the request that opened it.
	path, cancelled, err := s.pickDirectory(r.Context())
	switch {
	case err != nil:
		if r.Context().Err() != nil {
			// The caller or the server went away; the panel is already gone and
			// there is nobody left to answer.
			s.log.Debug("directory picker abandoned", "err", r.Context().Err())
			return
		}
		s.log.Error("directory picker", "err", err)
		writeAPIError(w, apiError(runtime.CodeDirectoryPickerFailed, "could not complete the folder selection"))
	case cancelled:
		w.WriteHeader(http.StatusNoContent)
	default:
		writeJSON(w, http.StatusOK, directoryPickerResponse{Path: path})
	}
}

// pickDirectoryViaOSAScript runs the folder panel and classifies its result into
// the closed set the handler answers from: a verified directory, the person's
// cancellation, or one opaque failure. Raw output never crosses back out — the
// same rule probeNativeLogin and the Claude startup classifier follow, for the
// same reason (TS-04.R15, TS-05.R15).
func pickDirectoryViaOSAScript(ctx context.Context) (string, bool, error) {
	// Two -e arguments are two script lines. `activate` foregrounds osascript so
	// the modal panel appears in front of the browser rather than behind it.
	cmd := exec.CommandContext(ctx, osascriptPath, "-e", "activate",
		"-e", `POSIX path of (choose folder with prompt "`+chooseFolderPrompt+`")`)
	cmd.Stdin = nil
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return "", false, ctx.Err()
		}
		if isPickerCancel(stderr.String()) {
			return "", true, nil
		}
		return "", false, errPickerUnavailable
	}
	path, ok := verifyPickedDirectory(stdout.String())
	if !ok {
		return "", false, errPickerBadOutput
	}
	return path, false, nil
}

// isPickerCancel recognizes the person dismissing the panel. AppleScript reports
// that as error -128 (userCanceledErr); the surrounding message text is
// localized and has changed across macOS releases, so only the stable numeric
// code is matched (INV §12).
func isPickerCancel(stderr string) bool {
	return strings.Contains(stderr, "-128")
}

// verifyPickedDirectory turns osascript's stdout into an absolute directory that
// exists, or reports that it could not (TS-03.R26). `POSIX path of` a folder
// ends in a separator, which is stripped so the stored value matches a
// hand-typed one.
func verifyPickedDirectory(out string) (string, bool) {
	path := strings.TrimSpace(out)
	if len(path) > 1 {
		path = strings.TrimRight(path, string(filepath.Separator))
	}
	if path == "" || !filepath.IsAbs(path) {
		return "", false
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return "", false
	}
	return path, true
}
