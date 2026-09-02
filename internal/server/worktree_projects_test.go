package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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

// ---- Worktree fork, recreation, and deletion (FS-19) ----

// newWorktreeTestRepo makes a real one-commit Git repository and returns its
// path. The whole feature is Git behavior, so these cases run against the
// installed binary rather than a stub.
func newWorktreeTestRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello\n"), 0o600); err != nil {
		t.Fatalf("write README: %v", err)
	}
	run("add", "README.md")
	run("commit", "-m", "initial")
	return dir
}

// seedWorktreeSource writes a project whose cwd is repo and returns the server.
func seedWorktreeSource(t *testing.T, srv *Server, id, repo string, project config.Project) {
	t.Helper()
	project.Title = project.Title + ""
	project.Cwd = repo
	if project.AddDirs == nil {
		project.AddDirs = []string{}
	}
	if err := srv.configStore.WriteProject(id, project); err != nil {
		t.Fatalf("WriteProject: %v", err)
	}
}

func forkProject(t *testing.T, srv *Server, sourceID string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	return doRequest(t, srv.routes(), http.MethodPost, "/api/projects/"+sourceID+"/worktree-fork", body)
}

// FS-19.A1 / A7: the fork creates branch + owned checkout + a project copying
// the source fields, the checkout is real, and two sibling forks off the same
// base get distinct branches and checkouts.
func TestWorktreeForkCreatesBranchCheckoutAndProject(t *testing.T) {
	repo := newWorktreeTestRepo(t)
	srv := testServer(t, false)
	seedWorktreeSource(t, srv, "app", repo, config.Project{
		Title: "App", Color: [3]int{10, 20, 30}, ContextPrompt: "shared context",
		AddDirs: []string{"/tmp/extra"},
	})

	rec := forkProject(t, srv, "app", map[string]any{"title": "App fork one", "branch": "agentdeck/fork-one"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("fork status = %d body=%s, want 201", rec.Code, rec.Body)
	}
	var first forkResult
	if err := json.Unmarshal(rec.Body.Bytes(), &first); err != nil {
		t.Fatalf("decode fork: %v", err)
	}
	if first.Branch != "agentdeck/fork-one" || first.Base != "main" {
		t.Fatalf("fork = %+v, want branch agentdeck/fork-one off main", first)
	}
	// The copied fields come from the source; only the cwd differs.
	fork, err := srv.configStore.ReadProject(first.Project.ProjectID)
	if err != nil {
		t.Fatalf("ReadProject(fork): %v", err)
	}
	if fork.Color != [3]int{10, 20, 30} || fork.ContextPrompt != "shared context" ||
		len(fork.AddDirs) != 1 || fork.AddDirs[0] != "/tmp/extra" {
		t.Fatalf("fork project = %+v, want the source's color, prompt, and add_dirs", fork)
	}
	if _, err := os.Stat(filepath.Join(fork.Cwd, "README.md")); err != nil {
		t.Fatalf("checkout content missing: %v", err)
	}
	want, err := srv.configStore.WorktreePath(first.Project.ProjectID)
	if err != nil {
		t.Fatalf("WorktreePath: %v", err)
	}
	if fork.Cwd != want {
		t.Fatalf("fork cwd = %q, want the owned location %q", fork.Cwd, want)
	}
	// Ownership is recorded, and the response already carries the branch so the
	// new card can render it without a second round trip (FS-02.R60).
	row, owned := srv.ownedWorktree(first.Project.ProjectID)
	if !owned || row.Branch != "agentdeck/fork-one" || row.RepoPath == "" {
		t.Fatalf("ownership row = %+v owned=%v", row, owned)
	}
	if first.Project.Worktree == nil || first.Project.Worktree.Branch != "agentdeck/fork-one" {
		t.Fatalf("fork response worktree = %+v", first.Project.Worktree)
	}

	// A sibling fork branches off the same base, not off the first fork.
	rec = forkProject(t, srv, "app", map[string]any{"title": "App fork two", "branch": "agentdeck/fork-two"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("second fork status = %d body=%s, want 201", rec.Code, rec.Body)
	}
	var second forkResult
	if err := json.Unmarshal(rec.Body.Bytes(), &second); err != nil {
		t.Fatalf("decode second fork: %v", err)
	}
	if second.Base != "main" {
		t.Fatalf("second fork base = %q, want main", second.Base)
	}
	if second.Project.Cwd == first.Project.Cwd {
		t.Fatalf("sibling forks share a checkout: %q", second.Project.Cwd)
	}
}

// FS-19.R11: forking a worktree project still branches off the effective base,
// not off the source project's own branch.
func TestWorktreeForkOfAForkUsesTheEffectiveBase(t *testing.T) {
	repo := newWorktreeTestRepo(t)
	srv := testServer(t, false)
	seedWorktreeSource(t, srv, "app", repo, config.Project{Title: "App"})

	rec := forkProject(t, srv, "app", map[string]any{"title": "First", "branch": "agentdeck/first"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("fork status = %d body=%s", rec.Code, rec.Body)
	}
	var first forkResult
	_ = json.Unmarshal(rec.Body.Bytes(), &first)

	rec = forkProject(t, srv, first.Project.ProjectID, map[string]any{"title": "Second", "branch": "agentdeck/second"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("nested fork status = %d body=%s", rec.Code, rec.Body)
	}
	var second forkResult
	_ = json.Unmarshal(rec.Body.Bytes(), &second)
	if second.Base != "main" {
		t.Fatalf("nested fork base = %q, want main (never the source's own branch)", second.Base)
	}
}

// FS-19.A2 / R3: a failing setup command still yields a launchable project and
// a visible warning carrying the captured output.
func TestWorktreeForkFailingSetupStillCreatesTheProject(t *testing.T) {
	repo := newWorktreeTestRepo(t)
	srv := testServer(t, false)
	seedWorktreeSource(t, srv, "app", repo, config.Project{
		Title: "App", SetupCommand: "echo bootstrapping; exit 3",
	})

	rec := forkProject(t, srv, "app", map[string]any{"title": "Fork", "branch": "agentdeck/setup-fails"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("fork status = %d body=%s, want 201", rec.Code, rec.Body)
	}
	var result forkResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode fork: %v", err)
	}
	if !strings.Contains(result.Warning, "setup command failed") || !strings.Contains(result.Warning, "bootstrapping") {
		t.Fatalf("warning = %q, want the failure and its captured output", result.Warning)
	}
	if _, err := srv.configStore.ReadProject(result.Project.ProjectID); err != nil {
		t.Fatalf("project was undone by a failing setup: %v", err)
	}
	if !isExistingDir(result.Project.Cwd) {
		t.Fatal("checkout was undone by a failing setup")
	}
	// The outcome survives the request on the ownership row (TS-12.R5).
	row, _ := srv.ownedWorktree(result.Project.ProjectID)
	if row.SetupOK == nil || *row.SetupOK {
		t.Fatalf("setup_ok = %v, want false", row.SetupOK)
	}
}

// FS-19.A6 / R9 / R10: a non-repo project offers no fork, and a colliding
// branch fails actionably leaving no project, branch, or directory behind.
func TestWorktreeForkRejectionsLeaveNothingBehind(t *testing.T) {
	repo := newWorktreeTestRepo(t)
	srv := testServer(t, false)
	seedWorktreeSource(t, srv, "app", repo, config.Project{Title: "App"})
	seedWorktreeSource(t, srv, "plain", t.TempDir(), config.Project{Title: "Plain"})

	rec := forkProject(t, srv, "plain", map[string]any{"title": "Nope", "branch": "agentdeck/nope"})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("non-repo fork status = %d body=%s, want 422", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "not inside a Git working tree") {
		t.Fatalf("non-repo body = %s", rec.Body)
	}

	rec = forkProject(t, srv, "app", map[string]any{"title": "Taken", "branch": "agentdeck/taken"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("first fork status = %d body=%s", rec.Code, rec.Body)
	}
	before, err := srv.configStore.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}

	rec = forkProject(t, srv, "app", map[string]any{"title": "Taken again", "branch": "agentdeck/taken"})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("colliding fork status = %d body=%s, want 422", rec.Code, rec.Body)
	}
	after, err := srv.configStore.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects after: %v", err)
	}
	if len(after) != len(before) {
		t.Fatalf("colliding fork left %d projects, want %d", len(after), len(before))
	}
	entries, err := os.ReadDir(srv.configStore.WorktreesRoot())
	if err != nil {
		t.Fatalf("ReadDir worktrees: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("worktrees root holds %d entries, want only the successful fork", len(entries))
	}
}

// An archived source cannot be forked (FS-19.R9, TS-12 §3).
func TestWorktreeForkRejectsArchivedSource(t *testing.T) {
	repo := newWorktreeTestRepo(t)
	srv := testServer(t, false)
	seedWorktreeSource(t, srv, "app", repo, config.Project{Title: "App", Archived: true})

	rec := forkProject(t, srv, "app", map[string]any{"title": "Fork", "branch": "agentdeck/x"})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d body=%s, want 422", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "archived") {
		t.Fatalf("body = %s", rec.Body)
	}
}

// FS-19.A3 / R7, TS-12.R4: with the owned checkout deleted out of band, the
// shared start step recreates it on the recorded branch and re-runs setup; with
// the branch also gone the start fails with an actionable error and creates
// nothing.
func TestWorktreeCheckoutRecreationAndMissingBranch(t *testing.T) {
	repo := newWorktreeTestRepo(t)
	srv := testServer(t, false)
	seedWorktreeSource(t, srv, "app", repo, config.Project{
		Title: "App", SetupCommand: "echo setup-ran > setup-marker.txt",
	})

	rec := forkProject(t, srv, "app", map[string]any{"title": "Fork", "branch": "agentdeck/recreate"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("fork status = %d body=%s", rec.Code, rec.Body)
	}
	var result forkResult
	_ = json.Unmarshal(rec.Body.Bytes(), &result)
	forkID, checkout := result.Project.ProjectID, result.Project.Cwd

	if err := os.RemoveAll(checkout); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}
	recreated, warning, ae := srv.ensureWorktreeCheckout(t.Context(), forkID, checkout)
	if ae != nil {
		t.Fatalf("ensureWorktreeCheckout: %v", ae)
	}
	if !recreated {
		t.Fatal("recreation was not reported")
	}
	if warning != "" {
		t.Fatalf("warning = %q, want none for a succeeding setup", warning)
	}
	if _, err := os.Stat(filepath.Join(checkout, "README.md")); err != nil {
		t.Fatalf("recreated checkout content missing: %v", err)
	}
	// Setup ran again inside the fresh checkout (FS-19.R7).
	if _, err := os.Stat(filepath.Join(checkout, "setup-marker.txt")); err != nil {
		t.Fatalf("setup did not re-run on recreation: %v", err)
	}
	// A present checkout is left completely alone.
	recreated, _, ae = srv.ensureWorktreeCheckout(t.Context(), forkID, checkout)
	if ae != nil || recreated {
		t.Fatalf("second call recreated=%v ae=%v, want a no-op", recreated, ae)
	}

	// Branch gone as well: the start fails and never substitutes the base.
	if err := os.RemoveAll(checkout); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}
	git := srv.worktreeGit()
	if err := git.PruneWorktrees(t.Context(), repo); err != nil {
		t.Fatalf("PruneWorktrees: %v", err)
	}
	if err := git.DeleteBranch(t.Context(), repo, "agentdeck/recreate"); err != nil {
		t.Fatalf("DeleteBranch: %v", err)
	}
	_, _, ae = srv.ensureWorktreeCheckout(t.Context(), forkID, checkout)
	if ae == nil {
		t.Fatal("start succeeded with the recorded branch deleted")
	}
	if !strings.Contains(ae.Message, "agentdeck/recreate") {
		t.Fatalf("error = %q, want it to name the missing branch", ae.Message)
	}
	if isExistingDir(checkout) {
		t.Fatal("a failed start created a checkout")
	}
}

// A project that owns nothing keeps the plain missing-directory error, so an
// ordinary misconfigured project is still diagnosable (INV §2 — one step, same
// message).
func TestEnsureWorktreeCheckoutOnUnownedProject(t *testing.T) {
	srv := testServer(t, false)
	_, _, ae := srv.ensureWorktreeCheckout(t.Context(), "plain", "/no/such/dir")
	if ae == nil || !strings.Contains(ae.Message, "does not exist") {
		t.Fatalf("ae = %v, want the missing-directory error", ae)
	}
}

// FS-19.A4 / R8, TS-12.R7: archiving with consent removes the checkout and its
// registration while the branch and commits survive; declining keeps it.
func TestArchiveDeletesCheckoutOnlyWithConsent(t *testing.T) {
	repo := newWorktreeTestRepo(t)
	srv := testServer(t, false)
	seedWorktreeSource(t, srv, "app", repo, config.Project{Title: "App"})

	rec := forkProject(t, srv, "app", map[string]any{"title": "Fork", "branch": "agentdeck/archive-me"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("fork status = %d body=%s", rec.Code, rec.Body)
	}
	var result forkResult
	_ = json.Unmarshal(rec.Body.Bytes(), &result)
	forkID, checkout := result.Project.ProjectID, result.Project.Cwd

	// Declining keeps the checkout: the absent field never deletes.
	rec = doRequest(t, srv.routes(), http.MethodPost, "/api/projects/"+forkID+"/archive", map[string]any{})
	if rec.Code != http.StatusOK {
		t.Fatalf("archive status = %d body=%s", rec.Code, rec.Body)
	}
	if !isExistingDir(checkout) {
		t.Fatal("archiving without consent deleted the checkout")
	}
	if _, owned := srv.ownedWorktree(forkID); !owned {
		t.Fatal("ownership row was dropped without consent")
	}

	rec = doRequest(t, srv.routes(), http.MethodPost, "/api/projects/"+forkID+"/restore", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("restore status = %d body=%s", rec.Code, rec.Body)
	}
	rec = doRequest(t, srv.routes(), http.MethodPost, "/api/projects/"+forkID+"/archive",
		map[string]any{"delete_checkout": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("consented archive status = %d body=%s", rec.Code, rec.Body)
	}
	var archived archiveProjectActionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &archived); err != nil {
		t.Fatalf("decode archive: %v", err)
	}
	if !archived.CheckoutDeleted || archived.CheckoutWarning != "" {
		t.Fatalf("archive response = %+v, want a clean deletion", archived)
	}
	if isExistingDir(checkout) {
		t.Fatal("consented archive left the checkout in place")
	}
	if _, owned := srv.ownedWorktree(forkID); owned {
		t.Fatal("ownership row survived the deletion")
	}
	// The branch and its commits survive in the source repository.
	exists, err := srv.worktreeGit().BranchExists(t.Context(), repo, "agentdeck/archive-me")
	if err != nil {
		t.Fatalf("BranchExists: %v", err)
	}
	if !exists {
		t.Fatal("the branch was deleted along with its checkout")
	}
}

// FS-19.A5 / R4: a project whose cwd is a worktree AgentDeck did not create is
// external — no ownership row, no deletion, and consent is a no-op.
func TestExternalCheckoutIsNeverDeleted(t *testing.T) {
	repo := newWorktreeTestRepo(t)
	srv := testServer(t, false)
	external := filepath.Join(t.TempDir(), "user-worktree")
	if err := srv.worktreeGit().AddWorktree(t.Context(), repo, external, "mine", "main"); err != nil {
		t.Fatalf("AddWorktree: %v", err)
	}
	seedWorktreeSource(t, srv, "ext", external, config.Project{Title: "External"})

	if _, owned := srv.ownedWorktree("ext"); owned {
		t.Fatal("an external checkout was recorded as owned")
	}
	rec := doRequest(t, srv.routes(), http.MethodPost, "/api/projects/ext/archive",
		map[string]any{"delete_checkout": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("archive status = %d body=%s", rec.Code, rec.Body)
	}
	var archived archiveProjectActionResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &archived)
	if archived.CheckoutDeleted {
		t.Fatal("consent deleted an external checkout")
	}
	if !isExistingDir(external) {
		t.Fatal("the external checkout was removed")
	}

	rec = doRequest(t, srv.routes(), http.MethodDelete, "/api/projects/ext?delete_checkout=true", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d body=%s, want 204", rec.Code, rec.Body)
	}
	if !isExistingDir(external) {
		t.Fatal("deleting the project removed the external checkout")
	}
}

// TS-12 §3 / R6: the status endpoint answers ownership, branch, base, and a
// dirty state that is known rather than assumed, and the projects list carries
// the cheap enrichment.
func TestWorktreeStatusAndListEnrichment(t *testing.T) {
	repo := newWorktreeTestRepo(t)
	srv := testServer(t, false)
	seedWorktreeSource(t, srv, "app", repo, config.Project{Title: "App"})
	h := srv.routes()

	rec := doGET(t, h, "/api/projects/app/worktree")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body)
	}
	var source worktreeStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &source); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if source.Owned || !source.RepoBacked || source.Base != "main" {
		t.Fatalf("source status = %+v, want repo-backed, unowned, base main", source)
	}

	rec = forkProject(t, srv, "app", map[string]any{"title": "Fork", "branch": "agentdeck/status"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("fork status = %d body=%s", rec.Code, rec.Body)
	}
	var result forkResult
	_ = json.Unmarshal(rec.Body.Bytes(), &result)
	forkID := result.Project.ProjectID

	rec = doGET(t, h, "/api/projects/"+forkID+"/worktree")
	var forked worktreeStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &forked); err != nil {
		t.Fatalf("decode fork status: %v", err)
	}
	if !forked.Owned || forked.Branch != "agentdeck/status" {
		t.Fatalf("fork status = %+v", forked)
	}
	if !forked.DirtyKnown || forked.Dirty {
		t.Fatalf("fresh checkout dirty=%v known=%v, want a known-clean answer", forked.Dirty, forked.DirtyKnown)
	}
	if err := os.WriteFile(filepath.Join(result.Project.Cwd, "wip.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write wip: %v", err)
	}
	rec = doGET(t, h, "/api/projects/"+forkID+"/worktree")
	_ = json.Unmarshal(rec.Body.Bytes(), &forked)
	if !forked.DirtyKnown || !forked.Dirty {
		t.Fatalf("dirty checkout reported dirty=%v known=%v", forked.Dirty, forked.DirtyKnown)
	}

	rec = doGET(t, h, "/api/projects")
	var list map[string]projectResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if entry := list["app"]; !entry.RepoBacked || entry.Worktree != nil {
		t.Fatalf("source list entry = %+v, want repo-backed and unowned", entry)
	}
	if entry := list[forkID]; entry.Worktree == nil || entry.Worktree.Branch != "agentdeck/status" {
		t.Fatalf("fork list entry = %+v", entry)
	}
}

