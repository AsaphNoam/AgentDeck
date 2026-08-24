package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentdeck/agentdeck/internal/backend/credcheck"
	"github.com/agentdeck/agentdeck/internal/config"
)

// testServerWithOkCreds builds a server that returns "ok" for all cred checks.
func testServerWithOkCreds(t *testing.T) *Server {
	t.Helper()
	srv := testServer(t, false)
	srv.credCheck = func(_ context.Context, _ config.Backend, _ config.Model, _ map[string]string) credcheck.CredResult {
		return credcheck.CredResult{Status: "ok"}
	}
	return srv
}

func TestGetBackendsFallsBackForIncompleteDocument(t *testing.T) {
	srv := testServer(t, false)
	if err := os.WriteFile(filepath.Join(srv.configStore.Home(), "backends.json"), []byte(`{"version":2}`), 0o600); err != nil {
		t.Fatal(err)
	}

	rec := doGET(t, srv.routes(), "/api/backends")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/backends status = %d body=%s, want 200", rec.Code, rec.Body)
	}
	var got config.BackendsConfig
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Version != 2 || len(got.Backends) == 0 {
		t.Fatalf("GET /api/backends = %+v, want default catalog", got)
	}
}

func TestGetConfigEmptyStoreNotSatisfied(t *testing.T) {
	srv := testServerWithOkCreds(t)
	h := srv.routes()
	rec := doGET(t, h, "/api/config")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/config status = %d, want 200", rec.Code)
	}
	var resp configResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("body: %v", err)
	}
	if resp.Onboarding.Satisfied {
		t.Error("empty store: satisfied should be false")
	}
	if resp.Onboarding.Steps.Backend.Done {
		t.Error("empty store: backend step should not be done")
	}
	if resp.Onboarding.Steps.Project.Done {
		t.Error("empty store: project step should not be done")
	}
	// Roles may be seeded — but we used testServer(false) so store is empty.
	if resp.Onboarding.Steps.Role.Done {
		t.Error("empty store: role step should not be done")
	}
	if resp.AppearanceSkin != "" || resp.AppearanceSkinWarning != "" {
		t.Fatalf("empty store appearance = skin %q warning %q, want Core without warning", resp.AppearanceSkin, resp.AppearanceSkinWarning)
	}
}

func TestGetConfigSatisfiedWhenAllStepsDone(t *testing.T) {
	srv := testServerWithOkCreds(t)

	// Seed backend: write a valid backends.json with a default backend.
	backends := config.BackendsConfig{
		Version: 2,
		Backends: map[string]config.Backend{
			"claude": {
				Name:         "Claude",
				Type:         "claude-acp",
				Default:      true,
				DefaultModel: "default",
				Models: map[string]config.Model{
					"default": {Name: "D", Model: "claude-sonnet-4-6"},
				},
			},
		},
	}
	if err := srv.configStore.WriteBackends(backends); err != nil {
		t.Fatalf("WriteBackends: %v", err)
	}

	// Seed a project.
	if err := srv.configStore.WriteProject("my-app", config.Project{Title: "My App", Cwd: "/tmp"}); err != nil {
		t.Fatalf("WriteProject: %v", err)
	}

	// Seed a role.
	if err := srv.configStore.WriteRole("implementer", config.Role{Title: "Implementer", SystemPrompt: ""}); err != nil {
		t.Fatalf("WriteRole: %v", err)
	}

	h := srv.routes()
	rec := doGET(t, h, "/api/config")
	var resp configResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if !resp.Onboarding.Steps.Backend.Done {
		t.Errorf("backend step done = false, want true. detail=%s", resp.Onboarding.Steps.Backend.Detail)
	}
	if !resp.Onboarding.Steps.Project.Done {
		t.Error("project step done = false, want true")
	}
	if !resp.Onboarding.Steps.Role.Done {
		t.Error("role step done = false, want true")
	}
	if !resp.Onboarding.Satisfied {
		t.Error("satisfied = false, want true")
	}
}

func TestGetConfigBadCredsMakesBackendNotDone(t *testing.T) {
	srv := testServer(t, false)
	srv.credCheck = func(_ context.Context, _ config.Backend, _ config.Model, _ map[string]string) credcheck.CredResult {
		return credcheck.CredResult{Status: "failed", Detail: "invalid_api_key"}
	}

	backends := config.BackendsConfig{
		Version: 2,
		Backends: map[string]config.Backend{
			"codex": {
				Type:         "codex-acp",
				Default:      true,
				DefaultModel: "gpt-4",
				Models:       map[string]config.Model{"gpt-4": {Model: "gpt-4"}},
			},
		},
	}
	srv.configStore.WriteBackends(backends)
	srv.configStore.WriteProject("p", config.Project{Title: "P", Cwd: "/tmp"})
	srv.configStore.WriteRole("r", config.Role{Title: "R"})

	h := srv.routes()
	rec := doGET(t, h, "/api/config")
	var resp configResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if resp.Onboarding.Steps.Backend.Done {
		t.Error("failed creds: backend step should not be done")
	}
	if resp.Onboarding.Satisfied {
		t.Error("failed creds: satisfied should be false")
	}
}

