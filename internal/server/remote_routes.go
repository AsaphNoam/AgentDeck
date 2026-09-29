package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/agentdeck/agentdeck/internal/remote"
	"github.com/agentdeck/agentdeck/internal/state"
)

// remoteAllowed is the tailnet listener's allowlist over the loopback route
// inventory (TS-13.R5). A value lists the only JSON body fields the phone may
// send (TS-13.R6); nil means the body is not filtered.
var remoteAllowed = map[string][]string{
	"GET /api/health":         nil,
	"GET /api/capabilities":   nil,
	"GET /api/events":         nil,
	"GET /api/config":         nil,
	"GET /api/projects":       nil,
	"GET /api/roles":          nil,
	"GET /api/pipelines":      nil,
	"GET /api/pipelines/{id}": nil,

	"GET /api/sessions":                  nil,
	"POST /api/sessions":                 {"role", "project"},
	"GET /api/sessions/{id}":             nil,
	"GET /api/sessions/{id}/transcript":  nil,
	"POST /api/sessions/{id}/prompt":     nil,
	"GET /api/sessions/{id}/prompt":      nil,
	"DELETE /api/sessions/{id}/prompt":   nil,
	"POST /api/sessions/{id}/steer":      nil,
	"POST /api/sessions/{id}/cancel":     nil,
	"POST /api/sessions/{id}/stop":       nil,
	"POST /api/sessions/{id}/resume":     nil,
	"POST /api/sessions/{id}/permission": nil,
	// Diff-line annotate-and-assign (FS-20.R32, TS-13.R16); nested anchors,
	// limits, and targets are validated by the shared FS-13 handler.
	"POST /api/sessions/{id}/annotations": {"annotations", "overall_instruction", "target"},

	"GET /api/tasks":              nil,
	"POST /api/tasks":             {"project", "display_name", "instruction", "target_kind", "role"},
	"GET /api/tasks/{id}":         nil,
	"POST /api/tasks/{id}/cancel": nil,
	"POST /api/tasks/{id}/result": nil,
	"POST /api/tasks/{id}/retry":  nil,
	"POST /api/tasks/{id}/rearm":  {"arms"},

	"GET /api/pipeline-runs":                      nil,
	"POST /api/pipeline-runs":                     {"request_id", "template_id", "display_name", "project", "goal", "inputs", "orchestrator", "dedicated_assignments", "acknowledge_shared_workspace"},
	"GET /api/pipeline-runs/{id}":                 nil,
	"POST /api/pipeline-runs/{id}/continue":       nil,
	"POST /api/pipeline-runs/{id}/retry":          nil,
	"POST /api/pipeline-runs/{id}/replace":        nil,
	"POST /api/pipeline-runs/{id}/repair-cleanup": nil,
	"POST /api/pipeline-runs/{id}/stop":           nil,
}

