package worktree

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// initRepo makes a real one-commit Git repository. Every case here runs against
// the installed Git rather than a fake, because the whole point of this package
// is that its argv and parsing survive a real binary's output (INV §12).
func initRepo(t *testing.T) string {
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
	run("-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "initial")
	return dir
}

func TestIsInsideWorkTree(t *testing.T) {
	repo := initRepo(t)
	g := Git{}
	ctx := context.Background()

	inside, err := g.IsInsideWorkTree(ctx, repo)
	if err != nil {
		t.Fatalf("IsInsideWorkTree(repo): %v", err)
	}
	if !inside {
		t.Fatal("repository reported as not inside a work tree")
	}

	// A plain directory is a plain answer, not an error: the fork action's
	// visibility check must not fail every non-repo project (FS-19.R9).
	plain := t.TempDir()
	inside, err = g.IsInsideWorkTree(ctx, plain)
	if err != nil {
		t.Fatalf("IsInsideWorkTree(plain): %v", err)
	}
	if inside {
		t.Fatal("plain directory reported as inside a work tree")
	}
}

// Only Git's documented not-a-repository answer is ordinary. A missing,
// unreadable, or corrupt repository must stay actionable instead of being
// flattened into false/empty query results (TS-12.R1, INV §7/§12).
func TestGitQueriesPreserveRepositoryErrors(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	missing := filepath.Join(t.TempDir(), "missing")
	corrupt := initRepo(t)
	if err := os.WriteFile(filepath.Join(corrupt, ".git", "config"), []byte("[core\n"), 0o600); err != nil {
		t.Fatalf("corrupt config: %v", err)
	}
	unreadable := t.TempDir()
	if err := os.Chmod(unreadable, 0o000); err != nil {
		t.Fatalf("chmod unreadable directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0o700) })

	paths := map[string]string{
		"missing":    missing,
		"corrupt":    corrupt,
		"unreadable": unreadable,
	}
	g := Git{}
	for name, path := range paths {
		t.Run(name, func(t *testing.T) {
			if _, err := g.IsInsideWorkTree(context.Background(), path); err == nil {
				t.Fatal("IsInsideWorkTree returned an ordinary non-repository answer")
			}
			if _, err := g.CurrentBranch(context.Background(), path); err == nil {
				t.Fatal("CurrentBranch returned an ordinary detached answer")
			}
			if _, err := g.BranchExists(context.Background(), path, "main"); err == nil {
				t.Fatal("BranchExists returned an ordinary missing-ref answer")
			}
			if _, err := g.RevExists(context.Background(), path, "main"); err == nil {
				t.Fatal("RevExists returned an ordinary missing-ref answer")
			}
			if _, err := g.DefaultBase(context.Background(), path); err == nil {
				t.Fatal("DefaultBase returned an ordinary branch answer")
			}
		})
	}
}

func TestDefaultBaseFallsBackToCurrentBranch(t *testing.T) {
	repo := initRepo(t)
	base, err := Git{}.DefaultBase(context.Background(), repo)
	if err != nil {
		t.Fatalf("DefaultBase: %v", err)
	}
	if base != "main" {
		t.Fatalf("base = %q, want main", base)
	}
}

func TestAddListRemoveWorktree(t *testing.T) {
	repo := initRepo(t)
	g := Git{}
	ctx := context.Background()
	checkout := filepath.Join(t.TempDir(), "fork")

	if err := g.AddWorktree(ctx, repo, checkout, "agentdeck/fork", "main"); err != nil {
		t.Fatalf("AddWorktree: %v", err)
	}
	if _, err := os.Stat(filepath.Join(checkout, "README.md")); err != nil {
		t.Fatalf("checkout content missing: %v", err)
	}

	exists, err := g.BranchExists(ctx, repo, "agentdeck/fork")
	if err != nil {
		t.Fatalf("BranchExists: %v", err)
	}
	if !exists {
		t.Fatal("branch not created")
	}

	entries, err := g.ListWorktrees(ctx, repo)
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}
	var found bool
	for _, e := range entries {
		if e.Branch == "agentdeck/fork" {
			found = true
		}
	}
	if !found {
		t.Fatalf("worktree list %+v does not contain the new branch", entries)
	}

	if err := g.RemoveWorktree(ctx, repo, checkout); err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}
	if _, err := os.Stat(checkout); !os.IsNotExist(err) {
		t.Fatalf("checkout still present after removal: %v", err)
	}
	// The branch is the durable thing and survives removing its checkout
	// (FS-19.R8).
	exists, err = g.BranchExists(ctx, repo, "agentdeck/fork")
	if err != nil {
		t.Fatalf("BranchExists after removal: %v", err)
	}
	if !exists {
		t.Fatal("branch was deleted along with its checkout")
	}
}

// A second fork onto an existing branch must fail rather than silently reuse
// it: the branch ref is the atomic claim (TS-12.R10).
func TestAddWorktreeRefusesExistingBranch(t *testing.T) {
	repo := initRepo(t)
	g := Git{}
	ctx := context.Background()
	base := t.TempDir()

	if err := g.AddWorktree(ctx, repo, filepath.Join(base, "one"), "agentdeck/dup", "main"); err != nil {
		t.Fatalf("first AddWorktree: %v", err)
	}
	if err := g.AddWorktree(ctx, repo, filepath.Join(base, "two"), "agentdeck/dup", "main"); err == nil {
		t.Fatal("second AddWorktree on the same branch succeeded")
	}
}

func TestIsDirty(t *testing.T) {
	repo := initRepo(t)
	g := Git{}
	ctx := context.Background()

	dirty, err := g.IsDirty(ctx, repo)
	if err != nil {
		t.Fatalf("IsDirty(clean): %v", err)
	}
	if dirty {
		t.Fatal("fresh repository reported dirty")
	}

	// An untracked file counts: it is exactly the work deleting a checkout loses.
	if err := os.WriteFile(filepath.Join(repo, "scratch.txt"), []byte("wip\n"), 0o600); err != nil {
		t.Fatalf("write scratch: %v", err)
	}
	dirty, err = g.IsDirty(ctx, repo)
	if err != nil {
		t.Fatalf("IsDirty(dirty): %v", err)
	}
	if !dirty {
		t.Fatal("repository with an untracked file reported clean")
	}
}

// An unreadable repository must surface an error so the caller can report
// "undeterminable" instead of fabricating a clean state (INV §8).
func TestIsDirtyErrorsOutsideRepository(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	if _, err := (Git{}).IsDirty(context.Background(), t.TempDir()); err == nil {
		t.Fatal("IsDirty on a non-repository returned no error")
	}
}

func TestRecreateCheckoutFromExistingBranch(t *testing.T) {
	repo := initRepo(t)
	g := Git{}
	ctx := context.Background()
	checkout := filepath.Join(t.TempDir(), "fork")

	if err := g.AddWorktree(ctx, repo, checkout, "agentdeck/recreate", "main"); err != nil {
		t.Fatalf("AddWorktree: %v", err)
	}
	// Delete the directory out of band, exactly as a person clearing disk space
	// would (FS-19.R7), then recreate from the recorded branch.
	if err := os.RemoveAll(checkout); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}
	if err := g.PruneWorktrees(ctx, repo); err != nil {
		t.Fatalf("PruneWorktrees: %v", err)
	}
	if err := g.AddWorktreeExisting(ctx, repo, checkout, "agentdeck/recreate"); err != nil {
		t.Fatalf("AddWorktreeExisting: %v", err)
	}
	if _, err := os.Stat(filepath.Join(checkout, "README.md")); err != nil {
		t.Fatalf("recreated checkout content missing: %v", err)
	}
}