func TestGetConfigOnboardingCompleteOverridesGate(t *testing.T) {
	// Once onboarding_complete=true is set, satisfied=true even if backend creds fail.
	srv := testServer(t, false)
	srv.credCheck = func(_ context.Context, _ config.Backend, _ config.Model, _ map[string]string) credcheck.CredResult {
		return credcheck.CredResult{Status: "failed", Detail: "bad"}
	}

	cfg := config.DefaultConfig()
	cfg.OnboardingComplete = true
	srv.configStore.WriteConfig(cfg)

	h := srv.routes()
	rec := doGET(t, h, "/api/config")
	var resp configResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if !resp.Onboarding.Satisfied {
		t.Error("onboarding_complete=true: satisfied should be true regardless of cred state")
	}
	if !resp.OnboardingComplete {
		t.Error("onboarding_complete field should be true")
	}
}

func TestPutConfigMergesFields(t *testing.T) {
	srv := testServerWithOkCreds(t)
	h := srv.routes()

	trueVal := true
	project := "my-app"
	if err := srv.configStore.WriteProject(project, config.Project{Title: "My App", Cwd: "/tmp"}); err != nil {
		t.Fatalf("WriteProject: %v", err)
	}
	rec := doRequest(t, h, http.MethodPut, "/api/config", map[string]any{
		"onboarding_complete": true,
		"default_project":     project,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /api/config status = %d body=%s, want 200", rec.Code, rec.Body)
	}

	_ = trueVal
	cfg, err := srv.configStore.ReadConfig()
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	if !cfg.OnboardingComplete {
		t.Error("onboarding_complete not persisted")
	}
	if cfg.DefaultProject != "my-app" {
		t.Errorf("default_project = %q, want my-app", cfg.DefaultProject)
	}
}

// FS-04.A23 — the dependent-work runtime budget round-trips and invalid values
// cannot replace the durable setting.
func TestTaskConcurrencySettingRoundTripsAndRejectsNonPositive(t *testing.T) {
	srv := testServerWithOkCreds(t)
	h := srv.routes()

	rec := doGET(t, h, "/api/config")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/config status = %d", rec.Code)
	}
	var initial configResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &initial); err != nil {
		t.Fatal(err)
	}
	if initial.TaskConcurrency != 10 {
		t.Fatalf("default task_concurrency = %d, want 10", initial.TaskConcurrency)
	}

	rec = doRequest(t, h, http.MethodPut, "/api/config", map[string]any{"task_concurrency": 4})
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT task_concurrency status = %d body=%s", rec.Code, rec.Body)
	}
	got, err := srv.configStore.ReadConfig()
	if err != nil || got.TaskConcurrency != 4 {
		t.Fatalf("saved task_concurrency = %d, %v; want 4", got.TaskConcurrency, err)
	}

	rec = doRequest(t, h, http.MethodPut, "/api/config", map[string]any{"task_concurrency": 0})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PUT zero task_concurrency status = %d, want 400", rec.Code)
	}
	got, err = srv.configStore.ReadConfig()
	if err != nil || got.TaskConcurrency != 4 {
		t.Fatalf("task_concurrency after rejection = %d, %v; want 4", got.TaskConcurrency, err)
	}
}

func TestPutConfigRejectsImmutableFields(t *testing.T) {
	srv := testServerWithOkCreds(t)
	h := srv.routes()

	// Reject version change.
	rec := doRequest(t, h, http.MethodPut, "/api/config", map[string]any{"version": 2})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("version change status = %d, want 400", rec.Code)
	}

	// Reject port change.
	rec = doRequest(t, h, http.MethodPut, "/api/config", map[string]any{"port": 9000})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("port change status = %d, want 400", rec.Code)
	}
}

