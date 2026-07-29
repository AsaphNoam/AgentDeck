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
// one-way managed mirror of the personal Codex home's SETUP into the profile:
// configuration, authentication, and the recognized setup assets are copied,
// while session/history/runtime state is never copied, no source symlink is ever
// created or followed outside the personal root, and the personal home is never
// written to. The mirror is built as one complete generation in a staging area
// and then published entry-by-entry, so a concurrently running child never sees a
// half-written generation and a copy failure leaves the prior generation intact.

const (
	dirCodexProfile          = "codex"
	dirCache                 = "cache"
	fileCodexProfileManifest = "codex-profile.json"
	codexProfileManifestVer  = 1
	dirCodexStaging          = "codex-profile.staging"
)

// codexSetupEntries is the explicit ALLOWLIST of top-level personal-CODEX_HOME
// names that hold provider-recognized setup (FS-09.R44, TS-04.R21). Only these
// are mirrored into the isolated profile. An allowlist is deliberate: Codex adds
// new session/history/runtime files across versions (state_5.sqlite*,
// logs_2.sqlite*, shell_snapshots, .tmp, caches, session rollouts, ...), and a
// denylist would silently mirror every new one into the profile — copying
// personal runtime state, risking an inconsistent live SQLite/WAL set, and
// replacing the child's own same-named private state on each start (INV §12,
// TS-02.R19).
var codexSetupEntries = map[string]struct{}{
	"config.toml":     {}, // configuration, incl. [mcp_servers] MCP setup
	"auth.json":       {}, // authentication
	"AGENTS.md":       {}, // global agent instructions
	"instructions.md": {}, // legacy global instructions
	"prompts":         {}, // custom prompts
	"skills":          {}, // skills
	"agents":          {}, // agents
	"rules":           {}, // rules
	"plugins":         {}, // plugins
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

// writeCodexProfileManifest is a seam for the transaction regression test. A
// manifest-write failure must restore the prior profile just like a copy failure
// does; otherwise a start reports failure after exposing a new partial
// generation (INV §5/§15).
var writeCodexProfileManifest = writeJSONAtomic

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
// uncopyable selected asset fails so the caller can abort before spawn — and,
// because the whole generation is staged before anything is published, such a
// failure leaves the previously published profile untouched (INV §5/§15).
func RefreshCodexProfile(profileDir string) error {
	codexProfileMu.Lock()
	defer codexProfileMu.Unlock()
	return refreshCodexProfile(profileDir)
}

// WithRefreshedCodexProfile runs start while the refreshed profile lock is held.
// This closes the refresh-to-exec window: a second Codex child cannot publish a
// generation while the first child is being created to consume the one it just
// prepared (INV §5/§15). The callback must only perform the process start, not
// wait for the process, so concurrent Codex agents remain independently live.
func WithRefreshedCodexProfile(profileDir string, start func() error) error {
	codexProfileMu.Lock()
	defer codexProfileMu.Unlock()
	if err := refreshCodexProfile(profileDir); err != nil {
		return err
	}
	return start()
}

func refreshCodexProfile(profileDir string) error {
	// Resolve and reject source/profile overlap before creating or chmodding the
	// destination. In particular, CODEX_HOME=<agentdeck-home> must not cause us
	// to create the profile inside the personal source before rejecting it.
	root, err := resolveCodexSource(profileDir)
	if err != nil {
		return err
	}

	// The destination must be a real directory we own — never a symlink or
	// non-directory that could alias (and let us mutate/delete through) the
	// personal home or any out-of-tree path (INV §14).
	if err := ensureCodexProfileDir(profileDir); err != nil {
		return err
	}

	home := filepath.Dir(profileDir)
	manifestPath := filepath.Join(home, dirCache, fileCodexProfileManifest)

	stagingDir := filepath.Join(home, dirCache, dirCodexStaging)
	if err := os.RemoveAll(stagingDir); err != nil {
		return fmt.Errorf("config: clear codex staging %q: %w", stagingDir, err)
	}
	if err := os.MkdirAll(stagingDir, 0o700); err != nil {
		return fmt.Errorf("config: create codex staging %q: %w", stagingDir, err)
	}
	defer os.RemoveAll(stagingDir)

	// 1. Build a COMPLETE generation of the setup mirror in staging. Every risky
	//    step (reading/copying/validating personal assets) happens here, so a
	//    failure returns with the live profile untouched (INV §5/§15).
	managed, err := stageCodexSetup(root, stagingDir)
	if err != nil {
		return err
	}

	// 2. Publish the staged generation into the live profile and prune stale
	//    managed copies. Publication is serialized with child creation, so each
	//    new child receives a completed refresh. Existing children must tolerate
	//    a setup entry changing while a later child refreshes this shared profile.
	prior := readCodexManifest(manifestPath)
	if err := publishCodexGeneration(stagingDir, profileDir, manifestPath, managed, prior.Managed); err != nil {
		return err
	}
	return nil
}

// ensureCodexProfileDir provisions the profile as an owner-only directory,
// rejecting a pre-existing symlink or non-directory before any mutation so the
// refresh cannot chmod/copy/prune through an alias of the personal home (INV §14).
func ensureCodexProfileDir(profileDir string) error {
	if fi, err := os.Lstat(profileDir); err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("config: codex profile %q is a symlink; refusing to mutate through it", profileDir)
		}
		if !fi.IsDir() {
			return fmt.Errorf("config: codex profile %q exists and is not a directory", profileDir)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("config: stat codex profile %q: %w", profileDir, err)
	}
	if err := os.MkdirAll(profileDir, 0o700); err != nil {
		return fmt.Errorf("config: create codex profile %q: %w", profileDir, err)
	}
	// MkdirAll never re-modes an existing dir; tighten a profile made by an older
	// build so its auth/config copies stay owner-only.
	if err := os.Chmod(profileDir, 0o700); err != nil {
		return fmt.Errorf("config: chmod codex profile %q: %w", profileDir, err)
	}
	return nil
}

