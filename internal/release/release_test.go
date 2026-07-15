package release

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// newLayout returns a layout rooted at a temp dir with EnsureLayout applied.
func newLayout(t *testing.T) *Layout {
	t.Helper()
	l := NewLayout(t.TempDir())
	if err := l.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout: %v", err)
	}
	return l
}

// stageVersion creates an (immutable) version directory as extraction would.
func stageVersion(t *testing.T, l *Layout, version string) string {
	t.Helper()
	name := VersionDirName(version)
	if err := os.MkdirAll(l.VersionDir(name), 0o700); err != nil {
		t.Fatalf("stage %s: %v", name, err)
	}
	return name
}

func TestAppRootHonorsEnvOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(appRootEnv, dir)
	got, err := AppRoot()
	if err != nil {
		t.Fatalf("AppRoot: %v", err)
	}
	if got != dir {
		t.Fatalf("AppRoot = %q, want %q", got, dir)
	}
}

func TestAppRootDefaultsToApplicationSupport(t *testing.T) {
	t.Setenv(appRootEnv, "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	got, err := AppRoot()
	if err != nil {
		t.Fatalf("AppRoot: %v", err)
	}
	want := filepath.Join(home, "Library", "Application Support", "AgentDeck")
	if got != want {
		t.Fatalf("AppRoot = %q, want %q", got, want)
	}
}

// EnsureLayout creates an owner-only skeleton (TS-05.R12).
func TestEnsureLayoutIsOwnerOnly(t *testing.T) {
	l := newLayout(t)
	for _, dir := range []string{l.Root(), l.VersionsDir(), l.BinDir()} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("stat %s: %v", dir, err)
		}
		if perm := info.Mode().Perm(); perm != 0o700 {
			t.Fatalf("%s mode = %o, want 700", dir, perm)
		}
	}
}

// EnsureLayout tightens an existing loose directory rather than leaving it.
func TestEnsureLayoutTightensExistingDir(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o755); err != nil {
		t.Fatal(err)
	}
	l := NewLayout(root)
	if err := l.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout: %v", err)
	}
	info, _ := os.Stat(root)
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Fatalf("root mode = %o, want 700", perm)
	}
}

func TestVersionDirName(t *testing.T) {
	if got := VersionDirName("1.2.3"); got != "agentdeck-1.2.3-darwin-arm64" {
		t.Fatalf("VersionDirName = %q", got)
	}
}

// The first activation sets current and retains no previous (TS-06.R18).
func TestActivateInitial(t *testing.T) {
	l := newLayout(t)
	name := stageVersion(t, l, "1.0.0")
	if err := l.Activate(name); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	cur, ok, err := l.Current()
	if err != nil || !ok || cur != name {
		t.Fatalf("Current = %q ok=%v err=%v, want %q", cur, ok, err, name)
	}
	if _, hasPrev, _ := l.Previous(); hasPrev {
		t.Fatal("initial activation should retain no previous")
	}
}

// Activating a second version records the first as previous.
func TestActivateRecordsPrevious(t *testing.T) {
	l := newLayout(t)
	v1 := stageVersion(t, l, "1.0.0")
	v2 := stageVersion(t, l, "2.0.0")
	if err := l.Activate(v1); err != nil {
		t.Fatalf("Activate v1: %v", err)
	}
	if err := l.Activate(v2); err != nil {
		t.Fatalf("Activate v2: %v", err)
	}
	if cur, _, _ := l.Current(); cur != v2 {
		t.Fatalf("current = %q, want %q", cur, v2)
	}
	if prev, ok, _ := l.Previous(); !ok || prev != v1 {
		t.Fatalf("previous = %q ok=%v, want %q", prev, ok, v1)
	}
}

// Re-activating the same version is a safe no-op that keeps previous unchanged.
func TestActivateSameVersionKeepsPrevious(t *testing.T) {
	l := newLayout(t)
	v1 := stageVersion(t, l, "1.0.0")
	v2 := stageVersion(t, l, "2.0.0")
	_ = l.Activate(v1)
	_ = l.Activate(v2)
	if err := l.Activate(v2); err != nil {
		t.Fatalf("re-activate v2: %v", err)
	}
	if prev, _, _ := l.Previous(); prev != v1 {
		t.Fatalf("previous changed on re-activation: %q, want %q", prev, v1)
	}
}

// Activate refuses a version directory that was not staged.
func TestActivateMissingVersionFails(t *testing.T) {
	l := newLayout(t)
	if err := l.Activate(VersionDirName("9.9.9")); err == nil {
		t.Fatal("Activate of a missing version should fail")
	}
	if _, ok, _ := l.Current(); ok {
		t.Fatal("failed activation must not set current")
	}
}

// Rollback restores the previous version and records the replaced one (TS-06.R18).
func TestRollbackRestoresPrevious(t *testing.T) {
	l := newLayout(t)
	v1 := stageVersion(t, l, "1.0.0")
	v2 := stageVersion(t, l, "2.0.0")
	_ = l.Activate(v1)
	_ = l.Activate(v2)
	if err := l.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if cur, _, _ := l.Current(); cur != v1 {
		t.Fatalf("current after rollback = %q, want %q", cur, v1)
	}
	if prev, _, _ := l.Previous(); prev != v2 {
		t.Fatalf("previous after rollback = %q, want %q (rollback should be undoable)", prev, v2)
	}
}

func TestRollbackWithoutPrevious(t *testing.T) {
	l := newLayout(t)
	v1 := stageVersion(t, l, "1.0.0")
	_ = l.Activate(v1)
	if err := l.Rollback(); !errors.Is(err, ErrNoPrevious) {
		t.Fatalf("Rollback err = %v, want ErrNoPrevious", err)
	}
	if cur, _, _ := l.Current(); cur != v1 {
		t.Fatalf("current changed on failed rollback: %q", cur)
	}
}

// A second holder cannot take the lock while the first holds it (FS-10.R13).
func TestLockSerializes(t *testing.T) {
	l := newLayout(t)
	lk, err := l.Lock()
	if err != nil {
		t.Fatalf("first Lock: %v", err)
	}
	if _, err := l.Lock(); !errors.Is(err, ErrLocked) {
		t.Fatalf("second Lock err = %v, want ErrLocked", err)
	}
	if err := lk.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
	lk2, err := l.Lock()
	if err != nil {
		t.Fatalf("Lock after release: %v", err)
	}
	_ = lk2.Release()
}
