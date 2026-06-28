package credcheck

import (
	"context"
	"os/exec"
	"strings"

	"github.com/agentdeck/agentdeck/internal/config"
)

// claudeProber validates claude-acp credentials by running the Claude Code
// CLI's non-interactive auth-status command (techspec §3.5).
type claudeProber struct{}

func (claudeProber) Check(ctx context.Context, _ config.Backend, _ config.Model, mergedEnv map[string]string) CredResult {
	// Find the claude CLI on PATH (or in the merged env if CLAUDE_PATH is set).
	cliBin := "claude"
	if p, ok := mergedEnv["CLAUDE_PATH"]; ok && p != "" {
		cliBin = p
	}

	path, err := exec.LookPath(cliBin)
	if err != nil {
		return CredResult{Status: "skipped", Detail: "cli_not_installed"}
	}

	// Run `claude auth status` non-interactively. Exit 0 + no "not logged in"
	// output → ok; exit non-0 or "not logged in" output → failed.
	cmd := exec.CommandContext(ctx, path, "auth", "status", "--no-color")
	cmd.Env = buildEnv(mergedEnv)
	out, err := cmd.CombinedOutput()

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
