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
	// claude-agent-acp adapter refuses a "nested" session when CLAUDECODE is set,
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

// ExtraEnvProvider is an optional interface a BackendAdapter may implement to
// contribute launch-derived environment variables ("K=V"), given the resolved
// model id and the effective skip-permissions flag (techspec §2.2, §2.3).
// OpenHands carries its model in LLM_MODEL and OpenCode injects a yolo
// permission block via OPENCODE_CONFIG_CONTENT — neither fits the ACP
// session/new params, so they ride the process env. Adapters that don't
// implement this contribute nothing. The runtime applies these AFTER
// StripEnvKeys, so every key an ExtraEnv value sets MUST also appear in
// StripEnvKeys to guarantee a single, authoritative dashboard-owned value.
type ExtraEnvProvider interface {
	ExtraEnv(modelID string, skipPerms bool) []string
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
	case "opencode-acp":
		return opencodeACP{}, true
	case "openhands-acp":
		return openhandsACP{}, true
	default:
		return nil, false
	}
}

// claudeACP is the adapter for the official claude-agent-acp package.
type claudeACP struct{}

func (claudeACP) Type() string         { return "claude-acp" }
func (claudeACP) Binary() string       { return "claude-agent-acp" }
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

// opencodeACP is the adapter for the OpenCode CLI (`opencode acp`). It speaks
// ACP over stdio like the other backends, so the chat runtime is unchanged; the
// binary/argv, env hygiene, and yolo-via-env differ (techspec §2.1, §2.3).
type opencodeACP struct{}

func (opencodeACP) Type() string         { return "opencode-acp" }
func (opencodeACP) Binary() string       { return "opencode" }
func (opencodeACP) LaunchArgs() []string { return []string{"acp"} }
func (opencodeACP) StripEnvKeys() []string {
	// CLAUDECODE: shared nested-session guard. OPENCODE_CONFIG / _CONTENT: strip
	// any shell-level config override so a user's ambient config never leaks into
	// a dashboard-managed agent — the adapter sets OPENCODE_CONFIG_CONTENT itself
	// for skip=true (ExtraEnv), and that value must be the only one.
	return []string{"CLAUDECODE", "OPENCODE_CONFIG", "OPENCODE_CONFIG_CONTENT"}
}

func (opencodeACP) ResolveResumeID(prevSessionID string, sameBackend bool) string {
	// GATED (7.4): attempt native ACP session/load with the prior id. Cross-backend
	// swaps have no compatible native session → "" drives the Phase 6 primer. If
	// 7.4 finds session/load unsupported, return "" unconditionally (primer floor).
	if !sameBackend {
		return ""
	}
	return prevSessionID
}

func (opencodeACP) CanSwitchModelOnResume() bool { return true }

// OpenCode has no AgentDeck hook surface; chat status derives from the ACP
// stream like every chat agent.
func (opencodeACP) HookMap() map[string]string      { return nil }
func (opencodeACP) UnsupportedHookEvents() []string { return unsupported(nil) }
func (opencodeACP) HookLaunchArgs(string) []string  { return nil }

// ExtraEnv injects the yolo permission config for skip=true (techspec §2.3):
// OPENCODE_CONFIG_CONTENT carries a full config JSON so the CLI auto-allows
// edit/bash/webfetch without raising ACP permission requests. Env-only — nothing
// on disk, torn down with the process. skip=false injects nothing (the CLI then
// raises requests, handled by the runtime permission gate). GATED: the exact key
// set is re-verified in 7.4. (The runtime gate also auto-approves as a backstop.)
func (opencodeACP) ExtraEnv(modelID string, skipPerms bool) []string {
	if !skipPerms {
		return nil
	}
	return []string{`OPENCODE_CONFIG_CONTENT={"permission":{"edit":"allow","bash":"allow","webfetch":"allow"}}`}
}

// openhandsACP is the adapter for the OpenHands CLI (`openhands acp`). Its one
// distinguishing trait is that model selection rides the LLM_MODEL env var
// rather than the ACP session param (techspec §2.2).
type openhandsACP struct{}

func (openhandsACP) Type() string         { return "openhands-acp" }
func (openhandsACP) Binary() string       { return "openhands" }
func (openhandsACP) LaunchArgs() []string { return []string{"acp"} }
func (openhandsACP) StripEnvKeys() []string {
	// CLAUDECODE: shared nested-session guard. LLM_MODEL: never inherit the shell's
	// model — the adapter sets it authoritatively from backend config (ExtraEnv).
	return []string{"CLAUDECODE", "LLM_MODEL"}
}

func (openhandsACP) ResolveResumeID(prevSessionID string, sameBackend bool) string {
	// GATED (7.4): same posture as OpenCode — native session/load attempt, primer
	// floor on cross-backend or a failed 7.4 verdict.
	if !sameBackend {
		return ""
	}
	return prevSessionID
}

func (openhandsACP) CanSwitchModelOnResume() bool { return true }

func (openhandsACP) HookMap() map[string]string      { return nil }
func (openhandsACP) UnsupportedHookEvents() []string { return unsupported(nil) }
func (openhandsACP) HookLaunchArgs(string) []string  { return nil }

// ExtraEnv sets LLM_MODEL from the resolved model id (OpenHands selects the
// model via env, not the ACP session param — techspec §2.2).
//
// skip=true (yolo) is deliberately NOT injected here: OpenHands exposes yolo as
// an ACP session permission mode / TUI flag (§2.3), but the session-mode arm
// would require a change to the shared sessionNewParams (forbidden by §1's
// "no runtime changes" rule, and claude's path doesn't select a mode), and the
// CLI's ACP always-approve flag is unverified. Meanwhile the shared runtime
// permission gate already auto-approves every request when SkipPerms is true
// (permission.go — backend-agnostic), so skip is functionally honored today.
// The CLI-side always-approve arm is GATED to 7.4 (its first question).
func (openhandsACP) ExtraEnv(modelID string, skipPerms bool) []string {
	if modelID == "" {
		return nil
	}
	return []string{"LLM_MODEL=" + modelID}
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