// resolveCodexSource resolves the personal Codex home to a canonical root and
// returns a canonical root. A missing personal home is a safe empty source: the
// caller stages nothing and still prunes stale managed copies. An overlap
// between the canonical source and profile (either contains the other) is
// rejected before mutation, so a profile aliasing the personal home cannot
// delete it (INV §14).
func resolveCodexSource(profileDir string) (root string, err error) {
	source, err := personalCodexHome()
	if err != nil {
		return "", err
	}
	root, err = filepath.EvalSymlinks(source)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // missing personal home: nothing to mirror, prune runs
		}
		return "", fmt.Errorf("config: resolve personal codex home %q: %w", source, err)
	}
	canonProfile, err := filepath.EvalSymlinks(profileDir)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("config: resolve codex profile %q: %w", profileDir, err)
		}
		parent, parentErr := filepath.EvalSymlinks(filepath.Dir(profileDir))
		if parentErr != nil {
			return "", fmt.Errorf("config: resolve codex profile parent %q: %w", filepath.Dir(profileDir), parentErr)
		}
		canonProfile = filepath.Join(parent, filepath.Base(profileDir))
	}
	if withinRoot(canonProfile, root) || withinRoot(root, canonProfile) {
		return "", fmt.Errorf("config: personal codex home %q overlaps profile %q; refusing to mutate", root, canonProfile)
	}
	return root, nil
}

