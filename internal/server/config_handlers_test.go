package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/backend/credcheck"
	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/state"
)

// doRequest fires a method+path against the handler with an optional JSON body.
func doRequest(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reqBody *bytes.Buffer
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reqBody = bytes.NewBuffer(b)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}
	req := newLocalRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// ---- Roles CRUD tests ----

func TestRolesCRUDRoundTrip(t *testing.T) {
	srv := testServer(t, false)
	h := srv.routes()

	// POST: create a role.
	rec := doRequest(t, h, http.MethodPost, "/api/roles", map[string]any{
		"role":             "security-reviewer",
		"title":            "Security Reviewer",
		"system_prompt":    "Audit for vulns.",
		"skip_permissions": false,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/roles status = %d body=%s, want 201", rec.Code, rec.Body)
	}
	var created roleResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("POST roles body: %v", err)
	}
	if created.RoleID != "security-reviewer" || created.Title != "Security Reviewer" {
		t.Fatalf("created role = %+v", created)
	}

	// GET: verify the role appears in the list.
	rec = doGET(t, h, "/api/roles")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/roles status = %d", rec.Code)
	}
	var roles map[string]config.Role
	if err := json.Unmarshal(rec.Body.Bytes(), &roles); err != nil {
		t.Fatalf("GET roles body: %v", err)
	}
	if _, ok := roles["security-reviewer"]; !ok {
		t.Fatalf("created role not in list: %v", roles)
	}

	// PUT: update the role.
	rec = doRequest(t, h, http.MethodPut, "/api/roles/security-reviewer", map[string]any{
		"title":         "Updated Reviewer",
		"system_prompt": "New prompt.",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /api/roles/security-reviewer status = %d body=%s, want 200", rec.Code, rec.Body)
	}
	var updated roleResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("PUT roles body: %v", err)
	}
	if updated.Title != "Updated Reviewer" {
		t.Fatalf("updated title = %q, want Updated Reviewer", updated.Title)
	}

	// GET again: verify the update persisted on disk.
	role, err := srv.configStore.ReadRole("security-reviewer")
	if err != nil {
		t.Fatalf("ReadRole: %v", err)
	}
	if role.Title != "Updated Reviewer" {
		t.Fatalf("disk title = %q, want Updated Reviewer", role.Title)
	}

	// DELETE.
	rec = doRequest(t, h, http.MethodDelete, "/api/roles/security-reviewer", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE /api/roles status = %d body=%s, want 204", rec.Code, rec.Body)
	}

	// GET after delete: must not appear.
	rec = doGET(t, h, "/api/roles")
	var rolesAfter map[string]config.Role
	if err := json.Unmarshal(rec.Body.Bytes(), &rolesAfter); err != nil {
		t.Fatalf("GET roles after delete: %v", err)
	}
	if _, ok := rolesAfter["security-reviewer"]; ok {
		t.Fatal("deleted role still in list")
	}
}

func TestRolesValidationFailures(t *testing.T) {
	srv := testServer(t, false)
	h := srv.routes()

	tests := []struct {
		name     string
		body     map[string]any
		wantCode int
		wantErr  string
	}{
		{
			name:     "invalid slug",
			body:     map[string]any{"role": "BAD SLUG", "title": "X"},
			wantCode: http.StatusBadRequest,
			wantErr:  "validation_failed",
		},
		{
			name:     "missing title",
			body:     map[string]any{"role": "good-slug"},
			wantCode: http.StatusBadRequest,
			wantErr:  "validation_failed",
		},
		{
			name:     "slug with dot",
			body:     map[string]any{"role": "a.b", "title": "X"},
			wantCode: http.StatusBadRequest,
			wantErr:  "validation_failed",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := doRequest(t, h, http.MethodPost, "/api/roles", tc.body)
			if rec.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d. body = %s", rec.Code, tc.wantCode, rec.Body)
			}
			var resp map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("body parse: %v", err)
			}
			if resp["error"] != tc.wantErr {
				t.Errorf("error = %q, want %q", resp["error"], tc.wantErr)
			}
		})
	}
}

func TestRolesAlreadyExists409(t *testing.T) {
	srv := testServer(t, false)
	h := srv.routes()
	body := map[string]any{"role": "my-role", "title": "X", "system_prompt": ""}
	doRequest(t, h, http.MethodPost, "/api/roles", body)
	rec := doRequest(t, h, http.MethodPost, "/api/roles", body)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate POST status = %d, want 409", rec.Code)
	}
}

