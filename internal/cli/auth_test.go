package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/agentdeck/agentdeck/internal/backend/providerauth"
	"github.com/agentdeck/agentdeck/internal/backend/providerexec"
	"github.com/agentdeck/agentdeck/internal/config"
)

// fakeAuthExit makes authCommandFor run a real process that exits with code,
// with child output discarded (AgentDeck never captures it).
func fakeAuthExit(t *testing.T, code int) {
	t.Helper()
	prev := authCommandFor
	authCommandFor = func(authTarget) (*exec.Cmd, error) {
		c := exec.Command("sh", "-c", fmt.Sprintf("exit %d", code))
		c.Stdout, c.Stderr = io.Discard, io.Discard
		return c, nil
	}
	t.Cleanup(func() { authCommandFor = prev })
}

// hermeticProviders makes target selection independent of the developer's
// catalog and installed CLIs: no catalog, and ambient overrides naming inert
// stand-in executables. It returns their paths by provider id.
func hermeticProviders(t *testing.T) map[string]string {
	t.Helper()
	prev := readAuthCatalog
	readAuthCatalog = func() (config.BackendsConfig, bool, error) { return config.BackendsConfig{}, false, nil }
	t.Cleanup(func() { readAuthCatalog = prev })
	dir := t.TempDir()
	paths := map[string]string{}
	for id, key := range map[string]string{"claude": "CLAUDE_CODE_EXECUTABLE", "codex": "CODEX_PATH"} {
		path := filepath.Join(dir, id)
		if err := os.WriteFile(path, []byte("#!/bin/sh\necho \"$0 $*\" > \"$0.argv\"\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv(key, path)
		paths[id] = path
	}
	return paths
}

func runAuthCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := NewRootCmd()
	var buf strings.Builder
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(append([]string{"auth"}, args...))
	err := root.Execute()
	return buf.String(), err
}

// A successful sign-in reports success and exits zero (FS-10.A3).
func TestAuthSuccess(t *testing.T) {
	hermeticProviders(t)
	fakeAuthExit(t, 0)
	out, err := runAuthCmd(t, "claude")
	if err != nil {
		t.Fatalf("auth claude: %v", err)
	}
	if !strings.Contains(out, "Signed in to Claude") {
		t.Fatalf("output = %q", out)
	}
}

// Ctrl-C (exit 130) is a cancellation, not a failure: exit zero, retry guidance.
func TestAuthCancelled(t *testing.T) {
	hermeticProviders(t)
	fakeAuthExit(t, 130)
	out, err := runAuthCmd(t, "codex")
	if err != nil {
		t.Fatalf("cancel should not be an error: %v", err)
	}
	if !strings.Contains(out, "cancelled") || !strings.Contains(out, "installation is ready") {
		t.Fatalf("output = %q", out)
	}
}

// A failed sign-in reports actionable retry guidance and exits non-zero, without
// claiming success (FS-10.R5, FS-10.R11).
func TestAuthFailed(t *testing.T) {
	hermeticProviders(t)
	fakeAuthExit(t, 1)
	out, err := runAuthCmd(t, "claude")
	if !errors.Is(err, errAuthFailed) {
		t.Fatalf("failed sign-in err = %v, want errAuthFailed", err)
	}
	if strings.Contains(out, "Signed in") {
		t.Fatalf("failure output falsely claimed success: %q", out)
	}
	if !strings.Contains(out, "did not complete") {
		t.Fatalf("output = %q", out)
	}
}

// A missing provider login tool is reported as an actionable outcome that leaves
// the installation working (FS-10.R11).
func TestAuthToolNotFound(t *testing.T) {
	hermeticProviders(t)
	t.Setenv("AGENTDECK_CLAUDE_LOGIN_CMD", "definitely-not-a-real-command-xyzzy")
	out, err := runAuthCmd(t, "claude")
	if !errors.Is(err, errAuthFailed) {
		t.Fatalf("missing tool err = %v, want errAuthFailed", err)
	}
	if !strings.Contains(out, "still works") {
		t.Fatalf("output should reassure the install works: %q", out)
	}
}

// The command must exist in the built binary's tree, and accept exactly the
// providers the shared table knows. A release whose `agentdeck auth` is missing
// or narrower than the table sends people to a command that cannot help them
// (TS-06.R22, FS-10.R5).
func TestAuthCommandIsPresentForEveryProvider(t *testing.T) {
	hermeticProviders(t)
	var auth *cobra.Command
	for _, c := range NewRootCmd().Commands() {
		if c.Name() == "auth" {
			auth = c
			break
		}
	}
	if auth == nil {
		t.Fatal("`agentdeck auth` is absent from the command tree")
	}
	fakeAuthExit(t, 0)
	for _, id := range providerauth.IDs() {
		out, err := runAuthCmd(t, id)
		if err != nil {
			t.Errorf("auth %s is not an accepted provider: %v (%s)", id, err, out)
		}
	}
}

func TestAuthUnknownProvider(t *testing.T) {
	hermeticProviders(t)
	if _, err := runAuthCmd(t, "openai"); err == nil {
		t.Fatal("unknown provider should error")
	}
}

// FS-10.A11, TS-04.R71: both providers run their native verbs on the selected
// executable — Claude `auth login`/`auth status`, Codex `login`/`login status`
// — with no adapter or PATH-found copy in between.
func TestAuthRunsNativeVerbsOnTheSelectedExecutable(t *testing.T) {
	claude, _ := providerauth.Lookup("claude")
	if strings.Join(claude.LoginArgs, " ") != "auth login" || strings.Join(claude.StatusArgs, " ") != "auth status" {
		t.Fatalf("Claude argv = %q / %q", claude.LoginArgs, claude.StatusArgs)
	}
	codex, _ := providerauth.Lookup("codex")
	if strings.Join(codex.LoginArgs, " ") != "login" || strings.Join(codex.StatusArgs, " ") != "login status" {
		t.Fatalf("Codex argv = %q / %q", codex.LoginArgs, codex.StatusArgs)
	}
	paths := hermeticProviders(t)
	for id, want := range map[string]string{"claude": "auth login", "codex": "login"} {
		out, err := runAuthCmd(t, id)
		if err != nil {
			t.Fatalf("auth %s: %v (%s)", id, err, out)
		}
		if !strings.Contains(out, "using "+paths[id]) {
			t.Errorf("auth %s did not name its executable first: %q", id, out)
		}
		argv, err := os.ReadFile(paths[id] + ".argv")
		if err != nil || strings.TrimSpace(string(argv)) != paths[id]+" "+want {
			t.Errorf("auth %s ran %q (%v)", id, argv, err)
		}
	}
}

// FS-10.R21: a missing provider gives install guidance and never starts login.
func TestAuthMissingProviderGivesInstallGuidance(t *testing.T) {
	hermeticProviders(t)
	t.Setenv("CLAUDE_CODE_EXECUTABLE", "")
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	prevDirs := providerexec.SystemDirs
	providerexec.SystemDirs = nil
	t.Cleanup(func() { providerexec.SystemDirs = prevDirs })
	started := false
	prev := authCommandFor
	authCommandFor = func(authTarget) (*exec.Cmd, error) { started = true; return exec.Command("true"), nil }
	t.Cleanup(func() { authCommandFor = prev })
	out, err := runAuthCmd(t, "claude")
	if !errors.Is(err, errProviderUnavailable) || started || !strings.Contains(out, "not installed") {
		t.Fatalf("missing provider: started=%v err=%v out=%q", started, err, out)
	}
}

// FS-10.A11, TS-04.R71: default, sole-provider, ambiguous, explicit backend/
// model and missing-catalog target selection.
func TestSelectAuthTarget(t *testing.T) {
	claude, _ := providerauth.Lookup("claude")
	models := map[string]config.Model{"sonnet": {Model: "s"}, "opus": {Model: "o"}}
	catalog := config.BackendsConfig{Backends: map[string]config.Backend{
		"work":  {Type: "claude-acp", Default: true, DefaultModel: "sonnet", Models: models},
		"home":  {Type: "claude-acp", DefaultModel: "opus", Models: models},
		"codex": {Type: "codex-acp", DefaultModel: "g", Models: map[string]config.Model{"g": {Model: "g"}}},
	}}
	for _, tc := range []struct {
		name                   string
		catalog                config.BackendsConfig
		have                   bool
		backend, model         string
		wantBackend, wantModel string
		wantErr                string
	}{
		{name: "default backend", catalog: catalog, have: true, wantBackend: "work", wantModel: "sonnet"},
		{name: "explicit backend and model", catalog: catalog, have: true, backend: "home", model: "sonnet", wantBackend: "home", wantModel: "sonnet"},
		{name: "explicit backend default model", catalog: catalog, have: true, backend: "home", wantBackend: "home", wantModel: "opus"},
		{name: "type mismatch", catalog: catalog, have: true, backend: "codex", wantErr: "not a Claude backend"},
		{name: "unknown model", catalog: catalog, have: true, backend: "home", model: "nope", wantErr: "no model"},
		{name: "ambiguous without default", catalog: config.BackendsConfig{Backends: map[string]config.Backend{
			"a": {Type: "claude-acp", DefaultModel: "sonnet", Models: models},
			"b": {Type: "claude-acp", DefaultModel: "sonnet", Models: models},
			"c": {Type: "codex-acp", Default: true, DefaultModel: "g", Models: map[string]config.Model{"g": {}}},
		}}, have: true, wantErr: "--backend"},
		{name: "sole matching backend", catalog: config.BackendsConfig{Backends: map[string]config.Backend{
			"a": {Type: "claude-acp", DefaultModel: "opus", Models: models},
			"c": {Type: "codex-acp", Default: true, DefaultModel: "g", Models: map[string]config.Model{"g": {}}},
		}}, have: true, wantBackend: "a", wantModel: "opus"},
		{name: "missing catalog is ambient"},
		{name: "missing catalog rejects --backend", backend: "work", wantErr: "no backend catalog"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			be, backendID, modelID, err := selectAuthTarget(claude, tc.catalog, tc.have, tc.backend, tc.model)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil || backendID != tc.wantBackend || modelID != tc.wantModel || be.Type != "claude-acp" {
				t.Fatalf("got %s/%s %v (%s)", backendID, modelID, err, be.Type)
			}
		})
	}
}

