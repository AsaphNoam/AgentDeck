package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/agentdeck/agentdeck/internal/backend/credcheck"
	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/configsource"
)

// FS-04.A20 (R40) / FS-08.A11 (R34) / FS-09.A20 (R47) — item-scoped backend
// creation and the project-free connection it can perform in the same action.

// createServer builds a seeded server with stubbed credential checks: item
// creation always amends an existing durable catalog.
func createServer(t *testing.T) (*Server, http.Handler) {
	t.Helper()
	srv := testServer(t, true)
	srv.credCheck = func(context.Context, config.Backend, config.Model, map[string]string) credcheck.CredResult {
		return credcheck.CredResult{Status: "ok"}
	}
	return srv, srv.routes()
}

func TestCreateBackendAddsOnlyTheRequestedEntry(t *testing.T) {
	srv, h := createServer(t)

	// A pre-existing hand edit stands in for durable catalog state the create
	// must not disturb; the browser's unsaved whole-catalog draft is never sent.
	before, _ := srv.configStore.ReadBackends()
	before.Backends["claude"] = config.Backend{
		Name: "My Claude", Type: "claude-acp", Default: true, DefaultModel: "sonnet",
		Models: map[string]config.Model{"sonnet": {Name: "Mine", Model: "sonnet"}},
	}
	if err := srv.configStore.WriteBackends(before); err != nil {
		t.Fatalf("seed: %v", err)
	}

	rec := doJSON(t, h, http.MethodPost, "/api/backends",
		`{"backend_id":"codex-2","name":"Second Codex","type":"codex-acp"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp createBackendResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("create body: %v", err)
	}
	// The starter is the canonical Codex template with the submitted name, not
	// an incomplete draft with an empty model.
	if resp.Backend.Name != "Second Codex" || resp.Backend.Type != "codex-acp" {
		t.Fatalf("created = %+v", resp.Backend)
	}
	if _, ok := resp.Backend.Models[resp.Backend.DefaultModel]; !ok || resp.Backend.DefaultModel == "" {
		t.Fatalf("starter model missing: %+v", resp.Backend)
	}
	if resp.Connection != nil {
		t.Errorf("create-only must omit connection: %+v", resp.Connection)
	}

	after, err := srv.configStore.ReadBackends()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if _, ok := after.Backends["codex-2"]; !ok {
		t.Fatal("created backend not durable")
	}
	// Every other entry, and the existing default, is untouched.
	if got := after.Backends["claude"]; got.Name != "My Claude" || !got.Default || len(got.Models) != 1 {
		t.Errorf("unrelated backend changed: %+v", got)
	}
	if after.Backends["codex-2"].Default {
		t.Error("a non-empty catalog must preserve its existing default")
	}
}

func TestCreateBackendIntoEmptyCatalogBecomesDefault(t *testing.T) {
	srv, h := createServer(t)
	if err := os.Remove(filepath.Join(srv.configStore.Home(), "backends.json")); err != nil {
		t.Fatalf("remove seeded catalog: %v", err)
	}

	rec := doJSON(t, h, http.MethodPost, "/api/backends",
		`{"backend_id":"only","name":"Only","type":"claude-acp"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	after, err := srv.configStore.ReadBackends()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !after.Backends["only"].Default {
		t.Errorf("the sole backend must become the catalog default: %+v", after.Backends["only"])
	}
}

func TestCreateBackendRejectsInvalidInputWithoutWriting(t *testing.T) {
	srv, h := createServer(t)
	before, _ := srv.configStore.ReadBackends()

	for _, tc := range []struct{ name, body string }{
		{"bad id", `{"backend_id":"Not A Slug","name":"X","type":"codex-acp"}`},
		{"blank name", `{"backend_id":"new-one","name":"   ","type":"codex-acp"}`},
		{"unknown type", `{"backend_id":"new-one","name":"X","type":"llama-acp"}`},
		{"connect unsupported", `{"backend_id":"new-one","name":"X","type":"opencode-acp","connect_native_configuration":true}`},
	} {
		rec := doJSON(t, h, http.MethodPost, "/api/backends", tc.body)
		if rec.Code != http.StatusBadRequest && rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: status = %d body=%s", tc.name, rec.Code, rec.Body.String())
		}
		after, _ := srv.configStore.ReadBackends()
		if len(after.Backends) != len(before.Backends) {
			t.Fatalf("%s: catalog changed on a rejected create", tc.name)
		}
	}
}

func TestCreateBackendReplayIsIdempotentAndConflictsOnDifference(t *testing.T) {
	srv, h := createServer(t)
	const body = `{"backend_id":"codex-2","name":"Second Codex","type":"codex-acp"}`

	if rec := doJSON(t, h, http.MethodPost, "/api/backends", body); rec.Code != http.StatusCreated {
		t.Fatalf("first create status = %d", rec.Code)
	}
	// A connection attempt already enabled sync; an exact replay must not reset it.
	catalog, _ := srv.configStore.ReadBackends()
	entry := catalog.Backends["codex-2"]
	entry.AutoSyncModels = true
	entry.Models["imported"] = config.Model{Name: "Imported", Model: "imported"}
	catalog.Backends["codex-2"] = entry
	if err := srv.configStore.WriteBackends(catalog); err != nil {
		t.Fatalf("write: %v", err)
	}

	rec := doJSON(t, h, http.MethodPost, "/api/backends", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("replay status = %d body=%s", rec.Code, rec.Body.String())
	}
	after, _ := srv.configStore.ReadBackends()
	replayed := after.Backends["codex-2"]
	if !replayed.AutoSyncModels {
		t.Error("replay reset the autosync flag")
	}
	if _, ok := replayed.Models["imported"]; !ok {
		t.Error("replay discarded already-imported models")
	}

	for _, tc := range []struct{ name, body string }{
		{"different name", `{"backend_id":"codex-2","name":"Renamed","type":"codex-acp"}`},
		{"different type", `{"backend_id":"codex-2","name":"Second Codex","type":"claude-acp"}`},
	} {
		rec := doJSON(t, h, http.MethodPost, "/api/backends", tc.body)
		if rec.Code != http.StatusConflict {
			t.Errorf("%s: status = %d, want 409", tc.name, rec.Code)
		}
		if got := rec.Body.String(); !strings.Contains(got, "backend_exists") {
			t.Errorf("%s: body = %s, want backend_exists", tc.name, got)
		}
	}
}

// TestCreateBackendsConcurrentlyKeepsEveryEntry pins the shared catalog lock: a
// read-modify-write per create without it loses entries (TS-07.R18).
func TestCreateBackendsConcurrentlyKeepsEveryEntry(t *testing.T) {
	srv, h := createServer(t)

	const n = 8
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("codex-%d", i)
			doJSON(t, h, http.MethodPost, "/api/backends",
				fmt.Sprintf(`{"backend_id":%q,"name":"Codex %d","type":"codex-acp"}`, id, i))
		}(i)
	}
	wg.Wait()

	after, err := srv.configStore.ReadBackends()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	for i := 0; i < n; i++ {
		if _, ok := after.Backends[fmt.Sprintf("codex-%d", i)]; !ok {
			t.Errorf("codex-%d was lost to a concurrent create", i)
		}
	}
	if verr := config.ValidateBackendsConfig(&after); verr != nil {
		t.Errorf("catalog is invalid after concurrent creates: %v", verr)
	}
}

