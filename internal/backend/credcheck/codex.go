package credcheck

import (
	"context"
	"fmt"
	"net/http"

	"github.com/agentdeck/agentdeck/internal/backend/providerauth"
	"github.com/agentdeck/agentdeck/internal/config"
)

// codexProber validates codex-acp readiness. Native sign-in and a configured
// API key are independent acceptable paths (FS-09.R34), so it asks the pinned
// Codex CLI for `login status` first and only then falls back to the OpenAI
// /v1/models auth ping. The ping costs no tokens and directly tests whether the
// key is accepted.
//
// The order matters: a person who signed in with ChatGPT has no OPENAI_API_KEY
// at all, and reporting them as unready was the behavior that made onboarding
// look broken for a correctly configured Codex.
type codexProber struct{}

func (codexProber) Check(ctx context.Context, _ config.Backend, _ config.Model, mergedEnv map[string]string) CredResult {
	native := nativeUnavailable
	if p, ok := providerauth.ForBackendType("codex-acp"); ok {
		native, _, _ = probeNativeLogin(ctx, p, mergedEnv)
	}
	if native == nativeReady {
		return CredResult{Status: "ok"}
	}

	apiKey := mergedEnv["OPENAI_API_KEY"]
	if apiKey == "" {
		// Distinguish "the CLI told us nobody is signed in" from "we could not
		// ask": the first is actionable sign-in guidance, the second is not a
		// verdict about the user's credentials (INV §12).
		if native == nativeNotLoggedIn {
			return CredResult{Status: "failed", Detail: "not_logged_in"}
		}
		return CredResult{Status: "skipped", Detail: "no_api_key"}
	}

	baseURL := mergedEnv["OPENAI_BASE_URL"]
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	// Strip trailing slash for clean concatenation.
	for len(baseURL) > 0 && baseURL[len(baseURL)-1] == '/' {
		baseURL = baseURL[:len(baseURL)-1]
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/models", nil)
	if err != nil {
		return CredResult{Status: "skipped", Detail: fmt.Sprintf("request_build: %v", err)}
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return CredResult{Status: "skipped", Detail: "timeout"}
		}
		return CredResult{Status: "skipped", Detail: fmt.Sprintf("network_error: %v", err)}
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return CredResult{Status: "ok"}
	case http.StatusUnauthorized, http.StatusForbidden:
		return CredResult{Status: "failed", Detail: "invalid_api_key"}
	default:
		return CredResult{Status: "skipped", Detail: fmt.Sprintf("unexpected_status: %d", resp.StatusCode)}
	}
}