func TestPutConfigPersistsNotificationSettings(t *testing.T) {
	srv := testServer(t, true)
	h := srv.routes()
	rec := doRequest(t, h, http.MethodPut, "/api/config", map[string]any{
		"notifications": map[string]any{
			"desktop_enabled": false,
			"muted": map[string]bool{
				"done": true, "waiting_input": false, "permission_required": false, "budget_exceeded": true,
			},
		},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /api/config status = %d body=%s, want 200", rec.Code, rec.Body)
	}
	cfg, err := srv.configStore.ReadConfig()
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	if cfg.Notifications.DesktopEnabled || !cfg.Notifications.Muted["done"] || !cfg.Notifications.Muted["budget_exceeded"] {
		t.Fatalf("notifications = %+v", cfg.Notifications)
	}
}

func TestConfigAppearanceSkinRoundTripAndCoreOmission(t *testing.T) {
	srv := testServer(t, true)
	h := srv.routes()

	rec := doRequest(t, h, http.MethodPut, "/api/config", map[string]any{
		"appearance_skin": config.AppearanceSkinSkyGrove,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT Sky & Grove status = %d body=%s, want 200", rec.Code, rec.Body)
	}
	rec = doGET(t, h, "/api/config")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET Sky & Grove status = %d body=%s, want 200", rec.Code, rec.Body)
	}
	var got configResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.AppearanceSkin != config.AppearanceSkinSkyGrove || got.AppearanceSkinWarning != "" {
		t.Fatalf("GET Sky & Grove = skin %q warning %q", got.AppearanceSkin, got.AppearanceSkinWarning)
	}
	// Omitting the optional field from a later partial update preserves it.
	rec = doRequest(t, h, http.MethodPut, "/api/config", map[string]any{"onboarding_complete": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT omitting appearance status = %d body=%s, want 200", rec.Code, rec.Body)
	}
	preserved, err := srv.configStore.ReadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if preserved.AppearanceSkin != config.AppearanceSkinSkyGrove {
		t.Fatalf("appearance after omitted partial update = %q, want %q", preserved.AppearanceSkin, config.AppearanceSkinSkyGrove)
	}

	rec = doRequest(t, h, http.MethodPut, "/api/config", map[string]any{"appearance_skin": ""})
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT Core status = %d body=%s, want 200", rec.Code, rec.Body)
	}
	raw, err := os.ReadFile(filepath.Join(srv.configStore.Home(), "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if _, ok := doc["appearance_skin"]; ok {
		t.Fatalf("Core PUT retained appearance_skin: %s", raw)
	}
}

func TestPutConfigRejectsUnsupportedAppearanceWithoutWriting(t *testing.T) {
	srv := testServer(t, true)
	path := filepath.Join(srv.configStore.Home(), "config.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	rec := doRequest(t, srv.routes(), http.MethodPut, "/api/config", map[string]any{
		"appearance_skin":     "forest-night",
		"onboarding_complete": true,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PUT unsupported skin status = %d body=%s, want 400", rec.Code, rec.Body)
	}
	var body validationFailedBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Errors) != 1 || body.Errors[0].Field != "appearance_skin" {
		t.Fatalf("unsupported skin errors = %+v", body.Errors)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("invalid PUT changed config.json:\nbefore=%s\nafter=%s", before, after)
	}
}

func TestGetConfigAppearanceSkinWarnings(t *testing.T) {
	t.Run("unknown hand edit", func(t *testing.T) {
		srv := testServer(t, true)
		cfg, err := srv.configStore.ReadConfig()
		if err != nil {
			t.Fatal(err)
		}
		cfg.AppearanceSkin = "forest-night"
		if err := srv.configStore.WriteConfig(cfg); err != nil {
			t.Fatal(err)
		}

		rec := doGET(t, srv.routes(), "/api/config")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET status = %d body=%s, want 200", rec.Code, rec.Body)
		}
		var got configResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.AppearanceSkin != "forest-night" || got.AppearanceSkinWarning != "unsupported" {
			t.Fatalf("unknown hand edit = skin %q warning %q", got.AppearanceSkin, got.AppearanceSkinWarning)
		}
	})

	t.Run("corrupt config", func(t *testing.T) {
		srv := testServer(t, true)
		path := filepath.Join(srv.configStore.Home(), "config.json")
		if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}

		rec := doGET(t, srv.routes(), "/api/config")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET status = %d body=%s, want 200", rec.Code, rec.Body)
		}
		var got configResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.AppearanceSkin != "" || got.AppearanceSkinWarning != "config_unreadable" {
			t.Fatalf("corrupt config = skin %q warning %q", got.AppearanceSkin, got.AppearanceSkinWarning)
		}
		var doc map[string]json.RawMessage
		if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
			t.Fatal(err)
		}
		if _, ok := doc["appearance_skin"]; ok {
			t.Fatalf("corrupt fallback returned a persisted skin: %s", rec.Body.Bytes())
		}
	})
}

func TestPutConfigWriteFailurePreservesAppearanceChoice(t *testing.T) {
	srv := testServer(t, true)
	cfg, err := srv.configStore.ReadConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.AppearanceSkin = config.AppearanceSkinSkyGrove
	if err := srv.configStore.WriteConfig(cfg); err != nil {
		t.Fatal(err)
	}
	srv.writeConfig = func(config.Config) error { return errors.New("injected write failure") }

	rec := doRequest(t, srv.routes(), http.MethodPut, "/api/config", map[string]any{"appearance_skin": ""})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("PUT with failed write status = %d body=%s, want 500", rec.Code, rec.Body)
	}
	got, err := srv.configStore.ReadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got.AppearanceSkin != config.AppearanceSkinSkyGrove {
		t.Fatalf("appearance after failed Core write = %q, want %q", got.AppearanceSkin, config.AppearanceSkinSkyGrove)
	}
}