// connectServer builds a federation test server whose Claude resolver AND
// model-import reader both read one private fixture home, so a connection can be
// driven end to end without touching the real user configuration.
func connectServer(t *testing.T) (*Server, http.Handler) {
	t.Helper()
	srv, root, _ := federationServer(t)
	// ImportConfiguredModels reads ~/.claude/settings.json via os.UserHomeDir.
	t.Setenv("HOME", filepath.Dir(root))
	// "sonnet" is already in the Claude starter, so it must not be re-added.
	if err := os.WriteFile(filepath.Join(root, "settings.json"),
		[]byte(`{"model":"user-model","availableModels":["claude-sonnet-4-6","claude-opus-4-8","sonnet"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	return srv, srv.routes()
}

func TestCreateBackendAndConnectImportsOnlyIntoTheTarget(t *testing.T) {
	srv, h := connectServer(t)

	// A second opted-in Claude backend proves the import is target-scoped and
	// never a whole-catalog startup sync.
	catalog, _ := srv.configStore.ReadBackends()
	catalog.Backends["claude"] = config.Backend{
		Name: "Claude", Type: "claude-acp", Default: true, AutoSyncModels: true,
		DefaultModel: "sonnet", Models: map[string]config.Model{"sonnet": {Name: "Sonnet", Model: "sonnet"}},
	}
	if err := srv.configStore.WriteBackends(catalog); err != nil {
		t.Fatalf("seed: %v", err)
	}

	rec := doJSON(t, h, http.MethodPost, "/api/backends",
		`{"backend_id":"claude-2","name":"Work Claude","type":"claude-acp","connect_native_configuration":true}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp createBackendResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("body: %v", err)
	}
	if resp.Connection == nil || resp.Connection.Status != "connected" {
		t.Fatalf("connection = %+v", resp.Connection)
	}
	// user-model, claude-sonnet-4-6, claude-opus-4-8 — "sonnet" is already
	// represented by the starter and is never duplicated.
	if !resp.Connection.ModelSyncEnabled || resp.Connection.ModelsAdded != 3 {
		t.Errorf("connection = %+v, want sync enabled and 3 imported models", resp.Connection)
	}
	if resp.Connection.Binding == nil || resp.Connection.Binding.Mode != configsource.ModeLinked {
		t.Fatalf("binding = %+v, want a linked binding", resp.Connection.Binding)
	}

	after, _ := srv.configStore.ReadBackends()
	target := after.Backends["claude-2"]
	if !target.AutoSyncModels {
		t.Error("target did not gain continuing sync")
	}
	for _, id := range []string{"user-model", "claude-sonnet-4-6", "claude-opus-4-8"} {
		if _, ok := target.Models[id]; !ok {
			t.Errorf("%s not imported into the target: %+v", id, target.Models)
		}
	}
	if target.Models["sonnet"].Name != "Claude Sonnet" {
		t.Errorf("an already-represented selector was overwritten: %+v", target.Models["sonnet"])
	}
	if target.DefaultModel != "sonnet" {
		t.Errorf("default_model changed to %q", target.DefaultModel)
	}
	if other := after.Backends["claude"]; len(other.Models) != 1 {
		t.Errorf("another opted-in backend was imported into: %+v", other.Models)
	}

	// The binding is durable, backend-global, and needed no project.
	sources, _ := srv.readConfigSources()
	if _, ok := sources.Sources["claude-2"]; !ok {
		t.Fatal("binding not persisted")
	}

	// An exact replay converges: no duplicate models, no second binding, 200.
	rec = doJSON(t, h, http.MethodPost, "/api/backends",
		`{"backend_id":"claude-2","name":"Work Claude","type":"claude-acp","connect_native_configuration":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("replay status = %d body=%s", rec.Code, rec.Body.String())
	}
	replayed, _ := srv.configStore.ReadBackends()
	if len(replayed.Backends["claude-2"].Models) != len(target.Models) {
		t.Errorf("replay duplicated models: %+v", replayed.Backends["claude-2"].Models)
	}
}

// TestProjectFreeConnectionOnASavedBackend covers the client-side half of the
// same flow: an already-saved backend connects with a project-free preview and
// an enabled bind, and can then refresh without naming a project (TS-03.R22).
func TestProjectFreeConnectionOnASavedBackend(t *testing.T) {
	srv, h := connectServer(t)

	rec := doJSON(t, h, http.MethodPost, "/api/config-sources/preview",
		`{"provider":"claude-code","root":"auto","mode":"linked","claims":["launch_defaults","model_catalog","setup"]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("preview status = %d body=%s", rec.Code, rec.Body.String())
	}
	var pv previewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &pv); err != nil {
		t.Fatalf("preview body: %v", err)
	}

	rec = doJSON(t, h, http.MethodPut, "/api/config-sources/claude",
		`{"preview_token":"`+pv.PreviewToken+`","overrides":{},"enable_model_sync":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("bind status = %d body=%s", rec.Code, rec.Body.String())
	}
	var bound bindResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &bound); err != nil {
		t.Fatalf("bind body: %v", err)
	}
	if !bound.ModelSyncEnabled || bound.ModelsAdded != 3 {
		t.Errorf("bind = %+v, want sync enabled and 3 imported models", bound)
	}
	if bound.Mode != configsource.ModeLinked {
		t.Errorf("mode = %q, want linked", bound.Mode)
	}

	catalog, _ := srv.configStore.ReadBackends()
	if !catalog.Backends["claude"].AutoSyncModels {
		t.Error("enabled bind did not turn on continuing sync")
	}

	// A backend-global binding refreshes without a project.
	if rec := doJSON(t, h, http.MethodPost, "/api/config-sources/claude/refresh", ""); rec.Code != http.StatusOK {
		t.Fatalf("refresh status = %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestMirroredBindRemainsCompatible pins that an explicit API caller can still
// choose the mirrored mode even though the normal flow never offers it.
func TestMirroredBindRemainsCompatible(t *testing.T) {
	_, h := connectServer(t)

	rec := doJSON(t, h, http.MethodPost, "/api/config-sources/preview",
		`{"provider":"claude-code","root":"auto","mode":"mirrored","claims":["launch_defaults"],"project":"fed"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("preview status = %d body=%s", rec.Code, rec.Body.String())
	}
	var pv previewResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &pv)

	rec = doJSON(t, h, http.MethodPut, "/api/config-sources/claude",
		`{"preview_token":"`+pv.PreviewToken+`","overrides":{}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("bind status = %d body=%s", rec.Code, rec.Body.String())
	}
	var bound bindResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &bound)
	if bound.Mode != configsource.ModeMirrored {
		t.Errorf("mode = %q, want mirrored", bound.Mode)
	}
	// An omitted flag keeps the source-only path: no sync claim, no import.
	if bound.ModelSyncEnabled || bound.ModelsAdded != 0 {
		t.Errorf("compatibility bind reported model sync: %+v", bound)
	}
}

func TestConnectWithNoProjectsConfigured(t *testing.T) {
	srv, h := connectServer(t)
	// Remove every project: the normal connection must not need one.
	projects, err := srv.configStore.ListProjects()
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	for id := range projects {
		if err := srv.configStore.DeleteProject(id); err != nil {
			t.Fatalf("delete %s: %v", id, err)
		}
	}

	rec := doJSON(t, h, http.MethodPost, "/api/backends",
		`{"backend_id":"claude-2","name":"Work Claude","type":"claude-acp","connect_native_configuration":true}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp createBackendResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Connection == nil || resp.Connection.Status != "connected" {
		t.Fatalf("connection = %+v", resp.Connection)
	}
}

func TestCreateBackendKeepsBackendSavedWhenConnectionFails(t *testing.T) {
	srv, root, _ := federationServer(t)
	h := srv.routes()
	// Remove the native source so discovery/preview cannot succeed.
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}

	rec := doJSON(t, h, http.MethodPost, "/api/backends",
		`{"backend_id":"claude-2","name":"Work Claude","type":"claude-acp","connect_native_configuration":true}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp createBackendResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("body: %v", err)
	}
	if resp.Connection == nil || resp.Connection.Status != "unbound" || resp.Connection.Error == nil {
		t.Fatalf("connection = %+v, want an unbound failure with a cause", resp.Connection)
	}
	if resp.Connection.Binding != nil || resp.Connection.ModelSyncEnabled {
		t.Errorf("a failed connection claimed state: %+v", resp.Connection)
	}
	// The valid backend remains saved and unbound so the ordinary action retries.
	after, _ := srv.configStore.ReadBackends()
	saved, ok := after.Backends["claude-2"]
	if !ok {
		t.Fatal("the created backend was compensated away by a failed connection")
	}
	if saved.AutoSyncModels {
		t.Error("a failed connection left autosync enabled")
	}
	sources, _ := srv.readConfigSources()
	if _, bound := sources.Sources["claude-2"]; bound {
		t.Error("a failed connection persisted a binding")
	}
}

func TestEnabledBindRestoresCatalogWhenManifestWriteFails(t *testing.T) {
	srv, h := connectServer(t)

	// Corrupt the source manifest so the bind's second write cannot proceed
	// after the catalog write already landed.
	manifest := filepath.Join(srv.configStore.Home(), "config-sources.json")
	if err := os.WriteFile(manifest, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	rec := doJSON(t, h, http.MethodPost, "/api/backends",
		`{"backend_id":"claude-2","name":"Work Claude","type":"claude-acp","connect_native_configuration":true}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp createBackendResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Connection == nil || resp.Connection.Status != "unbound" {
		t.Fatalf("connection = %+v, want unbound", resp.Connection)
	}

	after, _ := srv.configStore.ReadBackends()
	// The create survives; the merge is rolled back, so nothing claims a
	// connection the manifest does not record.
	if _, ok := after.Backends["claude-2"]; !ok {
		t.Fatal("the created backend was lost")
	}
	restored := after.Backends["claude-2"]
	if restored.AutoSyncModels {
		t.Error("catalog restoration left autosync enabled")
	}
	// The preimage is the just-created starter, so the merge is undone entirely.
	if _, leaked := restored.Models["user-model"]; leaked {
		t.Errorf("imported models survived restoration: %+v", restored.Models)
	}
	if _, starter := restored.Models["sonnet"]; !starter {
		t.Errorf("restoration lost the starter model: %+v", restored.Models)
	}
}
