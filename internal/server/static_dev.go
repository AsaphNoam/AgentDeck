//go:build dev

package server

import (
	"io/fs"
	"net/http"
	"os"
)

// In the `dev` build the UI is served from ui/dist on disk instead of being
// embedded, so a `vite build --watch` (or repeated builds) is picked up without
// recompiling the Go binary. Build with: go build -tags dev ./...
//
// distDir is resolved relative to the process working directory; run the dev
// binary from the repo root.
const distDir = "ui/dist"

// staticHandler serves ui/dist from disk with SPA fallback.
func (s *Server) staticHandler() http.Handler {
	sub := os.DirFS(distDir)
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		s.log.Warn("static(dev): ui/dist/index.html not found; run vite build", "err", err)
	}
	return spaHandler(sub)
}
