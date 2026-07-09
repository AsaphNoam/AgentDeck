package credcheck

import (
	"context"
	"path/filepath"

	"github.com/agentdeck/agentdeck/internal/config"
)

// opencodeProber validates opencode-acp credentials without spending tokens
// (techspec §3.5): it confirms the CLI is installed and that some form of auth
// is present — either the CLI's own auth store (`~/.local/share/opencode/
// auth.json`, written by `opencode auth login`) or a provider API key carried
// in the backend/model env. Missing pieces yield "skipped", never "failed", so
// a login the dashboard can't see never blocks a save.
type opencodeProber struct{}

func (opencodeProber) Check(ctx context.Context, _ config.Backend, _ config.Model, mergedEnv map[string]string) CredResult {
	cliBin := "opencode"
	if p := mergedEnv["OPENCODE_PATH"]; p != "" {
		cliBin = p
	}
	if _, err := lookPath(cliBin); err != nil {
		return CredResult{Status: "skipped", Detail: "cli_not_installed"}
	}
	if ctx.Err() != nil {
		return CredResult{Status: "skipped", Detail: "timeout"}
	}

	// CLI-side login: auth.json under the opencode data dir.
	authPath := filepath.Join(homeDir(mergedEnv), ".local", "share", "opencode", "auth.json")
	if fileExists(authPath) {
		return CredResult{Status: "ok"}
	}
	// Or a provider API key supplied via backend env (opencode reads provider
	// keys like ANTHROPIC_API_KEY / OPENAI_API_KEY from the environment).
	if hasProviderAPIKey(mergedEnv) {
		return CredResult{Status: "ok"}
	}
	return CredResult{Status: "skipped", Detail: "not_logged_in"}
}