// remoteDenied names every loopback route deliberately unreachable from a
// phone. The inventory test fails when a route is in neither set (INV §10).
var remoteDenied = map[string]bool{
	"POST /api/roles": true, "PUT /api/roles/{role}": true, "DELETE /api/roles/{role}": true,
	"POST /api/projects": true, "PUT /api/projects/{project}": true, "DELETE /api/projects/{project}": true,
	// The backend catalog carries backend and model env, which may hold keys.
	"GET /api/backends": true, "POST /api/backends": true, "PUT /api/backends": true,
	"PUT /api/config": true, "GET /api/remote": true, "PUT /api/remote": true,
	"POST /api/remote/pairings": true, "POST /api/remote/pairings/{id}/allow": true,
	"POST /api/remote/pairings/{id}/decline": true, "GET /api/remote/devices": true,
	"PATCH /api/remote/devices/{id}": true, "DELETE /api/remote/devices/{id}": true,
	"GET /api/layout": true, "PUT /api/layout": true,
	"POST /api/directory-picker": true, "POST /api/hook": true,
	"GET /api/archive": true, "GET /api/archive/projects/{project}": true,
	"POST /api/projects/{project}/archive": true, "POST /api/projects/{project}/restore": true,
	"POST /api/projects/{project}/worktree-fork": true, "GET /api/projects/{project}/worktree": true,
	"GET /api/pipeline-proposals": true, "POST /api/pipeline-proposals/{id}/decline": true,
	"DELETE /api/pipeline-proposals/{id}": true,
	"POST /api/pipelines":                 true, "POST /api/pipelines/validate": true,
	"PUT /api/pipelines/{id}": true, "DELETE /api/pipelines/{id}": true,
	"DELETE /api/pipeline-runs/{id}": true, "DELETE /api/tasks/{id}": true,
	"POST /api/signals":                true,
	"POST /api/sessions/{id}/rename":   true,
	"POST /api/sessions/{id}/identity": true, "POST /api/sessions/{id}/background-task-stop": true,
	"POST /api/sessions/{id}/clone": true, "POST /api/sessions/{id}/archive": true,
	"POST /api/sessions/{id}/restore": true, "POST /api/sessions/{id}/switch-runtime": true,
	"POST /api/sessions/{id}/session-config": true,
	"GET /api/sessions/{id}/files":           true, "GET /api/sessions/{id}/commands": true,
	"GET /api/sessions/{id}/file-search": true, "GET /api/sessions/{id}/file": true,
	"GET /api/sessions/{id}/available-commands": true, "GET /api/sessions/{id}/messages": true,
	"POST /api/groups/{group}/release": true,
	"GET /api/config-sources":          true, "POST /api/config-sources/preview": true,
	"PUT /api/config-sources/{backend_id}": true, "POST /api/config-sources/{backend_id}/refresh": true,
	"DELETE /api/config-sources/{backend_id}": true,
	"GET /api/sessions/{id}/terminal/ws":      true,
	"POST /mcp":                               true, "GET /mcp": true, "DELETE /mcp": true,
	// Loopback catch-alls and the desktop bundle; the tailnet mux has its own.
	"GET /api/": true, "OPTIONS /": true, "GET /": true,
}

// Remote error codes (TS-13.R4–R8).
const (
	codeRemoteRouteNotAvailable = "remote_route_not_available"
	codeRemoteFieldNotAllowed   = "remote_field_not_allowed"
	codeRemoteUnpaired          = "remote_unpaired"
	codeRemoteDeviceMismatch    = "remote_device_mismatch"
	codeRemoteForbidden         = "remote_forbidden"
	codeRemoteBodyTooLarge      = "remote_body_too_large"
)

// remoteDeviceCookie is the phone's credential cookie (TS-13.R7).
const remoteDeviceCookie = "__Host-remote_device"

const (
	remoteCookieMaxAge   = 400 * 24 * time.Hour
	remoteCookieReissue  = 24 * time.Hour
	remoteLastSeenPeriod = time.Minute
	remoteBodyLimit      = 1 << 20
)

func writeRemoteError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": msg, "details": map[string]any{}}})
}

type remoteCtxKey struct{}

// remoteRequest is what the tailnet chain attaches to a request context.
type remoteRequest struct {
	peer   remote.Peer
	device *state.RemoteDevice
}

// remoteFrom reports the tailnet identity of a request, or nil on loopback.
func remoteFrom(ctx context.Context) *remoteRequest {
	rr, _ := ctx.Value(remoteCtxKey{}).(*remoteRequest)
	return rr
}

// remoteDevices tracks each paired device's open requests so revoke and unpair
// can end them, plus the bounded per-device write throttles (TS-13.R7, R9).
type remoteDevices struct {
	mu       sync.Mutex
	open     map[string]map[*int]context.CancelFunc
	lastSeen map[string]time.Time
	issued   map[string]time.Time
}

func newRemoteDevices() *remoteDevices {
	return &remoteDevices{
		open:     map[string]map[*int]context.CancelFunc{},
		lastSeen: map[string]time.Time{},
		issued:   map[string]time.Time{},
	}
}

