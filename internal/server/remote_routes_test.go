package server

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/remote"
	"github.com/agentdeck/agentdeck/internal/state"
)

const testDomain = "agentdeck.tail1.ts.net"

// Every loopback route is either allowlisted or explicitly denied on the
// tailnet listener, and the allowlist names no route that does not exist
// (TS-13.R5, INV §10).
func TestRemoteRouteInventoryIsClassified(t *testing.T) {
	s := testServer(t, true)
	seen := map[string]bool{}
	for _, e := range s.routeTable() {
		seen[e.pattern] = true
		_, allowed := remoteAllowed[e.pattern]
		if allowed == remoteDenied[e.pattern] {
			t.Errorf("route %q must be exactly one of allowlisted or denied (allowed=%v)", e.pattern, allowed)
		}
	}
	for p := range remoteAllowed {
		if !seen[p] {
			t.Errorf("allowlisted route %q is not in the loopback inventory", p)
		}
	}
	for p := range remoteDenied {
		if !seen[p] {
			t.Errorf("denied route %q is not in the loopback inventory", p)
		}
	}
}

func testWhoIs(peers map[string]string) func(context.Context, string) (remote.Peer, error) {
	return func(_ context.Context, addr string) (remote.Peer, error) {
		if id, ok := peers[addr]; ok {
			return remote.Peer{StableID: id, Login: "me@example.com"}, nil
		}
		return remote.Peer{}, errNoPeer
	}
}

// pairTestDevice stores a paired device bound to nodeID and returns its token.
func pairTestDevice(t *testing.T, s *Server, id, nodeID string) string {
	t.Helper()
	token := "token-" + id
	if err := s.stateStore.InsertRemoteDevice(state.RemoteDevice{ID: id, Name: id, TokenHash: hashRemoteToken(token), NodeStableID: nodeID}); err != nil {
		t.Fatal(err)
	}
	return token
}

func phoneRequest(method, path, body, token string) *http.Request {
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	r.Host = testDomain
	r.RemoteAddr = "100.64.0.2:5000"
	if method != http.MethodGet {
		r.Header.Set("Origin", "https://"+testDomain)
	}
	if token != "" {
		r.AddCookie(&http.Cookie{Name: remoteDeviceCookie, Value: token})
	}
	return r
}

