package worktree

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// olderGitOnPath puts a stand-in for a Git predating `--path-format` (2.31) at
// the front of PATH. It reproduces the real behaviour that makes this fallback
// necessary: `rev-parse` does not reject an option it does not know, it echoes
// the option on stdout and answers anyway. Every other argument is forwarded to
// the installed Git, so the case still runs against a real binary and a real
// repository (INV §12). It returns the path of the argv log.
func olderGitOnPath(t *testing.T) string {
	t.Helper()
	real, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}
	dir := t.TempDir()
	log := filepath.Join(dir, "argv.log")
	script := "#!/bin/sh\n" +
		"echo \"$@\" >> \"" + log + "\"\n" +
		"n=$#\n" +
		"i=0\n" +
		"while [ $i -lt $n ]; do\n" +
		"  arg=\"$1\"; shift\n" +
		"  case \"$arg\" in\n" +
		"    --path-format=*) printf '%s\\n' \"$arg\" ;;\n" +
		"    *) set -- \"$@\" \"$arg\" ;;\n" +
		"  esac\n" +
		"  i=$((i + 1))\n" +
		"done\n" +
		"exec " + real + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o700); err != nil {
		t.Fatalf("write git shim: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return log
}

// TestCommonDirToleratesGitWithoutPathFormat pins TS-12.R1's version tolerance
// for the one query that carries an optional flag: an older Git must produce
// the same repository anchor a current one does, from the repository root and
// from a subdirectory whose answer comes back relative.
func TestCommonDirToleratesGitWithoutPathFormat(t *testing.T) {
	repo := initRepo(t)
	ctx := context.Background()
	want, err := Git{}.CommonDir(ctx, repo)
	if err != nil {
		t.Fatalf("CommonDir on the installed Git: %v", err)
	}
	sub := filepath.Join(repo, "nested")
	if err := os.Mkdir(sub, 0o700); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}

	log := olderGitOnPath(t)
	for _, dir := range []string{repo, sub} {
		got, err := Git{}.CommonDir(ctx, dir)
		if err != nil {
			t.Fatalf("CommonDir(%s) on an older Git: %v", dir, err)
		}
		if got != want {
			t.Fatalf("CommonDir(%s) = %q, want %q", dir, got, want)
		}
	}

	argv, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("read argv log: %v", err)
	}
	// Both forms must be on the wire: the preferred one, then the bare retry.
	for _, want := range []string{
		"-C " + repo + " rev-parse " + pathFormatAbsolute + " --git-common-dir",
		"-C " + repo + " rev-parse --git-common-dir",
	} {
		if !strings.Contains(string(argv), want+"\n") {
			t.Fatalf("argv log missing %q:\n%s", want, argv)
		}
	}
}

// TestCommonDirKeepsGitFailureActionable pins that the fallback does not turn a
// real failure into a second attempt or a silent empty answer.
func TestCommonDirKeepsGitFailureActionable(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	_, err := Git{}.CommonDir(context.Background(), t.TempDir())
	if err == nil {
		t.Fatal("CommonDir outside a repository: want an error")
	}
	if !isNotRepository(err) {
		t.Fatalf("CommonDir outside a repository: want Git's own answer, got %v", err)
	}
}
