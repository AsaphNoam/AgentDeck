package server

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// spaHandler serves files from fsys and falls back to index.html for any path
// that does not resolve to an existing file — the standard single-page-app
// routing behavior. Shared by both the embedded (production) and disk (dev)
// static handlers.
func spaHandler(fsys fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(fsys))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Normalize the request path to a clean, rooted filesystem path.
		reqPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if reqPath == "" || strings.HasPrefix(reqPath, "..") {
			reqPath = "index.html"
		}
		if _, err := fs.Stat(fsys, reqPath); err != nil {
			// Not a real asset → SPA fallback to index.html.
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, r2)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
