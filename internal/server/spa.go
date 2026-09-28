package server

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// phoneFiles are the phone app's own files. Only the tailnet listener serves
// them, and it never serves the desktop entry (TS-13.R14, TS-08.R73).
var phoneFiles = map[string]bool{
	"remote.html":        true,
	"remote-sw.js":       true,
	"remote.webmanifest": true,
	"remote-icon.svg":    true,
}

// spaHandler serves files from fsys and falls back to index.html for any path
// that does not resolve to an existing file — the standard single-page-app
// routing behavior. Shared by both the embedded (production) and disk (dev)
// static handlers. The desktop never serves the phone app's files.
func spaHandler(fsys fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(fsys))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Normalize the request path to a clean, rooted filesystem path.
		reqPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if reqPath == "" || strings.HasPrefix(reqPath, "..") {
			reqPath = "index.html"
		}
		if _, err := fs.Stat(fsys, reqPath); err != nil || phoneFiles[reqPath] {
			// Not a real asset → SPA fallback to index.html.
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, r2)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}

// phoneSPAHandler is the tailnet listener's static handler: built assets, the
// phone entry for every app route, and the service worker at /sw.js so it can
// control the whole origin. The desktop entry is never served.
func phoneSPAHandler(fsys fs.FS) http.Handler {
	assets := phoneAssetGraph(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		switch {
		case reqPath == "sw.js":
			w.Header().Set("Cache-Control", "no-cache")
			reqPath = "remote-sw.js"
		case reqPath == "" || strings.HasPrefix(reqPath, "..") || reqPath == "index.html":
			reqPath = "remote.html"
		}
		if st, err := fs.Stat(fsys, reqPath); err == nil && !st.IsDir() && !assets[reqPath] {
			writeRemoteError(w, http.StatusNotFound, codeRemoteRouteNotAvailable, "this file is only available on the Mac")
			return
		} else if err != nil || st.IsDir() {
			reqPath = "remote.html"
		}
		if reqPath == "remote.html" {
			w.Header().Set("Cache-Control", "no-cache")
			if _, err := fs.Stat(fsys, reqPath); err != nil {
				writeRemoteError(w, http.StatusNotFound, codeRemoteRouteNotAvailable, "the phone app is not built")
				return
			}
		}
		// Serve under the resolved name: ServeFileFS redirects any request
		// path ending in /index.html.
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/" + reqPath
		http.ServeFileFS(w, r2, fsys, reqPath)
	})
}

type viteManifestEntry struct {
	File           string   `json:"file"`
	CSS            []string `json:"css"`
	Assets         []string `json:"assets"`
	Imports        []string `json:"imports"`
	DynamicImports []string `json:"dynamicImports"`
}

// phoneAssetGraph resolves the phone entry through Vite's build manifest.
// Files outside this graph may exist in the shared dist but never cross the
// unauthenticated tailnet static surface (TS-13.R14).
func phoneAssetGraph(fsys fs.FS) map[string]bool {
	allowed := map[string]bool{}
	for name := range phoneFiles {
		allowed[name] = true
	}
	allowed["sw.js"] = true
	data, err := fs.ReadFile(fsys, ".vite/manifest.json")
	if err != nil {
		return allowed
	}
	manifest := map[string]viteManifestEntry{}
	if json.Unmarshal(data, &manifest) != nil {
		return allowed
	}
	seen := map[string]bool{}
	var visit func(string)
	visit = func(key string) {
		if seen[key] {
			return
		}
		seen[key] = true
		entry, ok := manifest[key]
		if !ok {
			return
		}
		for _, name := range append(append([]string{entry.File}, entry.CSS...), entry.Assets...) {
			if name != "" {
				allowed[name] = true
			}
		}
		for _, child := range append(entry.Imports, entry.DynamicImports...) {
			visit(child)
		}
	}
	visit("remote.html")
	return allowed
}
