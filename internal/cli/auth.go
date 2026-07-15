package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

// authProvider names a provider and the private adapter command AgentDeck
// delegates to. The command is only ever attached to the user's terminal —
// AgentDeck accepts no credential flags, captures no child stdout/stderr, and
// writes no credential material of its own (TS-06.R20). The release wrapper
// resolves these commands from the selected private runtime, rather than a
// globally installed provider or ACP adapter (FS-10.R3, R5).
type authProvider struct {
	name        string
	command     string
	args        []string
	statusArgs  []string
	loginEnvVar string // overrides "command arg arg..." for the gated verifier
}

var authProviders = map[string]authProvider{
	"claude": {name: "Claude", command: "claude-agent-acp", args: []string{"--cli", "auth", "login"}, statusArgs: []string{"--cli", "auth", "status"}, loginEnvVar: "AGENTDECK_CLAUDE_LOGIN_CMD"},
	"codex":  {name: "Codex", command: "codex-acp", args: []string{"login"}, loginEnvVar: "AGENTDECK_CODEX_LOGIN_CMD"},
}

// authOutcome is the bounded result of a delegated sign-in (FS-10.R11).
type authOutcome int

const (
	authSuccess authOutcome = iota
	authCancelled
	authFailed
)

// errAuthFailed is returned (already messaged) so the command exits non-zero
// without cobra reprinting anything.
var errAuthFailed = errors.New("sign-in did not complete")

// authCommandFor builds the login command with stdio attached to the caller's
// terminal. Overridable in tests so the outcome branches run against a fake
// provider (FS-10.A3).
var authCommandFor = func(p authProvider) (*exec.Cmd, error) {
	command, args := p.command, p.args
	if v := strings.TrimSpace(os.Getenv(p.loginEnvVar)); v != "" {
		fields := strings.Fields(v)
		command, args = fields[0], fields[1:]
	}
	path, err := exec.LookPath(command)
	if err != nil {
		return nil, fmt.Errorf("the %s sign-in tool %q was not found on PATH", p.name, command)
	}
	c := exec.Command(path, args...)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	return c, nil
}

// authStatusCommandFor is separate from the login factory because a readiness
// check must not inherit a test or operator login override. It keeps its output
// out of AgentDeck's logs while the provider examines its own credential store.
var authStatusCommandFor = func(p authProvider) (*exec.Cmd, error) {
	path, err := exec.LookPath(p.command)
	if err != nil {
		return nil, fmt.Errorf("the %s sign-in tool %q was not found on PATH", p.name, p.command)
	}
	if len(p.statusArgs) == 0 {
		return nil, fmt.Errorf("the %s adapter does not provide a non-interactive readiness check", p.name)
	}
	c := exec.Command(path, p.statusArgs...)
	c.Stdin = nil
	c.Stdout, c.Stderr = os.Stderr, os.Stderr
	return c, nil
}

// newAuthCmd builds `agentdeck auth <claude|codex>`.
func newAuthCmd() *cobra.Command {
	var check bool
	cmd := &cobra.Command{
		Use:           "auth <claude|codex>",
		Short:         "Sign in to a provider by delegating to its own login flow",
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			p, ok := authProviders[strings.ToLower(args[0])]
			if !ok {
				return fmt.Errorf("unknown provider %q; use 'claude' or 'codex'", args[0])
			}
			if check {
				return runAuthCheck(cmd, p)
			}
			return runAuth(cmd, p)
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "only check whether the provider is ready")
	_ = cmd.Flags().MarkHidden("check")
	return cmd
}

func runAuthCheck(cmd *cobra.Command, p authProvider) error {
	c, err := authStatusCommandFor(p)
	if err == nil {
		err = c.Run()
	}
	if err == nil {
		fmt.Fprintf(cmd.OutOrStdout(), "%s is ready.\n", p.name)
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s needs sign-in. Run 'agentdeck auth %s' to continue.\n", p.name, strings.ToLower(p.name))
	return errAuthFailed
}

// runAuth delegates sign-in to the provider's own flow and reports a truthful,
// actionable outcome. It never prints or records credentials; success and
// cancellation leave a working installation, and only a genuine failure exits
// non-zero (FS-10.R5, FS-10.R11).
func runAuth(cmd *cobra.Command, p authProvider) error {
	out := cmd.OutOrStdout()
	c, err := authCommandFor(p)
	if err != nil {
		fmt.Fprintf(out, "%s sign-in unavailable: %v.\n", p.name, err)
		fmt.Fprintf(out, "Your installation still works; retry with 'agentdeck auth %s' or sign in from the dashboard.\n", strings.ToLower(p.name))
		return errAuthFailed
	}
	switch classifyAuth(c.Run()) {
	case authSuccess:
		fmt.Fprintf(out, "Signed in to %s.\n", p.name)
		return nil
	case authCancelled:
		fmt.Fprintf(out, "%s sign-in cancelled. Your installation is ready; retry any time with 'agentdeck auth %s' or from the dashboard.\n", p.name, strings.ToLower(p.name))
		return nil
	default:
		fmt.Fprintf(out, "%s sign-in did not complete. Retry with 'agentdeck auth %s' or sign in from the dashboard.\n", p.name, strings.ToLower(p.name))
		return errAuthFailed
	}
}

// classifyAuth maps a login process result to a bounded outcome. A 130 exit
// (128+SIGINT) is the user pressing Ctrl-C: a cancellation, not a failure.
func classifyAuth(runErr error) authOutcome {
	if runErr == nil {
		return authSuccess
	}
	var ee *exec.ExitError
	if errors.As(runErr, &ee) && ee.ExitCode() == 130 {
		return authCancelled
	}
	return authFailed
}
