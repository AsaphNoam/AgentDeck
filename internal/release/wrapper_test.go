package release

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildRunnableVersion assembles a version whose libexec/agentdeck is a shell
// script that reports the PATH it runs under and which `node` it resolves, so a
// test can prove the private runtime is on PATH (TS-06.R15, FS-10.A2).
func buildRunnableVersion(t *testing.T, l *Layout, version string) string {
	t.Helper()
	name := VersionDirName(version)
	dir := l.VersionDir(name)

	nodeBin := filepath.Join(dir, "runtime/node/bin")
	if err := os.MkdirAll(nodeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	// A private `node` that identifies itself.
	if err := os.WriteFile(filepath.Join(nodeBin, "node"), []byte("#!/bin/sh\necho private-node\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	adapters := filepath.Join(dir, "runtime/node_modules/.bin")
	if err := os.MkdirAll(adapters, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, a := range []string{"claude-agent-acp", "codex-acp"} {
		if err := os.WriteFile(filepath.Join(adapters, a), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	libexec := filepath.Join(dir, "libexec")
	if err := os.MkdirAll(libexec, 0o755); err != nil {
		t.Fatal(err)
	}
	report := "#!/bin/sh\necho \"PATH=$PATH\"\necho \"NODE=$(command -v node)\"\necho \"ARGS=$*\"\n"
	if err := os.WriteFile(filepath.Join(libexec, "agentdeck"), []byte(report), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteWrapper(dir); err != nil {
		t.Fatal(err)
	}
	if err := WriteInternalManifest(dir, InternalManifest{Version: version, Target: Target, Components: testComponents(version)}); err != nil {
		t.Fatal(err)
	}
	return name
}

// The shim → wrapper → libexec chain prepends the private runtime to PATH and
// resolves `node` to the bundled copy (FS-10.A2, TS-06.R15).
func TestShimRunsPrivateRuntime(t *testing.T) {
	l := newLayout(t)
	name := buildRunnableVersion(t, l, "1.0.0")
	if err := l.Activate(name); err != nil {
		t.Fatal(err)
	}
	if err := l.WriteShim(); err != nil {
		t.Fatal(err)
	}

	// Run the shim with a deliberately minimal PATH so a resolved private `node`
	// can only come from the bundled runtime.
	cmd := exec.Command(l.ShimPath(), "extra-arg")
	cmd.Env = append(os.Environ(), "PATH=/usr/bin:/bin")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("shim run: %v\n%s", err, out)
	}
	text := string(out)

	// The wrapper resolves physical paths (pwd -P), so resolve the expectation
	// the same way (/var → /private/var on macOS).
	versionDir, err := filepath.EvalSymlinks(l.VersionDir(name))
	if err != nil {
		t.Fatal(err)
	}
	wantNodeDir := filepath.Join(versionDir, "runtime/node/bin")
	wantAdapterDir := filepath.Join(versionDir, "runtime/node_modules/.bin")

	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "PATH=") {
			p := strings.TrimPrefix(line, "PATH=")
			if !strings.HasPrefix(p, wantNodeDir+":"+wantAdapterDir+":") {
				t.Fatalf("PATH did not lead with the private runtime dirs.\n got: %s\nwant prefix: %s:%s:", p, wantNodeDir, wantAdapterDir)
			}
			// The remaining user PATH is preserved for provider tooling.
			if !strings.Contains(p, "/usr/bin") {
				t.Fatalf("user PATH not preserved: %s", p)
			}
		}
		if strings.HasPrefix(line, "NODE=") {
			if got := strings.TrimPrefix(line, "NODE="); got != filepath.Join(wantNodeDir, "node") {
				t.Fatalf("node resolved to %q, want the private %q", got, filepath.Join(wantNodeDir, "node"))
			}
		}
		if strings.HasPrefix(line, "ARGS=") {
			if got := strings.TrimPrefix(line, "ARGS="); got != "extra-arg" {
				t.Fatalf("args not forwarded: %q", got)
			}
		}
	}
}

// The shim follows the current pointer: after activating a new version the shim
// runs the new runtime without being rewritten (it bakes in the current path).
func TestShimFollowsCurrentPointer(t *testing.T) {
	l := newLayout(t)
	v1 := buildRunnableVersion(t, l, "1.0.0")
	if err := l.Activate(v1); err != nil {
		t.Fatal(err)
	}
	if err := l.WriteShim(); err != nil {
		t.Fatal(err)
	}
	v2 := buildRunnableVersion(t, l, "2.0.0")
	if err := l.Activate(v2); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(l.ShimPath()).CombinedOutput()
	if err != nil {
		t.Fatalf("shim run: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), v2) || strings.Contains(string(out), v1) {
		t.Fatalf("shim did not follow current to %s:\n%s", v2, out)
	}
}

// Rewriting the stable command replaces a complete executable shim and leaves
// no temporary command visible in its directory (TS-06.R17, INV §9).
func TestWriteShimReplacesStableCommandAtomically(t *testing.T) {
	l := newLayout(t)
	v1 := buildRunnableVersion(t, l, "1.0.0")
	if err := l.Activate(v1); err != nil {
		t.Fatal(err)
	}
	if err := l.WriteShim(); err != nil {
		t.Fatal(err)
	}

	v2 := buildRunnableVersion(t, l, "2.0.0")
	if err := l.Activate(v2); err != nil {
		t.Fatal(err)
	}
	if err := l.WriteShim(); err != nil {
		t.Fatal(err)
	}

	shim, err := os.ReadFile(l.ShimPath())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(shim), l.CurrentLink()) {
		t.Fatalf("rewritten shim does not resolve current pointer: %q", shim)
	}
	info, err := os.Stat(l.ShimPath())
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("shim permissions = %o, want 755", info.Mode().Perm())
	}
	leftovers, err := filepath.Glob(filepath.Join(l.BinDir(), ".agentdeck-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(leftovers) != 0 {
		t.Fatalf("temporary shims left behind: %v", leftovers)
	}
}
