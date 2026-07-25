package credcheck

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentdeck/agentdeck/internal/config"
)

func TestMergeEnv(t *testing.T) {
	// Model env overrides backend env on conflict; backend keys survive.
	merged := MergeEnv(
		map[string]string{"A": "backend", "B": "backend"},
		map[string]string{"B": "model", "C": "model"},
	)
	if merged["A"] != "backend" {
		t.Errorf("A = %q, want backend", merged["A"])
	}
	if merged["B"] != "model" {
		t.Errorf("B = %q, want model (model wins)", merged["B"])
	}
	if merged["C"] != "model" {
		t.Errorf("C = %q, want model", merged["C"])
	}
}

func TestMergeEnvEmpty(t *testing.T) {
	if got := MergeEnv(nil, nil); len(got) != 0 {
		t.Errorf("nil+nil merge = %v, want empty", got)
	}
	if got := MergeEnv(map[string]string{"X": "1"}, nil); got["X"] != "1" {
		t.Errorf("X = %q, want 1", got["X"])
	}
}

// mockProber implements Prober for testing.
type mockProber struct {
	result CredResult
}

func (m mockProber) Check(_ context.Context, _ config.Backend, _ config.Model, _ map[string]string) CredResult {
	return m.result
}

func TestCheckDispatchUnknownType(t *testing.T) {
	bk := config.Backend{Type: "unknown-acp"}
	result := Check(context.Background(), bk, config.Model{}, nil)
	if result.Status != "skipped" {
		t.Errorf("unknown type status = %q, want skipped", result.Status)
	}
}

func TestCheckWithMockProber(t *testing.T) {
	// Temporarily register a mock for a fake backend type.
	orig := probers
	defer func() { probers = orig }()

	probers = map[string]Prober{
		"test-acp": mockProber{result: CredResult{Status: "ok"}},
	}
	bk := config.Backend{Type: "test-acp"}
	result := Check(context.Background(), bk, config.Model{}, nil)
	if result.Status != "ok" {
		t.Errorf("status = %q, want ok", result.Status)
	}
}

// fakeCLI writes an executable stub at dir/<name> so lookPath resolves it.
func fakeCLI(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake %s: %v", name, err)
	}
	return p
}

func TestOpenCodeProber(t *testing.T) {
	home := t.TempDir()
	cli := fakeCLI(t, t.TempDir(), "opencode")

	// No CLI on PATH → skipped (cli_not_installed).
	if r := (opencodeProber{}).Check(context.Background(), config.Backend{}, config.Model{}, map[string]string{"OPENCODE_PATH": filepath.Join(home, "nope"), "HOME": home}); r.Status != "skipped" || r.Detail != "cli_not_installed" {
		t.Fatalf("missing cli = %+v, want skipped/cli_not_installed", r)
	}

	base := map[string]string{"OPENCODE_PATH": cli, "HOME": home}
	// CLI present but no auth.json and no provider key → skipped (not_logged_in).
	if r := (opencodeProber{}).Check(context.Background(), config.Backend{}, config.Model{}, base); r.Status != "skipped" || r.Detail != "not_logged_in" {
		t.Fatalf("no auth = %+v, want skipped/not_logged_in", r)
	}
	// Provider API key in env → ok.
	withKey := map[string]string{"OPENCODE_PATH": cli, "HOME": home, "ANTHROPIC_API_KEY": "sk-x"}
	if r := (opencodeProber{}).Check(context.Background(), config.Backend{}, config.Model{}, withKey); r.Status != "ok" {
		t.Fatalf("provider key = %+v, want ok", r)
	}
	// CLI-side auth.json present → ok (even without an env key).
	authDir := filepath.Join(home, ".local", "share", "opencode")
	if err := os.MkdirAll(authDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(authDir, "auth.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write auth.json: %v", err)
	}
	if r := (opencodeProber{}).Check(context.Background(), config.Backend{}, config.Model{}, base); r.Status != "ok" {
		t.Fatalf("auth.json = %+v, want ok", r)
	}
}

