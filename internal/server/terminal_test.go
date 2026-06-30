package server

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/agentdeck/agentdeck/internal/runtime/terminal"
)

// GET /api/capabilities reports the terminal driver matrix (§8.5): xterm is
// always available and is the default driver.
func TestCapabilitiesEndpoint(t *testing.T) {
	h := testServer(t, true).routes()
	rec := doGET(t, h, "/api/capabilities")
	if rec.Code != http.StatusOK {
		t.Fatalf("capabilities status = %d, want 200", rec.Code)
	}
	var caps terminal.Capabilities
	if err := json.Unmarshal(rec.Body.Bytes(), &caps); err != nil {
		t.Fatalf("capabilities body: %v", err)
	}
	if !caps.Terminal.Available || !caps.Terminal.Drivers.Xterm {
		t.Fatalf("xterm must be available: %+v", caps.Terminal)
	}
	if caps.Terminal.DefaultDriver != "xterm" {
		t.Fatalf("default_driver = %q, want xterm", caps.Terminal.DefaultDriver)
	}
}

// The terminal WS route is mounted and rejects an unknown agent with 404 JSON
// (rather than attempting a WebSocket upgrade).
func TestTerminalWSUnknownAgent(t *testing.T) {
	h := testServer(t, true).routes()
	rec := doGET(t, h, "/api/sessions/nope/terminal/ws")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("terminal ws unknown agent = %d, want 404", rec.Code)
	}
}
