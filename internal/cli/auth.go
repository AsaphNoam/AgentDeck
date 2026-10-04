package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/agentdeck/agentdeck/internal/backend/providerauth"
	"github.com/agentdeck/agentdeck/internal/backend/providerexec"
	"github.com/agentdeck/agentdeck/internal/config"
)

// authProvider is the provider sign-in metadata this command delegates to. It
// is owned by internal/backend/providerauth so the CLI's login argv and the
// backend readiness probe cannot drift apart (TS-04.R15, INV §2). The command is
// only ever attached to the user's terminal — AgentDeck accepts no credential
// flags, captures no child stdout/stderr, and writes no credential material of
// its own (TS-06.R20). The executable is the one the target backend/model would
// launch, selected by the shared resolver (FS-10.R21, TS-04.R71).
type authProvider = providerauth.Provider

// authTarget is the resolved sign-in target: the provider, the configured
// backend/model it serves (empty for ambient selection without a catalog),
// the selected executable and the provider environment the child inherits.
type authTarget struct {
	provider  authProvider
	backendID string
	modelID   string
	selection providerexec.Selection
	env       []string
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
// provider (FS-10.A3). The explicit login-command variable is an advanced
// login-only exception; it never replaces executable selection for readiness.
var authCommandFor = func(t authTarget) (*exec.Cmd, error) {
	command, args := t.selection.Path, t.provider.LoginArgs
	if v := strings.TrimSpace(os.Getenv(t.provider.LoginEnvVar)); v != "" {
		fields := strings.Fields(v)
		path, err := exec.LookPath(fields[0])
		if err != nil {
			return nil, fmt.Errorf("the %s login command %q was not found on PATH", t.provider.Name, fields[0])
		}
		command, args = path, fields[1:]
	}
	c := exec.Command(command, args...)
	c.Env = t.env
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	return c, nil
}

// authStatusCommandFor is separate from the login factory because a readiness
// check must not inherit a test or operator login override. It keeps its output
// out of AgentDeck's logs while the provider examines its own credential store.
var authStatusCommandFor = func(t authTarget) (*exec.Cmd, error) {
	if len(t.provider.StatusArgs) == 0 {
		return nil, fmt.Errorf("%s does not provide a non-interactive readiness check", t.provider.Name)
	}
	c := exec.Command(t.selection.Path, t.provider.StatusArgs...)
	c.Env = t.env
	c.Stdin = nil
	c.Stdout, c.Stderr = os.Stderr, os.Stderr
	return c, nil
}

// readAuthCatalog is the backend catalog the target is chosen from. A missing
// catalog means ambient selection; a corrupt or unreadable one is an error,
// never a silent default. Overridable in tests.
var readAuthCatalog = func() (config.BackendsConfig, bool, error) {
	store, err := config.New()
	if err != nil {
		return config.BackendsConfig{}, false, err
	}
	catalog, err := store.ReadBackends()
	if errors.Is(err, config.ErrNotFound) {
		return config.BackendsConfig{}, false, nil
	}
	if err != nil {
		return config.BackendsConfig{}, false, fmt.Errorf("read backend catalog: %w", err)
	}
	return catalog, true, nil
}

// selectAuthTarget picks the backend/model whose provider selection sign-in
// uses (TS-04.R71): an explicit --backend must be this provider's type;
// otherwise the default backend if it is, else the sole matching backend,
// else ambiguity. --model must name a model of that backend; otherwise its
// default model. Without a catalog, ambient selection applies.
func selectAuthTarget(p authProvider, catalog config.BackendsConfig, haveCatalog bool, backendID, modelID string) (config.Backend, string, string, error) {
	if !haveCatalog {
		if backendID != "" || modelID != "" {
			return config.Backend{}, "", "", fmt.Errorf("no backend catalog exists yet; omit --backend/--model")
		}
		return config.Backend{Type: p.BackendType}, "", "", nil
	}
	if backendID == "" {
		var matches []string
		for id, be := range catalog.Backends {
			if be.Type != p.BackendType {
				continue
			}
			if be.Default {
				matches = []string{id}
				break
			}
			matches = append(matches, id)
		}
		switch len(matches) {
		case 0:
			return config.Backend{Type: p.BackendType}, "", "", nil
		case 1:
			backendID = matches[0]
		default:
			sort.Strings(matches)
			return config.Backend{}, "", "", fmt.Errorf("several %s backends are configured (%s); choose one with --backend", p.Name, strings.Join(matches, ", "))
		}
	}
	be, ok := catalog.Backends[backendID]
	if !ok {
		return config.Backend{}, "", "", fmt.Errorf("unknown backend %q", backendID)
	}
	if be.Type != p.BackendType {
		return config.Backend{}, "", "", fmt.Errorf("backend %q is %s, not a %s backend", backendID, be.Type, p.Name)
	}
	if modelID == "" {
		modelID = be.DefaultModel
	}
	if _, ok := be.Models[modelID]; !ok {
		return config.Backend{}, "", "", fmt.Errorf("backend %q has no model %q", backendID, modelID)
	}
	return be, backendID, modelID, nil
}

// resolveAuthTarget selects the target and its provider executable before any
// provider process starts.
func resolveAuthTarget(p authProvider, backendID, modelID string) (authTarget, error) {
	catalog, haveCatalog, err := readAuthCatalog()
	if err != nil {
		return authTarget{}, err
	}
	be, backendID, modelID, err := selectAuthTarget(p, catalog, haveCatalog, backendID, modelID)
	if err != nil {
		return authTarget{}, err
	}
	model := be.Models[modelID]
	sel, _ := providerexec.ForBackend(be, model)
	layers := []map[string]string{be.Env, model.Env}
	if p.ID == "codex" {
		// Sign in to the personal profile launch refreshes from, never the
		// private session store (TS-04.R71).
		if home, err := config.PersonalCodexHome(); err == nil {
			layers = append(layers, map[string]string{"CODEX_HOME": home})
		}
	}
	return authTarget{provider: p, backendID: backendID, modelID: modelID, selection: sel, env: mergeEnv(os.Environ(), layers...)}, nil
}

func mergeEnv(base []string, layers ...map[string]string) []string {
	out := append([]string{}, base...)
	for _, layer := range layers {
		for k, v := range layer {
			out = append(out, k+"="+v)
		}
	}
	return out
}

// describeAuthTarget names the target and executable before login starts.
func describeAuthTarget(t authTarget) string {
	target := "no configured backend"
	if t.backendID != "" {
		target = fmt.Sprintf("backend %s, model %s", t.backendID, t.modelID)
	}
	return fmt.Sprintf("%s (%s) using %s", t.provider.Name, target, t.selection.Path)
}

// unavailableAuthTarget explains a provider that cannot be started, with
// install guidance instead of starting login (FS-10.R21).
func unavailableAuthTarget(t authTarget) string {
	switch t.selection.State {
	case providerexec.StateBundleUnavailable:
		return fmt.Sprintf("The AgentDeck bundle for %s is not available in this installation. Choose Installed provider for this backend in Settings, or reinstall AgentDeck.", t.provider.Name)
	case providerexec.StateMissing:
		if t.selection.Source == providerexec.SourceDetected {
			return fmt.Sprintf("%s is not installed. Install it (%s), then retry; or choose AgentDeck bundle for this backend in Settings.", t.provider.Name, providerInstallURL(t.provider.ID))
		}
		return fmt.Sprintf("The %s executable %q from %s was not found. Fix or clear that path in Settings, then retry.", t.provider.Name, t.selection.Override, t.selection.Provider.EnvKey)
	default:
		return fmt.Sprintf("The %s executable %q from %s is not usable. Use an absolute path or a command name.", t.provider.Name, t.selection.Override, t.selection.Provider.EnvKey)
	}
}

func providerInstallURL(id string) string {
	if id == "codex" {
		return "https://developers.openai.com/codex/cli"
	}
	return "https://code.claude.com/docs/en/setup"
}

// newAuthCmd builds `agentdeck auth <claude|codex>`.
func newAuthCmd() *cobra.Command {
	var check bool
	var backendID, modelID string
	cmd := &cobra.Command{
		Use:           "auth <claude|codex>",
		Short:         "Sign in to a provider by delegating to its own login flow",
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			p, ok := providerauth.Lookup(strings.ToLower(args[0]))
			if !ok {
				return fmt.Errorf("unknown provider %q; use 'claude' or 'codex'", args[0])
			}
			target, err := resolveAuthTarget(p, backendID, modelID)
			if err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "%s sign-in unavailable: %v.\n", p.Name, err)
				return errAuthFailed
			}
			if !target.selection.Available() {
				fmt.Fprintln(cmd.OutOrStdout(), unavailableAuthTarget(target))
				return errAuthFailed
			}
			if check {
				return runAuthCheck(cmd, target)
			}
			return runAuth(cmd, target)
		},
	}
	cmd.Flags().StringVar(&backendID, "backend", "", "configured backend whose provider to sign in")
	cmd.Flags().StringVar(&modelID, "model", "", "model of that backend (default: its default model)")
	cmd.Flags().BoolVar(&check, "check", false, "only check whether the provider is ready")
	_ = cmd.Flags().MarkHidden("check")
	return cmd
}

