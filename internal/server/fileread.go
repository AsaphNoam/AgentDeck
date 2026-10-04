package server

import (
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/AsaphNoam/Chuck/internal/config"
	"github.com/AsaphNoam/Chuck/internal/runtime"
	"github.com/AsaphNoam/Chuck/internal/state"
)

// fileReadLimit bounds one file read (TS-05.R21). The read is bounded by this
// explicit limit rather than by the file's own size, so a very large or growing
// file yields a labelled partial result instead of an unbounded allocation
// (INV §16).
const fileReadLimit = 512 * 1024

// fileContent is the success payload of GET /api/sessions/{id}/file (TS-03.R40).
// Path is the normalized requested spelling the viewer displays; Language is a
// highlighting hint derived from the extension alone, never from sniffing.
type fileContent struct {
	AgentID   string `json:"agent_id"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	ModTime   string `json:"mod_time"`
	LineCount int    `json:"line_count"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
	Language  string `json:"language"`
}

// handleFileRead serves GET /api/sessions/{id}/file?path=<p> for the chat file
// viewer (TS-03.R48, TS-05.R24, FS-03.R64). Absolute paths are opened directly;
// relative paths use the working directory recorded on the session. Unlike file-search this read is
// not gated on a running record, because an archived session's links must still
// work (FS-03.R55); it is gated on the session row that records the directory.
func (s *Server) handleFileRead(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	agent, err := s.stateStore.ReadAgent(id)
	if err != nil {
		// Only a missing row means "no such agent": a failing identity read is a
		// storage fault, not a verdict that the agent does not exist (INV §7).
		if !errors.Is(err, state.ErrNotFound) {
			writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
			return
		}
		writeAPIError(w, apiError(runtime.CodeNotFound, "no such agent: "+id))
		return
	}
	if agent.Interface != "chat" {
		writeAPIError(w, apiError(runtime.CodeConflict, "file reading is only available for chat agents"))
		return
	}
	snap, err := s.stateStore.ReadSession(id)
	if err != nil {
		if !errors.Is(err, state.ErrNotFound) {
			writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
			return
		}
		writeAPIError(w, apiError(runtime.CodeNotFound, "no such agent: "+id))
		return
	}
	// The request log records neither the query nor the refusal code, so each
	// refusal is logged here: a report of a link that would not open can then be
	// tied to the path that reached the server and the boundary it hit (INV §8).
	requested := r.URL.Query().Get("path")
	refuse := func(apiErr *runtime.APIError) {
		if remoteNoFollowFileRead(r.Context()) && apiErr.Code == codeRemoteFileNotTracked {
			s.log.Info("file read refused", "agent_id", id, "path", requested, "code", apiErr.Code)
			writeRemoteError(w, http.StatusNotFound, codeRemoteFileNotTracked, "that file is not tracked by this agent")
			return
		}
		s.log.Info("file read refused", "agent_id", id, "path", requested, "code", apiErr.Code)
		writeAPIError(w, apiErr)
	}
	path, display, apiErr := resolveFileReadPath(snap.Cwd, requested)
	if apiErr != nil {
		refuse(apiErr)
		return
	}
	out, apiErr := readLocalFileWithOptions(path, display, remoteNoFollowFileRead(r.Context()))
	if apiErr != nil {
		refuse(apiErr)
		return
	}
	out.AgentID = id
	writeJSON(w, http.StatusOK, out)
}

// resolveFileReadPath preserves a relative request as the display identity while
// resolving it from the recorded workspace. Absolute paths are authority in
// themselves and do not require an available workspace (TS-03.R48).
func resolveFileReadPath(cwd, raw string) (string, string, *runtime.APIError) {
	// Whitespace alone is a missing value; otherwise the supplied spelling is kept,
	// because a real filename may begin or end with spaces (FS-03.R64).
	if strings.TrimSpace(raw) == "" {
		return "", "", apiError(runtime.CodeValidation, "path is required")
	}
	if strings.ContainsRune(raw, 0) {
		return "", "", apiError(runtime.CodeValidation, "path is malformed")
	}
	path := filepath.Clean(filepath.FromSlash(raw))
	if path == "." || path == string(filepath.Separator) {
		return "", "", apiError(runtime.CodeNotAFile, "that path names a directory, not a file")
	}
	if filepath.IsAbs(path) {
		return path, filepath.ToSlash(path), nil
	}
	root, apiErr := resolveWorkspaceRoot(cwd)
	if apiErr != nil {
		return "", "", apiErr
	}
	return filepath.Join(root, path), filepath.ToSlash(path), nil
}

