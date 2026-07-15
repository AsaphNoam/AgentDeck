package cli

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentdeck/agentdeck/internal/release"
)

// fakeFetcher serves a prebuilt archive and manifest without any network.
type fakeFetcher struct {
	m       release.ReleaseManifest
	archive string
}

func (f fakeFetcher) Latest(context.Context) (release.ReleaseManifest, error) { return f.m, nil }

func (f fakeFetcher) Download(_ context.Context, m release.ReleaseManifest, destDir string) (string, error) {
	dest := filepath.Join(destDir, m.Archive)
	in, err := os.Open(f.archive)
	if err != nil {
		return "", err
	}
	defer in.Close()
	out, err := os.Create(dest)
	if err != nil {
		return "", err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return dest, err
}

// manifestOf reads a manifest.json path into a ReleaseManifest.
func manifestOf(t *testing.T, path string) release.ReleaseManifest {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m release.ReleaseManifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// installBase builds version and installs it as the current runtime in appRoot.
func installBase(t *testing.T, appRoot, version string) {
	t.Helper()
	archive, manifest := buildArchive(t, version)
	if _, err := release.NewLayout(appRoot).Install(archive, manifestOf(t, manifest)); err != nil {
		t.Fatalf("install base %s: %v", version, err)
	}
}

func withFetcher(t *testing.T, f release.Fetcher) {
	t.Helper()
	prev := newFetcher
	newFetcher = func(string) release.Fetcher { return f }
	t.Cleanup(func() { newFetcher = prev })
}

func runUpdateCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := NewRootCmd()
	var buf strings.Builder
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetIn(strings.NewReader("")) // non-terminal input → non-interactive
	root.SetArgs(append([]string{"update"}, args...))
	err := root.Execute()
	return buf.String(), err
}

// --check reports availability without installing (FS-10.R7).
func TestUpdateCheckReportsAvailable(t *testing.T) {
	appRoot := t.TempDir()
	t.Setenv("AGENTDECK_APP_ROOT", appRoot)
	installBase(t, appRoot, "1.0.0")

	archive, manifest := buildArchive(t, "2.0.0")
	withFetcher(t, fakeFetcher{m: manifestOf(t, manifest), archive: archive})

	out, err := runUpdateCmd(t, "--check")
	if err != nil {
		t.Fatalf("update --check: %v", err)
	}
	if !strings.Contains(out, "1.0.0 -> 2.0.0") {
		t.Fatalf("check output = %q", out)
	}
	// --check must not install: current stays 1.0.0.
	if v, _, _ := release.NewLayout(appRoot).CurrentVersion(); v != "1.0.0" {
		t.Fatalf("--check installed an update: current = %s", v)
	}
}

// --check reports up-to-date when latest equals current.
func TestUpdateCheckUpToDate(t *testing.T) {
	appRoot := t.TempDir()
	t.Setenv("AGENTDECK_APP_ROOT", appRoot)
	installBase(t, appRoot, "1.0.0")

	archive, manifest := buildArchive(t, "1.0.0")
	withFetcher(t, fakeFetcher{m: manifestOf(t, manifest), archive: archive})

	out, err := runUpdateCmd(t, "--check")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "up to date") {
		t.Fatalf("check output = %q", out)
	}
}

// --yes installs the new version and records the old one as previous (FS-10.R7).
func TestUpdateYesInstalls(t *testing.T) {
	appRoot := t.TempDir()
	t.Setenv("AGENTDECK_APP_ROOT", appRoot)
	installBase(t, appRoot, "1.0.0")

	archive, manifest := buildArchive(t, "2.0.0")
	withFetcher(t, fakeFetcher{m: manifestOf(t, manifest), archive: archive})

	if _, err := runUpdateCmd(t, "--yes"); err != nil {
		t.Fatalf("update --yes: %v", err)
	}
	l := release.NewLayout(appRoot)
	if v, _, _ := l.CurrentVersion(); v != "2.0.0" {
		t.Fatalf("current = %s, want 2.0.0", v)
	}
	if prev, ok, _ := l.Previous(); !ok || prev != release.VersionDirName("1.0.0") {
		t.Fatalf("previous = %s ok=%v, want 1.0.0 dir", prev, ok)
	}
}

// --rollback restores the previous release (FS-10.R7, TS-06.R18).
func TestUpdateRollback(t *testing.T) {
	appRoot := t.TempDir()
	t.Setenv("AGENTDECK_APP_ROOT", appRoot)
	installBase(t, appRoot, "1.0.0")
	installBase(t, appRoot, "2.0.0")

	out, err := runUpdateCmd(t, "--rollback")
	if err != nil {
		t.Fatalf("update --rollback: %v", err)
	}
	if !strings.Contains(out, "rolled back to 1.0.0") {
		t.Fatalf("rollback output = %q", out)
	}
	if v, _, _ := release.NewLayout(appRoot).CurrentVersion(); v != "1.0.0" {
		t.Fatalf("current after rollback = %s, want 1.0.0", v)
	}
}

// A bare `update` with an available release in a non-interactive context refuses
// rather than blocking on a prompt, and installs nothing (FS-10.R7).
func TestUpdateNonInteractiveRefusesWithoutYes(t *testing.T) {
	appRoot := t.TempDir()
	t.Setenv("AGENTDECK_APP_ROOT", appRoot)
	installBase(t, appRoot, "1.0.0")

	archive, manifest := buildArchive(t, "2.0.0")
	withFetcher(t, fakeFetcher{m: manifestOf(t, manifest), archive: archive})

	out, err := runUpdateCmd(t) // no --yes; test stdin is not a terminal
	if err == nil {
		t.Fatal("bare update in a non-interactive context should refuse")
	}
	if !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("refusal should point at --yes, got %q / %q", err.Error(), out)
	}
	if v, _, _ := release.NewLayout(appRoot).CurrentVersion(); v != "1.0.0" {
		t.Fatalf("refused update still changed current to %s", v)
	}
}

// A corrupt download preserves the current runtime (FS-10.R8, TS-05.R12).
func TestUpdateCorruptDownloadPreservesCurrent(t *testing.T) {
	appRoot := t.TempDir()
	t.Setenv("AGENTDECK_APP_ROOT", appRoot)
	installBase(t, appRoot, "1.0.0")

	// The manifest advertises 2.0.0 but carries a wrong checksum, so the served
	// archive fails verification during Install.
	archive, manifest := buildArchive(t, "2.0.0")
	m := manifestOf(t, manifest)
	m.SHA256 = "0000000000000000000000000000000000000000000000000000000000000000"
	withFetcher(t, fakeFetcher{m: m, archive: archive})

	if _, err := runUpdateCmd(t, "--yes"); err == nil {
		t.Fatal("update accepted a checksum-mismatched download")
	}
	if v, _, _ := release.NewLayout(appRoot).CurrentVersion(); v != "1.0.0" {
		t.Fatalf("failed update changed current to %s, want 1.0.0", v)
	}
}