func TestRolesPutNotFound404(t *testing.T) {
	srv := testServer(t, false)
	h := srv.routes()
	rec := doRequest(t, h, http.MethodPut, "/api/roles/nonexistent", map[string]any{"title": "X", "system_prompt": ""})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("PUT missing role status = %d, want 404", rec.Code)
	}
}

func TestRolesDeleteNotFound404(t *testing.T) {
	srv := testServer(t, false)
	h := srv.routes()
	rec := doRequest(t, h, http.MethodDelete, "/api/roles/nonexistent", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("DELETE missing role status = %d, want 404", rec.Code)
	}
}

// TestPathTraversalRejected ensures PUT and DELETE on roles/projects reject path ids
// that would escape the config subdirectory (dots, slashes, encoded sequences).
func TestPathTraversalRejected(t *testing.T) {
	srv := testServer(t, false)
	h := srv.routes()

	invalidIDs := []string{
		// Go ServeMux decodes percent-encoded path values, so these arrive decoded.
		// We only include IDs that are valid HTTP path segments (no raw spaces/empties)
		// but fail ValidSlug.
		"..",
		"../config",
		"UPPERCASE",
	}
	body := map[string]any{"title": "X", "system_prompt": ""}

	for _, id := range invalidIDs {
		id := id
		t.Run("role-PUT-"+id, func(t *testing.T) {
			rec := doRequest(t, h, http.MethodPut, "/api/roles/"+id, body)
			// Either 400 (invalid slug) or 404/405 from the mux rejecting the path.
			if rec.Code == http.StatusNoContent || rec.Code == http.StatusOK {
				t.Errorf("PUT /api/roles/%q: expected non-2xx, got %d", id, rec.Code)
			}
		})
		t.Run("role-DELETE-"+id, func(t *testing.T) {
			rec := doRequest(t, h, http.MethodDelete, "/api/roles/"+id, nil)
			if rec.Code == http.StatusNoContent || rec.Code == http.StatusOK {
				t.Errorf("DELETE /api/roles/%q: expected non-2xx, got %d", id, rec.Code)
			}
		})
	}

	projBody := map[string]any{"title": "X", "cwd": "/tmp", "add_dirs": []string{}, "context_prompt": ""}
	for _, id := range invalidIDs {
		id := id
		t.Run("project-PUT-"+id, func(t *testing.T) {
			rec := doRequest(t, h, http.MethodPut, "/api/projects/"+id, projBody)
			if rec.Code == http.StatusNoContent || rec.Code == http.StatusOK {
				t.Errorf("PUT /api/projects/%q: expected non-2xx, got %d", id, rec.Code)
			}
		})
		t.Run("project-DELETE-"+id, func(t *testing.T) {
			rec := doRequest(t, h, http.MethodDelete, "/api/projects/"+id, nil)
			if rec.Code == http.StatusNoContent || rec.Code == http.StatusOK {
				t.Errorf("DELETE /api/projects/%q: expected non-2xx, got %d", id, rec.Code)
			}
		})
	}
}

// TestPathTraversalEncodedDots explicitly tests that percent-encoded dots in path IDs
// are rejected. Go ServeMux decodes path values, so %2e%2e becomes ".." before
// our handler sees it — ValidSlug catches it there.
func TestPathTraversalEncodedDots(t *testing.T) {
	srv := testServer(t, false)
	h := srv.routes()
	body := map[string]any{"title": "X", "system_prompt": ""}

	// These percent-encoded values decode to ".." or "../config" after ServeMux processing.
	// The mux itself may 301/404 these before our handler if they contain actual slashes,
	// but pure dot segments still reach our handler decoded.
	for _, raw := range []string{"%2e%2e", "%2E%2E"} {
		rec := doRequest(t, h, http.MethodPut, "/api/roles/"+raw, body)
		if rec.Code == http.StatusOK {
			t.Errorf("PUT /api/roles/%s returned 200 — path traversal not blocked", raw)
		}
	}
	for _, raw := range []string{"%2e%2e", "%2E%2E"} {
		projBody := map[string]any{"title": "X", "cwd": "/tmp", "add_dirs": []string{}, "context_prompt": ""}
		rec := doRequest(t, h, http.MethodPut, "/api/projects/"+raw, projBody)
		if rec.Code == http.StatusOK {
			t.Errorf("PUT /api/projects/%s returned 200 — path traversal not blocked", raw)
		}
	}
}