// resolveWorkspaceRoot resolves the session's recorded working directory to its
// canonical form. A missing, unreadable, or non-directory record is the stated
// `workspace_unavailable` refusal a long-archived session produces (FS-03.R55).
func resolveWorkspaceRoot(cwd string) (string, *runtime.APIError) {
	cwd = strings.TrimSpace(cwd)
	if cwd == "" {
		return "", apiError(runtime.CodeWorkspaceUnavailable, "this agent has no recorded working directory")
	}
	if expanded, err := config.ExpandTilde(cwd); err == nil {
		cwd = expanded
	}
	root, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		return "", apiError(runtime.CodeWorkspaceUnavailable, "this agent's working directory is no longer available")
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return "", apiError(runtime.CodeWorkspaceUnavailable, "this agent's working directory is no longer available")
	}
	return root, nil
}

// readWorkspaceFile is retained for focused kind/race tests; production resolves
// both relative and absolute requests through readLocalFile (TS-05.R24).
func readWorkspaceFile(root, rel string) (fileContent, *runtime.APIError) {
	return readLocalFileAfterClassification(filepath.Join(root, filepath.FromSlash(rel)), rel, nil)
}

// readWorkspaceFileAfterClassification exposes the instant between classifying
// the target's kind and opening it — the check-then-act window this read has to
// survive — for the deterministic replacement regression. Production callers
// never supply a hook.
func readWorkspaceFileAfterClassification(root, rel string, afterClassification func()) (fileContent, *runtime.APIError) {
	return readLocalFileAfterClassification(filepath.Join(root, filepath.FromSlash(rel)), rel, afterClassification)
}

func readLocalFile(path, display string) (fileContent, *runtime.APIError) {
	return readLocalFileWithOptions(path, display, false)
}

func readLocalFileWithOptions(path, display string, noFollow bool) (fileContent, *runtime.APIError) {
	return readLocalFileAfterClassificationWithOptions(path, display, nil, noFollow)
}

// readLocalFileAfterClassification checks both before and after open: the first
// check avoids opening known special files, and the descriptor check rejects a
// target replaced during the check/open window (TS-05.R24).
func readLocalFileAfterClassification(path, display string, afterClassification func()) (fileContent, *runtime.APIError) {
	return readLocalFileAfterClassificationWithOptions(path, display, afterClassification, false)
}

