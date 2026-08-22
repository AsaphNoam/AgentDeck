package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/agentdeck/agentdeck/internal/messaging"
	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/transcript"
)

// contextMCPSession connects an MCP client to the dashboard's real /mcp route
// carrying a registered session token, so these tests exercise the wiring the
// dashboard actually ships rather than a hand-built service.
func contextMCPSession(t *testing.T, srv *Server, ts *httptest.Server, agentID, token string) *mcp.ClientSession {
	t.Helper()
	srv.messaging.RegisterSession(token, agentID, token)
	t.Cleanup(func() { srv.messaging.Revoke(token) })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	transport := &mcp.StreamableClientTransport{
		Endpoint: ts.URL + "/mcp",
		HTTPClient: &http.Client{Transport: contextTokenTransport{
			token: token, host: strings.TrimPrefix(ts.URL, "http://"), base: http.DefaultTransport}},
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "context-test", Version: "v0.0.1"}, nil)
	cs, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("connect mcp: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// The loopback guard rejects a non-local Host by design (INV §14), so the
// transport carries both the session token and a loopback Host.
type contextTokenTransport struct {
	token string
	host  string
	base  http.RoundTripper
}

func (c contextTokenTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r.Header.Set(messaging.TokenHeader, c.token)
	r.Host = c.host
	return c.base.RoundTrip(r)
}

func callContextTool(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) (map[string]any, bool) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	text, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("call %s: content[0] is %T", name, res.Content[0])
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(text.Text), &out); err != nil {
		t.Fatalf("call %s: bad JSON %q: %v", name, text.Text, err)
	}
	return out, res.IsError
}

// FS-15.A6 (R10) — sharing, listing, and reading context on the live dashboard
// start no model turn and publish no source content. Only the recipient's
// explicit read returns the bytes, and it returns them to the MCP caller rather
// than to any provider conversation.
func TestContextSharingStartsNoModelTurn(t *testing.T) {
	srv, ts, promptLog := activationTestServer(t)
	sharer := launchAndWaitIdle(t, ts, "impl", "tmpproj")
	recipient := launchAndWaitIdle(t, ts, "impl", "tmpproj")

	// Give the sharer a real completed turn to hand over. The prompt text is
	// durable transcript content, so it doubles as the marker that must never
	// appear anywhere except the recipient's explicit read result.
	secret := "the credential rotation plan"
	promptAndWaitTurnEnd(t, srv, ts, sharer, secret)

	baseline := promptCount(t, promptLog)
	if baseline == 0 {
		t.Fatal("prompt log recorded no turn for the real prompt; the counter cannot detect a leak")
	}
	events, unsub := srv.eventBus.Subscribe()
	defer unsub()

	sharerCS := contextMCPSession(t, srv, ts, sharer, "tok-ctx-sharer")
	res, isErr := callContextTool(t, sharerCS, "share_context", map[string]any{
		"to": recipient, "source": "latest_completed_turn", "label": "rotation plan"})
	if isErr || res["ok"] != true {
		t.Fatalf("share_context over the live dashboard: %v", res)
	}
	refID := res["context_ref_id"].(string)

	recipientCS := contextMCPSession(t, srv, ts, recipient, "tok-ctx-recipient")
	if list, isErr := callContextTool(t, recipientCS, "list_context_links", nil); isErr || len(list["links"].([]any)) != 1 {
		t.Fatalf("list_context_links: %v", list)
	}
	read, isErr := callContextTool(t, recipientCS, "read_context_link", map[string]any{"context_ref_id": refID})
	if isErr || !strings.Contains(read["text"].(string), secret) {
		t.Fatalf("read_context_link: %v", read)
	}

	// No context operation crossed the provider wire.
	time.Sleep(300 * time.Millisecond)
	if got := promptCount(t, promptLog); got != baseline {
		t.Fatalf("context operations sent %d provider prompts", got-baseline)
	}
	// No mailbox row and no activation was created.
	var messages, activations int
	if err := srv.stateStore.DB().QueryRow(
		`SELECT (SELECT COUNT(*) FROM messages), (SELECT COUNT(*) FROM activations)`).Scan(&messages, &activations); err != nil {
		t.Fatalf("count: %v", err)
	}
	if messages != 0 || activations != 0 {
		t.Fatalf("context operations produced %d messages and %d activations", messages, activations)
	}
	// Nothing the dashboard published carried the source bytes.
	for {
		select {
		case ev := <-events:
			payload, err := json.Marshal(ev)
			if err != nil {
				t.Fatalf("marshal published event: %v", err)
			}
			if strings.Contains(string(payload), secret) {
				t.Fatalf("published event carried source content: %s", payload)
			}
			continue
		default:
		}
		break
	}
	// And the sharer's transcript gained no record of the context operation.
	for _, ev := range readTestTranscript(t, srv, sharer) {
		if strings.Contains(string(ev.Data), "context_ref") || strings.Contains(string(ev.Data), "share_context") {
			t.Fatalf("context operation wrote a transcript event: %+v", ev)
		}
	}
}

