package credcheck

import (
	"context"
	"os/exec"
	"strings"

	"github.com/AsaphNoam/Chuck/internal/backend/providerauth"
	"github.com/AsaphNoam/Chuck/internal/backend/providerexec"
	"github.com/AsaphNoam/Chuck/internal/config"
)

// nativeOutcome is the bounded result of a provider-native readiness probe.
// There is deliberately no "raw output" member: the provider's status text can
// name the signed-in account, so only these categories cross back out
// (TS-04.R15).
type nativeOutcome int

const (
	// nativeReady means the provider reports a working native sign-in.
	nativeReady nativeOutcome = iota
	// nativeNotLoggedIn means the provider ran and reported no sign-in.
	nativeNotLoggedIn
	// nativeUnavailable means the probe could not be interrogated at all —
	// missing CLI, no status verb, timeout, or unparseable result. INV §12: an
	// unaskable tool is never reported as a failure, because a wrongly failed
	// gate blocks the user harder than no gate.
	nativeUnavailable
)

// selectedExecutable is the provider executable the shared resolver selects
// for this backend/model — the same one launch and terminal use (TS-04.R71).
// An unusable selection is a bounded skipped result naming installation or
// configuration, never a credential verdict (FS-09.R74).
func selectedExecutable(p providerauth.Provider, be config.Backend, model config.Model) (string, *CredResult) {
	be.Type = p.BackendType
	sel, _ := providerexec.ForBackend(be, model)
	switch sel.State {
	case providerexec.StateAvailable:
		return sel.Path, nil
	case providerexec.StateBundleUnavailable:
		return "", &CredResult{Status: "skipped", Detail: "bundle_unavailable"}
	case providerexec.StateMissing:
		return "", &CredResult{Status: "skipped", Detail: "cli_not_installed"}
	default:
		return "", &CredResult{Status: "skipped", Detail: "cli_invalid"}
	}
}

// probeNativeLogin asks the selected provider executable whether it is signed
// in, using the argv owned by providerauth (TS-04.R15). The child gets no
// stdin, inherits the merged provider environment, and is bounded by the
// caller's context deadline (TS-04.R16).
//
// extraArgs is appended to the shared status argv for provider-specific
// compatibility flags; it never replaces the fixed command or its base args.
func probeNativeLogin(ctx context.Context, path string, p providerauth.Provider, mergedEnv map[string]string, extraArgs ...string) (nativeOutcome, []byte, error) {
	if len(p.StatusArgs) == 0 {
		return nativeUnavailable, nil, nil
	}
	args := append(append([]string{}, p.StatusArgs...), extraArgs...)
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Env = buildEnv(mergedEnv)
	cmd.Stdin = nil
	out, runErr := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return nativeUnavailable, out, ctx.Err()
	}
	if runErr != nil {
		return classifyNativeOutput(out, nativeUnavailable), out, runErr
	}
	// A zero exit is not proof on its own: these CLIs can exit 0 while saying
	// they are not signed in.
	return classifyNativeOutput(out, nativeReady), out, nil
}

// classifyNativeOutput reads a provider's status text with a substring
// vocabulary rather than an exact format, so a wording or layout change
// degrades to the caller's fallback instead of a false verdict (INV §12).
func classifyNativeOutput(out []byte, zeroExit nativeOutcome) nativeOutcome {
	text := strings.ToLower(string(out))
	switch {
	case strings.Contains(text, "not logged in"),
		strings.Contains(text, "not authenticated"),
		strings.Contains(text, "not signed in"),
		strings.Contains(text, "no credentials"):
		return nativeNotLoggedIn
	case strings.Contains(text, "logged in"),
		strings.Contains(text, "authenticated"),
		strings.Contains(text, "signed in"):
		return nativeReady
	}
	return zeroExit
}
