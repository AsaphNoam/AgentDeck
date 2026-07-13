package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/configsource"
	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
)

func canonicalPath(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("EvalSymlinks(%s): %v", path, err)
	}
	return resolved
}

func bindFixture(t *testing.T, srv *Server, root, projectDir string) {
	t.Helper()
	sources, _ := srv.readConfigSources()
	sources.Sources["claude"] = config.SourceBinding{
		Provider: configsource.ProviderClaude, Mode: configsource.ModeLinked, Root: root,
		Claims: []string{"launch_defaults"}, Approved: []string{root, projectDir},
	}
	if err := srv.configStore.WriteConfigSources(sources); err != nil {
		t.Fatalf("WriteConfigSources: %v", err)
	}
}

// federationServer builds a seeded test server whose SourceManager resolves a
// fixture Claude tree (rather than the real user home) and registers a project
// pointing at a real directory so preview/bind work deterministically.
func federationServer(t *testing.T) (*Server, string, string) {
	t.Helper()
	srv := testServer(t, true)

	userHome := t.TempDir()
	root := filepath.Join(userHome, ".claude")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "settings.json"),
		[]byte(`{"model":"user-model","env":{"ANTHROPIC_API_KEY":"user-secret"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	projectDir := t.TempDir()
	if err := srv.configStore.WriteProject("fed", config.Project{Title: "Fed", Cwd: projectDir}); err != nil {
		t.Fatalf("WriteProject: %v", err)
	}

	srv.sourceMgr = configsource.NewManager(srv.configStore, map[string]configsource.Resolver{
		configsource.ProviderClaude: configsource.NewClaudeResolver(userHome),
	}, nil)
	// Federation persists canonical (symlink-resolved) roots; on macOS t.TempDir()
	// returns /var/... which is a symlink to /private/var/.... Canonicalize the
	// fixture paths so bound roots and approved-root fixtures match the resolver.
	return srv, canonicalPath(t, root), canonicalPath(t, projectDir)
}

func doJSON(t *testing.T, h http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := newLocalRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// FS-08.A1: preview/bind/refresh/unlink preserves consent and read-only source ownership.
func TestConfigSourcePreviewBindRefreshDelete(t *testing.T) {
	srv, root, _ := federationServer(t)
	h := srv.routes()

	// Preview (root=auto discovers ~/.claude).
	rec := doJSON(t, h, http.MethodPost, "/api/config-sources/preview",
		`{"provider":"claude-code","root":"auto","mode":"linked","claims":["launch_defaults"],"project":"fed"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("preview status = %d body=%s", rec.Code, rec.Body.String())
	}
	var pv previewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &pv); err != nil {
		t.Fatalf("preview body: %v", err)
	}
	if pv.PreviewToken == "" {
		t.Fatal("empty preview token")
	}
	if pv.Effective.Model == nil || *pv.Effective.Model != "user-model" {
		t.Fatalf("preview model = %v", pv.Effective.Model)
	}
	// The preview must not leak the secret env value.
	if strings.Contains(rec.Body.String(), "user-secret") {
		t.Fatalf("preview leaked a secret:\n%s", rec.Body.String())
	}

	// Bind with the token.
	rec = doJSON(t, h, http.MethodPut, "/api/config-sources/claude",
		`{"preview_token":"`+pv.PreviewToken+`","overrides":{}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("bind status = %d body=%s", rec.Code, rec.Body.String())
	}
	var bound configSourceBindingView
	if err := json.Unmarshal(rec.Body.Bytes(), &bound); err != nil {
		t.Fatalf("bind body: %v", err)
	}
	if bound.Root != root || bound.Health != configsource.HealthOK {
		t.Fatalf("bound = %+v (want root=%s health=ok)", bound, root)
	}

	// GET lists the binding.
	rec = doGET(t, h, "/api/config-sources?project=fed")
	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d", rec.Code)
	}
	var list configSourcesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("get body: %v", err)
	}
	if len(list.Bindings) != 1 || list.Bindings[0].BackendID != "claude" {
		t.Fatalf("bindings = %+v", list.Bindings)
	}

	// Refresh re-resolves.
	rec = doJSON(t, h, http.MethodPost, "/api/config-sources/claude/refresh?project=fed", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("refresh status = %d body=%s", rec.Code, rec.Body.String())
	}

	// Delete unbinds.
	rec = doJSON(t, h, http.MethodDelete, "/api/config-sources/claude", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d body=%s", rec.Code, rec.Body.String())
	}
	// The binding is gone.
	sources, _ := srv.readConfigSources()
	if _, ok := sources.Sources["claude"]; ok {
		t.Fatal("binding survived delete")
	}
}

func TestConfigSourceBindRejectsBadToken(t *testing.T) {
	srv, _, _ := federationServer(t)
	h := srv.routes()
	rec := doJSON(t, h, http.MethodPut, "/api/config-sources/claude",
		`{"preview_token":"deadbeef","overrides":{}}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("bad-token status = %d, want 409 body=%s", rec.Code, rec.Body.String())
	}
}

