package server

import (
	"context"
	"encoding/json"
	"net/http"
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
