package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AsaphNoam/Chuck/internal/backend/credcheck"
	"github.com/AsaphNoam/Chuck/internal/config"
	rt "github.com/AsaphNoam/Chuck/internal/runtime"
)

// refreshFixture seeds one Claude backend whose provider is a marker script
// that records every invocation and reports a version.
func refreshFixture(t *testing.T, autosync bool) (*Server, http.Handler, string) {
	t.Helper()
	srv := testServer(t, false)
	srv.credCheck = func(context.Context, config.Backend, config.Model, map[string]string) credcheck.CredResult {
		return credcheck.CredResult{Status: "ok"}
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "calls")
	exe := filepath.Join(dir, "claude")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\necho \"$*\" >> "+marker+"\necho '2.1.300 (Claude Code)'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := srv.configStore.WriteBackends(config.BackendsConfig{Version: 2, Backends: map[string]config.Backend{
		"claude": {Name: "Claude", Type: "claude-acp", Default: true, DefaultModel: "sonnet", AutoSyncModels: autosync,
			Env:    map[string]string{"CLAUDE_CODE_EXECUTABLE": exe},
			Models: map[string]config.Model{"sonnet": {Name: "Sonnet", Model: "sonnet", Efforts: []string{}}}},
	}}); err != nil {
		t.Fatal(err)
	}
	return srv, srv.routes(), marker
}

func backendsBody(t *testing.T, h http.Handler) (map[string]json.RawMessage, string) {
	t.Helper()
	rec := doGET(t, h, "/api/backends")
	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body, rec.Header().Get("ETag")
}

func refresh(t *testing.T, h http.Handler, etag, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := newLocalRequest(http.MethodPost, "/api/backends/claude/refresh-provider", bytes.NewBufferString(body))
	req.Header.Set("If-Match", etag)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// FS-09.A38/A43, TS-03.R52–R53, TS-04.R72: GET reports the selection without
// running the provider; Refresh runs it once, and its version then appears in
// GET metadata that never changes the catalog ETag.
func TestRefreshProviderObservesVersionWithoutExecOnRead(t *testing.T) {
	_, h, marker := refreshFixture(t, false)
	body, etag := backendsBody(t, h)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("GET /api/backends executed the provider")
	}
	var runtimes map[string]map[string]providerRuntime
	if err := json.Unmarshal(body["provider_runtimes"], &runtimes); err != nil {
		t.Fatal(err)
	}
	if got := runtimes["claude"]["sonnet"]; got.Source != "backend" || got.State != "available" || got.Version != "" {
		t.Fatalf("pre-refresh runtime = %+v", got)
	}

	rec := refresh(t, h, etag, "")
	if rec.Code != 200 {
		t.Fatalf("refresh = %d %s", rec.Code, rec.Body)
	}
	var res refreshProviderResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.Runtime.Version != "2.1.300" || res.Credentials.Status != "ok" || res.Catalog.Status != "disabled" || rec.Header().Get("ETag") != etag {
		t.Fatalf("refresh result = %+v etag=%s", res, rec.Header().Get("ETag"))
	}
	calls, _ := os.ReadFile(marker)
	if strings.TrimSpace(string(calls)) != "--version" {
		t.Fatalf("provider calls = %q, want one --version", calls)
	}

	body, etagAfter := backendsBody(t, h)
	_ = json.Unmarshal(body["provider_runtimes"], &runtimes)
	if runtimes["claude"]["sonnet"].Version != "2.1.300" || runtimes["claude"]["sonnet"].CheckedAt == "" {
		t.Fatalf("observation not projected: %+v", runtimes["claude"]["sonnet"])
	}
	if etagAfter != etag {
		t.Fatal("runtime metadata changed the catalog ETag")
	}
	if strings.Contains(string(mustRead(t, filepath.Join(os.Getenv("CHUCK_HOME"), "backends.json"))), "2.1.300") {
		t.Fatal("runtime metadata was persisted")
	}
}

