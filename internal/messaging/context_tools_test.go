package messaging

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/agentdeck/agentdeck/internal/contextref"
	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
	"github.com/agentdeck/agentdeck/internal/transcript"
)

// contextFixture stands up a store, a transcript home, the context service, and
// the MCP server they share, so the tools are exercised over the real transport
// with real token-derived identity (TS-04.R28, TS-05.R16).
type contextFixture struct {
	store *state.Store
	srv   *Server
	home  string
}

func newContextFixture(t *testing.T) *contextFixture {
	t.Helper()
	home := t.TempDir()
	store, err := state.Open(home)
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	srv := New(store, nil)
	srv.SetContextService(contextref.New(store, home))
	return &contextFixture{store: store, srv: srv, home: home}
}

func (f *contextFixture) transcriptEvents(t *testing.T, agentID string, events ...runtime.Event) {
	t.Helper()
	w, err := transcript.Open(f.home, agentID, &runtime.SessionMetaData{Name: agentID, Backend: "claude"})
	if err != nil {
		t.Fatalf("open transcript: %v", err)
	}
	defer w.Close()
	for _, e := range events {
		if err := w.Append(e); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
}

func contextEvent(t *testing.T, typ string, data any) runtime.Event {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return runtime.Event{Type: typ, Data: raw, Ts: "2026-08-22T10:00:00Z"}
}

// A2/A6 — two token-bound agents share and read a transcript span end to end,
// the grant survives a server restart, and the share creates no mail,
// activation, or transcript event (FS-15.R4–R5, R10).
func TestShareAndReadContextOverMCP(t *testing.T) {
	f := newContextFixture(t)
	liveAgent(t, f.store, "a_impl", "Atlas", "implementer", "my-app")
	liveAgent(t, f.store, "a_rev", "Nova", "reviewer", "my-app")
	f.transcriptEvents(t, "a_impl",
		contextEvent(t, runtime.EvUserPrompt, runtime.UserPromptData{Text: "assess the migration"}),
		contextEvent(t, runtime.EvAssistantText, runtime.AssistantTextData{Delta: "the migration is forward-only"}),
	)
	f.srv.RegisterSession("tok-impl", "a_impl", "gen-impl")
	f.srv.RegisterSession("tok-rev", "a_rev", "gen-rev")

	impl := connect(t, f.srv, "tok-impl")
	res, isErr := call(t, impl, "share_context", map[string]any{
		"to": "reviewer@my-app", "source": "current_turn", "label": "migration analysis",
	})
	if isErr || res["ok"] != true {
		t.Fatalf("share_context: %v", res)
	}
	refID, _ := res["context_ref_id"].(string)
	grantID, _ := res["grant_id"].(string)
	if refID == "" || grantID == "" {
		t.Fatalf("share_context returned no ids: %v", res)
	}
	if res["to"] != "a_rev" {
		t.Fatalf("share resolved to %v", res["to"])
	}
	// The share result carries source identity but never source content.
	if body, _ := json.Marshal(res); strings.Contains(string(body), "forward-only") {
		t.Fatalf("share result leaked source content: %s", body)
	}

	// No mail, no activation, and no new transcript record (FS-15.R10).
	var messages, activations int
	if err := f.store.DB().QueryRow(
		`SELECT (SELECT COUNT(*) FROM messages), (SELECT COUNT(*) FROM activations)`).Scan(&messages, &activations); err != nil {
		t.Fatalf("count: %v", err)
	}
	if messages != 0 || activations != 0 {
		t.Fatalf("context share produced %d messages and %d activations", messages, activations)
	}
	after, err := transcript.ReadFile(f.home, "a_impl", transcript.ReadOptions{IncludeMeta: true})
	if err != nil {
		t.Fatalf("read transcript: %v", err)
	}
	if len(after) != 3 {
		t.Fatalf("context share wrote transcript events (%d records)", len(after))
	}

	rev := connect(t, f.srv, "tok-rev")
	list, isErr := call(t, rev, "list_context_links", nil)
	if isErr || list["ok"] != true {
		t.Fatalf("list_context_links: %v", list)
	}
	links := list["links"].([]any)
	if len(links) != 1 {
		t.Fatalf("recipient list = %v", links)
	}
	entry := links[0].(map[string]any)
	if entry["label"] != "migration analysis" || entry["context_ref_id"] != refID {
		t.Fatalf("list entry = %v", entry)
	}
	// The list carries metadata only.
	if body, _ := json.Marshal(list); strings.Contains(string(body), "forward-only") {
		t.Fatalf("list leaked source content: %s", body)
	}

	read, isErr := call(t, rev, "read_context_link", map[string]any{"context_ref_id": refID})
	if isErr || read["ok"] != true {
		t.Fatalf("read_context_link: %v", read)
	}
	text, _ := read["text"].(string)
	if !strings.Contains(text, "forward-only") || !strings.Contains(text, "assess the migration") {
		t.Fatalf("read returned %q", text)
	}
	if read["complete"] != true {
		t.Fatalf("small source reported incomplete: %v", read)
	}

	// A2 — the grant is durable: a fresh server over the same database still
	// authorizes the recipient.
	restarted := New(f.store, nil)
	restarted.SetContextService(contextref.New(f.store, f.home))
	restarted.RegisterSession("tok-rev-2", "a_rev", "gen-rev-2")
	revAgain := connect(t, restarted, "tok-rev-2")
	read, isErr = call(t, revAgain, "read_context_link", map[string]any{"context_ref_id": refID})
	if isErr || read["ok"] != true {
		t.Fatalf("read after restart: %v", read)
	}
}

// A2 — the caller cannot name another agent's source: identity comes from the
// session token, and share_context accepts only the three server-derived
// selectors (TS-05.R16).
func TestShareContextRejectsSpoofedSources(t *testing.T) {
	f := newContextFixture(t)
	liveAgent(t, f.store, "a_impl", "Atlas", "implementer", "my-app")
	liveAgent(t, f.store, "a_rev", "Nova", "reviewer", "my-app")
	liveAgent(t, f.store, "a_snoop", "Snoop", "snoop", "my-app")
	f.transcriptEvents(t, "a_impl",
		contextEvent(t, runtime.EvAssistantText, runtime.AssistantTextData{Delta: "private reasoning"}))
	f.srv.RegisterSession("tok-snoop", "a_snoop", "gen-snoop")

	snoop := connect(t, f.srv, "tok-snoop")
	for _, source := range []string{"a_impl", "a_impl:1-1", "transcript_span", "", "../../etc/passwd"} {
		res, isErr := call(t, snoop, "share_context", map[string]any{"to": "reviewer@my-app", "source": source})
		if !isErr {
			t.Fatalf("source %q was accepted: %v", source, res)
		}
		if res["error"] != contextref.CodeValidation {
			t.Fatalf("source %q = %v, want %s", source, res["error"], contextref.CodeValidation)
		}
	}
	// Even the legitimate selector finds nothing: the snoop has no transcript.
	res, isErr := call(t, snoop, "share_context", map[string]any{"to": "reviewer@my-app", "source": "current_turn"})
	if !isErr || res["error"] != contextref.CodeSourceUnavailable {
		t.Fatalf("snoop current_turn = %v", res)
	}
	var refs int
	if err := f.store.DB().QueryRow(`SELECT COUNT(*) FROM context_references`).Scan(&refs); err != nil {
		t.Fatalf("count: %v", err)
	}
	if refs != 0 {
		t.Fatalf("spoofed shares created %d references", refs)
	}
}

// A4/A5 — every context tool is scoped to the token-derived caller: another
// agent cannot read, hide, or revoke someone else's link, and missing and
// unauthorized ids are indistinguishable (TS-05.R16).
func TestContextToolsAreCallerScoped(t *testing.T) {
	f := newContextFixture(t)
	liveAgent(t, f.store, "a_impl", "Atlas", "implementer", "my-app")
	liveAgent(t, f.store, "a_rev", "Nova", "reviewer", "my-app")
	liveAgent(t, f.store, "a_snoop", "Snoop", "snoop", "my-app")
	f.transcriptEvents(t, "a_impl",
		contextEvent(t, runtime.EvAssistantText, runtime.AssistantTextData{Delta: "sensitive material"}))
	f.srv.RegisterSession("tok-impl", "a_impl", "g1")
	f.srv.RegisterSession("tok-rev", "a_rev", "g2")
	f.srv.RegisterSession("tok-snoop", "a_snoop", "g3")

	impl := connect(t, f.srv, "tok-impl")
	shared, isErr := call(t, impl, "share_context", map[string]any{"to": "a_rev", "source": "current_turn"})
	if isErr {
		t.Fatalf("share: %v", shared)
	}
	refID := shared["context_ref_id"].(string)
	grantID := shared["grant_id"].(string)

	snoop := connect(t, f.srv, "tok-snoop")
	read, isErr := call(t, snoop, "read_context_link", map[string]any{"context_ref_id": refID})
	if !isErr || read["error"] != contextref.CodeNotFound {
		t.Fatalf("cross-agent read = %v", read)
	}
	missing, _ := call(t, snoop, "read_context_link", map[string]any{"context_ref_id": "cr_nope"})
	if missing["error"] != read["error"] || missing["message"] != read["message"] {
		t.Fatalf("unauthorized and missing distinguishable: %v vs %v", read, missing)
	}
	if hide, isErr := call(t, snoop, "set_context_link_visibility",
		map[string]any{"grant_id": grantID, "hidden": true}); !isErr || hide["error"] != contextref.CodeNotFound {
		t.Fatalf("cross-agent hide = %v", hide)
	}
	if rev, isErr := call(t, snoop, "revoke_context_grant",
		map[string]any{"grant_id": grantID}); !isErr || rev["error"] != contextref.CodeNotFound {
		t.Fatalf("cross-agent revoke = %v", rev)
	}
	// The recipient cannot revoke a grant it did not give.
	recipient := connect(t, f.srv, "tok-rev")
	if rev, isErr := call(t, recipient, "revoke_context_grant",
		map[string]any{"grant_id": grantID}); !isErr || rev["error"] != contextref.CodeNotFound {
		t.Fatalf("recipient revoke = %v", rev)
	}
	// And the grantor cannot change the recipient's personal visibility.
	if hide, isErr := call(t, impl, "set_context_link_visibility",
		map[string]any{"grant_id": grantID, "hidden": true}); !isErr || hide["error"] != contextref.CodeNotFound {
		t.Fatalf("grantor hide = %v", hide)
	}

	// The recipient's own hide/unhide works and revokes nothing.
	if hide, isErr := call(t, recipient, "set_context_link_visibility",
		map[string]any{"grant_id": grantID, "hidden": true}); isErr || hide["ok"] != true {
		t.Fatalf("recipient hide: %v", hide)
	}
	if list, _ := call(t, recipient, "list_context_links", nil); len(list["links"].([]any)) != 0 {
		t.Fatalf("hidden link still listed: %v", list)
	}
	if list, _ := call(t, recipient, "list_context_links", map[string]any{"include_hidden": true}); len(list["links"].([]any)) != 1 {
		t.Fatalf("include_hidden lost the link: %v", list)
	}
	if read, isErr := call(t, recipient, "read_context_link", map[string]any{"context_ref_id": refID}); isErr {
		t.Fatalf("hiding revoked authorization: %v", read)
	}

	// Grantor revocation ends access immediately.
	if rev, isErr := call(t, impl, "revoke_context_grant", map[string]any{"grant_id": grantID}); isErr || rev["ok"] != true {
		t.Fatalf("grantor revoke: %v", rev)
	}
	if read, isErr := call(t, recipient, "read_context_link",
		map[string]any{"context_ref_id": refID}); !isErr || read["error"] != contextref.CodeNotFound {
		t.Fatalf("read after revocation = %v", read)
	}
}

// A5 — context operations do not consume the mail budget (FS-15.R11).
func TestContextToolsDoNotConsumeMailBudget(t *testing.T) {
	f := newContextFixture(t)
	liveAgent(t, f.store, "a_impl", "Atlas", "implementer", "my-app")
	liveAgent(t, f.store, "a_rev", "Nova", "reviewer", "my-app")
	f.transcriptEvents(t, "a_impl",
		contextEvent(t, runtime.EvAssistantText, runtime.AssistantTextData{Delta: "work"}))
	f.srv.RegisterSession("tok-impl", "a_impl", "g1")
	f.srv.RegisterSession("tok-rev", "a_rev", "g2")

	impl := connect(t, f.srv, "tok-impl")
	rev := connect(t, f.srv, "tok-rev")
	shared, _ := call(t, impl, "share_context", map[string]any{"to": "a_rev", "source": "current_turn"})
	refID := shared["context_ref_id"].(string)
	for i := 0; i < MessageBudgetPerTurn+5; i++ {
		if _, isErr := call(t, rev, "read_context_link", map[string]any{"context_ref_id": refID}); isErr {
			t.Fatalf("read %d failed", i)
		}
		if _, isErr := call(t, rev, "list_context_links", nil); isErr {
			t.Fatalf("list %d failed", i)
		}
	}
	var budgetRows int
	if err := f.store.DB().QueryRow(`SELECT COUNT(*) FROM turn_budget`).Scan(&budgetRows); err != nil {
		t.Fatalf("count budget: %v", err)
	}
	if budgetRows != 0 {
		t.Fatalf("context operations wrote %d turn_budget rows", budgetRows)
	}
	// Ordinary mail still works afterwards, proving the budget was untouched.
	if res, isErr := call(t, rev, "send_message",
		map[string]any{"to": "implementer@my-app", "body": "read it"}); isErr || res["ok"] != true {
		t.Fatalf("mail after context reads: %v", res)
	}
}

// An unregistered or revoked session reaches no context surface at all.
func TestContextToolsRequireARegisteredSession(t *testing.T) {
	f := newContextFixture(t)
	liveAgent(t, f.store, "a_impl", "Atlas", "implementer", "my-app")
	cs := connect(t, f.srv, "tok-unknown")
	calls := map[string]map[string]any{
		"share_context":               {"to": "a_impl", "source": "current_turn"},
		"list_context_links":          nil,
		"read_context_link":           {"context_ref_id": "cr_1"},
		"set_context_link_visibility": {"grant_id": "cg_1", "hidden": true},
		"revoke_context_grant":        {"grant_id": "cg_1"},
	}
	for name, args := range calls {
		res, isErr := call(t, cs, name, args)
		if !isErr || res["error"] != "session_unknown" {
			t.Fatalf("%s with an unknown token = %v", name, res)
		}
	}
}

// All five tools are advertised on the one scoped MCP server, and no MCP
// resource surface is registered (TS-04.R28).
func TestContextToolsAreRegisteredWithoutResources(t *testing.T) {
	f := newContextFixture(t)
	liveAgent(t, f.store, "a_impl", "Atlas", "implementer", "my-app")
	f.srv.RegisterSession("tok-impl", "a_impl", "g1")
	cs := connect(t, f.srv, "tok-impl")

	tools, err := cs.ListTools(t.Context(), &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	got := map[string]bool{}
	for _, tool := range tools.Tools {
		got[tool.Name] = true
	}
	for _, want := range []string{"share_context", "list_context_links", "read_context_link",
		"set_context_link_visibility", "revoke_context_grant"} {
		if !got[want] {
			t.Errorf("tool %q is not registered", want)
		}
	}
	if res, err := cs.ListResources(t.Context(), &mcp.ListResourcesParams{}); err == nil && len(res.Resources) > 0 {
		t.Errorf("context shipped %d MCP resources; the initial surface is tools only", len(res.Resources))
	}
}

// The tools stay unusable rather than falling back to a looser authority when
// no context service is wired.
func TestContextToolsWithoutAServiceAreUnavailable(t *testing.T) {
	st := newStore(t)
	liveAgent(t, st, "a_impl", "Atlas", "implementer", "my-app")
	srv := New(st, nil)
	srv.RegisterSession("tok-impl", "a_impl", "g1")
	cs := connect(t, srv, "tok-impl")
	res, isErr := call(t, cs, "list_context_links", nil)
	if !isErr || res["error"] != "context_unavailable" {
		t.Fatalf("list without a service = %v", res)
	}
}
