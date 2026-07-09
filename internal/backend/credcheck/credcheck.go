// Package credcheck provides auth-ping credential validation for each
// backend type, used by PUT /api/backends. Checks are best-effort: timeouts
// and missing CLIs/keys yield "skipped", not "failed", so they never
// permanently block the save or onboarding by a flaky network.
package credcheck

import (
	"context"
	"time"

	"github.com/agentdeck/agentdeck/internal/config"
)

// DefaultTimeout is the per-probe deadline. Matches techspec §3.5.
const DefaultTimeout = 6 * time.Second

// CredResult is the outcome of a credential validation probe.
type CredResult struct {
	// Status is "ok", "failed", or "skipped".
	Status string `json:"status"`
	// Detail is a human-readable explanation for non-ok results.
	Detail string `json:"detail,omitempty"`
}

// Prober is the interface used by Check to run a backend-specific probe.
// Implementations are in claude.go and codex.go; tests inject a mock.
type Prober interface {
	Check(ctx context.Context, backend config.Backend, model config.Model, mergedEnv map[string]string) CredResult
}

var probers = map[string]Prober{
	"claude-acp":    claudeProber{},
	"codex-acp":     codexProber{},
	"opencode-acp":  opencodeProber{},
	"openhands-acp": openhandsProber{},
}

// Check dispatches to the backend-type-specific prober. Unknown backend types
// return skipped. The call is bounded by DefaultTimeout if the passed context
// has a longer deadline.
func Check(ctx context.Context, backend config.Backend, model config.Model, mergedEnv map[string]string) CredResult {
	p, ok := probers[backend.Type]
	if !ok {
		return CredResult{Status: "skipped", Detail: "unknown_backend_type"}
	}
	ctx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()
	return p.Check(ctx, backend, model, mergedEnv)
}

// MergeEnv merges backend-level env with model-level env (model wins on
// conflict). This is the §3.3 composition contract documented and tested here
// so backend storage and Phase 1 launch composition agree.
func MergeEnv(backendEnv, modelEnv map[string]string) map[string]string {
	merged := make(map[string]string, len(backendEnv)+len(modelEnv))
	for k, v := range backendEnv {
		merged[k] = v
	}
	for k, v := range modelEnv {
		merged[k] = v // model wins
	}
	return merged
}
