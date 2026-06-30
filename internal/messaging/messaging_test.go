package messaging

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/agentdeck/agentdeck/internal/state"
)

// tokenRoundTripper injects the per-agent session token header on every request,
// mirroring how a CLI's HTTP MCP client carries the token (techspec §3.6). An
// empty token sends no header (simulates an unregistered session).
type tokenRoundTripper struct {
	token string
	base  http.RoundTripper
}

func (t tokenRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	if t.token != "" {
		r.Header.Set(TokenHeader, t.token)
	}
	return t.base.RoundTrip(r)
}

// connect stands up the messaging server over the streamable HTTP transport and
// returns a connected go-sdk client session carrying the given token.
func connect(t *testing.T, srv *Server, token string) *mcp.ClientSession {
	t.Helper()
	httpSrv := httptest.NewServer(srv.Handler())
	t.Cleanup(httpSrv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	transport := &mcp.StreamableClientTransport{
		Endpoint:   httpSrv.URL + "/mcp",
		HTTPClient: &http.Client{Transport: tokenRoundTripper{token: token, base: http.DefaultTransport}},
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0.0.1"}, nil)
	cs, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// call invokes a tool and returns the decoded JSON map + IsError flag.
func call(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) (map[string]any, bool) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	if len(res.Content) == 0 {
		t.Fatalf("call %s: no content", name)
	}
	text, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("call %s: content[0] not text: %T", name, res.Content[0])
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(text.Text), &m); err != nil {
		t.Fatalf("call %s: bad JSON %q: %v", name, text.Text, err)
	}
	return m, res.IsError
}

// liveAgent writes an agent + running row into the store so it's addressable.
func liveAgent(t *testing.T, st *state.Store, id, name, role, project string) {
	t.Helper()
	if err := st.WriteAgent(state.Agent{
		AgentID: id, Name: name, Role: role, Project: project,
		Backend: "claude", Model: "sonnet", Interface: "chat", CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("WriteAgent %s: %v", id, err)
	}
	if err := st.WriteRunning(state.RunningEntry{
		AgentID: id, PID: 1, SessionID: "s_" + id, Interface: "chat", StartedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("WriteRunning %s: %v", id, err)
	}
}

func newStore(t *testing.T) *state.Store {
	t.Helper()
	st, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

// TestSendNudgeRoundTrip-free: the full send→check round-trip across two agents
// over the real HTTP transport, proving identity-from-session and the store.
func TestSendAndCheckRoundTrip(t *testing.T) {
	st := newStore(t)
	liveAgent(t, st, "a_impl", "Atlas", "implementer", "my-app")
	liveAgent(t, st, "a_rev", "Nova", "reviewer", "my-app")

	srv := New(st, nil)
	srv.Register("tok-impl", "a_impl")
	srv.Register("tok-rev", "a_rev")

	// Implementer lists agents — sees only the reviewer (self excluded).
	implCS := connect(t, srv, "tok-impl")
	res, isErr := call(t, implCS, "list_agents", nil)
	if isErr {
		t.Fatalf("list_agents error: %v", res)
	}
	agents := res["agents"].([]any)
	if len(agents) != 1 || agents[0].(map[string]any)["address"] != "reviewer@my-app" {
		t.Fatalf("list_agents = %v, want [reviewer@my-app]", agents)
	}

	// Implementer sends to reviewer by address.
	res, isErr = call(t, implCS, "send_message", map[string]any{
		"to": "reviewer@my-app", "body": "Please review the diff.", "subject": "Review",
	})
	if isErr || res["ok"] != true {
		t.Fatalf("send_message failed: %v", res)
	}
	if res["to"] != "a_rev" || res["to_address"] != "reviewer@my-app" {
		t.Fatalf("send_message result = %v", res)
	}

	// Reviewer checks messages — sees the message, from = sender's session id.
	revCS := connect(t, srv, "tok-rev")
	res, isErr = call(t, revCS, "check_messages", nil)
	if isErr {
		t.Fatalf("check_messages error: %v", res)
	}
	msgs := res["messages"].([]any)
	if len(msgs) != 1 {
		t.Fatalf("check_messages = %v, want 1", msgs)
	}
	m := msgs[0].(map[string]any)
	if m["from"] != "a_impl" || m["from_address"] != "implementer@my-app" || m["body"] != "Please review the diff." {
		t.Fatalf("message = %v", m)
	}
	// Marked read by default → no unread remaining.
	if remaining, _ := res["remaining"].(float64); remaining != 0 {
		t.Fatalf("remaining = %v, want 0", res["remaining"])
	}
}

// TestSendIdentityNotSpoofable: `from` is the session's agent_id regardless of
// any argument, and an unknown token is rejected with session_unknown.
func TestSendIdentityNotSpoofable(t *testing.T) {
	st := newStore(t)
	liveAgent(t, st, "a_impl", "Atlas", "implementer", "my-app")
	liveAgent(t, st, "a_rev", "Nova", "reviewer", "my-app")

	srv := New(st, nil)
	srv.Register("tok-impl", "a_impl")

	// Unknown/absent token → session_unknown on every tool (with per-tool valid
	// args, since the SDK validates the input schema before the handler runs).
	anonCS := connect(t, srv, "")
	anonCalls := []struct {
		tool string
		args map[string]any
	}{
		{"list_agents", nil},
		{"send_message", map[string]any{"to": "reviewer@my-app", "body": "hi"}},
		{"check_messages", nil},
	}
	for _, c := range anonCalls {
		res, isErr := call(t, anonCS, c.tool, c.args)
		if !isErr || res["error"] != "session_unknown" {
			t.Fatalf("%s with no token = %v (isErr=%v), want session_unknown", c.tool, res, isErr)
		}
	}

	// A registered sender cannot impersonate another agent — the row's from_agent
	// is the session's id even though no `from` argument exists in the schema.
	implCS := connect(t, srv, "tok-impl")
	if res, isErr := call(t, implCS, "send_message", map[string]any{"to": "a_rev", "body": "hi"}); isErr || res["ok"] != true {
		t.Fatalf("send_message: %v", res)
	}
	msgs, err := st.ListMessages("a_rev", true, 0)
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(msgs) != 1 || msgs[0].FromAgent != "a_impl" {
		t.Fatalf("from_agent = %+v, want a_impl", msgs)
	}
}

// TestSendErrors covers the not-found and ambiguous resolution error shapes.
func TestSendErrors(t *testing.T) {
	st := newStore(t)
	liveAgent(t, st, "a_impl", "Atlas", "implementer", "my-app")
	liveAgent(t, st, "a_e1", "Echo", "reviewer", "my-app")
	liveAgent(t, st, "a_e2", "Echo", "reviewer", "my-app")

	srv := New(st, nil)
	srv.Register("tok-impl", "a_impl")
	cs := connect(t, srv, "tok-impl")

	res, isErr := call(t, cs, "send_message", map[string]any{"to": "ghost@my-app", "body": "hi"})
	if !isErr || res["error"] != "recipient_not_found" {
		t.Fatalf("not-found = %v", res)
	}

	res, isErr = call(t, cs, "send_message", map[string]any{"to": "reviewer@my-app", "body": "hi"})
	if !isErr || res["error"] != "ambiguous_recipient" {
		t.Fatalf("ambiguous = %v", res)
	}
	if cands := res["candidates"].([]any); len(cands) != 2 {
		t.Fatalf("candidates = %v, want 2", cands)
	}

	res, isErr = call(t, cs, "send_message", map[string]any{"to": "a_e1", "body": ""})
	if !isErr || res["error"] != "invalid_body" {
		t.Fatalf("invalid body = %v", res)
	}
}

func TestSendMessageBudgetExceeded(t *testing.T) {
	st := newStore(t)
	liveAgent(t, st, "a_impl", "Atlas", "implementer", "my-app")
	liveAgent(t, st, "a_rev", "Nova", "reviewer", "my-app")
	if err := st.ResetTurnBudget("a_impl", "t_000000000001"); err != nil {
		t.Fatalf("ResetTurnBudget: %v", err)
	}

	srv := New(st, nil)
	srv.Register("tok-impl", "a_impl")
	var budgetEvents int
	srv.SetBudgetExceededSink(func(agentID, turnID string, used int) {
		if agentID != "a_impl" || turnID != "t_000000000001" || used != MessageBudgetPerTurn {
			t.Fatalf("budget event = %s/%s/%d, want a_impl/t_000000000001/%d", agentID, turnID, used, MessageBudgetPerTurn)
		}
		budgetEvents++
	})
	cs := connect(t, srv, "tok-impl")

	for i := 0; i < MessageBudgetPerTurn; i++ {
		res, isErr := call(t, cs, "send_message", map[string]any{"to": "a_rev", "body": "ping"})
		if isErr || res["ok"] != true {
			t.Fatalf("send %d = %v isErr=%v, want success", i+1, res, isErr)
		}
	}
	res, isErr := call(t, cs, "send_message", map[string]any{"to": "a_rev", "body": "one too many"})
	if !isErr || res["error"] != "message_budget_exceeded" {
		t.Fatalf("16th send = %v isErr=%v, want message_budget_exceeded", res, isErr)
	}
	if budgetEvents != 1 {
		t.Fatalf("budget events = %d, want 1", budgetEvents)
	}
	msgs, err := st.ListMessages("a_rev", false, 0)
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(msgs) != MessageBudgetPerTurn {
		t.Fatalf("messages inserted = %d, want %d", len(msgs), MessageBudgetPerTurn)
	}
	budget, err := st.CurrentTurnBudget("a_impl", MessageBudgetPerTurn)
	if err != nil {
		t.Fatalf("CurrentTurnBudget: %v", err)
	}
	if !budget.Breached || budget.Outbound != MessageBudgetPerTurn || budget.Remaining != 0 {
		t.Fatalf("budget row = %+v, want breached outbound=%d remaining=0", budget, MessageBudgetPerTurn)
	}
}

func TestCheckMessagesCapsAtRemainingBudget(t *testing.T) {
	st := newStore(t)
	liveAgent(t, st, "a_impl", "Atlas", "implementer", "my-app")
	liveAgent(t, st, "a_rev", "Nova", "reviewer", "my-app")
	if err := st.ResetTurnBudget("a_rev", "t_000000000001"); err != nil {
		t.Fatalf("ResetTurnBudget: %v", err)
	}
	if _, _, err := st.ConsumeTurnBudget("a_rev", MessageBudgetPerTurn-2, 0, MessageBudgetPerTurn); err != nil {
		t.Fatalf("ConsumeTurnBudget: %v", err)
	}
	for i := 0; i < 5; i++ {
		if _, err := st.InsertMessage(state.Message{
			FromAgent: "a_impl", FromAddress: "implementer@my-app", FromName: "Atlas",
			ToAgent: "a_rev", Body: "queued",
		}); err != nil {
			t.Fatalf("InsertMessage %d: %v", i, err)
		}
	}

	srv := New(st, nil)
	srv.Register("tok-rev", "a_rev")
	cs := connect(t, srv, "tok-rev")
	res, isErr := call(t, cs, "check_messages", map[string]any{"limit": 5})
	if isErr {
		t.Fatalf("check_messages = %v, want success", res)
	}
	msgs := res["messages"].([]any)
	if len(msgs) != 2 {
		t.Fatalf("returned messages = %d, want 2 due remaining budget", len(msgs))
	}
	if got := int(res["budget_remaining"].(float64)); got != 0 {
		t.Fatalf("budget_remaining = %d, want 0", got)
	}
	if got := int(res["remaining"].(float64)); got != 3 {
		t.Fatalf("remaining unread = %d, want 3", got)
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
