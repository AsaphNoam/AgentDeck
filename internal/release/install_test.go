package release

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Install performs the full transaction and yields a runnable shim (FS-10.A1).
func TestInstallProducesRunnableShim(t *testing.T) {
	l := newLayout(t)
	name := buildRunnableVersion(t, l, "1.0.0")
	// Repackage the built version into an archive, then install from scratch in a
	// second, empty application root — the true fresh-home path.
	archive := filepath.Join(t.TempDir(), "rel.tar.gz")
	if err := CreateArchive(l.VersionDir(name), archive); err != nil {
		t.Fatal(err)
	}
	sum, _ := ChecksumFile(archive)
	info, _ := os.Stat(archive)
	m := ReleaseManifest{Version: "1.0.0", Target: Target, Archive: "rel.tar.gz", Size: info.Size(), SHA256: sum}

	fresh := NewLayout(t.TempDir())
	got, err := fresh.Install(archive, m)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if got != name {
		t.Fatalf("Install returned %q, want %q", got, name)
	}
	if _, err := os.Stat(fresh.ShimPath()); err != nil {
		t.Fatalf("shim not installed: %v", err)
	}
	out, err := exec.Command(fresh.ShimPath()).CombinedOutput()
	if err != nil {
		t.Fatalf("run installed shim: %v\n%s", err, out)
	}
}

// Install into an application root leaves a separate AGENTDECK_HOME untouched
// (FS-10.R4, TS-06.R16/R21).
func TestInstallDoesNotTouchUserHome(t *testing.T) {
	l := newLayout(t)
	name := buildRunnableVersion(t, l, "1.0.0")
	archive := filepath.Join(t.TempDir(), "rel.tar.gz")
	if err := CreateArchive(l.VersionDir(name), archive); err != nil {
		t.Fatal(err)
	}
	sum, _ := ChecksumFile(archive)
	info, _ := os.Stat(archive)
	m := ReleaseManifest{Version: "1.0.0", Target: Target, Archive: "rel.tar.gz", Size: info.Size(), SHA256: sum}

	// A pre-existing user home with a sentinel config the install must not touch.
	home := t.TempDir()
	sentinel := filepath.Join(home, "config.json")
	if err := os.WriteFile(sentinel, []byte(`{"port":4317}`), 0o600); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(sentinel)

	fresh := NewLayout(t.TempDir())
	if _, err := fresh.Install(archive, m); err != nil {
		t.Fatalf("Install: %v", err)
	}
	after, err := os.ReadFile(sentinel)
	if err != nil {
		t.Fatalf("user home config disturbed: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("install modified a file under AGENTDECK_HOME")
	}
}

// A corrupt archive fails Install before activation, leaving no current runtime
// on a fresh root (FS-10.R8).
func TestInstallCorruptLeavesNoCurrent(t *testing.T) {
	l := newLayout(t)
	name := buildRunnableVersion(t, l, "1.0.0")
	archive := filepath.Join(t.TempDir(), "rel.tar.gz")
	if err := CreateArchive(l.VersionDir(name), archive); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(archive)
	m := ReleaseManifest{Version: "1.0.0", Target: Target, Archive: "rel.tar.gz", Size: info.Size(), SHA256: "deadbeef" + "00000000000000000000000000000000000000000000000000000000"}

	fresh := NewLayout(t.TempDir())
	if _, err := fresh.Install(archive, m); err == nil {
		t.Fatal("Install accepted a corrupt archive")
	}
	if _, ok, _ := fresh.Current(); ok {
		t.Fatal("failed install left a current pointer")
	}
}
