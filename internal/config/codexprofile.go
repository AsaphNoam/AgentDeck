package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Codex session-isolation profile (FS-09.R43/R44, TS-02.R19, TS-04.R20/R21).
//
// AgentDeck runs every codex-acp child with CODEX_HOME pointed at a private,
// owner-only directory (`<home>/codex`) so the child's rollouts and native
// session index stay there and AgentDeck-created Codex conversations never enter
// the user's personal `codex` resume picker or app history. The child still needs
// the user's real Codex setup, so before every child start this file makes a
// one-way managed mirror of the personal Codex home's setup into the profile:
// configuration, authentication, and any setup assets are copied, while
// session/history data is never copied, no source symlink is ever created or
// followed outside the personal root, and the personal home is never written to.

const (
	dirCodexProfile          = "codex"
	dirCache                 = "cache"
	fileCodexProfileManifest = "codex-profile.json"
	codexProfileManifestVer  = 1
)

// codexSessionEntries are the top-level personal-CODEX_HOME names that hold
// session/history/volatile state rather than setup. They are never mirrored, so
// AgentDeck-created Codex conversations stay private to the isolated profile and
// the user's own history is never duplicated (FS-09.R44, TS-04.R21).
var codexSessionEntries = map[string]struct{}{
	"sessions":            {},
	"session_index.jsonl": {},
	"history.jsonl":       {},
	"history":             {},
	"rollouts":            {},
	"archived_sessions":   {},
	"logs":                {},
	"log":                 {},
	"cache":               {},
	".cache":              {},
	"tmp":                 {},
	".git":                {},
}

// codexProfileManifest records only the top-level destination names AgentDeck
// refreshed into the profile, so a later personal removal removes the stale
// private copy without ever touching the child's own session/history data or any
// other unmanaged Codex state (TS-02.R19).
type codexProfileManifest struct {
	Version int      `json:"version"`
	Managed []string `json:"managed"`
}

// codexProfileMu serializes profile refreshes so concurrent codex-acp starts do
// not race the mirror and manifest for the shared profile (TS-04.R21).
var codexProfileMu sync.Mutex

// CodexProfileDir returns the AgentDeck-owned CODEX_HOME for codex-acp children
// under the given AgentDeck home. It is the single source of truth for both the
// composed child-env value and the refresh target (INV §2).
func CodexProfileDir(home string) string {
	return filepath.Join(home, dirCodexProfile)
}

// RefreshCodexProfile provisions profileDir (0700) and one-way refreshes it from
// the user's effective personal Codex home (`${CODEX_HOME:-~/.codex}`), copying
// setup into owner-only files and pruning private copies of setup the user has
// since removed. profileDir is the composed child CODEX_HOME (CodexProfileDir).
// The refresh is idempotent and safe to call before every child start; a missing
// personal home or asset is a non-fatal skip, while an unsafe (root-escaping) or
// uncopyable selected asset fails so the caller can abort before spawn.
func RefreshCodexProfile(profileDir string) error {
	codexProfileMu.Lock()
	defer codexProfileMu.Unlock()

	if err := os.MkdirAll(profileDir, 0o700); err != nil {
		return fmt.Errorf("config: create codex profile %q: %w", profileDir, err)
	}
	// MkdirAll never re-modes an existing dir; tighten a profile made by an
	// older build so its auth/config copies stay owner-only.
	if err := os.Chmod(profileDir, 0o700); err != nil {
		return fmt.Errorf("config: chmod codex profile %q: %w", profileDir, err)
	}

	home := filepath.Dir(profileDir)
	manifestPath := filepath.Join(home, dirCache, fileCodexProfileManifest)
	prior := readCodexManifest(manifestPath)

	managed, err := mirrorCodexSetup(profileDir)
	if err != nil {
		return err
	}

	// Prune private copies of setup that has left the personal home. Only names
	// AgentDeck itself mirrored are eligible, so Codex-owned session/history data
	// in the profile is never removed.
	managedSet := make(map[string]struct{}, len(managed))
	for _, name := range managed {
		managedSet[name] = struct{}{}
	}
	for _, name := range prior.Managed {
		if _, ok := managedSet[name]; ok {
			continue
		}
		if err := os.RemoveAll(filepath.Join(profileDir, name)); err != nil {
			return fmt.Errorf("config: prune stale codex setup %q: %w", name, err)
		}
	}

	return writeJSONAtomic(manifestPath, codexProfileManifest{Version: codexProfileManifestVer, Managed: managed})
}

