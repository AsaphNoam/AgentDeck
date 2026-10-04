package providerexec

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"
)

func writeExe(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// fixture builds a managed runtime root holding both bundled providers and a
// private .bin copy, plus a user bin directory holding newer CLIs.
func fixture(t *testing.T) (root, userBin string) {
	t.Helper()
	base := t.TempDir()
	root = filepath.Join(base, "versions", "v1", "runtime")
	for _, id := range []string{"claude", "codex"} {
		rel := BundledRelPath(id, runtime.GOOS, runtime.GOARCH)
		if rel == "" {
			t.Skip("no bundled layout for this platform")
		}
		writeExe(t, filepath.Join(root, rel))
		writeExe(t, filepath.Join(root, "node_modules", ".bin", id))
	}
	userBin = filepath.Join(base, "user", "bin")
	writeExe(t, filepath.Join(userBin, "claude"))
	writeExe(t, filepath.Join(userBin, "codex"))
	return root, userBin
}

// FS-09.A37/A38/A45, TS-04.R75: mode first, then override precedence, then
// discovery that a private dependency copy cannot shadow.
func TestResolveInstalledAndBundled(t *testing.T) {
	root, userBin := fixture(t)
	privateBin := filepath.Join(root, "node_modules", ".bin")
	process := []string{"PATH=" + privateBin + string(filepath.ListSeparator) + userBin, RuntimeRootEnv + "=" + root}
	other := writeExe(t, filepath.Join(t.TempDir(), "claude-other"))
	modelExe := writeExe(t, filepath.Join(t.TempDir(), "claude-model"))

	for _, tc := range []struct {
		name         string
		mode         string
		process      []string
		backendEnv   map[string]string
		modelEnv     map[string]string
		source, path string
		state        string
	}{
		{name: "omitted mode discovers the user CLI past the private copy", process: process,
			source: SourceDetected, path: filepath.Join(userBin, "claude"), state: StateAvailable},
		{name: "ambient override", process: append(process, "CLAUDE_CODE_EXECUTABLE="+other),
			source: SourceAmbient, path: other, state: StateAvailable},
		{name: "backend beats ambient", process: append(process, "CLAUDE_CODE_EXECUTABLE=/nope"),
			backendEnv: map[string]string{"CLAUDE_CODE_EXECUTABLE": other}, source: SourceBackend, path: other, state: StateAvailable},
		{name: "model beats backend", process: process, backendEnv: map[string]string{"CLAUDE_CODE_EXECUTABLE": other},
			modelEnv: map[string]string{"CLAUDE_CODE_EXECUTABLE": modelExe}, source: SourceModel, path: modelExe, state: StateAvailable},
		{name: "invalid winning override does not fall through", process: process,
			backendEnv: map[string]string{"CLAUDE_CODE_EXECUTABLE": other}, modelEnv: map[string]string{"CLAUDE_CODE_EXECUTABLE": "/missing/claude"},
			source: SourceModel, path: "/missing/claude", state: StateMissing},
		{name: "relative path with separator is invalid", process: process,
			backendEnv: map[string]string{"CLAUDE_CODE_EXECUTABLE": "bin/claude"}, source: SourceBackend, state: StateInvalid},
		{name: "bare command resolves on PATH", process: process,
			backendEnv: map[string]string{"CLAUDE_CODE_EXECUTABLE": "claude"}, source: SourceBackend, path: filepath.Join(userBin, "claude"), state: StateAvailable},
		{name: "empty model value requests discovery", process: append(process, "CLAUDE_CODE_EXECUTABLE="+other),
			modelEnv: map[string]string{"CLAUDE_CODE_EXECUTABLE": ""}, source: SourceDetected, path: filepath.Join(userBin, "claude"), state: StateAvailable},
		{name: "only the private copy means missing", process: []string{"PATH=" + privateBin, RuntimeRootEnv + "=" + root},
			source: SourceDetected, state: StateMissing},
		{name: "bundled ignores every override", mode: "bundled", process: append(process, "CLAUDE_CODE_EXECUTABLE="+other),
			backendEnv: map[string]string{"CLAUDE_CODE_EXECUTABLE": other}, modelEnv: map[string]string{"CLAUDE_CODE_EXECUTABLE": modelExe},
			source: SourceBundled, path: BundledPath(root, "claude"), state: StateAvailable},
		{name: "source build has no bundle", mode: "bundled", process: []string{"PATH=" + userBin},
			source: SourceBundled, state: StateBundleUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sel, ok := Resolve(Input{BackendType: "claude-acp", Mode: tc.mode, ProcessEnv: tc.process, BackendEnv: tc.backendEnv, ModelEnv: tc.modelEnv})
			if !ok {
				t.Fatal("claude-acp must resolve")
			}
			if sel.Source != tc.source || sel.Path != tc.path || sel.State != tc.state {
				t.Fatalf("got source=%s path=%s state=%s; want %s %s %s", sel.Source, sel.Path, sel.State, tc.source, tc.path, tc.state)
			}
		})
	}
}