// stageCodexSetup copies every allowlisted top-level setup entry from the
// personal root into a fresh stagingDir and returns the sorted managed names.
// Session/history/runtime entries are excluded by the allowlist. root == ""
// means there is no personal home, so nothing is staged.
func stageCodexSetup(root, stagingDir string) ([]string, error) {
	if root == "" {
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
		if _, ok := codexSetupEntries[name]; !ok {
			continue
		}
		src := filepath.Join(root, name)
		dst := filepath.Join(stagingDir, name)
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

// publishCodexGeneration swaps each staged managed entry into profileDir and
// prunes managed copies of setup that has left the personal home. Only names a
// prior refresh mirrored are eligible for pruning, and each must be a safe single
// path element so a corrupt manifest cannot delete outside the profile (INV §14).
func publishCodexGeneration(stagingDir, profileDir, manifestPath string, managed, prior []string) error {
	for _, name := range managed {
		if !isSafeCodexName(name) {
			return fmt.Errorf("config: unsafe staged codex setup name %q", name)
		}
	}
	managedSet := make(map[string]struct{}, len(managed))
	for _, name := range managed {
		managedSet[name] = struct{}{}
	}
	backupDir := filepath.Join(stagingDir, ".previous")
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return fmt.Errorf("config: create codex generation backup %q: %w", backupDir, err)
	}
	type replacement struct {
		name   string
		hadOld bool
	}
	replaced := make([]replacement, 0, len(managed))
	pruned := make([]string, 0, len(prior))
	rollback := func() {
		for i := len(replaced) - 1; i >= 0; i-- {
			name := replaced[i].name
			dst := filepath.Join(profileDir, name)
			_ = os.RemoveAll(dst)
			if replaced[i].hadOld {
				_ = os.Rename(filepath.Join(backupDir, name), dst)
			}
		}
		for i := len(pruned) - 1; i >= 0; i-- {
			name := pruned[i]
			_ = os.Rename(filepath.Join(backupDir, name), filepath.Join(profileDir, name))
		}
	}
	for _, name := range managed {
		dst := filepath.Join(profileDir, name)
		backup := filepath.Join(backupDir, name)
		hadOld := false
		if _, err := os.Lstat(dst); err == nil {
			if err := os.Rename(dst, backup); err != nil {
				rollback()
				return fmt.Errorf("config: back up codex setup %q: %w", dst, err)
			}
			hadOld = true
		} else if !os.IsNotExist(err) {
			rollback()
			return fmt.Errorf("config: stat codex setup %q: %w", dst, err)
		}
		if err := os.Rename(filepath.Join(stagingDir, name), dst); err != nil {
			if hadOld {
				_ = os.Rename(backup, dst)
			}
			rollback()
			return fmt.Errorf("config: publish codex setup %q: %w", dst, err)
		}
		replaced = append(replaced, replacement{name: name, hadOld: hadOld})
	}
	for _, name := range prior {
		if _, ok := managedSet[name]; ok {
			continue
		}
		if !isSafeCodexName(name) {
			continue
		}
		dst := filepath.Join(profileDir, name)
		if _, err := os.Lstat(dst); os.IsNotExist(err) {
			continue
		} else if err != nil {
			rollback()
			return fmt.Errorf("config: stat stale codex setup %q: %w", dst, err)
		}
		if err := os.Rename(dst, filepath.Join(backupDir, name)); err != nil {
			rollback()
			return fmt.Errorf("config: stage stale codex setup %q: %w", dst, err)
		}
		pruned = append(pruned, name)
	}
	if err := writeCodexProfileManifest(manifestPath, codexProfileManifest{Version: codexProfileManifestVer, Managed: managed}); err != nil {
		rollback()
		return fmt.Errorf("config: publish codex profile manifest: %w", err)
	}
	// The manifest and all new entries are now durable. The old generation is no
	// longer reachable from the live profile, so cleanup cannot expose a partial
	// setup to a child; a future refresh also clears any leftover staging data.
	_ = os.RemoveAll(backupDir)
	return nil
}

// copyCodexEntry mirrors a single source entry into dst, dereferencing symlinks
// that stay inside the personal root and failing on any that escape it. Irregular
// files (sockets, devices, fifos) are a non-fatal skip; anything selected but
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
	switch {
	case info.IsDir():
		if err := copyCodexTree(src, dst, root); err != nil {
			return false, err
		}
		return true, nil
	case info.Mode().IsRegular():
		if err := copyCodexFile(src, dst, info.Mode()); err != nil {
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

// copyCodexFile copies a regular file to dst as an owner-only private copy. The
// source owner-execute bit is preserved so executable setup assets (plugin/skill
// scripts and binaries) stay runnable in the profile, but group/world bits are
// always stripped since these copies can carry secrets (INV §10/§14).
func copyCodexFile(src, dst string, mode os.FileMode) error {
	perm := os.FileMode(0o600)
	if mode&0o100 != 0 {
		perm |= 0o100
	}
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("config: open codex setup %q: %w", src, err)
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
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
	// OpenFile perm is umask-masked; chmod so the executable bit and owner-only
	// restriction are exact regardless of the process umask.
	if err := os.Chmod(dst, perm); err != nil {
		return fmt.Errorf("config: chmod codex setup copy %q: %w", dst, err)
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

// isSafeCodexName reports whether name is a single, non-traversing path element,
// so a corrupt manifest cannot cause a prune to touch anything outside the
// profile directory (INV §14).
func isSafeCodexName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	return !strings.ContainsRune(name, os.PathSeparator) && filepath.Base(name) == name
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