func TestOpenHandsProber(t *testing.T) {
	home := t.TempDir()
	cli := fakeCLI(t, t.TempDir(), "openhands")

	if r := (openhandsProber{}).Check(context.Background(), config.Backend{}, config.Model{}, map[string]string{"OPENHANDS_PATH": filepath.Join(home, "nope"), "HOME": home}); r.Status != "skipped" || r.Detail != "cli_not_installed" {
		t.Fatalf("missing cli = %+v, want skipped/cli_not_installed", r)
	}

	base := map[string]string{"OPENHANDS_PATH": cli, "HOME": home}
	if r := (openhandsProber{}).Check(context.Background(), config.Backend{}, config.Model{}, base); r.Status != "skipped" || r.Detail != "no_llm_api_key" {
		t.Fatalf("no auth = %+v, want skipped/no_llm_api_key", r)
	}
	// LLM_API_KEY present → ok.
	withKey := map[string]string{"OPENHANDS_PATH": cli, "HOME": home, "LLM_API_KEY": "sk-x"}
	if r := (openhandsProber{}).Check(context.Background(), config.Backend{}, config.Model{}, withKey); r.Status != "ok" {
		t.Fatalf("llm key = %+v, want ok", r)
	}
	// settings.json present → ok.
	if err := os.MkdirAll(filepath.Join(home, ".openhands"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".openhands", "settings.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write settings.json: %v", err)
	}
	if r := (openhandsProber{}).Check(context.Background(), config.Backend{}, config.Model{}, base); r.Status != "ok" {
		t.Fatalf("settings.json = %+v, want ok", r)
	}
}

