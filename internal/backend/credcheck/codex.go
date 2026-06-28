package credcheck

import (
	"context"
	"fmt"
	"net/http"

	"github.com/agentdeck/agentdeck/internal/config"
)

// codexProber validates codex-acp credentials by hitting the OpenAI
// /v1/models endpoint (techspec §3.5). This is an auth ping: it costs
// no tokens and directly tests whether the key is accepted.
type codexProber struct{}

func (codexProber) Check(ctx context.Context, _ config.Backend, _ config.Model, mergedEnv map[string]string) CredResult {
	apiKey := mergedEnv["OPENAI_API_KEY"]
	if apiKey == "" {
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