func TestResolveCodexBundleAndOtherTypes(t *testing.T) {
	root, userBin := fixture(t)
	env := []string{"PATH=" + userBin, RuntimeRootEnv + "=" + root}
	sel, _ := Resolve(Input{BackendType: "codex-acp", Mode: "bundled", ProcessEnv: env})
	if sel.Path != BundledPath(root, "codex") || !sel.Available() || sel.Provider.EnvKey != "CODEX_PATH" {
		t.Fatalf("codex bundle = %+v", sel)
	}
	if _, ok := Resolve(Input{BackendType: "opencode-acp", ProcessEnv: env}); ok {
		t.Fatal("OpenCode is not selected by this resolver")
	}
}

// A symlink in a user directory pointing into the managed root is still the
// private copy; a launcher symlink elsewhere is preserved, not canonicalized.
func TestResolveSkipsSymlinkIntoManagedRootAndKeepsLauncherPath(t *testing.T) {
	root, _ := fixture(t)
	userBin := t.TempDir()
	if err := os.Symlink(filepath.Join(root, "node_modules", ".bin", "claude"), filepath.Join(userBin, "claude")); err != nil {
		t.Fatal(err)
	}
	sel, _ := Resolve(Input{BackendType: "claude-acp", ProcessEnv: []string{"PATH=" + userBin, RuntimeRootEnv + "=" + root}})
	if sel.State != StateMissing {
		t.Fatalf("managed symlink selected: %+v", sel)
	}

	real := writeExe(t, filepath.Join(t.TempDir(), "claude-2.1.300"))
	launcherDir := t.TempDir()
	launcher := filepath.Join(launcherDir, "claude")
	if err := os.Symlink(real, launcher); err != nil {
		t.Fatal(err)
	}
	sel, _ = Resolve(Input{BackendType: "claude-acp", ProcessEnv: []string{"PATH=" + launcherDir}})
	if sel.Path != launcher {
		t.Fatalf("launcher path = %q, want %q", sel.Path, launcher)
	}
}

