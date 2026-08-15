package server

import (
	"bytes"
	"context"
	"io/fs"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// File-search bounds (TS-03.R24 / FS-03.R34): traversal and result count are
// bounded so a very large working directory cannot hang the picker, and the git
// listing is time-capped for the same reason.
const (
	fileSearchResultCap = 50
	fileSearchScanCap   = 10000
	fileSearchTimeout   = 2 * time.Second
)

// searchWorkspaceFiles lists files under the session working directory that match
// query, ranked and capped (TS-03.R24). When cwd is inside a Git worktree it uses
// Git's tracked + untracked + effective-ignore view (which skips .git and ignored
// files); otherwise it does a bounded filesystem walk. Neither path follows
// directory symlinks or traverses .git, and every returned path is a slash-
// separated relative path confined to cwd. A missing/unreadable directory returns
// an empty list and no error so the composer can still send (FS-03.R32).
func searchWorkspaceFiles(cwd, query string, limit int) []string {
	if cwd == "" {
		return []string{}
	}
	root, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		return []string{}
	}
	candidates, ok := gitListFiles(root)
	if !ok {
		candidates = walkListFiles(root)
	}
	return rankFiles(root, candidates, query, limit)
}

// gitListFiles returns tracked and untracked-not-ignored files relative to root
// when root is inside a Git worktree. ok is false when Git is unavailable or root
// is not a worktree, so the caller falls back to a plain walk. The `-- .` pathspec
// confines the listing to root's subtree.
func gitListFiles(root string) ([]string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), fileSearchTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", root, "ls-files", "--cached", "--others", "--exclude-standard", "-z", "--", ".")
	out, err := cmd.Output()
	if err != nil {
		return nil, false
	}
	files := make([]string, 0, 256)
	for _, raw := range bytes.Split(out, []byte{0}) {
		if len(raw) == 0 {
			continue
		}
		rel := filepath.ToSlash(string(raw))
		// git ls-files can name files above cwd via "../"; never leave root.
		if strings.HasPrefix(rel, "../") || filepath.IsAbs(rel) {
			continue
		}
		files = append(files, rel)
		if len(files) >= fileSearchScanCap {
			break
		}
	}
	return files, true
}

// walkListFiles walks root's subtree, skipping .git and not descending into
// symlinked directories (WalkDir uses Lstat, so a symlink is a leaf entry, not a
// traversed directory). Only regular files are collected, bounded by the scan cap.
func walkListFiles(root string) []string {
	files := make([]string, 0, 256)
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries, keep walking (INV §7)
		}
		if d.IsDir() {
			if path != root && d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil // symlinks and other non-regular entries are not listed
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		files = append(files, filepath.ToSlash(rel))
		if len(files) >= fileSearchScanCap {
			return filepath.SkipAll
		}
		return nil
	})
	return files
}

// rankFiles filters candidates by query (case-insensitive) and returns the top
// matches, deterministically ordered. An empty query returns a bounded initial
// list ordered by path. Canonical containment is re-checked per result so a
// symlink pointing outside root is never returned.
func rankFiles(root string, candidates []string, query string, limit int) []string {
	if limit <= 0 {
		limit = fileSearchResultCap
	}
	q := strings.ToLower(strings.TrimSpace(query))
	type scored struct {
		path  string
		score int
	}
	matches := make([]scored, 0, len(candidates))
	for _, rel := range candidates {
		s := scoreFile(rel, q)
		if s < 0 {
			continue
		}
		matches = append(matches, scored{path: rel, score: s})
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		return matches[i].path < matches[j].path
	})
	out := make([]string, 0, limit)
	for _, m := range matches {
		if !withinRoot(root, m.path) {
			continue
		}
		out = append(out, m.path)
		if len(out) >= limit {
			break
		}
	}
	return out
}

// scoreFile ranks rel against a lowercased query: exact basename, basename prefix,
// basename substring, then path substring. A negative score means no match. An
// empty query matches everything at a neutral score (initial listing).
func scoreFile(rel, q string) int {
	if q == "" {
		return 0
	}
	lower := strings.ToLower(rel)
	base := strings.ToLower(filepath.Base(rel))
	switch {
	case base == q:
		return 4
	case strings.HasPrefix(base, q):
		return 3
	case strings.Contains(base, q):
		return 2
	case strings.Contains(lower, q):
		return 1
	default:
		return -1
	}
}

// withinRoot confirms rel resolves to a real path inside root, defending against a
// symlink candidate that points outside the working directory (TS-03.R24).
func withinRoot(root, rel string) bool {
	full := filepath.Join(root, filepath.FromSlash(rel))
	resolved, err := filepath.EvalSymlinks(full)
	if err != nil {
		return false
	}
	relToRoot, err := filepath.Rel(root, resolved)
	if err != nil {
		return false
	}
	return relToRoot != ".." && !strings.HasPrefix(relToRoot, ".."+string(filepath.Separator))
}