func readLocalFileAfterClassificationWithOptions(path, display string, afterClassification func(), noFollow bool) (fileContent, *runtime.APIError) {
	// Classify the target's kind before opening it. Opening is
	// what distinguishes a non-regular file on some platforms but not others: a
	// socket refuses the open on Linux and accepts it on macOS, and a FIFO would
	// block the open until a writer appears, so the descriptor is opened
	// non-blocking and classified again before any read.
	stat := os.Stat
	if noFollow {
		stat = os.Lstat
	}
	if info, statErr := stat(path); statErr == nil && !info.Mode().IsRegular() {
		if noFollow && info.Mode()&os.ModeSymlink != 0 {
			return fileContent{}, apiError(codeRemoteFileNotTracked, "that file is not tracked by this agent")
		}
		if info.IsDir() {
			return fileContent{}, apiError(runtime.CodeNotAFile, "that path names a directory, not a file")
		}
		return fileContent{}, apiError(runtime.CodeNotAFile, "that path does not name a regular file")
	}
	if afterClassification != nil {
		afterClassification()
	}
	flags := syscall.O_RDONLY | syscall.O_NONBLOCK | syscall.O_CLOEXEC
	if noFollow {
		flags |= syscall.O_NOFOLLOW
	}
	fd, err := syscall.Open(path, flags, 0)
	if err != nil {
		if noFollow && errors.Is(err, syscall.ELOOP) {
			return fileContent{}, apiError(codeRemoteFileNotTracked, "that file is not tracked by this agent")
		}
		if os.IsNotExist(err) {
			return fileContent{}, apiError(runtime.CodeNotFound, "that file no longer exists")
		}
		if errors.Is(err, fs.ErrPermission) {
			return fileContent{}, apiError(runtime.CodeFileUnreadable, "that file could not be read")
		}
		return fileContent{}, apiError(runtime.CodeFileUnreadable, "that file could not be read")
	}
	f := os.NewFile(uintptr(fd), path)
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return fileContent{}, apiError(runtime.CodeFileUnreadable, "that file could not be read")
	}
	if info.IsDir() {
		return fileContent{}, apiError(runtime.CodeNotAFile, "that path names a directory, not a file")
	}
	if !info.Mode().IsRegular() {
		return fileContent{}, apiError(runtime.CodeNotAFile, "that path does not name a regular file")
	}
	// One byte past the limit distinguishes "exactly at the limit" from "larger",
	// so the partial read is labelled only when content was actually cut.
	buf := make([]byte, fileReadLimit+1)
	n, err := io.ReadFull(f, buf)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return fileContent{}, apiError(runtime.CodeFileUnreadable, "that file could not be read")
	}
	truncated := n > fileReadLimit
	if truncated {
		n = fileReadLimit
	}
	body := buf[:n]
	if truncated {
		// Do not split a multi-byte rune at the limit: trim the trailing partial
		// rune rather than reporting valid text as invalid.
		body = trimPartialRune(body)
	}
	if !utf8.Valid(body) {
		return fileContent{}, apiError(runtime.CodeNotText, "that file is not text")
	}
	text := string(body)
	return fileContent{
		Path:      display,
		Size:      info.Size(),
		ModTime:   info.ModTime().UTC().Format(time.RFC3339),
		LineCount: countLines(text),
		Content:   text,
		Truncated: truncated,
		Language:  languageForPath(display),
	}, nil
}

// trimPartialRune drops a trailing incomplete UTF-8 sequence left by cutting a
// file at the byte limit.
func trimPartialRune(b []byte) []byte {
	for i := 0; i < utf8.UTFMax && i < len(b); i++ {
		cut := b[:len(b)-i]
		if r, size := utf8.DecodeLastRune(cut); r != utf8.RuneError || size > 1 {
			return cut
		}
	}
	return b
}

// countLines returns the number of lines in text, counting a final unterminated
// line and reporting empty content as zero lines.
func countLines(text string) int {
	if text == "" {
		return 0
	}
	n := strings.Count(text, "\n")
	if !strings.HasSuffix(text, "\n") {
		n++
	}
	return n
}

// languageForPath derives the highlighting hint from the file extension or, for
// extensionless well-known files, the basename (TS-03.R40). Content is never
// sniffed. An unknown extension yields "" and the viewer renders plain text.
func languageForPath(rel string) string {
	base := strings.ToLower(filepath.Base(rel))
	if lang, ok := languageByFilename[base]; ok {
		return lang
	}
	ext := strings.ToLower(filepath.Ext(base))
	return languageByExtension[ext]
}

// languageByExtension maps a lowercased extension to the highlighter's language
// id. Kept to what Chuck's own transcripts actually cite.
var languageByExtension = map[string]string{
	".bash": "bash", ".c": "c", ".cc": "cpp", ".cfg": "ini", ".conf": "ini",
	".cpp": "cpp", ".cs": "csharp", ".css": "css", ".go": "go", ".h": "c",
	".hpp": "cpp", ".htm": "html", ".html": "html", ".ini": "ini", ".java": "java",
	".js": "javascript", ".json": "json", ".jsonc": "json", ".jsx": "jsx",
	".kt": "kotlin", ".lua": "lua", ".md": "markdown", ".markdown": "markdown",
	".mjs": "javascript", ".php": "php", ".pl": "perl", ".py": "python",
	".rb": "ruby", ".rs": "rust", ".scss": "scss", ".sh": "bash", ".sql": "sql",
	".svg": "xml", ".swift": "swift", ".toml": "toml", ".ts": "typescript",
	".tsx": "tsx", ".txt": "", ".xml": "xml", ".yaml": "yaml", ".yml": "yaml",
	".zsh": "bash",
}

// languageByFilename maps well-known extensionless basenames to their language.
var languageByFilename = map[string]string{
	"dockerfile": "dockerfile",
	"makefile":   "makefile",
}
