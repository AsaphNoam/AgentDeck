package credcheck

import (
	"context"
	"os/exec"
	"strings"

	"github.com/agentdeck/agentdeck/internal/config"
)

// claudeProber validates claude-acp credentials by delegating through the same
// official adapter executable chat launch requires. The adapter's --cli mode
// runs its bundled compatible Claude executable.
type claudeProber struct{}

func (claudeProber) Check(ctx context.Context, _ config.Backend, _ config.Model, mergedEnv map[string]string) CredResult {
	path, err := exec.LookPath("claude-agent-acp")
	if err != nil {
		return CredResult{Status: "skipped", Detail: "cli_not_installed"}
	}

	// Run the adapter's bundled `claude auth status` non-interactively. Older
	// bundled Claude builds may not support `--no-color`, so retry once without
	// it before surfacing a failure.
	out, err := runClaudeAuthStatus(ctx, path, mergedEnv, true)
	if err != nil && strings.Contains(strings.ToLower(string(out)), "unknown option '--no-color'") {
		out, err = runClaudeAuthStatus(ctx, path, mergedEnv, false)
	}

	if ctx.Err() != nil {
		return CredResult{Status: "skipped", Detail: "timeout"}
	}
	if err != nil {
		output := strings.TrimSpace(string(out))
		if output == "" {
			output = err.Error()
		}
		// Mask any secrets before returning details.
		return CredResult{Status: "failed", Detail: sanitizeOutput(output)}
	}
	// Parse the output: the CLI may exit 0 but say "not logged in".
	output := strings.ToLower(string(out))
	if strings.Contains(output, "not logged in") || strings.Contains(output, "not authenticated") {
		return CredResult{Status: "failed", Detail: "not_logged_in"}
	}
	return CredResult{Status: "ok"}
}

func runClaudeAuthStatus(ctx context.Context, path string, mergedEnv map[string]string, noColor bool) ([]byte, error) {
	args := []string{"--cli", "auth", "status"}
	if noColor {
		args = append(args, "--no-color")
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Env = buildEnv(mergedEnv)
	return cmd.CombinedOutput()
}
