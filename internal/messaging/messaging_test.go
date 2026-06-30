package messaging

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// tokenRoundTripper injects the per-agent session token header on every request,
// mirroring how a CLI's HTTP MCP client carries the token (techspec §3.6).
type tokenRoundTripper struct {
	token string
	base  http.RoundTripper
}

func (t tokenRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	r.Header.Set(TokenHeader, t.token)
	return t.base.RoundTrip(r)
}

// TestSpikePingRoundTrip is the 5.1 handshake spike, exercised in-process: it
// stands up the messaging server over the streamable HTTP transport, connects
// the go-sdk MCP client with a per-agent token header, and round-trips a `ping`
// tool call. This proves the SDK integration + transport + per-agent session
// header work end-to-end without a real CLI (the live two-CLI confirmation is a
// gated acceptance — see HANDOFF Blocked on human).
func TestSpikePingRoundTrip(t *testing.T) {
	srv := New(nil, nil)
	srv.Register("tok-abc", "a_test1")

	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	transport := &mcp.StreamableClientTransport{
		Endpoint:   httpSrv.URL + "/mcp",
		HTTPClient: &http.Client{Transport: tokenRoundTripper{token: "tok-abc", base: http.DefaultTransport}},
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0.0.1"}, nil)
	cs, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer cs.Close()

	// The tool list must include the spike's ping tool.
	var names []string
	for tool, err := range cs.Tools(ctx, nil) {
		if err != nil {
			t.Fatalf("list tools: %v", err)
		}
		names = append(names, tool.Name)
	}
	if len(names) != 1 || names[0] != "ping" {
		t.Fatalf("tools = %v, want [ping]", names)
	}

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "ping",
		Arguments: map[string]any{"message": "hello"},
	})
	if err != nil {
		t.Fatalf("call ping: %v", err)
	}
	if res.IsError {
		t.Fatalf("ping returned tool error: %+v", res.Content)
	}
	if len(res.Content) == 0 {
		t.Fatal("ping returned no content")
	}
	text, ok := res.Content[0].(*mcp.TextContent)
	if !ok || text.Text != "hello" {
		t.Fatalf("ping content = %+v, want TextContent{hello}", res.Content[0])
	}
}

// TestSessionRegistry covers the token→agent_id binding the HTTP transport reads.
func TestSessionRegistry(t *testing.T) {
	srv := New(nil, nil)
	if _, ok := srv.Lookup("missing"); ok {
		t.Fatal("unknown token resolved")
	}
	srv.Register("tok-1", "a_one")
	if got, ok := srv.Lookup("tok-1"); !ok || got != "a_one" {
		t.Fatalf("Lookup(tok-1) = %q,%v want a_one,true", got, ok)
	}
	srv.Revoke("tok-1")
	if _, ok := srv.Lookup("tok-1"); ok {
		t.Fatal("revoked token still resolves")
	}
}
