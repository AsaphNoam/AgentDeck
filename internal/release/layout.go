// Package release manages the macOS release runtime: the application root that
// holds immutable version directories, the current/previous pointers that select
// the active runtime, and the staged install/update/rollback transaction that
// swaps them atomically. It is the single stage→verify→activate core shared by
// the bootstrap installer and `agentdeck update` (INV §2), and it never writes
// user configuration, sessions, or credentials (they live under $AGENTDECK_HOME).
package release

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Target is the only release target this MVP supports (FS-10.R1, TS-06.R13).
const Target = "darwin-arm64"

// appRootEnv overrides the default application root. It mirrors AGENTDECK_HOME so
// tests and non-darwin dev machines can exercise the transaction; production
// installs use the default macOS Application Support location.
const appRootEnv = "AGENTDECK_APP_ROOT"

// defaultAppRootSuffix is appended to ~/Library/Application Support for the
// default application root (TS-06.R16).
var defaultAppRootSuffix = filepath.Join("Library", "Application Support", "AgentDeck")

// Layout resolves the paths under one application root. The application root is
// deliberately distinct from $AGENTDECK_HOME: install, update, rollback, and
// uninstall operate here and must never touch user state (TS-06.R16, TS-05.R12).
type Layout struct {
	root string
}

// AppRoot resolves the application root: $AGENTDECK_APP_ROOT when set, else
// ~/Library/Application Support/AgentDeck.
func AppRoot() (string, error) {
	if v := strings.TrimSpace(os.Getenv(appRootEnv)); v != "" {
		abs, err := filepath.Abs(expandTilde(v))
		if err != nil {
			return "", fmt.Errorf("resolve %s: %w", appRootEnv, err)
		}
		return abs, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, defaultAppRootSuffix), nil
}

// expandTilde expands a leading ~ to the user's home directory.
func expandTilde(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}

// Open resolves the application root and returns its layout.
func Open() (*Layout, error) {
	root, err := AppRoot()
	if err != nil {
		return nil, err
	}
	return NewLayout(root), nil
}

// NewLayout returns a layout rooted at an explicit application root.
func NewLayout(root string) *Layout { return &Layout{root: root} }

// Root returns the application root directory.
func (l *Layout) Root() string { return l.root }

// VersionsDir is the parent of all immutable version directories.
func (l *Layout) VersionsDir() string { return filepath.Join(l.root, "versions") }

// VersionDir is the immutable directory for one release version.
func (l *Layout) VersionDir(name string) string { return filepath.Join(l.VersionsDir(), name) }

// BinDir holds the stable command shim; it is the one PATH entry AgentDeck owns.
func (l *Layout) BinDir() string { return filepath.Join(l.root, "bin") }

// ShimPath is the stable user command that resolves the current version.
func (l *Layout) ShimPath() string { return filepath.Join(l.BinDir(), "agentdeck") }

// CurrentLink points at the active version directory (relative to the root).
func (l *Layout) CurrentLink() string { return filepath.Join(l.root, "current") }

// PreviousLink points at the immediately preceding activated version, for rollback.
func (l *Layout) PreviousLink() string { return filepath.Join(l.root, "previous") }

// StagingDir is the same-filesystem scratch area for downloads/extraction so that
// activation is a rename, never a cross-device copy (TS-06.R17).
func (l *Layout) StagingDir() string { return filepath.Join(l.root, "staging") }

// LockPath is the advisory lock that serializes installers/updaters (TS-06.R19).
func (l *Layout) LockPath() string { return filepath.Join(l.root, "install.lock") }

// VersionDirName is the immutable directory name for a version, e.g.
// "agentdeck-1.2.3-darwin-arm64".
func VersionDirName(version string) string {
	return fmt.Sprintf("agentdeck-%s-%s", version, Target)
}

// EnsureLayout creates the application root and its owner-only skeleton. It never
// touches version contents or the pointers. Directories are 0700 so no other
// account can read or plant a runtime (TS-05.R12).
func (l *Layout) EnsureLayout() error {
	for _, dir := range []string{l.root, l.VersionsDir(), l.BinDir()} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
		// MkdirAll does not re-mode an existing directory; tighten explicitly so
		// an older, looser tree is repaired (TS-05.R5 pattern).
		if err := os.Chmod(dir, 0o700); err != nil {
			return fmt.Errorf("chmod %s: %w", dir, err)
		}
	}
	return nil
}
