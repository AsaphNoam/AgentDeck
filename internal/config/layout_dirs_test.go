package config

import (
	"os"
	"testing"
)

// TestHomeTreeIsOwnerOnly guards the security fix for world-readable
// ~/.agentdeck: the home tree holds secrets (backend env/API keys, tokens,
// transcripts), so EnsureLayout must create it owner-only AND tighten a home
// left behind by an older build (MkdirAll never re-modes an existing dir).
func TestHomeTreeIsOwnerOnly(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(envHome, dir)
	// Simulate an old install: home already exists world-readable.
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatalf("chmod pre-existing home: %v", err)
	}
	s, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := s.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout: %v", err)
	}

	assertOwnerOnly := func(path string) {
		t.Helper()
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		if perm := fi.Mode().Perm(); perm&0o077 != 0 {
			t.Errorf("%s perms = %04o, want no group/other bits", path, perm)
		}
	}
	assertOwnerOnly(s.Home())
	for _, d := range dataDirs {
		assertOwnerOnly(s.dirPath(d))
	}
}