func TestRolesInUseGuard(t *testing.T) {
	srv := testServer(t, false)
	h := srv.routes()

	// Create a role.
	doRequest(t, h, http.MethodPost, "/api/roles", map[string]any{
		"role": "busy-role", "title": "Busy", "system_prompt": "",
	})

	// Seed a running agent that references this role.
	agentID := seedRunningAgentWithRole(t, srv, "busy-role")

	// DELETE without force → 409.
	rec := doRequest(t, h, http.MethodDelete, "/api/roles/busy-role", nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("DELETE in-use status = %d body=%s, want 409", rec.Code, rec.Body)
	}
	var resp inUseBody
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("409 body parse: %v", err)
	}
	if resp.Error != "in_use" {
		t.Errorf("error = %q, want in_use", resp.Error)
	}
	if len(resp.Agents) == 0 || resp.Agents[0] != agentID {
		t.Errorf("agents = %v, want [%s]", resp.Agents, agentID)
	}

	// DELETE with ?force=true → 204.
	rec = doRequest(t, h, http.MethodDelete, "/api/roles/busy-role?force=true", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE force status = %d body=%s, want 204", rec.Code, rec.Body)
	}
}

// ---- Projects CRUD tests ----

func TestProjectsCRUDRoundTrip(t *testing.T) {
	srv := testServer(t, false)
	h := srv.routes()

	// POST: create a project.
	rec := doRequest(t, h, http.MethodPost, "/api/projects", map[string]any{
		"project":        "billing",
		"title":          "Billing",
		"color":          [3]int{200, 120, 60},
		"cwd":            "/tmp",
		"add_dirs":       []string{},
		"context_prompt": "Stripe-backed.",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/projects status = %d body=%s, want 201", rec.Code, rec.Body)
	}
	var created projectResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("POST projects body: %v", err)
	}
	if created.ProjectID != "billing" || created.Title != "Billing" {
		t.Fatalf("created project = %+v", created)
	}

	// GET: verify in list.
	rec = doGET(t, h, "/api/projects")
	var projects map[string]config.Project
	if err := json.Unmarshal(rec.Body.Bytes(), &projects); err != nil {
		t.Fatalf("GET projects: %v", err)
	}
	if _, ok := projects["billing"]; !ok {
		t.Fatalf("created project not in list: %v", projects)
	}

	// PUT: update.
	rec = doRequest(t, h, http.MethodPut, "/api/projects/billing", map[string]any{
		"title": "Billing Updated",
		"cwd":   "/tmp",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /api/projects/billing status = %d body=%s, want 200", rec.Code, rec.Body)
	}
	proj, err := srv.configStore.ReadProject("billing")
	if err != nil {
		t.Fatalf("ReadProject: %v", err)
	}
	if proj.Title != "Billing Updated" {
		t.Fatalf("disk title = %q, want Billing Updated", proj.Title)
	}

	// DELETE.
	rec = doRequest(t, h, http.MethodDelete, "/api/projects/billing", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE /api/projects status = %d, want 204", rec.Code)
	}

	// Verify gone from list.
	rec = doGET(t, h, "/api/projects")
	var projectsAfter map[string]config.Project
	json.Unmarshal(rec.Body.Bytes(), &projectsAfter)
	if _, ok := projectsAfter["billing"]; ok {
		t.Fatal("deleted project still in list")
	}
}

func TestProjectsValidationFailures(t *testing.T) {
	srv := testServer(t, false)
	h := srv.routes()

	tests := []struct {
		name string
		body map[string]any
	}{
		{"invalid slug", map[string]any{"project": "BAD!", "title": "X", "cwd": "/tmp"}},
		{"missing title", map[string]any{"project": "p", "cwd": "/tmp"}},
		{"missing cwd", map[string]any{"project": "p", "title": "X"}},
		{"color out of range", map[string]any{"project": "p", "title": "X", "cwd": "/tmp", "color": [3]int{0, 300, 0}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := doRequest(t, h, http.MethodPost, "/api/projects", tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400. body = %s", rec.Code, rec.Body)
			}
			var resp map[string]any
			json.Unmarshal(rec.Body.Bytes(), &resp)
			if resp["error"] != "validation_failed" {
				t.Errorf("error = %q, want validation_failed", resp["error"])
			}
		})
	}
}

func TestProjectsCwdNotFoundIsWarningNotError(t *testing.T) {
	srv := testServer(t, false)
	h := srv.routes()

	rec := doRequest(t, h, http.MethodPost, "/api/projects", map[string]any{
		"project": "phantom",
		"title":   "Phantom",
		"cwd":     "/nonexistent-999/xyz",
	})
	// Must be 201 (save succeeds) not 400.
	if rec.Code != http.StatusCreated {
		t.Fatalf("cwd_not_found status = %d body=%s, want 201", rec.Code, rec.Body)
	}
	var resp projectResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("body: %v", err)
	}
	found := false
	for _, w := range resp.Warnings {
		if w.Code == "cwd_not_found" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected cwd_not_found warning, got warnings=%v", resp.Warnings)
	}
	// Verify it was persisted on disk.
	if _, err := srv.configStore.ReadProject("phantom"); err != nil {
		t.Fatalf("project not persisted: %v", err)
	}
}