// mirrorCodexSetup copies every top-level setup entry from the personal Codex
// home into profileDir and returns the sorted managed destination names. Each
// managed entry is removed and re-copied so inner edits and deletions in the
// personal source are reflected exactly. Session/history entries are skipped.
func mirrorCodexSetup(profileDir string) ([]string, error) {
	source, err := personalCodexHome()
	if err != nil {
		return nil, err
	}
	// A missing personal home is a non-fatal skip: there is simply nothing to
	// mirror, but any prior managed copy is still pruned by the caller.
	root, err := filepath.EvalSymlinks(source)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("config: resolve personal codex home %q: %w", source, err)
	}
	// Refuse to mirror the profile into itself (e.g. a misconfigured home).
	if canonProfile, err := filepath.EvalSymlinks(profileDir); err == nil && canonProfile == root {
		return []string{}, nil
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("config: read personal codex home %q: %w", root, err)
	}

	managed := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if _, skip := codexSessionEntries[name]; skip {
			continue
		}
		src := filepath.Join(root, name)
		dst := filepath.Join(profileDir, name)
		copied, err := copyCodexEntry(src, dst, root)
		if err != nil {
			return nil, err
		}
		if copied {
			managed = append(managed, name)
		}
	}
	sort.Strings(managed)
	return managed, nil
}

// copyCodexEntry mirrors a single source entry into dst, dereferencing symlinks
// that stay inside the personal root and failing on any that escape it. It first
// removes dst so a stale copy cannot survive a source deletion. Irregular files
// (sockets, devices, fifos) are a non-fatal skip; anything selected but
// uncopyable is an error so the caller aborts before spawn (TS-04.R21).
func copyCodexEntry(src, dst, root string) (bool, error) {
	info, err := os.Lstat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil // vanished between ReadDir and copy: skip
		}
		return false, fmt.Errorf("config: stat codex setup %q: %w", src, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		resolved, err := filepath.EvalSymlinks(src)
		if err != nil {
			return false, fmt.Errorf("config: resolve codex setup link %q: %w", src, err)
		}
		if !withinRoot(resolved, root) {
			return false, fmt.Errorf("config: refusing codex setup link %q that escapes %q", src, root)
		}
		if info, err = os.Stat(resolved); err != nil {
			return false, fmt.Errorf("config: stat codex setup link target %q: %w", resolved, err)
		}
		src = resolved
	}
	if err := os.RemoveAll(dst); err != nil {
		return false, fmt.Errorf("config: clear codex setup %q: %w", dst, err)
	}
	switch {
	case info.IsDir():
		if err := copyCodexTree(src, dst, root); err != nil {
			return false, err
		}
		return true, nil
	case info.Mode().IsRegular():
		if err := copyCodexFile(src, dst); err != nil {
			return false, err
		}
		return true, nil
	default:
		return false, nil // not a setup asset
	}
}

// copyCodexTree recursively mirrors dir into dst with owner-only modes, applying
// the same symlink-escape safety to every nested entry.
func copyCodexTree(dir, dst, root string) error {
	if err := os.MkdirAll(dst, 0o700); err != nil {
		return fmt.Errorf("config: create codex setup dir %q: %w", dst, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("config: read codex setup dir %q: %w", dir, err)
	}
	for _, entry := range entries {
		src := filepath.Join(dir, entry.Name())
		if _, err := copyCodexEntry(src, filepath.Join(dst, entry.Name()), root); err != nil {
			return err
		}
	}
	return nil
}

// copyCodexFile copies a regular file to dst as an owner-only (0600) private
// copy — Codex auth/config carry secrets, so the mirror is never group/world
// readable.
func copyCodexFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("config: open codex setup %q: %w", src, err)
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("config: create codex setup copy %q: %w", dst, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return fmt.Errorf("config: copy codex setup %q: %w", src, err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("config: finalize codex setup copy %q: %w", dst, err)
	}
	return nil
}

// withinRoot reports whether p is root or lies inside it, both already resolved
// to canonical absolute paths.
func withinRoot(p, root string) bool {
	p = filepath.Clean(p)
	root = filepath.Clean(root)
	return p == root || strings.HasPrefix(p, root+string(os.PathSeparator))
}

// personalCodexHome resolves the user's effective personal Codex home from the
// AgentDeck process environment: `$CODEX_HOME` if set, else `~/.codex`. AgentDeck
// never rewrites its own CODEX_HOME, so this always names the real personal home
// even while children run with the isolated one (FS-09.R44).
func personalCodexHome() (string, error) {
	if h := os.Getenv("CODEX_HOME"); h != "" {
		expanded, err := ExpandTilde(h)
		if err != nil {
			return "", err
		}
		return filepath.Abs(expanded)
	}
	u, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(u, ".codex"), nil
}

// readCodexManifest loads the managed-path manifest, returning an empty manifest
// when it is absent or unreadable (a fresh or corrupt manifest just means nothing
// is pruned this pass; the mirror still runs and rewrites it).
func readCodexManifest(path string) codexProfileManifest {
	var m codexProfileManifest
	if err := readJSON(path, &m); err != nil {
		return codexProfileManifest{Version: codexProfileManifestVer}
	}
	return m
}
