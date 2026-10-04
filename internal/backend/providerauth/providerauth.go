// Package providerauth is the single source of truth for how Chuck talks to
// a provider's own sign-in tooling: the interactive login argv used by
// `chuck auth`, and the non-interactive readiness argv used by the backend
// credential probe (TS-04.R15).
//
// Both callers previously carried their own copy of these commands — the CLI in
// internal/cli/auth.go and the readiness probe in internal/backend/credcheck —
// which is exactly the parallel-construction drift INV §2 names. They now read
// the same table, so a provider command can only be changed in one place.
//
// Chuck never starts, proxies, or observes a login *flow* from the server:
// the interactive command is only ever attached to a human's terminal by the
// CLI, and the readiness command is a bounded, stdin-less probe whose raw output
// never crosses a process, log, or API boundary (TS-04.R15/R16).
package providerauth

// Provider describes one provider's sign-in surface. Argv values are fixed
// literals in this file: nothing derived from a request, a backend name, a
// model, an environment value, or provider output can contribute an argument
// (TS-04.R16). The executable is not a literal: it is the provider the shared
// resolver selects for the target backend/model (TS-04.R71), so sign-in,
// readiness and launch always use the same Claude/Codex.
type Provider struct {
	// ID is the lowercase provider selector (`chuck auth <id>`).
	ID string
	// Name is the human-facing provider label.
	Name string
	// BackendType is the backend type whose selected executable runs these
	// commands.
	BackendType string
	// LoginArgs run the provider's own interactive sign-in. Only the CLI runs
	// these, with the user's terminal attached.
	LoginArgs []string
	// StatusArgs run a non-interactive readiness check. Empty StatusArgs means
	// the provider offers no such check, which is reported as unavailable
	// rather than as a failure (INV §12).
	StatusArgs []string
	// LoginEnvVar names an advanced/test override for the interactive login
	// command only — never executable discovery or a readiness bypass. A
	// readiness probe deliberately does not honor it, so a gated verifier
	// cannot make an unready provider look ready.
	LoginEnvVar string
}

// providers is keyed by the CLI selector. Both run the selected provider's own
// native commands: Claude `auth login`/`auth status`, Codex `login`/`login
// status`. Neither depends on finding a different executable through PATH.
var providers = map[string]Provider{
	"claude": {
		ID:          "claude",
		Name:        "Claude",
		BackendType: "claude-acp",
		LoginArgs:   []string{"auth", "login"},
		StatusArgs:  []string{"auth", "status"},
		LoginEnvVar: "CHUCK_CLAUDE_LOGIN_CMD",
	},
	"codex": {
		ID:          "codex",
		Name:        "Codex",
		BackendType: "codex-acp",
		LoginArgs:   []string{"login"},
		StatusArgs:  []string{"login", "status"},
		LoginEnvVar: "CHUCK_CODEX_LOGIN_CMD",
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

// IDs returns the selectors accepted by `chuck auth`, in stable order.
func IDs() []string { return []string{"claude", "codex"} }
