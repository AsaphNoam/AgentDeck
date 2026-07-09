package server

import (
	"net/http"

	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/runtime/terminal"
	"github.com/coder/websocket"
)

// validateTerminalDriver returns a 422 terminal_unavailable APIError when an
// explicitly requested terminal driver is unavailable on this host (§3.5). The
// empty driver (the always-available xterm default) passes. Used by both the
// launch and switch-runtime paths so the two agree on driver honesty.
func validateTerminalDriver(driver string) *runtime.APIError {
	caps := terminal.Probe()
	if caps.DriverAvailable(driver) {
		return nil
	}
	return apiError(runtime.CodeTerminalUnavailable, terminalDriverReason(caps, driver))
}

// terminalDriverReason returns the UI-facing reason an unavailable driver was
// rejected, preferring the capability probe's own reason for iterm2 (§3.5).
func terminalDriverReason(caps terminal.Capabilities, driver string) string {
	switch driver {
	case "iterm2":
		if r := caps.Terminal.Drivers.ITerm2.Reason; r != "" {
			return r
		}
		return "iTerm2 driver is not available on this host"
	case "tmux":
		return "tmux is not installed on this host"
	default:
		return "unknown terminal driver: " + driver
	}
}

// handleCapabilities implements GET /api/capabilities (techspec §8.5): which
// terminal drivers this host can use. The xterm default is always available, so
// the terminal interface is never globally disabled; tmux/iTerm2 are reported
// per-host so the UI can disable unavailable optional drivers.
func (s *Server) handleCapabilities(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, terminal.Probe())
}

// handleTerminalWS implements GET /api/sessions/{id}/terminal/ws (techspec
// §3.4): the PTY↔WebSocket bridge for a terminal agent's xterm.js panel.
// Keystrokes flow browser→PTY master, PTY output flows back as frames, and
// {cols,rows} control frames resize the PTY.
func (s *Server) handleTerminalWS(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if s.terminal == nil {
		writeAPIError(w, apiError(runtime.CodeNotImplemented, "terminal runtime unavailable"))
		return
	}
	if _, err := s.stateStore.ReadAgent(id); err != nil {
		writeAPIError(w, apiError(runtime.CodeNotFound, "no such agent: "+id))
		return
	}
	// Resolve the agent's live PTY before upgrading, so a non-terminal or
	// stopped agent gets a clean JSON error instead of a half-open socket.
	conn, err := s.terminal.Bridge(id)
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeNotFound, "no terminal bridge for agent: "+id))
		return
	}

	// Loopback-only server (§ bind), so the browser origin is trusted; skip the
	// origin check that would otherwise reject the same-machine UI.
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		s.log.Debug("terminal ws accept failed", "agent", id, "err", err)
		_ = conn.Close() // unsubscribe from the hub so the accept-error path leaks no subscriber
		return           // Accept already wrote the failure response
	}
	// Bridge owns the conn for its lifetime and closes it on return.
	_ = terminal.ServeWS(r.Context(), ws, conn)
}
