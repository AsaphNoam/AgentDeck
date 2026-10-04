package providerexec

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
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
