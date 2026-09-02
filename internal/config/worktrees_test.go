package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TS-12.R8: the owned root is created owner-only and the leaf path is returned
// for Git to create, not created here.
func TestEnsureWorktreeRootCreatesOwnerOnlyRoot(t *testing.T) {
	home := t.TempDir()
	s := NewWithHome(home)

	leaf, err := s.EnsureWorktreeRoot("myproj")
	if err != nil {
		t.Fatalf("EnsureWorktreeRoot: %v", err)
	}
	want := filepath.Join(home, "worktrees", "myproj")
	if leaf != want {
		t.Fatalf("leaf = %q, want %q", leaf, want)
	}
	fi, err := os.Stat(filepath.Join(home, "worktrees"))
	if err != nil {
		t.Fatalf("stat root: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0o700 {
		t.Fatalf("root mode = %o, want 0700", perm)
	}
	// `git worktree add` requires a non-existent or empty target, so the leaf
	// must not be pre-created.
	if _, err := os.Stat(leaf); !os.IsNotExist(err) {
		t.Fatalf("leaf was created: %v", err)
	}
}

// An id that is not a valid slug never reaches path construction (FS-11.R8).
func TestWorktreePathRejectsInvalidID(t *testing.T) {
	s := NewWithHome(t.TempDir())
	for _, id := range []string{"", "../escape", "Upper", "with/slash"} {
		if _, err := s.WorktreePath(id); err == nil {
			t.Fatalf("WorktreePath(%q) accepted an invalid id", id)
		}
	}
}

// A symlink planted at the leaf is refused, not followed: otherwise a later
// consented deletion would remove whatever the link points at (INV §14).
func TestEnsureWorktreeRootRejectsSymlinkLeaf(t *testing.T) {
	home := t.TempDir()
	s := NewWithHome(home)
	if _, err := s.EnsureWorktreeRoot("myproj"); err != nil {
		t.Fatalf("EnsureWorktreeRoot: %v", err)
	}
	target := t.TempDir()
	link := filepath.Join(home, "worktrees", "myproj")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	_, err := s.EnsureWorktreeRoot("myproj")
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("err = %v, want a symlink refusal", err)
	}
}
