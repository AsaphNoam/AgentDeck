package server

import "net/http"

// routes builds the Go 1.22 ServeMux with all Phase 0 GET routes. Method+path
// patterns mean a non-GET request to a registered route yields 405 automatically,
// and an unmatched /api/* path falls to the explicit 404-JSON catch-all.
//
// API handlers are wrapped with CORS + request-logging middleware. The static UI
// handler is mounted at "/" with SPA fallback.
func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	api := func(pattern string, h http.HandlerFunc) {
		mux.Handle(pattern, withMiddleware(s.log, h))
	}

	api("GET /api/health", s.handleHealth)
	api("GET /api/sessions", s.handleSessions)
	api("GET /api/roles", s.handleRoles)
	api("POST /api/roles", s.handlePostRole)
	api("PUT /api/roles/{role}", s.handlePutRole)
	api("DELETE /api/roles/{role}", s.handleDeleteRole)
	api("GET /api/projects", s.handleProjects)
	api("POST /api/projects", s.handlePostProject)
	api("PUT /api/projects/{project}", s.handlePutProject)
	api("DELETE /api/projects/{project}", s.handleDeleteProject)
	api("GET /api/backends", s.handleBackends)
	api("PUT /api/backends", s.handlePutBackends)
	api("GET /api/config", s.handleGetConfig)
	api("PUT /api/config", s.handlePutConfig)
	api("GET /api/layout", s.handleLayout)
	api("PUT /api/layout", s.handlePutLayout)
	api("POST /api/hook", s.handleHook)
	api("GET /api/events", s.handleEvents)
	api("GET /api/archive", s.handleArchive)

	// Phase 1 session lifecycle (launch, control). The {id} routes
	// are more specific than the GET /api/ catch-all and win via mux precedence.
	api("POST /api/sessions", s.handleLaunch)
	api("GET /api/sessions/{id}", s.handleSessionDetail)
	api("GET /api/sessions/{id}/transcript", s.handleTranscript)
	api("POST /api/sessions/{id}/prompt", s.handlePrompt)
	api("POST /api/sessions/{id}/cancel", s.handleCancel)
	api("POST /api/sessions/{id}/stop", s.handleStop)
	api("POST /api/sessions/{id}/rename", s.handleRename)
	api("POST /api/sessions/{id}/permission", s.handlePermission)
	api("POST /api/sessions/{id}/resume", s.handleResume)
	api("POST /api/sessions/{id}/switch-runtime", s.handleSwitchRuntime)
	api("GET /api/sessions/{id}/files", s.handleFiles)
	api("GET /api/sessions/{id}/commands", s.handleCommands)
	api("GET /api/sessions/{id}/messages", s.handleMessages)

	// Phase 6 terminal runtime: capability probe (§8.5) and the PTY↔WebSocket
	// bridge (§3.4). The WS route is registered raw (no API middleware): the
	// coder/websocket handshake manages its own headers, and the CORS wrapper's
	// OPTIONS short-circuit / header rewriting would interfere with the upgrade.
	api("GET /api/capabilities", s.handleCapabilities)
	mux.Handle("GET /api/sessions/{id}/terminal/ws", http.HandlerFunc(s.handleTerminalWS))

	// Catch-all for any other /api/* path → 404 JSON (more specific GET routes
	// above win via the 1.22 mux precedence rules). Registered GET-only on
	// purpose: a non-GET request to a known route then matches no pattern's
	// method, so the mux returns 405 automatically instead of being swallowed
	// here as a 404. A GET to an unknown /api path still lands here as 404 JSON.
	api("GET /api/", s.handleAPINotFound)

	// CORS preflight: OPTIONS must be matched at the mux level, otherwise a
	// preflight to a GET-only /api route matches no pattern and the mux returns
	// 405 before the CORS middleware (which answers OPTIONS with 204) is reached.
	// The wrapped handler body never runs — cors() short-circuits OPTIONS. A
	// method-specific "OPTIONS /" does not conflict with "GET /..." patterns.
	mux.Handle("OPTIONS /", withMiddleware(s.log, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})))

	// In-process MCP messaging server (Phase 5, techspec §2.2 (A)): the go-sdk
	// streamable HTTP transport. Registered for the explicit methods the
	// transport uses (POST messages, GET SSE stream, DELETE session teardown) —
	// a method-agnostic "/mcp" would conflict with "OPTIONS /". Mounted raw (no
	// API middleware): the transport speaks its own protocol, not the JSON API.
	if s.messaging != nil {
		h := s.messaging.Handler()
		mux.Handle("POST /mcp", h)
		mux.Handle("GET /mcp", h)
		mux.Handle("DELETE /mcp", h)
	}

	// Everything else is the embedded UI with SPA fallback. Registered for GET
	// only — a method-agnostic "/" would match every request, including non-GET
	// requests to /api/* routes, and silently serve the SPA (200) instead of
	// letting the mux return 405 for a wrong method. (A GET-only "/" does not
	// conflict with "GET /api/"; a HEAD "/" would, per the mux precedence rules.)
	mux.Handle("GET /", withMiddleware(s.log, s.staticHandler()))

	return mux
}
