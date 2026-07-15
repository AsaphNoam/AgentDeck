package cli

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"testing"
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

func TestAuthUnknownProvider(t *testing.T) {
	if _, err := runAuthCmd(t, "openai"); err == nil {
		t.Fatal("unknown provider should error")
	}
}

// Provider sign-in must resolve the selected release's private ACP adapter,
// never a globally installed Claude/Codex executable (FS-10.A2, A3).
func TestAuthUsesPrivateAdapterCommands(t *testing.T) {
	if got := authProviders["claude"]; got.command != "claude-agent-acp" || strings.Join(got.args, " ") != "--cli auth login" {
		t.Fatalf("Claude login = %s %q, want claude-agent-acp --cli auth login", got.command, got.args)
	}
	if got := authProviders["codex"]; got.command != "codex-acp" || strings.Join(got.args, " ") != "login" {
		t.Fatalf("Codex login = %s %q, want codex-acp login", got.command, got.args)
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