// admit registers a request while holding the same boundary used by end, then
// revalidates the credential. A revoke that deleted the row before admission is
// observed by valid; a revoke after admission finds and cancels the request.
func (d *remoteDevices) admit(ctx context.Context, id string, valid func() error) (context.Context, func(), error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := valid(); err != nil {
		return ctx, func() {}, err
	}
	ctx, cancel := context.WithCancel(ctx)
	key := new(int)
	if d.open[id] == nil {
		d.open[id] = map[*int]context.CancelFunc{}
	}
	d.open[id][key] = cancel
	return ctx, func() {
		cancel()
		d.mu.Lock()
		delete(d.open[id], key)
		if len(d.open[id]) == 0 {
			delete(d.open, id)
		}
		d.mu.Unlock()
	}, nil
}

// end cancels every open request of a removed device and drops its throttles.
func (d *remoteDevices) end(id string) {
	d.mu.Lock()
	cancels := d.open[id]
	delete(d.open, id)
	delete(d.lastSeen, id)
	delete(d.issued, id)
	d.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}

// endAll cancels every open remote request (remote control turned off).
func (d *remoteDevices) endAll() {
	d.mu.Lock()
	ids := make([]string, 0, len(d.open))
	for id := range d.open {
		ids = append(ids, id)
	}
	d.mu.Unlock()
	for _, id := range ids {
		d.end(id)
	}
}

// due reports whether a throttled per-device action should run now.
func (d *remoteDevices) due(m map[string]time.Time, id string, now time.Time, period time.Duration) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if last, ok := m[id]; ok && now.Sub(last) < period {
		return false
	}
	m[id] = now
	return true
}

func hashRemoteToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func setRemoteDeviceCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name: remoteDeviceCookie, Value: token, Path: "/",
		MaxAge: int(remoteCookieMaxAge / time.Second), Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode,
	})
}

// remoteRoutes builds the tailnet listener's handler: guard → device
// authentication → allowlist → the loopback handlers (TS-13.R4). It never
// passes through localOnly or the loopback CORS middleware (TS-05.R23).
func (s *Server) remoteRoutes(domain string, whois func(context.Context, string) (remote.Peer, error)) http.Handler {
	authed := http.NewServeMux()
	for _, e := range s.routeTable() {
		fields, ok := remoteAllowed[e.pattern]
		if !ok {
			continue
		}
		var h http.Handler = e.handler
		if e.pattern == "POST /api/pipeline-runs" {
			h = remotePipelineRuntimeFilter(h)
		}
		if fields != nil {
			h = remoteFieldFilter(fields, h)
		}
		authed.Handle(e.pattern, h)
	}
	// Tailnet-only phone routes (TS-03.R46).
	authed.HandleFunc("GET /api/remote/home", s.handleRemoteHome)
	authed.HandleFunc("GET /api/remote/self", s.handleGetSelf)
	authed.HandleFunc("PUT /api/remote/self/push", s.handlePutSelfPush)
	authed.HandleFunc("DELETE /api/remote/self/push", s.handleDeleteSelfPush)
	authed.HandleFunc("PATCH /api/remote/self", s.handleRenameSelf)
	authed.HandleFunc("DELETE /api/remote/self", s.handleUnpairSelf)
	authed.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		writeRemoteError(w, http.StatusNotFound, codeRemoteRouteNotAvailable, "this action is only available on the Mac")
	})

	// Pairing is the only unauthenticated non-static surface (TS-13.R8).
	outer := http.NewServeMux()
	outer.HandleFunc("POST /api/remote/pair", s.handlePairClaim)
	outer.HandleFunc("GET /api/remote/pair/{pending_id}", s.handlePairWait)
	// Everything under /api/ needs a paired device. Other GETs are the phone
	// app itself, which an unpaired device may load to learn it must pair
	// (FS-20.R7); it carries no agent, task, project, or transcript data.
	api, static := s.remoteAuth(remoteBodyFilter(authed)), s.phoneStaticHandler()
	outer.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/"):
			api.ServeHTTP(w, r)
		case r.Method == http.MethodGet || r.Method == http.MethodHead:
			static.ServeHTTP(w, r)
		default:
			writeRemoteError(w, http.StatusNotFound, codeRemoteRouteNotAvailable, "this action is only available on the Mac")
		}
	})
	return s.remoteRequestLog(s.remoteGuard(domain, whois, outer))
}

