package server

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

// localOnly is the browser-facing security boundary for the loopback-only
// server (see bind.go). It enforces two checks on every request:
//
//   - The Host header must name this machine's loopback (localhost or a
//     loopback IP). Without this, a DNS-rebinding page (attacker.com resolving
//     to 127.0.0.1) becomes same-origin with the dashboard in the victim's
//     browser and can read and drive the whole unauthenticated API.
//   - The Origin header, when a browser sends one, must be a local origin.
//     This is what actually stops cross-site WebSocket connects and
//     no-preflight ("simple") cross-site POSTs — CORS response headers alone
//     never block the request from being sent and handled.
//
// Non-browser local clients (hook curl, agent MCP clients, the CLI) send no
// Origin header and pass. The guard wraps the entire mux — API, /mcp, the
// terminal WebSocket, and the static UI — so no route can be mounted outside
// it (routes.go returns localOnly(mux)).
func localOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isLocalHost(r.Host) {
			writeError(w, http.StatusForbidden, "forbidden non-local Host header")
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && !isLocalOrigin(origin) {
			writeError(w, http.StatusForbidden, "forbidden cross-origin request")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// isLocalHost reports whether a Host header value ("host", "host:port",
// "[v6]", "[v6]:port") names the local machine's loopback.
func isLocalHost(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	} else {
		host = strings.Trim(host, "[]")
	}
	return isLocalHostname(host)
}

// isLocalOrigin reports whether an Origin header value is a local origin. The
// opaque "null" origin and anything unparseable are not local.
func isLocalOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	return isLocalHostname(u.Hostname())
}

// isLocalHostname reports whether a bare hostname (no port, no brackets) is
// "localhost" or a loopback IP. A trailing FQDN dot is tolerated.
func isLocalHostname(hostname string) bool {
	h := strings.ToLower(strings.TrimSuffix(hostname, "."))
	if h == "localhost" {
		return true
	}
	if ip := net.ParseIP(h); ip != nil {
		return ip.IsLoopback()
	}
	return false
}