func TestProjectsInUseGuard(t *testing.T) {
	srv := testServer(t, false)
	h := srv.routes()

	doRequest(t, h, http.MethodPost, "/api/projects", map[string]any{
		"project": "busy-proj", "title": "Busy", "cwd": "/tmp",
	})
	agentID := seedRunningAgentWithProject(t, srv, "busy-proj")

	// DELETE without force → 409.
	rec := doRequest(t, h, http.MethodDelete, "/api/projects/busy-proj", nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("DELETE in-use status = %d body=%s, want 409", rec.Code, rec.Body)
	}
	var resp inUseBody
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Error != "in_use" {
		t.Errorf("error = %q, want in_use", resp.Error)
	}
	if len(resp.Agents) == 0 || resp.Agents[0] != agentID {
		t.Errorf("agents = %v, want [%s]", resp.Agents, agentID)
	}

	// DELETE with force → 204.
	rec = doRequest(t, h, http.MethodDelete, "/api/projects/busy-proj?force=true", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE force status = %d, want 204", rec.Code)
	}
}

// TestSelectableWithoutRestart verifies that a role created via POST is immediately
// returned by GET /api/roles in the same server process (disk-on-demand, §3.4).
func TestSelectableWithoutRestart(t *testing.T) {
	srv := testServer(t, false)
	h := srv.routes()

	doRequest(t, h, http.MethodPost, "/api/roles", map[string]any{
		"role": "fresh-role", "title": "Fresh", "system_prompt": "",
	})
	rec := doGET(t, h, "/api/roles")
	var roles map[string]config.Role
	json.Unmarshal(rec.Body.Bytes(), &roles)
	if _, ok := roles["fresh-role"]; !ok {
		t.Fatal("freshly created role not returned by GET without restart")
	}

	doRequest(t, h, http.MethodPost, "/api/projects", map[string]any{
		"project": "fresh-proj", "title": "Fresh", "cwd": "/tmp",
	})
	rec = doGET(t, h, "/api/projects")
	var projects map[string]config.Project
	json.Unmarshal(rec.Body.Bytes(), &projects)
	if _, ok := projects["fresh-proj"]; !ok {
		t.Fatal("freshly created project not returned by GET without restart")
	}
}

// ---- Backends PUT tests ----

// testServerWithCredCheck builds a test server with an injected credCheck stub.
func testServerWithCredCheck(t *testing.T, result credcheck.CredResult) *Server {
	t.Helper()
	srv := testServer(t, false)
	srv.credCheck = func(_ context.Context, _ config.Backend, _ config.Model, _ map[string]string) credcheck.CredResult {
		return result
	}
	return srv
}

func validBackendsBody() map[string]any {
	return map[string]any{
		"version": 2,
		"backends": map[string]any{
			"claude": map[string]any{
				"name":          "Claude",
				"type":          "claude-acp",
				"default":       true,
				"default_model": "default",
				"models": map[string]any{
					"default": map[string]any{
						"name":  "Default",
						"model": "claude-sonnet-4-6",
					},
				},
			},
		},
	}
}

func TestPutBackendsValid(t *testing.T) {
	srv := testServerWithCredCheck(t, credcheck.CredResult{Status: "ok"})
	h := srv.routes()
	rec := doRequest(t, h, http.MethodPut, "/api/backends", validBackendsBody())
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /api/backends status = %d body=%s, want 200", rec.Code, rec.Body)
	}
	var resp backendsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("body: %v", err)
	}
	if resp.Version != 2 {
		t.Errorf("version = %d, want 2", resp.Version)
	}
	if resp.Credentials["claude"].Status != "ok" {
		t.Errorf("cred status = %q, want ok", resp.Credentials["claude"].Status)
	}
	// Verify document persisted on disk.
	stored, err := srv.configStore.ReadBackends()
	if err != nil {
		t.Fatalf("ReadBackends: %v", err)
	}
	if stored.Version != 2 {
		t.Errorf("stored version = %d, want 2", stored.Version)
	}
}