// remoteRequestLog is requestLog with the pairing wait token redacted from the
// path (TS-13.R12).
func (s *Server) remoteRequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		path := r.URL.Path
		if strings.HasPrefix(path, "/api/remote/pair/") {
			path = "/api/remote/pair/{pending_id}"
		}
		s.log.Info("remote request", "method", r.Method, "path", path, "status", rec.status,
			"dur_ms", time.Since(start).Milliseconds())
	})
}

// remoteGuard pins Host and Origin to the node's certificate domain and
// requires the peer to be a tailnet device (TS-13.R4).
func (s *Server) remoteGuard(domain string, whois func(context.Context, string) (remote.Peer, error), next http.Handler) http.Handler {
	origin := "https://" + domain
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, port, err := net.SplitHostPort(host); err == nil && port == "443" {
			host = h
		}
		if !strings.EqualFold(host, domain) {
			writeRemoteError(w, http.StatusForbidden, codeRemoteForbidden, "forbidden host")
			return
		}
		o := r.Header.Get("Origin")
		if (o != "" && o != origin) || (o == "" && r.Method != http.MethodGet && r.Method != http.MethodHead) {
			writeRemoteError(w, http.StatusForbidden, codeRemoteForbidden, "forbidden origin")
			return
		}
		peer, err := whois(r.Context(), r.RemoteAddr)
		if err != nil || peer.StableID == "" {
			writeRemoteError(w, http.StatusForbidden, codeRemoteForbidden, "unknown tailnet device")
			return
		}
		ctx := context.WithValue(r.Context(), remoteCtxKey{}, &remoteRequest{peer: peer})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// remoteAuth admits only a paired device whose credential is bound to the
// requesting tailnet node (TS-13.R7).
func (s *Server) remoteAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rr := remoteFrom(r.Context())
		c, err := r.Cookie(remoteDeviceCookie)
		if err != nil || c.Value == "" {
			writeRemoteError(w, http.StatusUnauthorized, codeRemoteUnpaired, "pair this phone from AgentDeck on your Mac")
			return
		}
		device, err := s.stateStore.RemoteDeviceByTokenHash(hashRemoteToken(c.Value))
		if errors.Is(err, state.ErrNotFound) {
			writeRemoteError(w, http.StatusUnauthorized, codeRemoteUnpaired, "pair this phone from AgentDeck on your Mac")
			return
		}
		if err != nil {
			s.log.Error("remote: read device", "err", err)
			writeRemoteError(w, http.StatusInternalServerError, "internal", "internal error")
			return
		}
		if device.NodeStableID != rr.peer.StableID {
			writeRemoteError(w, http.StatusUnauthorized, codeRemoteDeviceMismatch, "this pairing belongs to another device")
			return
		}
		rr.device = &device
		now := time.Now().UTC()
		if s.remoteDevices.due(s.remoteDevices.lastSeen, device.ID, now, remoteLastSeenPeriod) {
			if err := s.stateStore.TouchRemoteDevice(device.ID, now); err != nil && !errors.Is(err, state.ErrNotFound) {
				s.log.Warn("remote: touch device", "err", err)
			}
		}
		// Sliding credential: refresh the cookie's lifetime at most daily so an
		// active phone never reaches the browser's cap (TS-13.R7).
		if s.remoteDevices.due(s.remoteDevices.issued, device.ID, now, remoteCookieReissue) {
			setRemoteDeviceCookie(w, c.Value)
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			s.log.Info("remote: mutation", "device_id", device.ID, "method", r.Method, "path", r.URL.Path)
		}
		ctx, done, err := s.remoteDevices.admit(r.Context(), device.ID, func() error {
			current, err := s.stateStore.RemoteDeviceByTokenHash(hashRemoteToken(c.Value))
			if err != nil {
				return err
			}
			if current.ID != device.ID || current.NodeStableID != rr.peer.StableID {
				return state.ErrNotFound
			}
			return nil
		})
		if errors.Is(err, state.ErrNotFound) {
			writeRemoteError(w, http.StatusUnauthorized, codeRemoteUnpaired, "pair this phone from AgentDeck on your Mac")
			return
		}
		if err != nil {
			s.log.Error("remote: revalidate device", "err", err)
			writeRemoteError(w, http.StatusInternalServerError, "internal", "internal error")
			return
		}
		defer done()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// remoteBodyFilter buffers every authenticated tailnet mutation body up to
// remoteBodyLimit and refuses a larger one before any route filter or handler
// runs (TS-13.R5/R6, INV §16).
func remoteBodyFilter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, remoteBodyLimit+1))
		if err != nil {
			writeRemoteError(w, http.StatusBadRequest, codeRemoteFieldNotAllowed, "unreadable request body")
			return
		}
		if len(body) > remoteBodyLimit {
			writeRemoteError(w, http.StatusRequestEntityTooLarge, codeRemoteBodyTooLarge, "request body too large")
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		next.ServeHTTP(w, r)
	})
}

