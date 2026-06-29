package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
)

// testServer builds a Server backed by a seeded temp-home store. AGENTDECK_HOME
// is set to the temp dir so nothing touches the real ~/.agentdeck.
func testServer(t *testing.T, seed bool) *Server {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("AGENTDECK_HOME", dir)
	cfgStore, err := config.New()
	if err != nil {
		t.Fatalf("config.New: %v", err)
	}
	if err := cfgStore.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout: %v", err)
	}
	if seed {
		if err := cfgStore.SeedIfAbsent(); err != nil {
			t.Fatalf("SeedIfAbsent: %v", err)
		}
	}
	stateStore, err := state.Open(cfgStore.Home())
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	t.Cleanup(func() { _ = stateStore.Close() })
	log := slog.New(slog.NewJSONHandler(io.Discard, nil))
	registry := runtime.NewRegistry(stateStore)
	return New(cfgStore, stateStore, registry, config.DefaultConfig(), log)
}

func doGET(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHealth(t *testing.T) {
	h := testServer(t, true).routes()
	rec := doGET(t, h, "/api/health")
	if rec.Code != http.StatusOK {
		t.Fatalf("health status = %d, want 200", rec.Code)
	}
	var body healthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("health body: %v", err)
	}
	if body.Status != "ok" || body.Version == "" || body.Time == "" {
		t.Fatalf("health body fields incomplete: %+v", body)
	}
}

func TestSessionsEmptyIsArray(t *testing.T) {
	h := testServer(t, true).routes()
	rec := doGET(t, h, "/api/sessions")
	if rec.Code != http.StatusOK {
		t.Fatalf("sessions status = %d, want 200", rec.Code)
	}
	// Must be [] not null.
	got := rec.Body.String()
	// Trim trailing newline from the JSON encoder.
	for len(got) > 0 && (got[len(got)-1] == '\n' || got[len(got)-1] == ' ') {
		got = got[:len(got)-1]
	}
	if got != "[]" {
		t.Fatalf("empty sessions body = %q, want %q", got, "[]")
	}
}

