package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/AsaphNoam/Chuck/internal/release"
)

// buildArchive assembles a minimal but complete release version directory, packs
// it, and writes the matching release manifest JSON. Returns the archive and
// manifest paths.
func buildArchive(t *testing.T, version string) (string, string) {
	t.Helper()
	work := t.TempDir()
	dir := filepath.Join(work, release.VersionDirName(version))
	files := map[string]string{
		"libexec/chuck":         "#!/bin/sh\necho ran\n",
		"runtime/node/bin/node": "#!/bin/sh\n",
		"runtime/node_modules/@agentclientprotocol/claude-agent-acp/dist/index.js":              "#!/bin/sh\n",
		"runtime/node_modules/@agentclientprotocol/codex-acp/dist/index.js":                     "#!/bin/sh\n",
		"runtime/node_modules/@anthropic-ai/claude-agent-sdk-darwin-arm64/claude":               "#!/bin/sh\n",
		"runtime/node_modules/@openai/codex-darwin-arm64/vendor/aarch64-apple-darwin/bin/codex": "#!/bin/sh\n",
	}
	for rel, body := range files {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := release.WriteWrapper(dir); err != nil {
		t.Fatal(err)
	}
	if err := release.WriteInternalManifest(dir, release.InternalManifest{Version: version, Target: release.Target, Components: map[string]string{
		"node": "22.0.0", "claude-agent-acp": "0.85.1", "codex-acp": "2.1.1+chuck.1",
		"claude": "2.1.286", "codex": "0.159.3", "chuck": version,
	}}); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(work, "chuck-"+version+"-"+release.Target+".tar.gz")
	if err := release.CreateArchive(dir, archive); err != nil {
		t.Fatal(err)
	}
	sum, err := release.ChecksumFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(archive)
	m := release.ReleaseManifest{Version: version, Target: release.Target, Archive: filepath.Base(archive), Size: info.Size(), SHA256: sum}
	manifest := filepath.Join(work, "manifest.json")
	data, _ := json.Marshal(m)
	if err := os.WriteFile(manifest, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return archive, manifest
}

// `chuck release install` installs into CHUCK_APP_ROOT and yields a
// runnable shim (FS-10.A1, TS-06.R17).
func TestReleaseInstallCommand(t *testing.T) {
	archive, manifest := buildArchive(t, "1.0.0")
	appRoot := t.TempDir()
	t.Setenv("CHUCK_APP_ROOT", appRoot)

	root := NewRootCmd()
	root.SetArgs([]string{"release", "install", "--archive", archive, "--manifest", manifest})
	if err := root.Execute(); err != nil {
		t.Fatalf("release install: %v", err)
	}

	shim := filepath.Join(appRoot, "bin", "chuck")
	if _, err := os.Stat(shim); err != nil {
		t.Fatalf("shim missing after install: %v", err)
	}
	out, err := exec.Command(shim).CombinedOutput()
	if err != nil {
		t.Fatalf("installed shim failed: %v\n%s", err, out)
	}
	if string(out) != "ran\n" {
		t.Fatalf("shim output = %q, want %q", out, "ran\n")
	}
}

// A checksum mismatch is rejected and installs nothing (FS-10.R8, TS-05.R12).
func TestReleaseInstallRejectsCorrupt(t *testing.T) {
	archive, manifest := buildArchive(t, "1.0.0")
	// Corrupt the manifest's checksum.
	data, _ := os.ReadFile(manifest)
	var m release.ReleaseManifest
	_ = json.Unmarshal(data, &m)
	m.SHA256 = "0000000000000000000000000000000000000000000000000000000000000000"
	data, _ = json.Marshal(m)
	_ = os.WriteFile(manifest, data, 0o600)

	appRoot := t.TempDir()
	t.Setenv("CHUCK_APP_ROOT", appRoot)
	root := NewRootCmd()
	root.SetArgs([]string{"release", "install", "--archive", archive, "--manifest", manifest})
	if err := root.Execute(); err == nil {
		t.Fatal("release install accepted a corrupt archive")
	}
	if _, err := os.Stat(filepath.Join(appRoot, "current")); !os.IsNotExist(err) {
		t.Fatal("corrupt install left a current pointer")
	}
}