// The selected backend's provider mode and environment reach the child, and a
// Codex sign-in targets the personal home, not the private session store.
func TestResolveAuthTargetUsesBackendSelectionAndPersonalCodexHome(t *testing.T) {
	paths := hermeticProviders(t)
	backendExe := filepath.Join(t.TempDir(), "codex-backend")
	if err := os.WriteFile(backendExe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	personal := t.TempDir()
	t.Setenv("CODEX_HOME", personal)
	readAuthCatalog = func() (config.BackendsConfig, bool, error) {
		return config.BackendsConfig{Backends: map[string]config.Backend{"codex": {
			Type: "codex-acp", Default: true, DefaultModel: "g",
			Env:    map[string]string{"CODEX_PATH": backendExe, "CODEX_HOME": "/scoped", "OPENAI_BASE_URL": "https://proxy"},
			Models: map[string]config.Model{"g": {Model: "g"}},
		}}}, true, nil
	}
	codex, _ := providerauth.Lookup("codex")
	target, err := resolveAuthTarget(codex, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if target.selection.Path != backendExe || target.selection.Path == paths["codex"] {
		t.Fatalf("selection = %+v", target.selection)
	}
	c := exec.Command("true")
	c.Env = target.env
	got := map[string]string{}
	for _, kv := range c.Env {
		if i := strings.IndexByte(kv, '='); i > 0 {
			got[kv[:i]] = kv[i+1:]
		}
	}
	if got["CODEX_HOME"] != personal || got["OPENAI_BASE_URL"] != "https://proxy" {
		t.Fatalf("child env CODEX_HOME=%q OPENAI_BASE_URL=%q", got["CODEX_HOME"], got["OPENAI_BASE_URL"])
	}
}

func TestAuthCheckReportsReadiness(t *testing.T) {
	hermeticProviders(t)
	prev := authStatusCommandFor
	authStatusCommandFor = func(authTarget) (*exec.Cmd, error) {
		c := exec.Command("sh", "-c", "exit 0")
		c.Stdout, c.Stderr = io.Discard, io.Discard
		return c, nil
	}
	t.Cleanup(func() { authStatusCommandFor = prev })
	out, err := runAuthCmd(t, "claude", "--check")
	if err != nil || !strings.Contains(out, "Claude is ready") {
		t.Fatalf("auth check = %q, %v", out, err)
	}
}

func TestClassifyAuth(t *testing.T) {
	if got := classifyAuth(nil); got != authSuccess {
		t.Fatalf("nil err = %v, want success", got)
	}
	cancel := exec.Command("sh", "-c", "exit 130").Run()
	if got := classifyAuth(cancel); got != authCancelled {
		t.Fatalf("exit 130 = %v, want cancelled", got)
	}
	fail := exec.Command("sh", "-c", "exit 2").Run()
	if got := classifyAuth(fail); got != authFailed {
		t.Fatalf("exit 2 = %v, want failed", got)
	}
}