// TS-12.R4 / INV §5: two starts observing the same missing checkout must not
// both run `git worktree add` at that path. Exactly one recreates; the other
// waits and finds the directory already there.
func TestConcurrentCheckoutRecreationClaimsOnce(t *testing.T) {
	repo := newWorktreeTestRepo(t)
	srv := testServer(t, false)
	seedWorktreeSource(t, srv, "app", repo, config.Project{Title: "App"})

	rec := forkProject(t, srv, "app", map[string]any{"title": "Fork", "branch": "agentdeck/race"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("fork status = %d body=%s", rec.Code, rec.Body)
	}
	var result forkResult
	_ = json.Unmarshal(rec.Body.Bytes(), &result)
	forkID, checkout := result.Project.ProjectID, result.Project.Cwd

	if err := os.RemoveAll(checkout); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}

	const starts = 4
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		creates  int
		failures []string
	)
	start := make(chan struct{})
	for i := 0; i < starts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			recreated, _, ae := srv.ensureWorktreeCheckout(context.Background(), forkID, checkout)
			mu.Lock()
			defer mu.Unlock()
			if ae != nil {
				failures = append(failures, ae.Message)
				return
			}
			if recreated {
				creates++
			}
		}()
	}
	close(start)
	wg.Wait()

	if len(failures) > 0 {
		t.Fatalf("concurrent starts failed: %v", failures)
	}
	if creates != 1 {
		t.Fatalf("recreations = %d, want exactly 1", creates)
	}
	if _, err := os.Stat(filepath.Join(checkout, "README.md")); err != nil {
		t.Fatalf("checkout missing after the race: %v", err)
	}
}
