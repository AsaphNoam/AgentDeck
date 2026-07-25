package credcheck

import (
	"context"
	"strings"

	"github.com/agentdeck/agentdeck/internal/backend/providerauth"
	"github.com/agentdeck/agentdeck/internal/config"
)

// claudeProber validates claude-acp credentials by delegating through the same
// official adapter executable chat launch requires. The adapter's --cli mode
// runs its bundled compatible Claude executable.
type claudeProber struct{}

func (claudeProber) Check(ctx context.Context, _ config.Backend, _ config.Model, mergedEnv map[string]string) CredResult {
	p, ok := providerauth.ForBackendType("claude-acp")
	if !ok {
		return CredResult{Status: "skipped", Detail: "unknown_backend_type"}
	}
	if _, err := lookPath(p.StatusCommand); err != nil {
		return CredResult{Status: "skipped", Detail: "cli_not_installed"}
	}

	// Run the adapter's bundled `claude auth status` non-interactively. Older
	// bundled Claude builds may not support `--no-color`, so retry once without
	// it before surfacing a failure (INV §12).
	outcome, out, err := probeNativeLogin(ctx, p, mergedEnv, "--no-color")
	if err != nil && strings.Contains(strings.ToLower(string(out)), "unknown option '--no-color'") {
		outcome, out, err = probeNativeLogin(ctx, p, mergedEnv)
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
		output := strings.TrimSpace(string(out))
		if output == "" {
			output = err.Error()
		}
		// Mask any secrets before returning details.
		return CredResult{Status: "failed", Detail: sanitizeOutput(output)}
	}
	return CredResult{Status: "ok"}
}
