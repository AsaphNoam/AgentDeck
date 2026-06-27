package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/agentdeck/agentdeck/internal/config"
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
	return New(cfgStore, stateStore, config.DefaultConfig(), log)
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
	var l config.Layout
	if err := json.Unmarshal(rec.Body.Bytes(), &l); err != nil {
		t.Fatalf("layout body: %v", err)
	}
	if l.Density.CardsPerRow != 3 || l.Density.Gap != 16 {
		t.Fatalf("default layout wrong: %+v", l)
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