func TestClaudeProberRetriesWithoutNoColor(t *testing.T) {
	dir := t.TempDir()
	cliPath := filepath.Join(dir, "claude-agent-acp")
	script := `#!/bin/sh
if [ "$1" = "--cli" ] && [ "$2" = "auth" ] && [ "$3" = "status" ] && [ "$4" = "--no-color" ]; then
  echo "error: unknown option '--no-color'" >&2
  exit 1
fi
if [ "$1" = "--cli" ] && [ "$2" = "auth" ] && [ "$3" = "status" ]; then
  echo "logged in"
  exit 0
fi
echo "unexpected args: $@" >&2
exit 2
`
	if err := os.WriteFile(cliPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake claude: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	result := claudeProber{}.Check(
		context.Background(),
		config.Backend{},
		config.Model{},
		map[string]string{},
	)
	if result.Status != "ok" {
		t.Fatalf("status = %q, want ok (detail=%q)", result.Status, result.Detail)
	}
}

// Status-check failures return only bounded vocabulary; raw CLI output and
// account identity never cross the API boundary (TS-04.R15, INV §8/§12).
func TestClaudeProberReturnsOnlyBoundedErrorOutput(t *testing.T) {
	dir := t.TempDir()
	cliPath := filepath.Join(dir, "claude-agent-acp")
	script := `#!/bin/sh
if [ "$1" = "--cli" ] && [ "$2" = "auth" ] && [ "$3" = "status" ]; then
  echo "status call failed for user user@example.com with long raw diagnostic output that should never be exposed" >&2
  exit 1
fi
echo "unexpected args: $@" >&2
exit 2
`
	if err := os.WriteFile(cliPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake claude: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	result := claudeProber{}.Check(
		context.Background(),
		config.Backend{},
		config.Model{},
		map[string]string{},
	)
	if result.Status != "failed" {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if result.Detail != "status_check_failed" {
		t.Errorf("detail = %q, want status_check_failed: account identity and raw output must not cross the API boundary", result.Detail)
	}
}

// fakeCodexCLI installs a fake `codex` on PATH whose `login status` prints out
// and exits with code. It records every invocation so a test can prove no
// interactive login was ever started (FS-09.A13).
func fakeCodexCLI(t *testing.T, out string, code int) (calls string) {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "calls.log")
	script := fmt.Sprintf(`#!/bin/sh
echo "$@" >> %q
cat >/dev/null 2>&1 &
if [ "$1" = "login" ] && [ "$2" = "status" ]; then
  echo %q
  exit %d
fi
echo "unexpected args: $@" >&2
exit 2
`, log, out, code)
	if err := os.WriteFile(filepath.Join(dir, "codex"), []byte(script), 0o755); err != nil {
		t.Fatalf("write fake codex: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return log
}

func readCalls(t *testing.T, log string) string {
	t.Helper()
	b, err := os.ReadFile(log)
	if err != nil {
		return ""
	}
	return string(b)
}

// A ChatGPT-signed-in Codex has no OPENAI_API_KEY at all; native sign-in alone
// must be sufficient readiness, and the provider's status text (which names the
// account) must not reach the result (FS-09.R34/A13, TS-04.R15).
func TestCodexProberAcceptsNativeLoginWithoutAPIKey(t *testing.T) {
	log := fakeCodexCLI(t, "Logged in using ChatGPT", 0)

	result := codexProber{}.Check(context.Background(), config.Backend{}, config.Model{}, map[string]string{})

	if result.Status != "ok" {
		t.Fatalf("status = %q, want ok (detail=%q)", result.Status, result.Detail)
	}
	if result.Detail != "" {
		t.Errorf("detail = %q, want empty: raw provider status must not cross the API boundary", result.Detail)
	}
	if calls := readCalls(t, log); !strings.Contains(calls, "login status") {
		t.Errorf("probe argv = %q, want a non-interactive 'login status'", calls)
	} else if strings.Contains(calls, "login\n") {
		t.Errorf("probe started an interactive login: %q", calls)
	}
}

// "The CLI says nobody is signed in" is actionable; it must not be reported as
// the generic no-key skip that hides the real next step.
func TestCodexProberReportsNativeNotLoggedIn(t *testing.T) {
	fakeCodexCLI(t, "Not logged in", 1)

	result := codexProber{}.Check(context.Background(), config.Backend{}, config.Model{}, map[string]string{})

	if result.Status != "failed" || result.Detail != "not_logged_in" {
		t.Fatalf("result = %+v, want failed/not_logged_in", result)
	}
}

// An uninterrogable CLI is not a verdict about the user's credentials (INV §12).
func TestCodexProberSkipsWhenCLIAbsentAndNoKey(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	result := codexProber{}.Check(context.Background(), config.Backend{}, config.Model{}, map[string]string{})

	if result.Status != "skipped" || result.Detail != "no_api_key" {
		t.Fatalf("result = %+v, want skipped/no_api_key", result)
	}
}

// Native sign-in and an API key are independent paths: an unsigned-in CLI with a
// working key is still ready (FS-09.R34).
func TestCodexProberFallsBackToAPIKeyWhenNotSignedIn(t *testing.T) {
	fakeCodexCLI(t, "Not logged in", 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	result := codexProber{}.Check(context.Background(), config.Backend{}, config.Model{}, map[string]string{
		"OPENAI_API_KEY":  "sk-test",
		"OPENAI_BASE_URL": srv.URL,
	})

	if result.Status != "ok" {
		t.Fatalf("status = %q, want ok (detail=%q)", result.Status, result.Detail)
	}
}

// A hung Codex CLI must not consume the entire context deadline; the API-key
// fallback must still have time to run (TS-04.R15, INV §12, FS-09.R34).
func TestCodexProberAPIKeyFallbackAfterHungCLI(t *testing.T) {
	dir := t.TempDir()
	cliPath := filepath.Join(dir, "codex")
	script := `#!/bin/sh
if [ "$1" = "login" ] && [ "$2" = "status" ]; then
  sleep 30 &
  wait
fi
exit 1
`
	if err := os.WriteFile(cliPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake codex: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	result := codexProber{}.Check(context.Background(), config.Backend{}, config.Model{}, map[string]string{
		"OPENAI_API_KEY":  "sk-test",
		"OPENAI_BASE_URL": srv.URL,
	})

	if result.Status != "ok" {
		t.Fatalf("status = %q, want ok despite hung CLI (detail=%q)", result.Status, result.Detail)
	}
}

// "not logged in" contains "logged in": a substring vocabulary read in the wrong
// order would call an unsigned-in provider ready.
func TestClassifyNativeOutputPrefersTheNegative(t *testing.T) {
	for _, tc := range []struct {
		out  string
		want nativeOutcome
	}{
		{"Not logged in", nativeNotLoggedIn},
		{"not authenticated", nativeNotLoggedIn},
		{"Logged in using ChatGPT", nativeReady},
		{"totally unrecognized wording", nativeUnavailable},
	} {
		if got := classifyNativeOutput([]byte(tc.out), nativeUnavailable); got != tc.want {
			t.Errorf("classify(%q) = %v, want %v", tc.out, got, tc.want)
		}
	}
}
