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

	// Everything else is the embedded UI with SPA fallback. Registered for GET
	// only — a method-agnostic "/" would match every request, including non-GET
	// requests to /api/* routes, and silently serve the SPA (200) instead of
	// letting the mux return 405 for a wrong method. (A GET-only "/" does not
	// conflict with "GET /api/"; a HEAD "/" would, per the mux precedence rules.)
	mux.Handle("GET /", withMiddleware(s.log, s.staticHandler()))

	return mux
}