func TestRemoteGuardAndDeviceAuth(t *testing.T) {
	s := testServer(t, true)
	h := s.remoteRoutes(testDomain, testWhoIs(map[string]string{"100.64.0.2:5000": "node-phone"}))
	token := pairTestDevice(t, s, "d1", "node-phone")
	other := pairTestDevice(t, s, "d2", "node-other")

	cases := []struct {
		name   string
		mutate func(*http.Request)
		token  string
		status int
		code   string
	}{
		{"wrong host", func(r *http.Request) { r.Host = "127.0.0.1:4317" }, token, 403, codeRemoteForbidden},
		{"foreign origin", func(r *http.Request) { r.Header.Set("Origin", "https://evil.example") }, token, 403, codeRemoteForbidden},
		{"unknown peer", func(r *http.Request) { r.RemoteAddr = "100.64.0.9:1" }, token, 403, codeRemoteForbidden},
		{"no cookie", nil, "", 401, codeRemoteUnpaired},
		{"unknown cookie", nil, "forged", 401, codeRemoteUnpaired},
		{"cookie from another node", nil, other, 401, codeRemoteDeviceMismatch},
		{"paired", nil, token, 200, ""},
		{"paired with :443 host", func(r *http.Request) { r.Host = testDomain + ":443" }, token, 200, ""},
	}
	for _, tc := range cases {
		r := phoneRequest(http.MethodGet, "/api/health", "", tc.token)
		if tc.mutate != nil {
			tc.mutate(r)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		if rec.Code != tc.status || errorCode(t, rec) != tc.code {
			t.Errorf("%s: %d %s, want %d %s", tc.name, rec.Code, rec.Body, tc.status, tc.code)
		}
	}

	// A mutation without Origin is refused before authentication.
	r := phoneRequest(http.MethodPost, "/api/sessions/x/cancel", "", token)
	r.Header.Del("Origin")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != 403 {
		t.Fatalf("originless POST = %d", rec.Code)
	}

	// The first authenticated response slides the credential; the next does not.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, phoneRequest(http.MethodGet, "/api/health", "", token))
	if got := rec.Header().Get("Set-Cookie"); got != "" {
		t.Fatalf("cookie re-issued twice within a day: %q", got)
	}
}

func TestRemoteCookieIsHostScopedAndSliding(t *testing.T) {
	s := testServer(t, true)
	h := s.remoteRoutes(testDomain, testWhoIs(map[string]string{"100.64.0.2:5000": "n"}))
	token := pairTestDevice(t, s, "d1", "n")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, phoneRequest(http.MethodGet, "/api/health", "", token))
	c := rec.Result().Cookies()
	if len(c) != 1 || c[0].Name != remoteDeviceCookie || c[0].Value != token || !c[0].Secure || !c[0].HttpOnly ||
		c[0].SameSite != http.SameSiteStrictMode || c[0].Path != "/" || c[0].MaxAge != 400*24*3600 || c[0].Domain != "" {
		t.Fatalf("cookie = %+v", c)
	}
	devices, _ := s.stateStore.ListRemoteDevices()
	if time.Since(devices[0].LastSeenAt) > time.Minute {
		t.Fatalf("last seen not recorded: %v", devices[0].LastSeenAt)
	}
}

func TestRemoteAllowlistAndFieldFilter(t *testing.T) {
	s := testServer(t, true)
	h := s.remoteRoutes(testDomain, testWhoIs(map[string]string{"100.64.0.2:5000": "n"}))
	token := pairTestDevice(t, s, "d1", "n")

	for _, c := range []struct{ method, path string }{
		{"GET", "/api/backends"},
		{"GET", "/api/remote"},
		{"PUT", "/api/config"},
		{"POST", "/api/sessions/a/clone"},
		{"POST", "/api/sessions/a/switch-runtime"},
		{"GET", "/api/sessions/a/terminal/ws"},
		{"POST", "/mcp"},
		{"POST", "/api/hook"},
		{"GET", "/api/archive"},
		{"POST", "/"},
		{"GET", "/api/unknown"},
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, phoneRequest(c.method, c.path, "{}", token))
		if rec.Code != 404 || errorCode(t, rec) != codeRemoteRouteNotAvailable {
			t.Errorf("%s %s = %d %s", c.method, c.path, rec.Code, rec.Body)
		}
	}

	for _, c := range []struct{ path, body string }{
		{"/api/sessions", `{"role":"agentdecker","project":"my-app","interface":"terminal"}`},
		{"/api/sessions", `{"role":"agentdecker","project":"my-app","group":"g"}`},
		{"/api/tasks", `{"project":"my-app","display_name":"x","instruction":"y","arms":[]}`},
		{"/api/tasks", `{"project":"my-app","target_kind":"agent","target_agent_id":"a"}`},
		{"/api/sessions", `not json`},
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, phoneRequest(http.MethodPost, c.path, c.body, token))
		if rec.Code != 400 || errorCode(t, rec) != codeRemoteFieldNotAllowed {
			t.Errorf("POST %s %s = %d %s", c.path, c.body, rec.Code, rec.Body)
		}
	}

	// An allowed body reaches the shared handler (which then validates it).
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, phoneRequest(http.MethodPost, "/api/tasks", `{"project":"nope","display_name":"x","instruction":"y"}`, token))
	if code := errorCode(t, rec); code != "not_found" {
		t.Fatalf("allowed body blocked: %d %s", rec.Code, rec.Body)
	}
	// Allowlisted reads use the loopback handlers.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, phoneRequest(http.MethodGet, "/api/projects", "", token))
	if rec.Code != 200 {
		t.Fatalf("GET /api/projects = %d", rec.Code)
	}
}

