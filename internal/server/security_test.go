package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIsLocalHost(t *testing.T) {
	cases := []struct {
		host string
		want bool
	}{
		{"127.0.0.1:4317", true},
		{"127.0.0.1", true},
		{"127.0.0.2:4317", true}, // whole 127/8 is loopback
		{"localhost:4317", true},
		{"localhost", true},
		{"LOCALHOST:4317", true},
		{"localhost.:4317", true}, // trailing FQDN dot
		{"[::1]:4317", true},
		{"[::1]", true},
		{"attacker.example:4317", false},
		{"attacker.example", false},
		{"10.0.0.5:4317", false},
		{"192.168.1.2", false},
		{"evil-localhost.example", false},
		{"sub.localhost", false}, // strict: only bare localhost
		{"", false},
	}
	for _, c := range cases {
		if got := isLocalHost(c.host); got != c.want {
			t.Errorf("isLocalHost(%q) = %v, want %v", c.host, got, c.want)
		}
	}
}

func TestIsLocalOrigin(t *testing.T) {
	cases := []struct {
		origin string
		want   bool
	}{
		{"http://localhost:5173", true},
		{"http://localhost:4317", true},
		{"http://127.0.0.1:4317", true},
		{"http://[::1]:4317", true},
		{"https://localhost", true},
		{"http://attacker.example", false},
		{"http://attacker.example:4317", false},
		{"null", false},     // opaque origin (sandboxed iframe, file://)
		{"garbage!", false}, // unparseable
		{"file:///tmp", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isLocalOrigin(c.origin); got != c.want {
			t.Errorf("isLocalOrigin(%q) = %v, want %v", c.origin, got, c.want)
		}
	}
}

// TestDNSRebindingHostRejected: a DNS-rebinding page makes the browser send
// requests to 127.0.0.1 with the attacker's Host header. Every route — API,
// the raw-mounted /mcp, the terminal WS path, and the static UI — must refuse
// a non-local Host with 403 before any handler runs.
func TestDNSRebindingHostRejected(t *testing.T) {
	h := testServer(t, true).routes()
	paths := []struct{ method, path string }{
		{http.MethodGet, "/api/health"},
		{http.MethodGet, "/api/sessions"},
		{http.MethodPost, "/mcp"},
		{http.MethodGet, "/api/sessions/a_x/terminal/ws"},
		{http.MethodGet, "/"},
	}
	for _, p := range paths {
		req := httptest.NewRequest(p.method, p.path, nil) // Host: example.com
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s with Host example.com: status = %d, want 403", p.method, p.path, rec.Code)
		}
	}
	// Same request with a local Host passes the guard.
	rec := doGET(t, h, "/api/health")
	if rec.Code != http.StatusOK {
		t.Fatalf("local-Host /api/health status = %d, want 200", rec.Code)
	}
}

// TestCrossOriginRequestRejected: CORS response headers never stop a "simple"
// cross-site POST or a cross-origin WebSocket handshake from reaching the
// handler — the guard must reject a hostile Origin server-side with 403 while
// letting the local Vite dev origin and Origin-less (non-browser) clients pass.
func TestCrossOriginRequestRejected(t *testing.T) {
	h := testServer(t, true).routes()

	// Simple (no-preflight) cross-site POST with a hostile Origin → 403.
	req := newLocalRequest(http.MethodPost, "/api/sessions", strings.NewReader(`{}`))
	req.Header.Set("Origin", "http://attacker.example")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("hostile-Origin POST status = %d, want 403", rec.Code)
	}

	// Cross-origin terminal-WS handshake from a hostile page → 403 before the
	// upgrade (regression: the WS route used to accept any origin).
	req = newLocalRequest(http.MethodGet, "/api/sessions/a_x/terminal/ws", nil)
	req.Header.Set("Origin", "http://attacker.example")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("hostile-Origin WS handshake status = %d, want 403", rec.Code)
	}

	// The allowed local dev origin still passes the guard.
	req = newLocalRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("Origin", devOrigin)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("dev-origin GET status = %d, want 200", rec.Code)
	}

	// Origin-less non-browser client (hook curl, MCP client) passes.
	rec = doGET(t, h, "/api/health")
	if rec.Code != http.StatusOK {
		t.Fatalf("origin-less GET status = %d, want 200", rec.Code)
	}
}
