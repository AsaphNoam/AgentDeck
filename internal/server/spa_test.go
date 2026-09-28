package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

var twoEntryDist = fstest.MapFS{
	"index.html":         {Data: []byte("desktop")},
	"remote.html":        {Data: []byte("phone")},
	"remote-sw.js":       {Data: []byte("sw")},
	"remote.webmanifest": {Data: []byte("manifest")},
	"assets/app.js":      {Data: []byte("js")},
}

func serveBody(t *testing.T, h http.Handler, path string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	body, _ := io.ReadAll(rec.Body)
	return rec.Code, string(body)
}

// The tailnet serves only the phone entry and the loopback never serves it
// (TS-13.R14, TS-08.R73).
func TestStaticEntriesStayOnTheirListener(t *testing.T) {
	phone := phoneSPAHandler(twoEntryDist)
	for path, want := range map[string]string{
		"/": "phone", "/pair": "phone", "/agent/a1": "phone", "/index.html": "phone",
		"/sw.js": "sw", "/remote.webmanifest": "manifest", "/assets/app.js": "js", "/assets": "phone",
	} {
		if code, body := serveBody(t, phone, path); code != 200 || body != want {
			t.Errorf("phone %s = %d %q, want %q", path, code, body, want)
		}
	}

	desktop := spaHandler(twoEntryDist)
	for path, want := range map[string]string{
		"/": "desktop", "/remote.html": "desktop", "/remote-sw.js": "desktop",
		"/remote.webmanifest": "desktop", "/assets/app.js": "js", "/settings": "desktop",
	} {
		if code, body := serveBody(t, desktop, path); code != 200 || body != want {
			t.Errorf("desktop %s = %d %q, want %q", path, code, body, want)
		}
	}

	if code, _ := serveBody(t, phoneSPAHandler(fstest.MapFS{"index.html": {Data: []byte("desktop")}}), "/"); code != 404 {
		t.Fatalf("unbuilt phone app = %d, want 404 rather than the desktop bundle", code)
	}
}
