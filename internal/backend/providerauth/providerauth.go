// Package providerauth is the single source of truth for how AgentDeck talks to
// a provider's own sign-in tooling: the interactive login argv used by
// `agentdeck auth`, and the non-interactive readiness argv used by the backend
// credential probe (TS-04.R15).
//
// Both callers previously carried their own copy of these commands — the CLI in
// internal/cli/auth.go and the readiness probe in internal/backend/credcheck —
// which is exactly the parallel-construction drift INV §2 names. They now read
// the same table, so a provider command can only be changed in one place.
//
// AgentDeck never starts, proxies, or observes a login *flow* from the server:
// the interactive command is only ever attached to a human's terminal by the
// CLI, and the readiness command is a bounded, stdin-less probe whose raw output
// never crosses a process, log, or API boundary (TS-04.R15/R16).
package providerauth

// Provider describes one provider's sign-in surface. Command/argv values are
// fixed literals in this file: nothing derived from a request, a backend name, a
// model, an environment value, or provider output can contribute an executable
// or an argument (TS-04.R16).
type Provider struct {
	// ID is the lowercase provider selector (`agentdeck auth <id>`).
	ID string
	// Name is the human-facing provider label.
	Name string
	// LoginCommand/LoginArgs run the provider's own interactive sign-in. Only
	// the CLI runs these, with the user's terminal attached.
	LoginCommand string
	LoginArgs    []string
	// StatusCommand/StatusArgs run a non-interactive readiness check. Empty
	// StatusArgs means the provider offers no such check, which is reported as
	// unavailable rather than as a failure (INV §12).
	StatusCommand string
	StatusArgs    []string
	// LoginEnvVar names an operator/test override for the interactive login
	// command only. A readiness probe deliberately does not honor it, so a
	// gated verifier cannot make an unready provider look ready.
	LoginEnvVar string
}

// providers is keyed by the CLI selector. Claude delegates both verbs to the
// official ACP adapter's bundled Claude executable. Codex uses the Codex CLI
// itself for both: `codex-acp` is a stdio ACP server that ignores argv, so it
// can neither log in nor report status (TS-06.R22 pins that CLI into the
// private release runtime for this reason).
var providers = map[string]Provider{
	"claude": {
		ID:            "claude",
		Name:          "Claude",
		LoginCommand:  "claude-agent-acp",
		LoginArgs:     []string{"--cli", "auth", "login"},
		StatusCommand: "claude-agent-acp",
		StatusArgs:    []string{"--cli", "auth", "status"},
		LoginEnvVar:   "AGENTDECK_CLAUDE_LOGIN_CMD",
	},
	"codex": {
		ID:            "codex",
		Name:          "Codex",
		LoginCommand:  "codex",
		LoginArgs:     []string{"login"},
		StatusCommand: "codex",
		StatusArgs:    []string{"login", "status"},
		LoginEnvVar:   "AGENTDECK_CODEX_LOGIN_CMD",
	},
}

// Lookup returns the provider for a CLI selector.
func Lookup(id string) (Provider, bool) {
	p, ok := providers[id]
	return p, ok
}

// ForBackendType maps a backend type to the provider that owns its sign-in, so
// the credential prober and the CLI agree on one command set. Backend types
// without provider-native sign-in return false.
func ForBackendType(backendType string) (Provider, bool) {
	switch backendType {
	case "claude-acp":
		return providers["claude"], true
	case "codex-acp":
		return providers["codex"], true
	}
	return Provider{}, false
}

// IDs returns the selectors accepted by `agentdeck auth`, in stable order.
func IDs() []string { return []string{"claude", "codex"} }