// TS-03.R53: a stale ETag is rejected before any process work; bad bodies and
// targets are typed; excess concurrency is 429.
func TestRefreshProviderRejectsStaleBadAndBusy(t *testing.T) {
	_, h, marker := refreshFixture(t, false)
	_, etag := backendsBody(t, h)
	if rec := refresh(t, h, `"stale"`, ""); rec.Code != 409 || errorCodeOf(t, rec.Body.Bytes()) != rt.CodeBackendCatalogChanged {
		t.Fatalf("stale = %d %s", rec.Code, rec.Body)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("a stale refresh ran the provider")
	}
	if rec := refresh(t, h, etag, `{"model_id":"sonnet","path":"/bin/sh"}`); rec.Code != 422 {
		t.Fatalf("extra field = %d", rec.Code)
	}
	if rec := refresh(t, h, etag, `{"model_id":"nope"}`); rec.Code != 404 {
		t.Fatalf("unknown model = %d", rec.Code)
	}
	providerRefreshSlots <- struct{}{}
	providerRefreshSlots <- struct{}{}
	rec := refresh(t, h, etag, "")
	<-providerRefreshSlots
	<-providerRefreshSlots
	if rec.Code != 429 || errorCodeOf(t, rec.Body.Bytes()) != rt.CodeProviderCheckBusy {
		t.Fatalf("busy = %d %s", rec.Code, rec.Body)
	}
}

// A catalog saved while the check runs wins: the refresh returns 409 and
// publishes no observation (TS-03.R53, FS-09.A43).
func TestRefreshProviderLosesToAConcurrentSave(t *testing.T) {
	srv, h, _ := refreshFixture(t, false)
	_, etag := backendsBody(t, h)
	srv.credCheck = func(context.Context, config.Backend, config.Model, map[string]string) credcheck.CredResult {
		b, _ := srv.configStore.ReadBackends()
		be := b.Backends["claude"]
		be.Name = "Renamed meanwhile"
		b.Backends["claude"] = be
		_ = srv.configStore.WriteBackends(b)
		return credcheck.CredResult{Status: "ok"}
	}
	if rec := refresh(t, h, etag, ""); rec.Code != 409 {
		t.Fatalf("concurrent save = %d %s", rec.Code, rec.Body)
	}
	body, _ := backendsBody(t, h)
	var runtimes map[string]map[string]providerRuntime
	_ = json.Unmarshal(body["provider_runtimes"], &runtimes)
	if runtimes["claude"]["sonnet"].Version != "" {
		t.Fatal("a rejected refresh leaked its observation")
	}
}

// FS-09.A40: with autosync, refresh adds new local candidates without changing
// the default; a missing source is a non-blocking unavailable result.
func TestRefreshProviderImportsAddOnly(t *testing.T) {
	srv, h, _ := refreshFixture(t, true)
	home := t.TempDir()
	t.Setenv("HOME", home)
	settings := filepath.Join(home, ".claude", "settings.json")
	_, etag := backendsBody(t, h)
	rec := refresh(t, h, etag, "")
	var res refreshProviderResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if res.Catalog.Status != "unavailable" {
		t.Fatalf("missing source = %+v", res.Catalog)
	}
	if err := os.MkdirAll(filepath.Dir(settings), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settings, []byte(`{"model":"claude-opus-5-5"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	rec = refresh(t, h, etag, "")
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if res.Catalog.Status != "added" || res.Catalog.AddedCount != 1 || rec.Header().Get("ETag") == etag {
		t.Fatalf("import = %+v etag=%s", res.Catalog, rec.Header().Get("ETag"))
	}
	b, _ := srv.configStore.ReadBackends()
	if _, ok := b.Backends["claude"].Models["claude-opus-5-5"]; !ok || b.Backends["claude"].DefaultModel != "sonnet" {
		t.Fatalf("catalog after import = %+v", b.Backends["claude"])
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func errorCodeOf(t *testing.T, body []byte) string { return apiErrorCode(t, body) }