func TestConfigSourcePreviewUnknownProject(t *testing.T) {
	srv, _, _ := federationServer(t)
	h := srv.routes()
	rec := doJSON(t, h, http.MethodPost, "/api/config-sources/preview",
		`{"provider":"claude-code","root":"auto","mode":"linked","project":"nope"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown-project status = %d, want 400 body=%s", rec.Code, rec.Body.String())
	}
}

func TestConfigSourceDetachNotImplemented(t *testing.T) {
	srv, _, _ := federationServer(t)
	// Persist a binding directly so DELETE has something to act on.
	sources, _ := srv.readConfigSources()
	sources.Sources["claude"] = config.SourceBinding{
		Provider: configsource.ProviderClaude, Mode: configsource.ModeLinked,
		Root: filepath.Join(t.TempDir(), ".claude"), Claims: []string{}, Approved: []string{},
	}
	if err := srv.configStore.WriteConfigSources(sources); err != nil {
		t.Fatal(err)
	}
	h := srv.routes()
	rec := doJSON(t, h, http.MethodDelete, "/api/config-sources/claude?detach=true", "")
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("detach status = %d, want 501 body=%s", rec.Code, rec.Body.String())
	}
}

func TestConfigSourceTOCTOURejectedAtBind(t *testing.T) {
	srv, root, _ := federationServer(t)
	h := srv.routes()
	rec := doJSON(t, h, http.MethodPost, "/api/config-sources/preview",
		`{"provider":"claude-code","root":"auto","mode":"linked","claims":["launch_defaults"],"project":"fed"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("preview status = %d", rec.Code)
	}
	var pv previewResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &pv)
	// Mutate the source between preview and bind.
	if err := os.WriteFile(filepath.Join(root, "settings.json"), []byte(`{"model":"changed"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	rec = doJSON(t, h, http.MethodPut, "/api/config-sources/claude",
		`{"preview_token":"`+pv.PreviewToken+`","overrides":{}}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("TOCTOU bind status = %d, want 409 body=%s", rec.Code, rec.Body.String())
	}
}

// TestComposeLaunchFreezesFederationConfig proves a launch against a bound source
// resolves fresh and freezes the redacted provenance into the session's launch
// config, without leaking any secret value.
func TestComposeLaunchFreezesFederationConfig(t *testing.T) {
	srv, root, projectDir := federationServer(t)
	bindFixture(t, srv, root, projectDir)

	spec, _, ae := srv.composeLaunch(context.Background(), launchRequest{Role: "implementer", Project: "fed"})
	if ae != nil {
		t.Fatalf("composeLaunch: %s", ae.Message)
	}
	if len(spec.LaunchConfig) == 0 {
		t.Fatal("launch config was not frozen for a bound source")
	}
	var doc launchConfigDoc
	if err := json.Unmarshal(spec.LaunchConfig, &doc); err != nil {
		t.Fatalf("launch config decode: %v", err)
	}
	if doc.Binding.BackendID != "claude" || doc.Binding.Provider != configsource.ProviderClaude {
		t.Fatalf("binding = %+v", doc.Binding)
	}
	if doc.Resolved.Model == nil || *doc.Resolved.Model != "user-model" {
		t.Fatalf("resolved model = %v, want user-model", doc.Resolved.Model)
	}
	if doc.Generation == "" || len(doc.Fingerprints) == 0 {
		t.Fatalf("missing provenance: %+v", doc)
	}
	if !doc.NativeInherited {
		t.Error("expected native_inherited=true for a default (unspecified) model")
	}
	// The composition must OMIT the model over ACP for a native-inherited launch so
	// the CLI resolves its own configured model instead of AgentDeck forcing the
	// backend default (federation §2.4 — the core of the "defaults never applied" bug).
	if spec.ModelID != "" {
		t.Fatalf("native-inherited launch ModelID = %q, want empty (omitted)", spec.ModelID)
	}
	if strings.Contains(string(spec.LaunchConfig), "user-secret") {
		t.Fatalf("launch config leaked a secret:\n%s", spec.LaunchConfig)
	}
}

// TestComposeLaunchExplicitModelOverridesSource proves an explicitly chosen model
// wins over native inheritance: the ACP model is the chosen backend model's CLI id
// and the frozen object records native_inherited=false.
func TestComposeLaunchExplicitModelOverridesSource(t *testing.T) {
	srv, root, projectDir := federationServer(t)
	bindFixture(t, srv, root, projectDir)

	backends, err := srv.configStore.ReadBackends()
	if err != nil {
		t.Fatalf("ReadBackends: %v", err)
	}
	be := backends.Backends["claude"]
	model := be.Models[be.DefaultModel]

	spec, _, ae := srv.composeLaunch(context.Background(),
		launchRequest{Role: "implementer", Project: "fed", Model: be.DefaultModel})
	if ae != nil {
		t.Fatalf("composeLaunch: %s", ae.Message)
	}
	if spec.ModelID != model.Model {
		t.Fatalf("explicit-model ModelID = %q, want %q", spec.ModelID, model.Model)
	}
	var doc launchConfigDoc
	if err := json.Unmarshal(spec.LaunchConfig, &doc); err != nil {
		t.Fatalf("launch config decode: %v", err)
	}
	if doc.NativeInherited {
		t.Error("expected native_inherited=false when a model was explicitly chosen")
	}
}

// TestComposeLaunchBlocksInvalidSource proves an invalid bound source blocks the
// launch (never composes from stale cache), returning 422 source_invalid.
func TestComposeLaunchBlocksInvalidSource(t *testing.T) {
	srv, root, projectDir := federationServer(t)
	bindFixture(t, srv, root, projectDir)
	if err := os.WriteFile(filepath.Join(root, "settings.json"), []byte(`{bad json`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, ae := srv.composeLaunch(context.Background(), launchRequest{Role: "implementer", Project: "fed"})
	if ae == nil || ae.Code != runtime.CodeSourceInvalid {
		t.Fatalf("ae = %+v, want source_invalid", ae)
	}
}

// TestComposeLaunchNoBindingLeavesConfigEmpty proves an unbound backend launches
// normally with no frozen federation object.
func TestComposeLaunchNoBindingLeavesConfigEmpty(t *testing.T) {
	srv, _, _ := federationServer(t)
	spec, _, ae := srv.composeLaunch(context.Background(), launchRequest{Role: "implementer", Project: "fed"})
	if ae != nil {
		t.Fatalf("composeLaunch: %s", ae.Message)
	}
	if len(spec.LaunchConfig) != 0 {
		t.Fatalf("launch config = %s, want empty for an unbound backend", spec.LaunchConfig)
	}
}

// TestComposeResumeSpecCarriesFrozenLaunchConfig proves resume reproduces the
// frozen federation object from the snapshot (the default, no config_refresh).
func TestComposeResumeSpecCarriesFrozenLaunchConfig(t *testing.T) {
	srv := testServer(t, true)
	backends, err := srv.configStore.ReadBackends()
	if err != nil {
		t.Fatalf("ReadBackends: %v", err)
	}
	be := backends.Backends["claude"]
	model := be.Models[be.DefaultModel]

	frozen := json.RawMessage(`{"version":1,"binding":{"backend_id":"claude"}}`)
	snap := state.SessionSnapshot{
		AgentID: "a_res", Name: "Nova", Role: "implementer", Project: "fed",
		Backend: "claude", Model: be.DefaultModel, Interface: "chat",
		Cwd: t.TempDir(), LaunchConfig: frozen,
	}
	agent := state.Agent{
		AgentID: "a_res", Name: "Nova", Role: "implementer", Project: "fed",
		Backend: "claude", Model: be.DefaultModel, Interface: "chat",
	}
	spec, ae := srv.composeResumeSpec(agent, snap, be, model)
	if ae != nil {
		t.Fatalf("composeResumeSpec: %s", ae.Message)
	}
	if string(spec.LaunchConfig) != string(frozen) {
		t.Fatalf("resume LaunchConfig = %s, want frozen %s", spec.LaunchConfig, frozen)
	}
}

// TestComposeLaunchRejectsReservedMCPCollision proves that a native config
// declaring the reserved messaging-MCP id blocks the launch with source_conflict.
func TestComposeLaunchRejectsReservedMCPCollision(t *testing.T) {
	srv, root, projectDir := federationServer(t)
	// Rewrite the fixture settings to declare the reserved MCP id.
	if err := os.WriteFile(filepath.Join(root, "settings.json"),
		[]byte(`{"model":"user-model","mcpServers":{"`+messagingMCPName+`":{"type":"http","url":"http://x"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	bindFixture(t, srv, root, projectDir)
	_, _, ae := srv.composeLaunch(context.Background(), launchRequest{Role: "implementer", Project: "fed"})
	if ae == nil || ae.Code != runtime.CodeSourceConflict {
		t.Fatalf("ae = %+v, want source_conflict", ae)
	}
}
