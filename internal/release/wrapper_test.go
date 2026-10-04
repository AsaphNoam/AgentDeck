package release

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/agentdeck/agentdeck/internal/backend/providerexec"
)

// buildRunnableVersion assembles a version with the full required layout whose
// libexec/agentdeck is a shell script reporting the environment it runs under,
// so a test can prove what the wrapper does and does not change (TS-06.R15/R30,
// FS-10.A2).
func buildRunnableVersion(t *testing.T, l *Layout, version string) string {
	t.Helper()
	name := VersionDirName(version)
	dir := l.VersionDir(name)
	for _, rel := range requiredLayout {
		if rel == internalManifestName || rel == "bin/agentdeck" {
			continue
		}
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	report := "#!/bin/sh\necho \"PATH=$PATH\"\necho \"ROOT=$AGENTDECK_RUNTIME_ROOT\"\necho \"CODEX_PATH=$CODEX_PATH\"\necho \"CLAUDE_CODE_EXECUTABLE=$CLAUDE_CODE_EXECUTABLE\"\necho \"ARGS=$*\"\n"
	if err := os.WriteFile(filepath.Join(dir, "libexec", "agentdeck"), []byte(report), 0o755); err != nil {
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

func reportLines(out []byte) map[string]string {
	got := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		if i := strings.IndexByte(line, '='); i > 0 {
			got[line[:i]] = line[i+1:]
		}
	}
	return got
}

// The shim → wrapper → libexec chain publishes the physical managed runtime
// root and forwards args, but leaves PATH and every provider executable
// override exactly as the user had them: no private provider bin can shadow
// an installed CLI (TS-06.R30, FS-09.A37).
func TestShimPublishesManagedRootWithoutShadowingProviders(t *testing.T) {
	l := newLayout(t)
	name := buildRunnableVersion(t, l, "1.0.0")
	if err := l.Activate(name); err != nil {
		t.Fatal(err)
	}
	if err := l.WriteShim(); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(l.ShimPath(), "extra-arg")
	cmd.Env = append(os.Environ(), "PATH=/usr/bin:/bin", "CODEX_PATH=", "CLAUDE_CODE_EXECUTABLE=/custom/claude")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("shim run: %v\n%s", err, out)
	}
	got := reportLines(out)
	// pwd -P resolves physical paths (/var → /private/var on macOS).
	versionDir, err := filepath.EvalSymlinks(l.VersionDir(name))
	if err != nil {
		t.Fatal(err)
	}
	if got["ROOT"] != filepath.Join(versionDir, "runtime") {
		t.Errorf("managed root = %q, want %q", got["ROOT"], filepath.Join(versionDir, "runtime"))
	}
	if got["PATH"] != "/usr/bin:/bin" {
		t.Errorf("wrapper changed PATH: %q", got["PATH"])
	}
	if got["CODEX_PATH"] != "" || got["CLAUDE_CODE_EXECUTABLE"] != "/custom/claude" {
		t.Errorf("wrapper changed executable overrides: CODEX_PATH=%q CLAUDE_CODE_EXECUTABLE=%q", got["CODEX_PATH"], got["CLAUDE_CODE_EXECUTABLE"])
	}
	if got["ARGS"] != "extra-arg" {
		t.Errorf("args not forwarded: %q", got["ARGS"])
	}
}

// The verified layout names exactly the bundle the resolver selects: an
// explicit Bundle choice under the published root finds both providers
// without any global install (TS-06.R30, FS-09.A45).
func TestPublishedRootSelectsTheBundledProviders(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("the release bundle exists only for macOS (TS-06.R13)")
	}
	l := newLayout(t)
	name := buildRunnableVersion(t, l, "1.0.0")
	if err := VerifyLayout(l.VersionDir(name)); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(l.VersionDir(name), "runtime")
	for _, typ := range []string{"claude-acp", "codex-acp"} {
		sel, _ := providerexec.Resolve(providerexec.Input{BackendType: typ, Mode: "bundled", ProcessEnv: []string{
			"PATH=/usr/bin:/bin", providerexec.RuntimeRootEnv + "=" + root, "CLAUDE_CODE_EXECUTABLE=/custom", "CODEX_PATH=/custom",
		}})
		if !sel.Available() || !strings.HasPrefix(sel.Path, root+string(filepath.Separator)) {
			t.Errorf("%s bundle = %+v", typ, sel)
		}
	}
}

// A required layout entry that no manifest component names (or the reverse) is
// how the runtime and its own identity record drift apart (INV §2).
func TestRequiredLayoutAndManifestComponentsAgree(t *testing.T) {
	components := testComponents("1.0.0")
	for _, rel := range requiredLayout {
		base := filepath.Base(rel)
		if base == "index.js" {
			base = filepath.Base(filepath.Dir(filepath.Dir(rel))) // the adapter package
		}
		if base == "agentdeck" || base == "manifest.json" {
			continue // wrapper/binary/manifest are versioned by the release itself
		}
		if _, ok := components[base]; !ok {
			t.Errorf("required layout entry %q has no manifest component version", rel)
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
