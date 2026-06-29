// Package hooks owns the shell hook script set that terminal-bound agents use to
// push lifecycle status to POST /api/hook, plus the helpers that install them and
// compose the per-backend CLI hook registration (techspec §4). The scripts are
// embedded in the binary and rewritten on server startup so they always match
// the running binary's expectations.
package hooks

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed scripts/_post.sh scripts/session-start.sh scripts/user-prompt-submit.sh scripts/pre-tool-use.sh scripts/post-tool-use.sh scripts/stop.sh
var scriptFS embed.FS

// scriptByEvent maps an AgentDeck lifecycle event to its wrapper script's file
// name. _post.sh is the shared helper every wrapper execs.
var scriptByEvent = map[string]string{
	"SessionStart":     "session-start.sh",
	"UserPromptSubmit": "user-prompt-submit.sh",
	"PreToolUse":       "pre-tool-use.sh",
	"PostToolUse":      "post-tool-use.sh",
	"Stop":             "stop.sh",
}

// allScripts is every embedded script file (wrappers + the shared helper).
var allScripts = []string{
	"_post.sh",
	"session-start.sh", "user-prompt-submit.sh",
	"pre-tool-use.sh", "post-tool-use.sh", "stop.sh",
}

// Dir returns the hooks directory under the AgentDeck home ({home}/hooks).
func Dir(home string) string { return filepath.Join(home, "hooks") }

// ScriptPath returns the absolute path of the wrapper script for an AgentDeck
// lifecycle event, or "" if the event has no script.
func ScriptPath(home, event string) string {
	name, ok := scriptByEvent[event]
	if !ok {
		return ""
	}
	return filepath.Join(Dir(home), name)
}

// Install (re)writes every embedded hook script into {home}/hooks with 0o755
// perms. Idempotent and overwriting — the scripts always match the running
// binary so a server upgrade never leaves a stale script behind.
func Install(home string) error {
	dir := Dir(home)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("hooks: create dir %q: %w", dir, err)
	}
	for _, name := range allScripts {
		data, err := scriptFS.ReadFile("scripts/" + name)
		if err != nil {
			return fmt.Errorf("hooks: read embedded %q: %w", name, err)
		}
		dst := filepath.Join(dir, name)
		// Write atomically (temp + rename) so a concurrent agent never reads a
		// half-written script.
		tmp := dst + ".tmp"
		if err := os.WriteFile(tmp, data, 0o755); err != nil {
			return fmt.Errorf("hooks: write %q: %w", dst, err)
		}
		if err := os.Chmod(tmp, 0o755); err != nil {
			_ = os.Remove(tmp)
			return fmt.Errorf("hooks: chmod %q: %w", dst, err)
		}
		if err := os.Rename(tmp, dst); err != nil {
			_ = os.Remove(tmp)
			return fmt.Errorf("hooks: install %q: %w", dst, err)
		}
	}
	return nil
}

// ClaudeSettings composes a Claude Code settings object whose "hooks" block maps
// each CLI hook key (from the backend adapter's hookMap: AgentDeck event → CLI
// key) to a command that runs the matching wrapper script. The shape matches
// Claude Code's settings.json hooks format:
//
//	{"hooks": {"PreToolUse": [{"hooks": [{"type":"command","command":"<path>"}]}], ...}}
//
// Events absent from hookMap (the backend can't emit them) are skipped.
func ClaudeSettings(home string, hookMap map[string]string) map[string]any {
	hooksBlock := map[string]any{}
	for adEvent, cliKey := range hookMap {
		script := ScriptPath(home, adEvent)
		if script == "" {
			continue
		}
		hooksBlock[cliKey] = []any{
			map[string]any{
				"hooks": []any{
					map[string]any{"type": "command", "command": script},
				},
			},
		}
	}
	return map[string]any{"hooks": hooksBlock}
}

// WriteAgentSettings writes the composed settings JSON to a per-agent file under
// {home}/hooks/agents/{agentID}.json and returns its path. The runtime points the
// CLI at this file at launch so the agent's hooks fire for the right events.
func WriteAgentSettings(home, agentID string, settings map[string]any) (string, error) {
	dir := filepath.Join(Dir(home), "agents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("hooks: create agent settings dir: %w", err)
	}
	path := filepath.Join(dir, agentID+".json")
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return "", fmt.Errorf("hooks: marshal settings: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("hooks: write settings %q: %w", path, err)
	}
	return path, nil
}

// RemoveAgentSettings deletes the per-agent settings file written by
// WriteAgentSettings. A missing file is not an error, so cleanup is idempotent
// across stop/shutdown/relaunch.
func RemoveAgentSettings(home, agentID string) error {
	path := filepath.Join(Dir(home), "agents", agentID+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("hooks: remove settings %q: %w", path, err)
	}
	return nil
}

// RemoveAllAgentSettings deletes the entire per-agent settings directory. Used
// on dashboard shutdown to leave no stale registration artifacts behind.
func RemoveAllAgentSettings(home string) error {
	dir := filepath.Join(Dir(home), "agents")
	if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("hooks: remove settings dir %q: %w", dir, err)
	}
	return nil
}