func runAuthCheck(cmd *cobra.Command, t authTarget) error {
	p := t.provider
	c, err := authStatusCommandFor(t)
	if err == nil {
		err = c.Run()
	}
	if err == nil {
		fmt.Fprintf(cmd.OutOrStdout(), "%s is ready.\n", p.Name)
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s needs sign-in. Run 'agentdeck auth %s' to continue.\n", p.Name, p.ID)
	return errAuthFailed
}

// runAuth delegates sign-in to the provider's own flow and reports a truthful,
// actionable outcome. It never prints or records credentials; success and
// cancellation leave a working installation, and only a genuine failure exits
// non-zero (FS-10.R5, FS-10.R11).
func runAuth(cmd *cobra.Command, t authTarget) error {
	p := t.provider
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Signing in to %s.\n", describeAuthTarget(t))
	c, err := authCommandFor(t)
	if err != nil {
		fmt.Fprintf(out, "%s sign-in unavailable: %v.\n", p.Name, err)
		fmt.Fprintf(out, "Your installation still works; retry with 'agentdeck auth %s'.\n", p.ID)
		return errAuthFailed
	}
	switch classifyAuth(c.Run()) {
	case authSuccess:
		fmt.Fprintf(out, "Signed in to %s.\n", p.Name)
		return nil
	case authCancelled:
		fmt.Fprintf(out, "%s sign-in cancelled. Your installation is ready; retry any time with 'agentdeck auth %s' or from the dashboard.\n", p.Name, p.ID)
		return nil
	default:
		fmt.Fprintf(out, "%s sign-in did not complete. Retry with 'agentdeck auth %s' or sign in from the dashboard.\n", p.Name, p.ID)
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