// FS-15.A7 (R12) — a stopped recipient keeps its grants and reads them after an
// ordinary resume; the grant itself never woke it.
func TestStoppedRecipientKeepsContextAcrossResume(t *testing.T) {
	srv, ts, promptLog := activationTestServer(t)
	sharer := launchAndWaitIdle(t, ts, "impl", "tmpproj")
	recipient := launchThenStop(t, srv, ts)
	promptAndWaitTurnEnd(t, srv, ts, sharer, "durable conclusion")

	baseline := promptCount(t, promptLog)
	if baseline == 0 {
		t.Fatal("prompt log recorded no turn for the real prompt; the counter cannot detect a leak")
	}
	sharerCS := contextMCPSession(t, srv, ts, sharer, "tok-ctx-sharer")
	res, isErr := callContextTool(t, sharerCS, "share_context", map[string]any{
		"to": recipient, "source": "latest_completed_turn"})
	if isErr || res["ok"] != true {
		t.Fatalf("share to a stopped agent: %v", res)
	}
	refID := res["context_ref_id"].(string)

	// The share did not wake it and started no turn.
	time.Sleep(300 * time.Millisecond)
	waitRunning(t, srv, recipient, false)
	if got := promptCount(t, promptLog); got != baseline {
		t.Fatalf("sharing to a stopped agent sent %d provider prompts", got-baseline)
	}

	// After an ordinary resume the grant is still there.
	if resp, body := post(t, ts.URL+"/api/sessions/"+recipient+"/resume", nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("resume status = %d: %s", resp.StatusCode, body)
	}
	waitRunning(t, srv, recipient, true)
	recipientCS := contextMCPSession(t, srv, ts, recipient, "tok-ctx-recipient")
	list, isErr := callContextTool(t, recipientCS, "list_context_links", nil)
	if isErr || len(list["links"].([]any)) != 1 {
		t.Fatalf("resumed recipient list = %v", list)
	}
	if read, isErr := callContextTool(t, recipientCS, "read_context_link",
		map[string]any{"context_ref_id": refID}); isErr || !strings.Contains(read["text"].(string), "durable conclusion") {
		t.Fatalf("resumed recipient read = %v", read)
	}
}

// promptAndWaitTurnEnd drives one real fake-ACP turn and waits for its durable
// turn_end, so the shared span is transcript content the runtime actually wrote.
func promptAndWaitTurnEnd(t *testing.T, srv *Server, ts *httptest.Server, agentID, text string) {
	t.Helper()
	if resp, body := post(t, ts.URL+"/api/sessions/"+agentID+"/prompt",
		map[string]string{"text": text}); resp.StatusCode != http.StatusAccepted {
		t.Fatalf("prompt status = %d: %s", resp.StatusCode, body)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		for _, ev := range readTestTranscript(t, srv, agentID) {
			if ev.Type == runtime.EvTurnEnd {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no durable turn_end for %s", agentID)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func readTestTranscript(t *testing.T, srv *Server, agentID string) []runtime.Event {
	t.Helper()
	events, err := transcript.ReadFile(srv.configStore.Home(), agentID, transcript.ReadOptions{IncludeMeta: true})
	if err != nil {
		t.Fatalf("read transcript for %s: %v", agentID, err)
	}
	return events
}
