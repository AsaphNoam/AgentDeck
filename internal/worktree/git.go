// Package worktree is AgentDeck's single Git execution boundary (TS-12.R1).
//
// Every Git invocation in the product goes through this package. Commands are
// built argv-only (never a shell), rooted with `git -C <path>`, run with stdin
// closed, `GIT_TERMINAL_PROMPT=0`, and a bounded timeout, so no operation can
// hang waiting for a credential prompt. Parsing uses plumbing commands and
// tolerates version variance (INV §12): output is read as a stable field set,
// never as scraped porcelain UI text.
package worktree

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ErrGitMissing reports that no `git` binary is installed on this host. It is
// actionable on its own: nothing this package does can proceed without it.
var ErrGitMissing = errors.New("worktree: git is not installed")

// ErrDetachedHead reports a repository whose HEAD names no branch and that has
// no origin/HEAD to fall back to, so no base branch can be derived (TS-12.R9).
var ErrDetachedHead = errors.New("worktree: repository HEAD is detached and has no origin/HEAD")

// Command timeouts. Read-only plumbing answers immediately on a healthy repo;
// creating or removing a checkout copies files and is given real headroom. Both
// are bounded so a wedged Git can never hold a request open (TS-12.R1).
const (
	queryTimeout  = 30 * time.Second
	mutateTimeout = 5 * time.Minute
)

// Git runs Git commands. The zero value is usable and resolves the binary on
// first use.
type Git struct{}

// Entry is one row of `git worktree list --porcelain`: the checkout path and
// the branch it has checked out (empty when detached).
type Entry struct {
	Path   string
	Branch string
}

// run executes `git -C dir <args...>` and returns trimmed stdout. A non-zero
// exit becomes an error carrying the captured stderr, which is what makes Git
// failures actionable to the person who triggered them.
func (g Git) run(ctx context.Context, timeout time.Duration, dir string, args ...string) (string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", ErrGitMissing
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	argv := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(ctx, "git", argv...)
	cmd.Stdin = nil
	// A minimal environment plus the prompt suppression: Git must never block on
	// a terminal or askpass prompt inside a server request.
	cmd.Env = append(cmd.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=",
		"SSH_ASKPASS=",
		// The expected Git failure messages below are classified deliberately.
		// Pin their language so a localized Git installation does not turn an
		// ordinary non-repository answer into an unexpected server error.
		"LC_ALL=C",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		return "", fmt.Errorf("worktree: git %s timed out after %s", args[0], timeout)
	}
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return strings.TrimSpace(stdout.String()), &commandError{
			args:   append([]string{}, args...),
			detail: detail,
			err:    err,
		}
	}
	return strings.TrimSpace(stdout.String()), nil
}

// commandError preserves Git's process result for the small set of plumbing
// commands whose non-zero status is an ordinary answer. Callers must classify
// only their documented expected status; every other failure stays actionable.
type commandError struct {
	args   []string
	detail string
	err    error
}

func (e *commandError) Error() string {
	return fmt.Sprintf("worktree: git %s: %s", strings.Join(e.args, " "), e.detail)
}

func (e *commandError) Unwrap() error { return e.err }

func gitExitCode(err error, code int) bool {
	var commandErr *commandError
	if !errors.As(err, &commandErr) {
		return false
	}
	var exitErr *exec.ExitError
	return errors.As(commandErr.err, &exitErr) && exitErr.ExitCode() == code
}

func isNotRepository(err error) bool {
	var commandErr *commandError
	return errors.As(err, &commandErr) &&
		gitExitCode(err, 128) &&
		strings.Contains(commandErr.detail, "not a git repository")
}

func isMissingSymbolicRef(err error) bool {
	var commandErr *commandError
	return errors.As(err, &commandErr) &&
		(gitExitCode(err, 1) || gitExitCode(err, 128)) &&
		strings.Contains(commandErr.detail, "is not a symbolic ref")
}

// IsInsideWorkTree reports whether dir resolves inside a Git working tree. A
// path that is simply not a repository is reported as false with no error;
// only a missing binary or an unreadable repository is an error.
func (g Git) IsInsideWorkTree(ctx context.Context, dir string) (bool, error) {
	out, err := g.run(ctx, queryTimeout, dir, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		if errors.Is(err, ErrGitMissing) {
			return false, err
		}
		// "not a git repository" is the ordinary answer for a plain directory.
		if isNotRepository(err) {
			return false, nil
		}
		return false, err
	}
	return out == "true", nil
}

