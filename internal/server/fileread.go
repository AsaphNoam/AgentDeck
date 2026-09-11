package server

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
)

// fileReadLimit bounds one file read (TS-05.R21). The read is bounded by this
// explicit limit rather than by the file's own size, so a very large or growing
// file yields a labelled partial result instead of an unbounded allocation
// (INV §16).
const fileReadLimit = 512 * 1024

// fileContent is the success payload of GET /api/sessions/{id}/file (TS-03.R40).
// Path is the slash-separated relative form the viewer displays; Language is a
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
// viewer (TS-03.R40, TS-05.R21, FS-03.R52/R55). The readable root is the working
// directory recorded on that agent's own session snapshot: the caller supplies a
// path and can never supply or influence a root. Unlike file-search this read is
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
	root, apiErr := resolveWorkspaceRoot(snap.Cwd)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	rel, apiErr := relativeReadPath(snap.Cwd, root, r.URL.Query().Get("path"))
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	out, apiErr := readWorkspaceFile(root, rel)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	out.AgentID = id
	writeJSON(w, http.StatusOK, out)
}

// relativeReadPath decides the request's path on its own form, without ever
// touching that path on disk, so the route cannot report whether a file exists
// outside the working directory (TS-05.R21). It returns the cleaned
// slash-separated relative path. cwd and root are used only to re-express an
// absolute request path; neither the request's path nor its target is opened here.
func relativeReadPath(cwd, root, raw string) (string, *runtime.APIError) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", apiError(runtime.CodeValidation, "path is required")
	}
	if strings.ContainsRune(raw, 0) {
		return "", apiError(runtime.CodeValidation, "path is malformed")
	}
	p := filepath.FromSlash(raw)
	if filepath.IsAbs(p) {
		rebased, ok := rebaseAbsolute(cwd, root, p)
		if !ok {
			return "", apiError(runtime.CodePathRefused, "that path is outside this agent's working directory")
		}
		p = rebased
	}
	rel := filepath.Clean(p)
	if rel == "." || rel == string(filepath.Separator) {
		return "", apiError(runtime.CodeNotAFile, "that path names a directory, not a file")
	}
	if escapesRoot(rel) {
		return "", apiError(runtime.CodePathRefused, "that path is outside this agent's working directory")
	}
	if namesGitDir(rel) {
		return "", apiError(runtime.CodePathRefused, "the .git directory is not readable")
	}
	return filepath.ToSlash(rel), nil
}

// rebaseAbsolute expresses an absolute request path relative to the working
// directory. Both the recorded form and the symlink-resolved form are accepted
// bases, because an agent writes the path it saw: on macOS a recorded `/var/...`
// directory resolves to `/private/var/...`, and a link written against either
// spelling names the same file. The result is still lexical and is re-checked
// against the resolved root after symlink resolution.
func rebaseAbsolute(cwd, root, abs string) (string, bool) {
	abs = filepath.Clean(abs)
	bases := []string{root}
	if expanded, err := config.ExpandTilde(strings.TrimSpace(cwd)); err == nil {
		bases = append(bases, filepath.Clean(expanded))
	}
	for _, base := range bases {
		if base == "" || base == "." || !filepath.IsAbs(base) {
			continue
		}
		rel, err := filepath.Rel(base, abs)
		if err != nil || escapesRoot(rel) {
			continue
		}
		return rel, true
	}
	return "", false
}

// escapesRoot reports whether a cleaned relative path leaves its root.
func escapesRoot(rel string) bool {
	if filepath.IsAbs(rel) {
		return true
	}
	return rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// namesGitDir reports whether any segment of a cleaned relative path is `.git`.
func namesGitDir(rel string) bool {
	for _, seg := range strings.Split(filepath.ToSlash(rel), "/") {
		if seg == ".git" {
			return true
		}
	}
	return false
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

// readWorkspaceFile opens rel inside the already-resolved root and returns its
// bounded text. Containment is re-checked after symlink resolution through
// filesearch.go's shipped withinRoot, so a symlink inside the directory cannot
// lead out of it and the read shares one spelling of that rule with the composer
// search (TS-05.R21, INV §2).
func readWorkspaceFile(root, rel string) (fileContent, *runtime.APIError) {
	full := filepath.Join(root, filepath.FromSlash(rel))
	if _, err := os.Lstat(full); err != nil {
		if os.IsNotExist(err) {
			return fileContent{}, apiError(runtime.CodeNotFound, "that file no longer exists")
		}
		return fileContent{}, apiError(runtime.CodeNotFound, "that file could not be opened")
	}
	if !withinRoot(root, rel) {
		// EvalSymlinks failing here means the link target is gone; either way the
		// path is not proven inside the root, so it is refused as such.
		return fileContent{}, apiError(runtime.CodePathRefused, "that path is outside this agent's working directory")
	}
	info, err := os.Stat(full)
	if err != nil {
		return fileContent{}, apiError(runtime.CodeNotFound, "that file no longer exists")
	}
	if info.IsDir() {
		return fileContent{}, apiError(runtime.CodeNotAFile, "that path names a directory, not a file")
	}
	if !info.Mode().IsRegular() {
		return fileContent{}, apiError(runtime.CodeNotAFile, "that path does not name a regular file")
	}
	f, err := os.Open(full)
	if err != nil {
		return fileContent{}, apiError(runtime.CodeNotAFile, "that file could not be read")
	}
	defer f.Close()
	// One byte past the limit distinguishes "exactly at the limit" from "larger",
	// so the partial read is labelled only when content was actually cut.
	buf := make([]byte, fileReadLimit+1)
	n, err := io.ReadFull(f, buf)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return fileContent{}, apiError(runtime.CodeNotAFile, "that file could not be read")
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
		Path:      rel,
		Size:      info.Size(),
		ModTime:   info.ModTime().UTC().Format(time.RFC3339),
		LineCount: countLines(text),
		Content:   text,
		Truncated: truncated,
		Language:  languageForPath(rel),
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
// id. Kept to what AgentDeck's own transcripts actually cite.
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
