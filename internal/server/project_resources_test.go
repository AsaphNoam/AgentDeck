package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// httpGetBody performs a GET and returns the response plus its full body.
func httpGetBody(t *testing.T, url string) (*http.Response, []byte) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp, body
}

// resourcesEnvValue returns the value of AGENTDECK_PROJECT_RESOURCES in a composed
// env slice, or "" if absent.
func resourcesEnvValue(env []string) string {
	const key = envProjectResources + "="
	for _, kv := range env {
		if strings.HasPrefix(kv, key) {
			return strings.TrimPrefix(kv, key)
		}
	}
	return ""
}

// FS-11.A2: a launched agent's composed spec carries the canonical resource path
// in AGENTDECK_PROJECT_RESOURCES, in add_dirs, and in the composed instruction,
// while its cwd stays the project working directory.
func TestLaunchComposesProjectResources(t *testing.T) {
	srv, _ := switchTestServer(t)

	spec, _, ae := srv.composeLaunch(t.Context(), launchRequest{Role: "impl", Project: "tmpproj"})
	if ae != nil {
		t.Fatalf("composeLaunch: %s", ae.Message)
	}

	want := filepath.Join(srv.configStore.Home(), "project-resources", "tmpproj")
	if got := resourcesEnvValue(spec.Env); got != want {
		t.Errorf("AGENTDECK_PROJECT_RESOURCES = %q, want %q", got, want)
	}
	if !containsStr(spec.AddDirs, want) {
		t.Errorf("AddDirs = %v, missing resource path %q", spec.AddDirs, want)
	}
	if !strings.Contains(spec.SystemPrompt, want) ||
		!strings.Contains(spec.SystemPrompt, "Shared project resources:") {
		t.Errorf("SystemPrompt missing resource instruction: %q", spec.SystemPrompt)
	}
	if spec.Cwd == want {
		t.Errorf("cwd was set to the resource dir; it must stay the project cwd")
	}

	// The directory was actually created, owner-only (FS-11.A1).
	fi, err := os.Stat(want)
	if err != nil || !fi.IsDir() {
		t.Fatalf("resource dir not created: stat err=%v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0o700 {
		t.Errorf("resource dir mode = %o, want 0700", perm)
	}
}

// FS-11.R3/R7, INV §2: resume and switch re-add the env var, and the path they
// carry in add_dirs/prompt comes from the frozen snapshot (identical to launch).
func TestResumeAndSwitchCarryProjectResources(t *testing.T) {
	srv, ts := switchTestServer(t)
	id := launchAndWaitIdle(t, ts, "impl", "tmpproj")

	agent, err := srv.stateStore.ReadAgent(id)
	if err != nil {
		t.Fatalf("ReadAgent: %v", err)
	}
	snap, err := srv.stateStore.ReadSession(id)
	if err != nil {
		t.Fatalf("ReadSession: %v", err)
	}
	want := filepath.Join(srv.configStore.Home(), "project-resources", "tmpproj")
	if !containsStr(snap.AddDirs, want) {
		t.Fatalf("frozen snapshot AddDirs = %v, missing %q", snap.AddDirs, want)
	}
	if !strings.Contains(snap.SystemPrompt, want) {
		t.Fatalf("frozen snapshot SystemPrompt missing resource path: %q", snap.SystemPrompt)
	}

	backends, err := srv.configStore.ReadBackends()
	if err != nil {
		t.Fatalf("ReadBackends: %v", err)
	}
	be := backends.Backends[agent.Backend]
	model := be.Models[agent.Model]

	rspec, ae := srv.composeResumeSpec(agent, snap, be, model)
	if ae != nil {
		t.Fatalf("composeResumeSpec: %s", ae.Message)
	}
	if got := resourcesEnvValue(rspec.Env); got != want {
		t.Errorf("resume env AGENTDECK_PROJECT_RESOURCES = %q, want %q", got, want)
	}
	if !containsStr(rspec.AddDirs, want) {
		t.Errorf("resume AddDirs = %v, missing %q", rspec.AddDirs, want)
	}

	sspec, ae := srv.composeSwitchSpec(agent, "")
	if ae != nil {
		t.Fatalf("composeSwitchSpec: %s", ae.Message)
	}
	if got := resourcesEnvValue(sspec.Env); got != want {
		t.Errorf("switch env AGENTDECK_PROJECT_RESOURCES = %q, want %q", got, want)
	}
}

// FS-11.R6: a resource-creation failure fails the create request and leaves no
// project definition behind.
func TestProjectCreateFailsWhenResourcesUnusable(t *testing.T) {
	srv := testServer(t, true)
	ts := httptest.NewServer(srv.routes())
	t.Cleanup(ts.Close)

	// A regular file where the project-resources parent must be forces
	// EnsureProjectResources to fail on the "not a directory" check.
	if err := os.WriteFile(filepath.Join(srv.configStore.Home(), "project-resources"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write blocker file: %v", err)
	}

	resp, body := post(t, ts.URL+"/api/projects", map[string]any{
		"title": "Blocked", "cwd": t.TempDir(), "color": []int{1, 2, 3},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("create status = %d, want 400: %s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), "resource_dir_unavailable") {
		t.Fatalf("body missing resource_dir_unavailable code: %s", body)
	}

	// No project definition for the failed create was written.
	projects, err := srv.configStore.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	for id, p := range projects {
		if p.Title == "Blocked" {
			t.Fatalf("project %q was written despite resource failure: %+v", id, p)
		}
	}
}

// FS-11.R4/R5, TS-03.R12: project read/create responses expose the read-only
// resource_dir path, and (FS-11.R10/A4) the directory's contents are never leaked
// into the listing merely because the directory exists.
func TestProjectResponsesExposePathNotContents(t *testing.T) {
	srv := testServer(t, true)
	ts := httptest.NewServer(srv.routes())
	t.Cleanup(ts.Close)

	resp, body := post(t, ts.URL+"/api/projects", map[string]any{
		"title": "Demo", "cwd": t.TempDir(), "color": []int{1, 2, 3},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d: %s", resp.StatusCode, body)
	}
	var created projectResponse
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("unmarshal create: %v", err)
	}
	wantPath := filepath.Join(srv.configStore.Home(), "project-resources", created.ProjectID)
	if created.ResourceDir != wantPath {
		t.Fatalf("create resource_dir = %q, want %q", created.ResourceDir, wantPath)
	}

	// Drop a secret file INTO the resource directory; it must never appear in the
	// project listing payload (only the path is metadata; contents are opaque).
	const secret = "TOP-SECRET-RESOURCE-CONTENT"
	if err := os.WriteFile(filepath.Join(wantPath, "notes.md"), []byte(secret), 0o600); err != nil {
		t.Fatalf("write resource file: %v", err)
	}

	resp, body = httpGetBody(t, ts.URL+"/api/projects")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d: %s", resp.StatusCode, body)
	}
	var list map[string]projectResponse
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	got, ok := list[created.ProjectID]
	if !ok {
		t.Fatalf("project %q missing from listing", created.ProjectID)
	}
	if got.ResourceDir != wantPath {
		t.Errorf("list resource_dir = %q, want %q", got.ResourceDir, wantPath)
	}
	if strings.Contains(string(body), secret) || strings.Contains(string(body), "notes.md") {
		t.Fatalf("resource contents leaked into project listing: %s", body)
	}
}
