package credcheck

import (
	"context"
	"path/filepath"

	"github.com/agentdeck/agentdeck/internal/config"
)

// openhandsProber validates openhands-acp credentials without spending tokens
// (techspec §3.5): it confirms the CLI is installed and that auth is present —
// either LLM_API_KEY in the backend/model env (OpenHands authenticates the LLM
// via env) or the CLI's own settings file (`~/.openhands/settings.json`).
// Missing pieces yield "skipped", never "failed".
type openhandsProber struct{}

func (openhandsProber) Check(ctx context.Context, _ config.Backend, _ config.Model, mergedEnv map[string]string) CredResult {
	cliBin := "openhands"
	if p := mergedEnv["OPENHANDS_PATH"]; p != "" {
		cliBin = p
	}
	if _, err := lookPath(cliBin); err != nil {
		return CredResult{Status: "skipped", Detail: "cli_not_installed"}
	}
	if ctx.Err() != nil {
		return CredResult{Status: "skipped", Detail: "timeout"}
	}

	if mergedEnv["LLM_API_KEY"] != "" {
		return CredResult{Status: "ok"}
	}
	settingsPath := filepath.Join(homeDir(mergedEnv), ".openhands", "settings.json")
	if fileExists(settingsPath) {
		return CredResult{Status: "ok"}
	}
	return CredResult{Status: "skipped", Detail: "no_llm_api_key"}
}
