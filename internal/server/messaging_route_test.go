package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/agentdeck/agentdeck/internal/messaging"
)

// tokenRoundTripper injects the per-agent session token header on every request.
type tokenRoundTripper struct {
	token string
	base  http.RoundTripper
}

func (t tokenRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	r.Header.Set(messaging.TokenHeader, t.token)
	return t.base.RoundTrip(r)
}

// TestMCPRouteMounted proves the in-process MCP messaging server is reachable
// through the real dashboard mux at /mcp: a go-sdk client connects over the
// streamable HTTP transport and round-trips the spike `ping` tool (Phase 5.1).
func TestMCPRouteMounted(t *testing.T) {
	srv := testServer(t, true)
	srv.messaging.Register("tok-route", "a_route")

	ts := httptest.NewServer(srv.routes())
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	transport := &mcp.StreamableClientTransport{
		Endpoint:   ts.URL + "/mcp",
		HTTPClient: &http.Client{Transport: tokenRoundTripper{token: "tok-route", base: http.DefaultTransport}},
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0.0.1"}, nil)
	cs, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("connect via dashboard mux: %v", err)
	}
	defer cs.Close()

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "ping",
		Arguments: map[string]any{"message": "wired"},
	})
	if err != nil {
		t.Fatalf("call ping: %v", err)
	}
	if res.IsError || len(res.Content) == 0 {
		t.Fatalf("ping failed: %+v", res)
	}
	if text, ok := res.Content[0].(*mcp.TextContent); !ok || text.Text != "wired" {
		t.Fatalf("ping content = %+v, want TextContent{wired}", res.Content[0])
	}
}
