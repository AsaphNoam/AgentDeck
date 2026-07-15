package release

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// buildFakeVersion assembles a version directory with the full required layout
// and an internal manifest, as release assembly would.
func buildFakeVersion(t *testing.T, parent, version string) string {
	t.Helper()
	name := VersionDirName(version)
	dir := filepath.Join(parent, name)
	for _, rel := range requiredLayout {
		if rel == internalManifestName {
			continue
		}
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := WriteInternalManifest(dir, InternalManifest{
		Version: version, Target: Target,
		Components: testComponents(version),
	}); err != nil {
		t.Fatal(err)
	}
	return dir
}

func testComponents(version string) map[string]string {
	return map[string]string{
		"node": "22.0.0", "claude-agent-acp": "0.59.0", "codex-acp": "1.1.2", "agentdeck": version,
	}
}

// Release assembly produces an archive and a public manifest that fully agree
// before either reaches GitHub Releases (TS-06.R17, R21).
func TestPackageRelease(t *testing.T) {
	version := "1.2.3"
	versionDir := buildFakeVersion(t, t.TempDir(), version)
	outDir := t.TempDir()
	m, err := PackageRelease(versionDir, outDir, version)
	if err != nil {
		t.Fatalf("PackageRelease: %v", err)
	}
	if err := VerifyArchive(filepath.Join(outDir, m.Archive), m); err != nil {
		t.Fatalf("packaged archive failed verification: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(outDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fromDisk ReleaseManifest
	if err := json.Unmarshal(data, &fromDisk); err != nil {
		t.Fatal(err)
	}
	if fromDisk != m {
		t.Fatalf("manifest = %+v, want %+v", fromDisk, m)
	}
}

// releaseFrom packages a built version dir into an archive and returns its path
// and matching release manifest.
func releaseFrom(t *testing.T, srcDir, version string) (string, ReleaseManifest) {
	t.Helper()
	archive := filepath.Join(t.TempDir(), "agentdeck-"+version+"-"+Target+".tar.gz")
	if err := CreateArchive(srcDir, archive); err != nil {
		t.Fatalf("CreateArchive: %v", err)
	}
	sum, err := ChecksumFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(archive)
	return archive, ReleaseManifest{
		Version: version, Target: Target,
		Archive: filepath.Base(archive), Size: info.Size(), SHA256: sum,
	}
}

// A verified archive stages into versions/ and activates as current (FS-10.A1).
func TestStageAndActivateRoundTrip(t *testing.T) {
	src := buildFakeVersion(t, t.TempDir(), "1.2.3")
	archive, m := releaseFrom(t, src, "1.2.3")

	l := newLayout(t)
	name, err := l.StageArchive(archive, m)
	if err != nil {
		t.Fatalf("StageArchive: %v", err)
	}
	if err := VerifyLayout(l.VersionDir(name)); err != nil {
		t.Fatalf("staged layout invalid: %v", err)
	}
	// Executable bit survives the archive round-trip.
	info, err := os.Stat(filepath.Join(l.VersionDir(name), "libexec/agentdeck"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Fatalf("libexec/agentdeck lost its executable bit: %o", info.Mode().Perm())
	}
	if err := l.Activate(name); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if cur, _, _ := l.Current(); cur != name {
		t.Fatalf("current = %q, want %q", cur, name)
	}
}

// Re-staging the same version is idempotent and does not disturb the dir.
func TestStageSameVersionIsIdempotent(t *testing.T) {
	src := buildFakeVersion(t, t.TempDir(), "1.0.0")
	archive, m := releaseFrom(t, src, "1.0.0")
	l := newLayout(t)
	name, err := l.StageArchive(archive, m)
	if err != nil {
		t.Fatal(err)
	}
	// Mark the installed dir so we can prove the second stage did not replace it.
	sentinel := filepath.Join(l.VersionDir(name), "sentinel")
	if err := os.WriteFile(sentinel, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := l.StageArchive(archive, m); err != nil {
		t.Fatalf("second StageArchive: %v", err)
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatal("re-staging replaced an already-installed immutable version")
	}
}

// A corrupt download is rejected before extraction and leaves nothing staged
// (TS-05.R12).
func TestVerifyArchiveRejectsCorrupt(t *testing.T) {
	src := buildFakeVersion(t, t.TempDir(), "1.0.0")
	archive, m := releaseFrom(t, src, "1.0.0")
	m.SHA256 = strings.Repeat("0", 64) // wrong checksum

	l := newLayout(t)
	if _, err := l.StageArchive(archive, m); err == nil {
		t.Fatal("StageArchive accepted a checksum mismatch")
	}
	if entries, _ := os.ReadDir(l.VersionsDir()); len(entries) != 0 {
		t.Fatalf("failed stage left %d version dirs behind", len(entries))
	}
}

// A size mismatch is caught even if the checksum field is well-formed.
func TestVerifyArchiveRejectsSizeMismatch(t *testing.T) {
	src := buildFakeVersion(t, t.TempDir(), "1.0.0")
	archive, m := releaseFrom(t, src, "1.0.0")
	m.Size += 1
	if err := VerifyArchive(archive, m); err == nil {
		t.Fatal("VerifyArchive accepted a size mismatch")
	}
}

// An archive whose top-level dir disagrees with the manifest version is rejected.
func TestStageRejectsVersionMismatch(t *testing.T) {
	src := buildFakeVersion(t, t.TempDir(), "1.0.0")
	archive, m := releaseFrom(t, src, "1.0.0")
	m.Version = "9.9.9" // manifest claims a different version than the archive holds
	// Recompute checksum/size so only the version disagrees.
	sum, _ := ChecksumFile(archive)
	info, _ := os.Stat(archive)
	m.SHA256, m.Size = sum, info.Size()

	l := newLayout(t)
	if _, err := l.StageArchive(archive, m); err == nil {
		t.Fatal("StageArchive accepted a version/top-level mismatch")
	}
}

// A missing required component fails layout verification (TS-06.R15).
func TestVerifyLayoutMissingComponent(t *testing.T) {
	src := buildFakeVersion(t, t.TempDir(), "1.0.0")
	if err := os.Remove(filepath.Join(src, "runtime/node/bin/node")); err != nil {
		t.Fatal(err)
	}
	if err := VerifyLayout(src); err == nil {
		t.Fatal("VerifyLayout passed with node runtime missing")
	}
}

// Extraction rejects a path-traversal entry (INV §9).
func TestExtractRejectsTraversal(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "evil.tar.gz")
	writeTarGz(t, archive, map[string]string{"../escape": "pwned"})
	if _, err := ExtractArchive(archive, t.TempDir()); err == nil {
		t.Fatal("ExtractArchive honored a traversal entry")
	}
}

// Extraction rejects a symlink entry rather than materializing it.
func TestExtractRejectsSymlink(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "link.tar.gz")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{Name: "pkg/link", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd", Mode: 0o777})
	tw.Close()
	gz.Close()
	f.Close()
	if _, err := ExtractArchive(archive, t.TempDir()); err == nil {
		t.Fatal("ExtractArchive materialized a symlink entry")
	}
}

// writeTarGz builds a minimal gzip tar with the given name→content entries.
func writeTarGz(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for name, body := range entries {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	tw.Close()
	gz.Close()
	f.Close()
}