// readEventTypes collects SSE event names until the stream ends or the
// deadline passes.
func readEventTypes(body *bufio.Reader, until time.Time, stop func(string) bool) ([]string, error) {
	var types []string
	for time.Now().Before(until) {
		line, err := body.ReadString('\n')
		if err != nil {
			return types, err
		}
		if name, ok := strings.CutPrefix(strings.TrimSpace(line), "event: "); ok {
			types = append(types, name)
			if stop != nil && stop(name) {
				return types, nil
			}
		}
	}
	return types, nil
}

// Over a real listener: the phone stream never carries remote_update, revoking
// the device closes its stream promptly, and turning remote off closes every
// open phone connection (TS-13.R3/R9, FS-20.R4).
func TestRemoteStreamFilteringRevokeAndDisable(t *testing.T) {
	s, _ := remoteTestServer(t)
	token := pairTestDevice(t, s, "d1", "node-phone")
	var stop func()
	lnReady := make(chan string, 1)
	s.remote = remote.NewManager(func() (remote.Node, error) {
		return &fakeTailnetNode{domain: testDomain}, nil
	}, func(l remote.Listener) func() {
		l.WhoIs = func(context.Context, string) (remote.Peer, error) { return remote.Peer{StableID: "node-phone"}, nil }
		stop = s.serveRemote(l)
		lnReady <- l.Listener.Addr().String()
		return stop
	}, func(st remote.Status) { s.eventBus.Publish("remote_update", nil, s.remoteViewFor(st)) }, s.log)
	s.remote.SetPollInterval(time.Millisecond)
	s.remote.Enable()
	t.Cleanup(s.remote.Disable)
	addr := <-lnReady

	open := func() (*http.Response, *bufio.Reader) {
		req, _ := http.NewRequest(http.MethodGet, "http://"+addr+"/api/events", nil)
		req.Host = testDomain
		req.AddCookie(&http.Cookie{Name: remoteDeviceCookie, Value: token})
		resp, err := http.DefaultClient.Do(req)
		if err != nil || resp.StatusCode != 200 {
			t.Fatalf("open stream: %v %v", resp, err)
		}
		return resp, bufio.NewReader(resp.Body)
	}

	resp, body := open()
	// The hydration marker is a state_update; the test snapshot is empty.
	if _, err := readEventTypes(body, time.Now().Add(2*time.Second), func(n string) bool { return n == "state_update" }); err != nil {
		t.Fatal(err)
	}
	s.eventBus.Publish("remote_update", nil, s.remoteViewFor(s.remote.Status()))
	s.eventBus.Publish("task_update", nil, map[string]any{})
	types, err := readEventTypes(body, time.Now().Add(2*time.Second), func(n string) bool { return n == "task_update" })
	if err != nil {
		t.Fatal(err)
	}
	for _, typ := range types {
		if typ == "remote_update" {
			t.Fatal("phone stream carried remote_update")
		}
	}

	start := time.Now()
	s.remoteDevices.end("d1")
	if _, err := readEventTypes(body, time.Now().Add(3*time.Second), nil); err == nil {
		t.Fatal("revoked device stream stayed open")
	}
	if time.Since(start) > time.Second {
		t.Fatalf("revoke took %v", time.Since(start))
	}
	resp.Body.Close()

	resp, body = open()
	defer resp.Body.Close()
	putRemote(t, s.routes(), `{"enabled":false}`)
	if _, err := readEventTypes(body, time.Now().Add(3*time.Second), nil); err == nil {
		t.Fatal("disable left the phone stream open")
	}
	if _, err := http.Get("http://" + addr + "/api/health"); err == nil {
		t.Fatal("phone address still answers after disable")
	}
}
