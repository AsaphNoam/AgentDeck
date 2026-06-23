//go:build !dev

package server

import (
	"embed"
	"io/fs"
	"net/http"
)

// In the default (production) build the built UI is compiled into the binary.
// The directory ui/dist MUST exist for `go build` to succeed; install.sh / the
// Makefile produce it via `vite build`, and a minimal placeholder is committed
// so the embed compiles even before a real UI build runs.
//
//go:embed all:ui/dist
var distFS embed.FS

// uiFS returns the embedded ui/dist subtree as an fs.FS.
func uiFS() (fs.FS, error) {
	return fs.Sub(distFS, "ui/dist")
}

// staticHandler serves the embedded UI with SPA fallback to index.html.
func (s *Server) staticHandler() http.Handler {
	sub, err := uiFS()
	if err != nil {
		s.log.Error("static: embedded UI unavailable", "err", err)
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			writeError(w, http.StatusInternalServerError, "ui assets unavailable")
		})
	}
	return spaHandler(sub)
}
