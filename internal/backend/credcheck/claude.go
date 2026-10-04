package credcheck

import (
	"context"
	"strings"

	"github.com/AsaphNoam/Chuck/internal/backend/providerauth"
	"github.com/AsaphNoam/Chuck/internal/config"
)

// claudeProber validates claude-acp credentials with the same Claude
// executable chat and terminal launch select for this backend/model.
type claudeProber struct{}

func (claudeProber) Check(ctx context.Context, backend config.Backend, model config.Model, mergedEnv map[string]string) CredResult {
	p, ok := providerauth.ForBackendType("claude-acp")
	if !ok {
		return CredResult{Status: "skipped", Detail: "unknown_backend_type"}
	}
	path, unusable := selectedExecutable(p, backend, model)
	if unusable != nil {
		return *unusable
	}

	// Run `claude auth status` non-interactively. Older Claude builds may not
	// support `--no-color`, so retry once without it before surfacing a
	// failure (INV §12).
	outcome, out, err := probeNativeLogin(ctx, path, p, mergedEnv, "--no-color")
	if err != nil && rejectsNoColorFlag(out) {
		outcome, out, err = probeNativeLogin(ctx, path, p, mergedEnv)
	}

	if ctx.Err() != nil {
		return CredResult{Status: "skipped", Detail: "timeout"}
	}
	switch outcome {
	case nativeReady:
		return CredResult{Status: "ok"}
	case nativeNotLoggedIn:
		return CredResult{Status: "failed", Detail: "not_logged_in"}
	}
	if err != nil {
		// A CLI that rejects the status argv itself is incompatible, not
		// un-credentialed. INV §12: a
		// tool that cannot be interrogated reports skipped, never failed,
		// because a wrongly failed gate sends the operator to repair
		// credentials that are fine (FS-04.R34/A14).
		if rejectsArgument(out) {
			return CredResult{Status: "skipped", Detail: "cli_incompatible"}
		}
		// Return only a bounded vocabulary; raw CLI output and account
		// identity never cross the API boundary (TS-04.R15, INV §8/§12).
		return CredResult{Status: "failed", Detail: "status_check_failed"}
	}
	return CredResult{Status: "ok"}
}

// rejectsNoColorFlag recognizes common CLI-parser diagnostics while requiring
// both the optional flag and unsupported-argument vocabulary. That keeps an
// unrelated auth/status failure from triggering the compatibility retry.
func rejectsNoColorFlag(out []byte) bool {
	return strings.Contains(strings.ToLower(string(out)), "-no-color") && rejectsArgument(out)
}

// rejectsArgument reports whether output is a CLI parser refusing one of the
// arguments Chuck passed, rather than a status answer. The vocabulary is
// substring-based so a wording change degrades to the caller's fallback
// instead of a false verdict (INV §12).
func rejectsArgument(out []byte) bool {
	text := strings.ToLower(string(out))
	for _, marker := range []string{
		"unknown option",
		"unknown flag",
		"unrecognized option",
		"unrecognized flag",
		"unsupported option",
		"unsupported flag",
		"invalid option",
		"invalid flag",
		"flag provided but not defined",
		"unexpected argument",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}