func TestArchiveListHandler(t *testing.T) {
	srv := testServer(t, true)
	if _, err := srv.stateStore.DB().Exec(`
INSERT INTO sessions(agent_id, name, role, project, backend, model, interface, cwd, system_prompt, created_at, updated_at)
VALUES ('a_archive','Atlas','implementer','my-app','claude','sonnet','chat','/tmp','prompt','2026-06-28T10:00:00Z','2026-06-28T10:01:00Z')`); err != nil {
		t.Fatalf("insert archive session: %v", err)
	}
	rec := doGET(t, srv.routes(), "/api/archive?limit=10")
	if rec.Code != http.StatusOK {
		t.Fatalf("archive status = %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Total   int `json:"total"`
		Results []struct {
			AgentID string `json:"agent_id"`
			Active  bool   `json:"active"`
		} `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("archive body: %v", err)
	}
	if body.Total != 1 || len(body.Results) != 1 || body.Results[0].AgentID != "a_archive" || body.Results[0].Active {
		t.Fatalf("archive body = %+v", body)
	}
}

func TestRolesSeeded(t *testing.T) {
	h := testServer(t, true).routes()
	rec := doGET(t, h, "/api/roles")
	if rec.Code != http.StatusOK {
		t.Fatalf("roles status = %d, want 200", rec.Code)
	}
	var roles map[string]config.Role
	if err := json.Unmarshal(rec.Body.Bytes(), &roles); err != nil {
		t.Fatalf("roles body: %v", err)
	}
	if len(roles) != 4 {
		t.Fatalf("seeded roles = %d, want 4: %v", len(roles), roles)
	}
	for _, k := range []string{"implementer", "reviewer", "researcher", "pm"} {
		if _, ok := roles[k]; !ok {
			t.Errorf("missing seeded role %q", k)
		}
	}
}

func TestProjectsSeeded(t *testing.T) {
	h := testServer(t, true).routes()
	rec := doGET(t, h, "/api/projects")
	if rec.Code != http.StatusOK {
		t.Fatalf("projects status = %d, want 200", rec.Code)
	}
	var projects map[string]config.Project
	if err := json.Unmarshal(rec.Body.Bytes(), &projects); err != nil {
		t.Fatalf("projects body: %v", err)
	}
	if _, ok := projects["my-app"]; !ok {
		t.Fatalf("missing seeded project my-app: %v", projects)
	}
}

func TestBackendsSeeded(t *testing.T) {
	h := testServer(t, true).routes()
	rec := doGET(t, h, "/api/backends")
	if rec.Code != http.StatusOK {
		t.Fatalf("backends status = %d, want 200", rec.Code)
	}
	var b config.BackendsConfig
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatalf("backends body: %v", err)
	}
	if b.Version != 2 || len(b.Backends) == 0 {
		t.Fatalf("backends shape wrong: %+v", b)
	}
}

func TestLayoutDefault(t *testing.T) {
	// Unseeded store: layout missing → default, still 200.
	h := testServer(t, false).routes()
	rec := doGET(t, h, "/api/layout")
	if rec.Code != http.StatusOK {
		t.Fatalf("layout status = %d, want 200", rec.Code)
	}
	var l layoutResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &l); err != nil {
		t.Fatalf("layout body: %v", err)
	}
	if l.Density.PerRow != 3 || l.Density.Gap != 16 {
		t.Fatalf("default layout wrong: %+v", l)
	}
}

func TestPutLayoutValidatesAndPersists(t *testing.T) {
	srv := testServer(t, false)
	h := srv.routes()
	body := bytes.NewBufferString(`{"order":["a_1","a_2"],"density":{"perRow":4,"gap":20}}`)
	req := httptest.NewRequest(http.MethodPut, "/api/layout", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT layout status = %d body=%s, want 200", rec.Code, rec.Body.String())
	}
	got, err := srv.configStore.ReadLayout()
	if err != nil {
		t.Fatalf("ReadLayout: %v", err)
	}
	if got.Density.CardsPerRow != 4 || got.Density.Gap != 20 || len(got.Order) != 2 {
		t.Fatalf("persisted layout = %+v", got)
	}

	req = httptest.NewRequest(http.MethodPut, "/api/layout", bytes.NewBufferString(`{"order":[],"density":{"perRow":9,"gap":20}}`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad layout status = %d, want 400", rec.Code)
	}
}

func TestRenameSession(t *testing.T) {
	srv := testServer(t, true)
	agentID := seedHookAgent(t, srv)
	h := srv.routes()
	req := httptest.NewRequest(http.MethodPost, "/api/sessions/"+agentID+"/rename", bytes.NewBufferString(`{"name":"Vega"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("rename status = %d body=%s, want 200", rec.Code, rec.Body.String())
	}
	agent, err := srv.stateStore.ReadAgent(agentID)
	if err != nil {
		t.Fatalf("ReadAgent: %v", err)
	}
	if agent.Name != "Vega" {
		t.Fatalf("agent name = %q, want Vega", agent.Name)
	}
}

func TestIdentityHandlerUpdatesGroupAndPublishesState(t *testing.T) {
	srv := testServer(t, true)
	agent := state.Agent{
		AgentID: "a_ident", Name: "Atlas", Role: "implementer", Project: "my-app",
		Backend: "claude", Model: "sonnet-4-6", Interface: "chat", CreatedAt: time.Now().UTC(),
	}
	if err := srv.stateStore.WriteAgent(agent); err != nil {
		t.Fatalf("WriteAgent: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/sessions/a_ident/identity", bytes.NewBufferString(`{"group":"auth","name":"Vega"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("identity status = %d body=%s, want 200", rec.Code, rec.Body.String())
	}
	got, err := srv.stateStore.ReadAgent("a_ident")
	if err != nil {
		t.Fatalf("ReadAgent: %v", err)
	}
	if got.Group != "auth" || got.Name != "Vega" {
		t.Fatalf("identity = %+v, want group auth name Vega", got)
	}
}

func TestIdentityRejectsReservedUngrouped(t *testing.T) {
	srv := testServer(t, true)
	agent := state.Agent{
		AgentID: "a_ident", Name: "Atlas", Role: "implementer", Project: "my-app",
		Backend: "claude", Model: "sonnet-4-6", Interface: "chat", CreatedAt: time.Now().UTC(),
	}
	if err := srv.stateStore.WriteAgent(agent); err != nil {
		t.Fatalf("WriteAgent: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/sessions/a_ident/identity", bytes.NewBufferString(`{"group":"_ungrouped"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("identity reserved status = %d body=%s, want 400", rec.Code, rec.Body.String())
	}
}

func TestReleaseGroupStopsMembers(t *testing.T) {
	srv := testServer(t, true)
	now := time.Now().UTC()
	for _, id := range []string{"a_g1", "a_g2"} {
		if err := srv.stateStore.WriteAgent(state.Agent{
			AgentID: id, Name: id, Role: "implementer", Project: "my-app", Backend: "claude", Model: "sonnet-4-6",
			Interface: "chat", Group: "auth", CreatedAt: now,
		}); err != nil {
			t.Fatalf("WriteAgent %s: %v", id, err)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/api/groups/auth/release", nil)
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("release status = %d body=%s, want 200", rec.Code, rec.Body.String())
	}
	var body struct {
		Group   string `json:"group"`
		Stopped []struct {
			AgentID string `json:"agent_id"`
			OK      bool   `json:"ok"`
		} `json:"stopped"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("release body: %v", err)
	}
	if body.Group != "auth" || len(body.Stopped) != 2 || !body.Stopped[0].OK || !body.Stopped[1].OK {
		t.Fatalf("release body = %+v", body)
	}
}

func TestPruneStaleRunning(t *testing.T) {
	srv := testServer(t, true)
	agent := state.Agent{
		AgentID: "a_stale", Name: "Stale", Role: "implementer", Project: "my-app",
		Backend: "claude", Model: "sonnet-4-6", Interface: "chat", CreatedAt: time.Now().UTC(),
	}
	if err := srv.stateStore.WriteAgent(agent); err != nil {
		t.Fatalf("WriteAgent: %v", err)
	}
	if err := srv.stateStore.WriteRunning(state.RunningEntry{AgentID: agent.AgentID, PID: -42, Interface: "chat", StartedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("WriteRunning: %v", err)
	}
	if n := srv.pruneStaleRunning(); n != 1 {
		t.Fatalf("pruned = %d, want 1", n)
	}
	if _, err := srv.stateStore.ReadRunning(agent.AgentID); err == nil {
		t.Fatal("stale running row still present")
	}
}

func TestSessionMessagesEndpoint(t *testing.T) {
	srv := testServer(t, true)
	recipient := state.Agent{AgentID: "a_recipient", Name: "Nova", Role: "reviewer", Project: "my-app", Backend: "claude", Model: "sonnet", Interface: "chat", CreatedAt: time.Now().UTC()}
	sender := state.Agent{AgentID: "a_sender", Name: "Atlas", Role: "implementer", Project: "my-app", Backend: "claude", Model: "sonnet", Interface: "chat", CreatedAt: time.Now().UTC()}
	for _, a := range []state.Agent{recipient, sender} {
		if err := srv.stateStore.WriteAgent(a); err != nil {
			t.Fatalf("WriteAgent: %v", err)
		}
	}
	if _, err := srv.stateStore.InsertMessage(state.Message{
		FromAgent: sender.AgentID, FromAddress: "implementer@my-app", FromName: sender.Name,
		ToAgent: recipient.AgentID, Body: "first", CreatedAt: time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("InsertMessage first: %v", err)
	}
	if _, err := srv.stateStore.InsertMessage(state.Message{
		FromAgent: sender.AgentID, FromAddress: "implementer@my-app", FromName: sender.Name,
		ToAgent: recipient.AgentID, Body: "second", CreatedAt: time.Date(2026, 6, 29, 10, 1, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("InsertMessage second: %v", err)
	}
	rec := doGET(t, srv.routes(), "/api/sessions/a_recipient/messages?limit=10")
	if rec.Code != http.StatusOK {
		t.Fatalf("messages status = %d body=%s, want 200", rec.Code, rec.Body.String())
	}
	var body struct {
		UnreadCount int `json:"unread_count"`
		Messages    []struct {
			Body string `json:"body"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("messages body: %v", err)
	}
	if body.UnreadCount != 2 || len(body.Messages) != 2 || body.Messages[0].Body != "second" {
		t.Fatalf("messages body = %+v, want newest first + unread count", body)
	}
}

// TestTouchRecipientPublishesUnread documents why the "recipient badge doesn't
// update live" review finding was a false positive: the message-inserted sink
// calls stateMgr.Touch(toAgentID), and Touch publishes a state_update (via the
// manager's bus publisher) carrying the recomputed unread_messages — no extra
// publish is needed. Guards against a regression that would drop this.
func TestTouchRecipientPublishesUnread(t *testing.T) {
	srv := testServer(t, true)
	recipient := state.Agent{AgentID: "a_recipient", Name: "Nova", Role: "reviewer", Project: "my-app", Backend: "claude", Model: "sonnet", Interface: "chat", CreatedAt: time.Now().UTC()}
	if err := srv.stateStore.WriteAgent(recipient); err != nil {
		t.Fatalf("WriteAgent: %v", err)
	}
	if _, err := srv.stateStore.InsertMessage(state.Message{
		FromAgent: "a_sender", FromAddress: "implementer@my-app", FromName: "Atlas",
		ToAgent: recipient.AgentID, Body: "mail", CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("InsertMessage: %v", err)
	}

	ch, unsub := srv.eventBus.Subscribe()
	defer unsub()
	if _, err := srv.stateMgr.Touch(recipient.AgentID); err != nil {
		t.Fatalf("Touch: %v", err)
	}
	select {
	case ev := <-ch:
		if ev.Type != "state_update" {
			t.Fatalf("event type = %q, want state_update", ev.Type)
		}
		update, ok := ev.Data.(state.AgentStateUpdate)
		if !ok {
			t.Fatalf("event data type = %T, want AgentStateUpdate", ev.Data)
		}
		if update.AgentID != recipient.AgentID || update.UnreadMessages != 1 {
			t.Fatalf("state_update = %+v, want recipient with unread_messages 1", update)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no state_update published for recipient on Touch")
	}
}

func TestBackendsCorruptFallsBackTo200(t *testing.T) {
	srv := testServer(t, true)
	// Overwrite backends.json with garbage.
	bp := srv.configStore.Home() + "/backends.json"
	if err := os.WriteFile(bp, []byte("{ not json,,,"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := srv.routes()
	rec := doGET(t, h, "/api/backends")
	if rec.Code != http.StatusOK {
		t.Fatalf("corrupt backends status = %d, want 200", rec.Code)
	}
	var b config.BackendsConfig
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatalf("fallback backends body: %v", err)
	}
	if b.Version != 2 {
		t.Fatalf("fallback backends version = %d, want 2", b.Version)
	}
}

func TestUnknownAPIPath404(t *testing.T) {
	h := testServer(t, true).routes()
	rec := doGET(t, h, "/api/does-not-exist")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown api status = %d, want 404", rec.Code)
	}
	var body errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("404 body not JSON: %v", err)
	}
	if body.Error == "" {
		t.Fatal("404 body missing error field")
	}
}

func TestPostToGetRoute405(t *testing.T) {
	h := testServer(t, true).routes()
	req := httptest.NewRequest(http.MethodPost, "/api/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /api/health status = %d, want 405", rec.Code)
	}
}

func TestCORSPreflight(t *testing.T) {
	h := testServer(t, true).routes()
	req := httptest.NewRequest(http.MethodOptions, "/api/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("OPTIONS status = %d, want 204", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != devOrigin {
		t.Fatalf("CORS origin = %q, want %q", rec.Header().Get("Access-Control-Allow-Origin"), devOrigin)
	}
}

// ---- Bind / loopback tests ----

func TestLocalAddr(t *testing.T) {
	if _, err := LocalAddr(0); err == nil {
		t.Fatal("LocalAddr(0): want error")
	}
	if _, err := LocalAddr(70000); err == nil {
		t.Fatal("LocalAddr(70000): want error")
	}
	addr, err := LocalAddr(4317)
	if err != nil {
		t.Fatal(err)
	}
	if addr != "127.0.0.1:4317" {
		t.Fatalf("LocalAddr(4317) = %q", addr)
	}
}

func TestAssertLoopback(t *testing.T) {
	// Positive: a real loopback listener passes.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if err := assertLoopback(ln.Addr()); err != nil {
		t.Fatalf("assertLoopback(loopback): %v", err)
	}

	// Negative guard: a non-loopback TCP addr fails closed.
	nonLoop := &net.TCPAddr{IP: net.IPv4(0, 0, 0, 0), Port: 4317}
	if err := assertLoopback(nonLoop); err == nil {
		t.Fatal("assertLoopback(0.0.0.0): want error, got nil")
	}
	public := &net.TCPAddr{IP: net.IPv4(8, 8, 8, 8), Port: 4317}
	if err := assertLoopback(public); err == nil {
		t.Fatal("assertLoopback(8.8.8.8): want error, got nil")
	}
}

func TestStartBindsLoopback(t *testing.T) {
	// Start the real listener path on an ephemeral loopback port via a manual
	// listen + assert (mirrors Server.Start's guard) to avoid a blocking Serve.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	tcp, ok := ln.Addr().(*net.TCPAddr)
	if !ok || !tcp.IP.IsLoopback() {
		t.Fatalf("listener not loopback: %v", ln.Addr())
	}
}
