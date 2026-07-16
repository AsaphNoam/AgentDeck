package config

import (
	"os"
	"path/filepath"
	"testing"
)

// FS-11.A1 / TS-02.R13: creating resources yields an owner-only empty directory
// at the canonical path, and the call is idempotent for a pre-existing project.
func TestEnsureProjectResourcesCreatesOwnerOnly(t *testing.T) {
	home := t.TempDir()
	s := NewWithHome(home)

	got, err := s.EnsureProjectResources("myproj")
	if err != nil {
		t.Fatalf("EnsureProjectResources: %v", err)
	}
	want := filepath.Join(home, "project-resources", "myproj")
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}

	for _, dir := range []string{filepath.Join(home, "project-resources"), got} {
		fi, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("stat %q: %v", dir, err)
		}
		if !fi.IsDir() {
			t.Fatalf("%q is not a directory", dir)
		}
		if perm := fi.Mode().Perm(); perm != 0o700 {
			t.Fatalf("%q mode = %o, want 0700", dir, perm)
		}
	}

	// The leaf is empty: the writability probe leaves no residue behind.
	entries, err := os.ReadDir(got)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("leaf not empty after ensure: %v", entries)
	}

	// Idempotent: a second call on the existing directory succeeds.
	if _, err := s.EnsureProjectResources("myproj"); err != nil {
		t.Fatalf("second EnsureProjectResources: %v", err)
	}
}

// FS-11.R8: an id that fails slug validation never constructs a path.
func TestEnsureProjectResourcesRejectsInvalidID(t *testing.T) {
	s := NewWithHome(t.TempDir())
	for _, bad := range []string{"", "../escape", "has/slash", "Upper", "dot.dot"} {
		if _, err := s.EnsureProjectResources(bad); err == nil {
			t.Errorf("EnsureProjectResources(%q) = nil error, want rejection", bad)
		}
		if _, err := s.ProjectResourcesPath(bad); err == nil {
			t.Errorf("ProjectResourcesPath(%q) = nil error, want rejection", bad)
		}
	}
}

// TS-02.R13 / TS-05.R13: a symlinked parent or leaf is rejected, never followed.
func TestEnsureProjectResourcesRejectsSymlink(t *testing.T) {
	t.Run("leaf symlink", func(t *testing.T) {
		home := t.TempDir()
		s := NewWithHome(home)
		root := filepath.Join(home, "project-resources")
		if err := os.Mkdir(root, 0o700); err != nil {
			t.Fatalf("mkdir root: %v", err)
		}
		target := t.TempDir()
		if err := os.Symlink(target, filepath.Join(root, "proj")); err != nil {
			t.Fatalf("symlink: %v", err)
		}
		if _, err := s.EnsureProjectResources("proj"); err == nil {
			t.Fatalf("EnsureProjectResources followed a leaf symlink, want rejection")
		}
	})

	t.Run("parent symlink", func(t *testing.T) {
		home := t.TempDir()
		s := NewWithHome(home)
		target := t.TempDir()
		if err := os.Symlink(target, filepath.Join(home, "project-resources")); err != nil {
			t.Fatalf("symlink: %v", err)
		}
		if _, err := s.EnsureProjectResources("proj"); err == nil {
			t.Fatalf("EnsureProjectResources followed a parent symlink, want rejection")
		}
	})
}

// TS-02.R13: a non-directory at the parent or leaf path is rejected.
func TestEnsureProjectResourcesRejectsNonDir(t *testing.T) {
	home := t.TempDir()
	s := NewWithHome(home)
	// A regular file where the project-resources parent should be.
	if err := os.WriteFile(filepath.Join(home, "project-resources"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if _, err := s.EnsureProjectResources("proj"); err == nil {
		t.Fatalf("EnsureProjectResources accepted a non-directory parent, want rejection")
	}
}

// TS-03.R12: ProjectResourcesPath computes the path without creating anything.
func TestProjectResourcesPathDoesNotCreate(t *testing.T) {
	home := t.TempDir()
	s := NewWithHome(home)
	got, err := s.ProjectResourcesPath("proj")
	if err != nil {
		t.Fatalf("ProjectResourcesPath: %v", err)
	}
	if got != filepath.Join(home, "project-resources", "proj") {
		t.Fatalf("unexpected path %q", got)
	}
	if _, err := os.Stat(filepath.Join(home, "project-resources")); !os.IsNotExist(err) {
		t.Fatalf("ProjectResourcesPath created the directory (stat err = %v)", err)
	}
}