// remoteFieldFilter rejects a body carrying any field the phone's forms do not
// set, before any process work (TS-13.R6). remoteBodyFilter has bounded it.
func remoteFieldFilter(allowed []string, next http.Handler) http.Handler {
	ok := map[string]bool{}
	for _, f := range allowed {
		ok[f] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		var fields map[string]json.RawMessage
		if err == nil {
			err = json.Unmarshal(body, &fields)
		}
		if err != nil {
			writeRemoteError(w, http.StatusBadRequest, codeRemoteFieldNotAllowed, "invalid JSON body")
			return
		}
		for name := range fields {
			if !ok[name] {
				writeRemoteError(w, http.StatusBadRequest, codeRemoteFieldNotAllowed, "field not allowed from a phone: "+name)
				return
			}
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		next.ServeHTTP(w, r)
	})
}

// remotePipelineRuntimeFilter keeps the desktop-owned runtime assignments at
// their empty/default phone representation (TS-13.R5/R6, FS-20.R15).
// remoteBodyFilter has bounded the body.
func remotePipelineRuntimeFilter(next http.Handler) http.Handler {
	type assignment struct {
		Backend string `json:"backend"`
		Model   string `json:"model"`
		Effort  string `json:"effort"`
		Fast    bool   `json:"fast"`
	}
	empty := func(a assignment) bool {
		return a.Backend == "" && a.Model == "" && a.Effort == "" && !a.Fast
	}
	allowedAssignment := func(raw json.RawMessage) bool {
		if len(raw) == 0 || string(raw) == "null" {
			return true
		}
		fields := map[string]json.RawMessage{}
		if json.Unmarshal(raw, &fields) != nil {
			return false
		}
		for name := range fields {
			if name != "backend" && name != "model" && name != "effort" && name != "fast" {
				return false
			}
		}
		var value assignment
		return json.Unmarshal(raw, &value) == nil && empty(value)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		var request struct {
			Orchestrator         json.RawMessage            `json:"orchestrator"`
			DedicatedAssignments map[string]json.RawMessage `json:"dedicated_assignments"`
		}
		if err == nil {
			err = json.Unmarshal(body, &request)
		}
		if err != nil {
			writeRemoteError(w, http.StatusBadRequest, codeRemoteFieldNotAllowed, "invalid JSON body")
			return
		}
		if !allowedAssignment(request.Orchestrator) {
			writeRemoteError(w, http.StatusBadRequest, codeRemoteFieldNotAllowed, "runtime assignment not allowed from a phone")
			return
		}
		for _, raw := range request.DedicatedAssignments {
			if !allowedAssignment(raw) {
				writeRemoteError(w, http.StatusBadRequest, codeRemoteFieldNotAllowed, "runtime assignment not allowed from a phone")
				return
			}
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		next.ServeHTTP(w, r)
	})
}

// serveRemote serves a ready tailnet listener until its stop function runs,
// which ends every open phone request and connection (FS-20.R4).
func (s *Server) serveRemote(l remote.Listener) func() {
	baseCtx, cancel := context.WithCancel(context.Background())
	srv := &http.Server{
		Handler:           s.remoteRoutes(l.Domain, l.WhoIs),
		BaseContext:       func(net.Listener) context.Context { return baseCtx },
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		if err := srv.Serve(l.Listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.log.Warn("remote: serve", "err", err)
		}
	}()
	return func() {
		cancel()
		s.remoteDevices.endAll()
		_ = srv.Close()
	}
}