func TestResolveHomeFallbackAndNonExecutable(t *testing.T) {
	home := t.TempDir()
	exe := writeExe(t, filepath.Join(home, ".local", "bin", "codex"))
	sel, _ := Resolve(Input{BackendType: "codex-acp", ProcessEnv: []string{"PATH="}, Home: home})
	if sel.Path != exe {
		t.Fatalf("home fallback = %+v", sel)
	}
	plain := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(plain, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	sel, _ = Resolve(Input{BackendType: "codex-acp", BackendEnv: map[string]string{"CODEX_PATH": plain}})
	if sel.State != StateNotExecutable {
		t.Fatalf("non-executable override = %+v", sel)
	}
}

// TS-04.R72: the probe parses a version, and timeout, failure, garbage or
// huge output all stay unknown.
func TestProbeVersionIsBounded(t *testing.T) {
	write := func(body string) string {
		p := filepath.Join(t.TempDir(), "p")
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	ctx := context.Background()
	if v := ProbeVersion(ctx, write(`echo "2.1.286 (Claude Code)"`), nil); v != "2.1.286" {
		t.Errorf("claude version = %q", v)
	}
	if v := ProbeVersion(ctx, write(`echo "codex-cli 0.159.2"`), nil); v != "0.159.2" {
		t.Errorf("codex version = %q", v)
	}
	for name, body := range map[string]string{
		"garbage": `echo "no version here"`,
		"failure": `echo "1.2.3"; exit 2`,
		"huge":    `yes x | head -c 100000; echo 9.9.9`,
		"prefix":  `echo 2.1.300; yes x | head -c 100000`,
		"stderr":  `echo 2.1.300; yes x | head -c 100000 >&2`,
		"timeout": `sleep 5; echo 1.2.3`,
	} {
		start := time.Now()
		if v := ProbeVersion(ctx, write(body), nil); v != "" {
			t.Errorf("%s: version = %q, want unknown", name, v)
		}
		if time.Since(start) > 4*time.Second {
			t.Errorf("%s: probe was not bounded", name)
		}
	}
}

// waitForFile polls until path exists and holds want, or fails the test at
// the deadline. No fixed sleeps: this only waits as long as the process
// actually needs to write its marker.
func waitForFile(t *testing.T, path, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(path); err == nil && string(b) == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s to hold %q", path, want)
}

// alive reports whether pid is still running, using a zero-signal probe that
// delivers nothing (FS-09.A37: the running process must receive no signal).
func alive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

// writeMarkerScript writes a script that records marker on start, traps
// TERM/HUP/INT into sigfile instead of dying from them, and then blocks
// until killed so the test can observe whether it ever received a signal.
func writeMarkerScript(t *testing.T, path, marker string) {
	t.Helper()
	body := "#!/bin/sh\n" +
		"printf '" + marker + "' > \"$1\"\n" +
		"trap 'printf signal >> \"$2\"' TERM HUP INT\n" +
		"while true; do sleep 0.2; done\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

// FS-09.A37: replacing the installed launcher a running process was started
// from must not touch that process, and the next start through the same
// resolver call must pick up the replacement. This is the one executable-
// marker regression at the resolver level: everything else (launch, resume,
// switch, clone, pipelines, terminal) composes a spec from the same
// providerexec.Resolve call this test exercises directly, so duplicating a
// lifecycle suite per caller would not add coverage.
func TestResolveThenStartObservesLauncherRetargetWithoutSignalingTheRunningProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signal/symlink semantics differ on windows")
	}
	dir := t.TempDir()
	v1 := filepath.Join(dir, "claude-v1.sh")
	v2 := filepath.Join(dir, "claude-v2.sh")
	writeMarkerScript(t, v1, "v1")
	writeMarkerScript(t, v2, "v2")

	launcher := filepath.Join(dir, "claude")
	if err := os.Symlink(v1, launcher); err != nil {
		t.Fatal(err)
	}

	// The resolver call every process start makes (FS-09.R68/R75): an
	// Installed-mode override naming the launcher, resolved fresh per start.
	resolve := func() Selection {
		sel, ok := Resolve(Input{BackendType: "claude-acp", BackendEnv: map[string]string{"CLAUDE_CODE_EXECUTABLE": launcher}})
		if !ok || !sel.Available() {
			t.Fatalf("launcher did not resolve: %+v", sel)
		}
		return sel
	}

	sel1 := resolve()
	marker1, sig1 := filepath.Join(dir, "marker1"), filepath.Join(dir, "sig1")
	cmd1 := exec.Command(sel1.Path, marker1, sig1)
	if err := cmd1.Start(); err != nil {
		t.Fatalf("start process 1: %v", err)
	}
	defer func() {
		_ = cmd1.Process.Kill()
		_, _ = cmd1.Process.Wait()
	}()
	waitForFile(t, marker1, "v1")

	// Retarget the installed launcher while process 1 keeps running, exactly
	// as an update replaces an installed CLI under a live dashboard/agent.
	if err := os.Remove(launcher); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(v2, launcher); err != nil {
		t.Fatal(err)
	}

	sel2 := resolve()
	if sel2.Path != launcher {
		t.Fatalf("retarget changed the resolved path: %q, want %q", sel2.Path, launcher)
	}
	marker2, sig2 := filepath.Join(dir, "marker2"), filepath.Join(dir, "sig2")
	cmd2 := exec.Command(sel2.Path, marker2, sig2)
	if err := cmd2.Start(); err != nil {
		t.Fatalf("start process 2: %v", err)
	}
	defer func() {
		_ = cmd2.Process.Kill()
		_, _ = cmd2.Process.Wait()
	}()
	waitForFile(t, marker2, "v2")

	if !alive(cmd1.Process.Pid) {
		t.Fatal("process 1 (old marker) no longer alive after retarget")
	}
	if b, err := os.ReadFile(sig1); err == nil && len(b) > 0 {
		t.Fatalf("process 1 received a signal on retarget: %q", b)
	} else if err != nil && !os.IsNotExist(err) {
		t.Fatalf("reading sig1: %v", err)
	}
}
