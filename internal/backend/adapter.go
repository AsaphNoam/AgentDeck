// Package backend holds the per-backend adapter that encapsulates everything
// that differs between the ACP backends (claude-acp, codex-acp): the launch
// binary/argv, the env keys that must be stripped before spawn, the native
// resume mechanism, the hook event-name map, and the model-on-resume capability
// flag (techspec §6.3). The chat runtime stays generic and consults an adapter
// rather than branching on backend type inline.
package backend

import "sort"

// BackendAdapter abstracts one ACP backend type. Implementations are pure data +
// logic (no process or runtime imports) so the runtime can import this package
// without a cycle.
type BackendAdapter interface {
	// Type is the backends.json type string this adapter handles
	// ("claude-acp" | "codex-acp").
	Type() string

	// Binary is the default CLI adapter binary to exec. Tests may override the
	// command on the runtime; production uses this.
	Binary() string

	// LaunchArgs are the default adapter args (empty for both ACP backends today).
	LaunchArgs() []string

	// StripEnvKeys lists env keys to drop from the spawned process env. The
	// claude-code-acp adapter refuses a "nested" session when CLAUDECODE is set,
	// so claude strips it; codex strips nothing.
	StripEnvKeys() []string

	// ResolveResumeID returns the native session id to resume with, given the
	// previous native session id and whether the backend is unchanged across the
	// (re)launch. A cross-backend swap (sameBackend == false) has no compatible
	// native session, so it returns "" — the caller then drives the history-primer
	// path (techspec §5.3, wired in 6.5). Same-backend returns prevSessionID.
	ResolveResumeID(prevSessionID string, sameBackend bool) string

	// CanSwitchModelOnResume reports whether the backend keeps its native session
	// when only the model arg changes on resume. Drives §5.3's native-resume vs
	// primer decision for same-backend model swaps.
	CanSwitchModelOnResume() bool

	// HookMap maps an AgentDeck lifecycle event (SessionStart, UserPromptSubmit,
	// PreToolUse, PostToolUse, Stop) to the CLI's own hook key for registration
	// (techspec §2.3, §6.3). Events the backend cannot emit are absent from the
	// map; UnsupportedHookEvents lists them so the terminal runtime knows which
	// states to backfill from the ACP/notification channel.
	HookMap() map[string]string

	// UnsupportedHookEvents lists AgentDeck lifecycle events this backend has no
	// hook for (sorted). Empty when every event maps.
	UnsupportedHookEvents() []string

	// HookLaunchArgs are the adapter args that point the CLI at the composed
	// per-agent hook settings file at launch (techspec §2.3). Claude Code reads a
	// settings JSON via `--settings <path>`. Returns nil when the backend has no
	// settings-file mechanism (or it is not yet confirmed for that backend).
	HookLaunchArgs(settingsPath string) []string
}

// agentDeckHookEvents is the canonical AgentDeck lifecycle event set (techspec §4.2).
var agentDeckHookEvents = []string{
	"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "Stop",
}

// For returns the adapter for a backends.json type, or (nil, false) if unknown.
func For(backendType string) (BackendAdapter, bool) {
	switch backendType {
	case "claude-acp":
		return claudeACP{}, true
	case "codex-acp":
		return codexACP{}, true
	default:
		return nil, false
	}
}

// claudeACP is the adapter for Anthropic's claude-code-acp.
type claudeACP struct{}

func (claudeACP) Type() string         { return "claude-acp" }
func (claudeACP) Binary() string       { return "claude-code-acp" }
func (claudeACP) LaunchArgs() []string { return nil }
func (claudeACP) StripEnvKeys() []string {
	// The adapter refuses a nested session when CLAUDECODE is set (true when
	// AgentDeck itself is launched from a Claude Code terminal); AgentDeck spawns
	// independent agents, so the nested-session guard must never apply.
	return []string{"CLAUDECODE"}
}

func (claudeACP) ResolveResumeID(prevSessionID string, sameBackend bool) string {
	if !sameBackend {
		return ""
	}
	return prevSessionID
}

func (claudeACP) CanSwitchModelOnResume() bool { return true }

func (claudeACP) HookMap() map[string]string {
	// Claude Code exposes a 1:1 hook for every AgentDeck lifecycle event.
	return map[string]string{
		"SessionStart":     "SessionStart",
		"UserPromptSubmit": "UserPromptSubmit",
		"PreToolUse":       "PreToolUse",
		"PostToolUse":      "PostToolUse",
		"Stop":             "Stop",
	}
}

func (claudeACP) UnsupportedHookEvents() []string { return unsupported(claudeACP{}.HookMap()) }

func (claudeACP) HookLaunchArgs(settingsPath string) []string {
	if settingsPath == "" {
		return nil
	}
	return []string{"--settings", settingsPath}
}

// codexACP is the adapter for the codex-acp backend. It speaks ACP over stdio
// like claude-acp, so it reuses the chat runtime transport; only the binary,
// env, resume mechanism and hook map differ (techspec §2.4, §6).
type codexACP struct{}

func (codexACP) Type() string         { return "codex-acp" }
func (codexACP) Binary() string       { return "codex-acp" }
func (codexACP) LaunchArgs() []string { return nil }
func (codexACP) StripEnvKeys() []string {
	// Codex has no nested-session guard; nothing to strip.
	return nil
}

func (codexACP) ResolveResumeID(prevSessionID string, sameBackend bool) string {
	// Codex resumes via its CODEX_HOME-backed session store keyed by the native
	// session id; same shape as claude for resolve purposes. Cross-backend → "".
	if !sameBackend {
		return ""
	}
	return prevSessionID
}

func (codexACP) CanSwitchModelOnResume() bool { return true }

func (codexACP) HookMap() map[string]string {
	// NOTE (gated): the real codex-acp hook surface has not been confirmed against
	// a credentialed live CLI, same class as the Phase 1 real-CLI acceptance. We
	// target the same lifecycle keys Claude exposes; if a live Codex rejects any,
	// remove it here (it then lands in UnsupportedHookEvents and the terminal
	// runtime backfills that state from the ACP stream).
	return map[string]string{
		"SessionStart":     "SessionStart",
		"UserPromptSubmit": "UserPromptSubmit",
		"PreToolUse":       "PreToolUse",
		"PostToolUse":      "PostToolUse",
		"Stop":             "Stop",
	}
}

func (codexACP) UnsupportedHookEvents() []string { return unsupported(codexACP{}.HookMap()) }

func (codexACP) HookLaunchArgs(settingsPath string) []string {
	// GATED: the codex-acp hook settings mechanism is unconfirmed (no credentialed
	// CLI). Until verified, codex registers no settings file — the terminal runtime
	// backfills its status from the ACP stream. To enable: return the correct args.
	return nil
}

// unsupported returns the AgentDeck lifecycle events absent from hookMap, sorted.
func unsupported(hookMap map[string]string) []string {
	var out []string
	for _, ev := range agentDeckHookEvents {
		if _, ok := hookMap[ev]; !ok {
			out = append(out, ev)
		}
	}
	sort.Strings(out)
	return out
}