func TestPutBackendsAutoPromoteDefault(t *testing.T) {
	srv := testServerWithCredCheck(t, credcheck.CredResult{Status: "ok"})
	h := srv.routes()
	// Send a body with no default backend set.
	body := map[string]any{
		"version": 2,
		"backends": map[string]any{
			"claude": map[string]any{
				"name":          "Claude",
				"type":          "claude-acp",
				"default":       false, // no default
				"default_model": "default",
				"models": map[string]any{
					"default": map[string]any{"name": "D", "model": "m"},
				},
			},
		},
	}
	rec := doRequest(t, h, http.MethodPut, "/api/backends", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT status = %d body=%s", rec.Code, rec.Body)
	}
	var resp backendsResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	// Auto-promoted: the returned doc must have Default=true.
	if !resp.Backends["claude"].Default {
		t.Error("claude not auto-promoted to default in response")
	}
}

func TestPutBackendsUnsupportedVersion400NothingWritten(t *testing.T) {
	srv := testServerWithCredCheck(t, credcheck.CredResult{Status: "ok"})
	h := srv.routes()
	body := validBackendsBody()
	body["version"] = 1 // wrong version
	rec := doRequest(t, h, http.MethodPut, "/api/backends", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("version=1 status = %d, want 400", rec.Code)
	}
	// Verify nothing was written to disk.
	if _, err := srv.configStore.ReadBackends(); err == nil {
		t.Error("backends file should not exist after a 400")
	}
}

func TestPutBackendsMultipleDefaults400(t *testing.T) {
	srv := testServerWithCredCheck(t, credcheck.CredResult{Status: "ok"})
	h := srv.routes()
	body := map[string]any{
		"version": 2,
		"backends": map[string]any{
			"a": map[string]any{
				"type": "claude-acp", "default": true,
				"default_model": "m", "models": map[string]any{"m": map[string]any{"model": "x"}},
			},
			"b": map[string]any{
				"type": "codex-acp", "default": true,
				"default_model": "m", "models": map[string]any{"m": map[string]any{"model": "x"}},
			},
		},
	}
	rec := doRequest(t, h, http.MethodPut, "/api/backends", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("multiple-defaults status = %d, want 400", rec.Code)
	}
}

func TestPutBackendsFailedCredCheckStillPersists(t *testing.T) {
	// A 200 with failed creds means the bytes are saved — the UI shouldn't lose user edits.
	srv := testServerWithCredCheck(t, credcheck.CredResult{Status: "failed", Detail: "invalid_api_key"})
	h := srv.routes()
	rec := doRequest(t, h, http.MethodPut, "/api/backends", validBackendsBody())
	if rec.Code != http.StatusOK {
		t.Fatalf("failed-creds status = %d, want 200", rec.Code)
	}
	var resp backendsResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Credentials["claude"].Status != "failed" {
		t.Errorf("cred status = %q, want failed", resp.Credentials["claude"].Status)
	}
	// Must be persisted.
	if _, err := srv.configStore.ReadBackends(); err != nil {
		t.Errorf("backends not persisted on failed cred-check: %v", err)
	}
}

// ---- Helpers ----

// seedRunningAgentWithRole creates agent+running entries in the state store
// where the agent references the given role. Returns the agent ID.
func seedRunningAgentWithRole(t *testing.T, srv *Server, role string) string {
	t.Helper()
	return seedRunningAgentWith(t, srv, role, "")
}

func seedRunningAgentWithProject(t *testing.T, srv *Server, project string) string {
	t.Helper()
	return seedRunningAgentWith(t, srv, "", project)
}

func seedRunningAgentWith(t *testing.T, srv *Server, role, project string) string {
	t.Helper()
	id := fmt.Sprintf("a_%s", role+project)
	agent := state.Agent{
		AgentID:   id,
		Name:      "test-agent",
		Role:      role,
		Project:   project,
		Backend:   "claude",
		Model:     "default",
		Interface: "chat",
		CreatedAt: time.Now(),
	}
	if err := srv.stateStore.WriteAgent(agent); err != nil {
		t.Fatalf("WriteAgent: %v", err)
	}
	running := state.RunningEntry{
		AgentID:   id,
		PID:       12345,
		SessionID: "sess-test",
		Interface: "chat",
		StartedAt: time.Now(),
	}
	if err := srv.stateStore.WriteRunning(running); err != nil {
		t.Fatalf("WriteRunning: %v", err)
	}
	return id
}
