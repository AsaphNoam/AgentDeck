package cli

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/agentdeck/agentdeck/internal/backend/providerauth"
)

// fakeAuthExit makes authCommandFor run a real process that exits with code,
// with child output discarded (AgentDeck never captures it).
func fakeAuthExit(t *testing.T, code int) {
	t.Helper()
	prev := authCommandFor
	authCommandFor = func(authProvider) (*exec.Cmd, error) {
		c := exec.Command("sh", "-c", fmt.Sprintf("exit %d", code))
		c.Stdout, c.Stderr = io.Discard, io.Discard
		return c, nil
	}
	t.Cleanup(func() { authCommandFor = prev })
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
	if _, err := runAuthCmd(t, "openai"); err == nil {
		t.Fatal("unknown provider should error")
	}
}

// Provider sign-in resolves commands from the private runtime on the wrapper
// PATH, never a globally installed provider (FS-10.A2, A3). Codex delegates to
// the Codex CLI itself: `codex-acp` is a stdio ACP server that ignores argv, so
// `codex-acp login` never signed anyone in — it started a server and hung
// (TS-06.R22 pins that CLI into the release runtime).
func TestAuthUsesPrivateRuntimeCommands(t *testing.T) {
	claude, _ := providerauth.Lookup("claude")
	if claude.LoginCommand != "claude-agent-acp" || strings.Join(claude.LoginArgs, " ") != "--cli auth login" {
		t.Fatalf("Claude login = %s %q, want claude-agent-acp --cli auth login", claude.LoginCommand, claude.LoginArgs)
	}
	if strings.Join(claude.StatusArgs, " ") != "--cli auth status" {
		t.Fatalf("Claude status args = %q, want --cli auth status", claude.StatusArgs)
	}
	codex, _ := providerauth.Lookup("codex")
	if codex.LoginCommand != "codex" || strings.Join(codex.LoginArgs, " ") != "login" {
		t.Fatalf("Codex login = %s %q, want codex login", codex.LoginCommand, codex.LoginArgs)
	}
	if codex.StatusCommand != "codex" || strings.Join(codex.StatusArgs, " ") != "login status" {
		t.Fatalf("Codex status = %s %q, want codex login status", codex.StatusCommand, codex.StatusArgs)
	}
}

// Both verbs of a provider must come from one table, or the CLI can sign a user
// in through a command the readiness probe never checks (TS-04.R15, INV §2).
func TestAuthLoginAndStatusShareOneExecutable(t *testing.T) {
	for _, id := range providerauth.IDs() {
		p, ok := providerauth.Lookup(id)
		if !ok {
			t.Fatalf("provider %q missing from the shared table", id)
		}
		if p.LoginCommand != p.StatusCommand {
			t.Errorf("%s: login uses %q but readiness probes %q; they must examine the same credential store",
				id, p.LoginCommand, p.StatusCommand)
		}
		if len(p.StatusArgs) == 0 {
			t.Errorf("%s: no non-interactive readiness argv", id)
		}
	}
}

func TestAuthCheckReportsReadiness(t *testing.T) {
	prev := authStatusCommandFor
	authStatusCommandFor = func(authProvider) (*exec.Cmd, error) {
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
