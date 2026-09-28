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
	"assets/phone.js":    {Data: []byte("phone js")},
	"assets/shared.js":   {Data: []byte("shared js")},
	"assets/main.js":     {Data: []byte("desktop js")},
	".vite/manifest.json": {Data: []byte(`{
		"remote.html":{"file":"assets/phone.js","imports":["_shared.js"]},
		"_shared.js":{"file":"assets/shared.js"},
		"index.html":{"file":"assets/main.js","imports":["_shared.js"]}
	}`)},
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
		"/sw.js": "sw", "/remote.webmanifest": "manifest", "/assets/phone.js": "phone js", "/assets/shared.js": "shared js", "/assets": "phone",
	} {
		if code, body := serveBody(t, phone, path); code != 200 || body != want {
			t.Errorf("phone %s = %d %q, want %q", path, code, body, want)
		}
	}

	desktop := spaHandler(twoEntryDist)
	for path, want := range map[string]string{
		"/": "desktop", "/remote.html": "desktop", "/remote-sw.js": "desktop",
		"/remote.webmanifest": "desktop", "/assets/main.js": "desktop js", "/settings": "desktop",
	} {
		if code, body := serveBody(t, desktop, path); code != 200 || body != want {
			t.Errorf("desktop %s = %d %q, want %q", path, code, body, want)
		}
	}

	if code, _ := serveBody(t, phoneSPAHandler(fstest.MapFS{"index.html": {Data: []byte("desktop")}}), "/"); code != 404 {
		t.Fatalf("unbuilt phone app = %d, want 404 rather than the desktop bundle", code)
	}
	if code, _ := serveBody(t, phone, "/assets/main.js"); code != 404 {
		t.Fatalf("desktop entry asset = %d, want 404", code)
	}
}
