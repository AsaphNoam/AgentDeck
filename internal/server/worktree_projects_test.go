package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentdeck/agentdeck/internal/config"
)

// FS-04.A25 / FS-04.R45: base_branch and setup_command round-trip through the
// project CRUD surface exactly like every other field.
func TestProjectBaseBranchAndSetupCommandRoundTrip(t *testing.T) {
	srv := testServer(t, false)
	h := srv.routes()

	rec := doRequest(t, h, http.MethodPost, "/api/projects", map[string]any{
		"project":       "billing",
		"title":         "Billing",
		"cwd":           "/tmp",
		"base_branch":   " develop ",
		"setup_command": " npm ci ",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST status = %d body=%s, want 201", rec.Code, rec.Body)
	}
	var created projectResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	if created.BaseBranch != "develop" || created.SetupCommand != "npm ci" {
		t.Fatalf("created = %+v, want trimmed develop/npm ci", created)
	}

	stored, err := srv.configStore.ReadProject("billing")
	if err != nil {
		t.Fatalf("ReadProject: %v", err)
	}
	if stored.BaseBranch != "develop" || stored.SetupCommand != "npm ci" {
		t.Fatalf("stored = %+v", stored)
	}

	rec = doRequest(t, h, http.MethodPut, "/api/projects/billing", map[string]any{
		"title":         "Billing",
		"cwd":           "/tmp",
		"base_branch":   "main",
		"setup_command": "",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT status = %d body=%s, want 200", rec.Code, rec.Body)
	}
	stored, err = srv.configStore.ReadProject("billing")
	if err != nil {
		t.Fatalf("ReadProject after PUT: %v", err)
	}
	if stored.BaseBranch != "main" || stored.SetupCommand != "" {
		t.Fatalf("stored after PUT = %+v", stored)
	}
}

// FS-04.A25: a project file written before these fields existed stays valid,
// reads as empty, and a save that does not touch them leaves the rest intact.
func TestLegacyProjectFileWithoutWorktreeFields(t *testing.T) {
	srv := testServer(t, false)
	h := srv.routes()

	legacy := []byte(`{"title":"Legacy","color":[10,20,30],"cwd":"/tmp","add_dirs":[],"context_prompt":"keep me","archived":false}`)
	path := filepath.Join(srv.configStore.Home(), "projects", "legacy.json")
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatalf("write legacy project: %v", err)
	}

	stored, err := srv.configStore.ReadProject("legacy")
	if err != nil {
		t.Fatalf("ReadProject: %v", err)
	}
	if stored.BaseBranch != "" || stored.SetupCommand != "" {
		t.Fatalf("legacy project = %+v, want empty worktree fields", stored)
	}

	rec := doRequest(t, h, http.MethodPut, "/api/projects/legacy", map[string]any{
		"title":          "Legacy",
		"color":          [3]int{10, 20, 30},
		"cwd":            "/tmp",
		"context_prompt": "keep me",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT status = %d body=%s, want 200", rec.Code, rec.Body)
	}
	saved, err := srv.configStore.ReadProject("legacy")
	if err != nil {
		t.Fatalf("ReadProject after PUT: %v", err)
	}
	want := config.Project{Title: "Legacy", Color: [3]int{10, 20, 30}, Cwd: "/tmp", AddDirs: []string{}, ContextPrompt: "keep me"}
	if saved.Title != want.Title || saved.Cwd != want.Cwd || saved.ContextPrompt != want.ContextPrompt ||
		saved.Color != want.Color || saved.BaseBranch != "" || saved.SetupCommand != "" {
		t.Fatalf("saved = %+v, want %+v", saved, want)
	}
}