// RepoRoot returns the absolute top-level directory of the working tree
// containing dir.
func (g Git) RepoRoot(ctx context.Context, dir string) (string, error) {
	return g.run(ctx, queryTimeout, dir, "rev-parse", "--show-toplevel")
}

// CommonDir returns the repository's shared .git directory. Every linked
// worktree of one repository reports the same value, which is how a checkout is
// matched back to the repository that owns it.
//
// `--path-format` arrived in Git 2.31 and this package declares no minimum
// version (TS-12.R1), so the answer cannot depend on it. `rev-parse` does not
// reject an option it does not know: it echoes the option back on stdout and
// still answers, exiting zero. An older Git therefore returns the flag text
// followed by the path, which is why the flag's own echo — not an error — is
// the detection here (INV §12).
func (g Git) CommonDir(ctx context.Context, dir string) (string, error) {
	out, err := g.run(ctx, queryTimeout, dir, "rev-parse", pathFormatAbsolute, "--git-common-dir")
	if err != nil {
		// A real failure (not a repository, unreadable, missing binary) is the
		// same failure without the flag, so it stays actionable as it is.
		return "", err
	}
	if echoed, _, more := strings.Cut(out, "\n"); !more || echoed != pathFormatAbsolute {
		return out, nil
	}
	out, err = g.run(ctx, queryTimeout, dir, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	return absoluteRepoPath(dir, out), nil
}

// pathFormatAbsolute is named because it is both an argument and, on a Git that
// predates it, the output that reveals it was not understood.
const pathFormatAbsolute = "--path-format=absolute"

// absoluteRepoPath makes a `rev-parse` answer absolute the way
// --path-format=absolute would. Without the flag the path is relative to the
// `-C` directory, and Git resolves that directory before printing, so symlinks
// are resolved here too: one repository must report one common directory
// however the caller's path reached it, or an owned checkout stops matching the
// repository that owns it (TS-12.R2).
func absoluteRepoPath(dir, out string) string {
	path := out
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return filepath.Clean(path)
}

// DefaultBase resolves the repository's default branch: origin/HEAD's target
// when the remote publishes one, else the branch checked out in the
// repository's main worktree. A detached HEAD with no origin/HEAD returns
// ErrDetachedHead (TS-12.R9).
//
// The fallback deliberately reads the MAIN worktree's branch rather than dir's
// own HEAD. Called from inside a linked worktree — which is exactly what
// forking a fork does — dir's HEAD is that fork's branch, and using it would
// stack the new branch on the source's work. FS-19.R11 requires the opposite:
// nothing ever stacks implicitly.
func (g Git) DefaultBase(ctx context.Context, dir string) (string, error) {
	if out, err := g.run(ctx, queryTimeout, dir, "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil {
		if branch := strings.TrimPrefix(out, "origin/"); branch != "" && branch != out {
			return branch, nil
		}
	} else if errors.Is(err, ErrGitMissing) {
		return "", err
	} else if !isMissingSymbolicRef(err) {
		return "", err
	}
	// `worktree list` reports the main worktree first, in every version that
	// supports the porcelain format.
	entries, err := g.ListWorktrees(ctx, dir)
	if err != nil {
		return "", err
	}
	if len(entries) > 0 && entries[0].Branch != "" {
		return entries[0].Branch, nil
	}
	branch, err := g.CurrentBranch(ctx, dir)
	if err != nil {
		return "", err
	}
	if branch == "" {
		return "", ErrDetachedHead
	}
	return branch, nil
}

// CurrentBranch returns the branch HEAD points at, or "" when HEAD is detached.
func (g Git) CurrentBranch(ctx context.Context, dir string) (string, error) {
	out, err := g.run(ctx, queryTimeout, dir, "symbolic-ref", "--short", "--quiet", "HEAD")
	if err != nil {
		if errors.Is(err, ErrGitMissing) {
			return "", err
		}
		// --quiet makes a detached HEAD an empty, non-fatal answer.
		if gitExitCode(err, 1) {
			return "", nil
		}
		return "", err
	}
	return out, nil
}

// BranchExists reports whether refs/heads/<branch> resolves in the repository.
func (g Git) BranchExists(ctx context.Context, dir, branch string) (bool, error) {
	if _, err := g.run(ctx, queryTimeout, dir, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch); err != nil {
		if errors.Is(err, ErrGitMissing) {
			return false, err
		}
		if gitExitCode(err, 1) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// RevExists reports whether rev resolves to a commit in the repository. It is
// how a fork validates its base before creating anything.
func (g Git) RevExists(ctx context.Context, dir, rev string) (bool, error) {
	if _, err := g.run(ctx, queryTimeout, dir, "rev-parse", "--verify", "--quiet", rev+"^{commit}"); err != nil {
		if errors.Is(err, ErrGitMissing) {
			return false, err
		}
		if gitExitCode(err, 1) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// AddWorktree creates a new branch at base and checks it out into path as a
// linked worktree of repo.
func (g Git) AddWorktree(ctx context.Context, repo, path, branch, base string) error {
	_, err := g.run(ctx, mutateTimeout, repo, "worktree", "add", "-b", branch, path, base)
	return err
}

// CreateBranch claims a new local branch at base. Git's ref lock makes this
// atomic: an existing or concurrently-created branch is never overwritten.
func (g Git) CreateBranch(ctx context.Context, repo, branch, base string) error {
	_, err := g.run(ctx, mutateTimeout, repo, "branch", branch, base)
	return err
}

// AddWorktreeExisting checks an already-existing branch out into path. It is
// the recreation path (FS-19.R7): the branch is the durable thing, the checkout
// is not.
func (g Git) AddWorktreeExisting(ctx context.Context, repo, path, branch string) error {
	_, err := g.run(ctx, mutateTimeout, repo, "worktree", "add", path, branch)
	return err
}

// RemoveWorktree removes a linked worktree and its registration. --force is
// deliberate: the archive dialog already disclosed any uncommitted changes and
// the person consented (FS-19.R8, TS-12.R7).
func (g Git) RemoveWorktree(ctx context.Context, repo, path string) error {
	_, err := g.run(ctx, mutateTimeout, repo, "worktree", "remove", "--force", path)
	return err
}

// PruneWorktrees drops registrations whose directories are gone. It runs after
// an out-of-band deletion so a recreation at the same path is not refused.
func (g Git) PruneWorktrees(ctx context.Context, repo string) error {
	_, err := g.run(ctx, queryTimeout, repo, "worktree", "prune")
	return err
}

// DeleteBranch force-deletes a local branch. It is only used to unwind a fork
// whose branch was created moments earlier and carries no commits (TS-12.R3).
func (g Git) DeleteBranch(ctx context.Context, repo, branch string) error {
	_, err := g.run(ctx, mutateTimeout, repo, "branch", "-D", branch)
	return err
}

// ListWorktrees parses `worktree list --porcelain`. The porcelain format is a
// blank-line-separated record of `key value` lines; unknown keys are ignored so
// a newer Git that adds fields still parses (INV §12).
func (g Git) ListWorktrees(ctx context.Context, repo string) ([]Entry, error) {
	out, err := g.run(ctx, queryTimeout, repo, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	entries := []Entry{}
	var current Entry
	flush := func() {
		if current.Path != "" {
			entries = append(entries, current)
		}
		current = Entry{}
	}
	// The output is already fully captured in memory, so it is split directly
	// rather than re-scanned: no line-length limit can truncate this list.
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			flush()
			continue
		}
		key, value, _ := strings.Cut(line, " ")
		switch key {
		case "worktree":
			flush()
			current.Path = value
		case "branch":
			current.Branch = strings.TrimPrefix(value, "refs/heads/")
		}
	}
	flush()
	return entries, nil
}

// IsDirty reports whether the working tree at dir holds uncommitted changes,
// including untracked files — the only thing deleting a checkout can lose
// (FS-19.R8). An error means the state is undeterminable and must be reported
// as such, never as clean (INV §8).
func (g Git) IsDirty(ctx context.Context, dir string) (bool, error) {
	out, err := g.run(ctx, queryTimeout, dir, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return out != "", nil
}
