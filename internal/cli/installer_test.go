package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/release"
)

// buildBootstrapFixture makes the small self-contained release the shell
// bootstrap needs in order to exercise its platform/download/activation path.
// Its fake binary records post-install calls, letting this test prove that a
// non-interactive install never signs in, starts the dashboard, opens a browser,
// or edits a shell profile (FS-10.A1, A5).
func buildBootstrapFixture(t *testing.T, version, dir string) (archive, manifest string) {
	t.Helper()
	versionDir := filepath.Join(dir, release.VersionDirName(version))
	files := map[string]string{
		"libexec/agentdeck": `#!/bin/sh
set -eu
case "${1:-}:${2:-}" in
release:install)
  mkdir -p "$AGENTDECK_APP_ROOT/bin"
  cp "$0" "$AGENTDECK_APP_ROOT/bin/agentdeck"
  ;;
auth:*|dashboard:*) printf '%s\n' "$*" >> "$AGENTDECK_TEST_CALL_LOG" ;;
--version:*) echo "agentdeck version test" ;;
esac
`,
		"runtime/node/bin/node":                      "#!/bin/sh\n",
		"runtime/node_modules/.bin/claude-agent-acp": "#!/bin/sh\n",
		"runtime/node_modules/.bin/codex-acp":        "#!/bin/sh\n",
	}
	for rel, body := range files {
		path := filepath.Join(versionDir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := release.WriteWrapper(versionDir); err != nil {
		t.Fatal(err)
	}
	if err := release.WriteInternalManifest(versionDir, release.InternalManifest{
		Version: version, Target: release.Target,
		Components: map[string]string{"node": "22.0.0", "claude-agent-acp": "0.59.0", "codex-acp": "1.1.2", "agentdeck": version},
	}); err != nil {
		t.Fatal(err)
	}
	archive = filepath.Join(dir, "agentdeck-"+version+"-"+release.Target+".tar.gz")
	if err := release.CreateArchive(versionDir, archive); err != nil {
		t.Fatal(err)
	}
	sum, err := release.ChecksumFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(archive)
	if err != nil {
		t.Fatal(err)
	}
	m := release.ReleaseManifest{Version: version, Target: release.Target, Archive: filepath.Base(archive), Size: info.Size(), SHA256: sum}
	manifest = filepath.Join(dir, "manifest.json")
	data, _ := json.Marshal(m)
	if err := os.WriteFile(manifest, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return archive, manifest
}

func writeBootstrapCommand(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nset -eu\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestBootstrapNonInteractiveDoesNotStartOrEditProfile(t *testing.T) {
	fixture := t.TempDir()
	archive, manifest := buildBootstrapFixture(t, "1.0.0", fixture)
	fakeBin := filepath.Join(fixture, "bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	writeBootstrapCommand(t, fakeBin, "uname", `case "$1" in -s) echo Darwin ;; -m) echo arm64 ;; *) exit 1 ;; esac`)
	writeBootstrapCommand(t, fakeBin, "curl", `
out=""
while [ "$#" -gt 0 ]; do
  case "$1" in -o) out="$2"; shift 2 ;; *) url="$1"; shift ;; esac
done
case "$url" in *manifest.json) cp "$AGENTDECK_TEST_MANIFEST" "$out" ;; *) cp "$AGENTDECK_TEST_ARCHIVE" "$out" ;; esac`)

	home := filepath.Join(fixture, "home")
	appRoot := filepath.Join(fixture, "app")
	callLog := filepath.Join(fixture, "calls")
	installer := filepath.Join("..", "..", "scripts", "release", "install.sh")
	cmd := exec.Command("bash", installer, "--version", "1.0.0", "--non-interactive")
	cmd.Env = append(os.Environ(),
		"PATH="+fakeBin+":"+os.Getenv("PATH"),
		"HOME="+home,
		"AGENTDECK_APP_ROOT="+appRoot,
		"AGENTDECK_TEST_ARCHIVE="+archive,
		"AGENTDECK_TEST_MANIFEST="+manifest,
		"AGENTDECK_TEST_CALL_LOG="+callLog,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bootstrap install: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(appRoot, "bin", "agentdeck")); err != nil {
		t.Fatalf("stable shim missing: %v", err)
	}
	if calls, err := os.ReadFile(callLog); err == nil && strings.TrimSpace(string(calls)) != "" {
		t.Fatalf("non-interactive install invoked post-install commands: %q", calls)
	}
	if _, err := os.Stat(filepath.Join(home, ".zshrc")); !os.IsNotExist(err) {
		t.Fatalf("non-interactive install edited a shell profile: %v", err)
	}
	if !strings.Contains(string(out), "Start AgentDeck when ready") {
		t.Fatalf("missing manual-start guidance: %s", out)
	}
}

// A second bootstrap exits while the first holds the shared lock through its
// download, so it cannot independently reach activation (FS-10.R13, TS-06.R19).
func TestBootstrapContenderExitsDuringDownload(t *testing.T) {
	if _, err := exec.LookPath("lockf"); err != nil {
		t.Skip("macOS lockf is required for the bootstrap contention test")
	}
	fixture := t.TempDir()
	archive, manifest := buildBootstrapFixture(t, "1.0.0", fixture)
	fakeBin := filepath.Join(fixture, "bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	writeBootstrapCommand(t, fakeBin, "uname", `case "$1" in -s) echo Darwin ;; -m) echo arm64 ;; *) exit 1 ;; esac`)
	writeBootstrapCommand(t, fakeBin, "curl", `
if [ -n "${AGENTDECK_TEST_CURL_STARTED:-}" ]; then
  : > "$AGENTDECK_TEST_CURL_STARTED"
  sleep 2
fi
out=""
while [ "$#" -gt 0 ]; do
  case "$1" in -o) out="$2"; shift 2 ;; *) url="$1"; shift ;; esac
done
case "$url" in *manifest.json) cp "$AGENTDECK_TEST_MANIFEST" "$out" ;; *) cp "$AGENTDECK_TEST_ARCHIVE" "$out" ;; esac`)

	home := filepath.Join(fixture, "home")
	appRoot := filepath.Join(fixture, "app")
	started := filepath.Join(fixture, "curl-started")
	installer := filepath.Join("..", "..", "scripts", "release", "install.sh")
	baseEnv := append(os.Environ(),
		"PATH="+fakeBin+":"+os.Getenv("PATH"),
		"HOME="+home,
		"AGENTDECK_APP_ROOT="+appRoot,
		"AGENTDECK_TEST_ARCHIVE="+archive,
		"AGENTDECK_TEST_MANIFEST="+manifest,
	)
	first := exec.Command("bash", installer, "--version", "1.0.0", "--non-interactive")
	first.Env = append(baseEnv, "AGENTDECK_TEST_CURL_STARTED="+started)
	if err := first.Start(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(started); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
		if i == 49 {
			_ = first.Process.Kill()
			t.Fatal("first bootstrap did not begin downloading")
		}
	}

	second := exec.Command("bash", installer, "--version", "1.0.0", "--non-interactive")
	second.Env = baseEnv
	if out, err := second.CombinedOutput(); err == nil {
		t.Fatalf("contending bootstrap unexpectedly succeeded: %s", out)
	}
	if err := first.Wait(); err != nil {
		t.Fatalf("first bootstrap: %v", err)
	}
}
