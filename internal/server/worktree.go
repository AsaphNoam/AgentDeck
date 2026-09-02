package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
	"github.com/agentdeck/agentdeck/internal/worktree"
)

// Worktree-project orchestration (FS-19, TS-12). Everything here sits above the
// one Git boundary in internal/worktree; no other file in this package shells
// out to Git.

const (
	// setupTimeout bounds the project bootstrap command (TS-12.R5).
	setupTimeout = 10 * time.Minute
	// setupWarningLimit clamps the captured setup output on its way to a human
	// surface. The 64 KiB stored tail is a storage bound; a warning banner needs
	// a display bound too (INV §8).
	setupWarningLimit = 2000
	// setupOutputLimit is the bounded stored tail described by TS-12.R5. The
	// setup writer applies it while the command is streaming, rather than after
	// its process has already accumulated an unbounded CombinedOutput buffer.
	setupOutputLimit     = 64 * 1024
	mutateCleanupTimeout = 30 * time.Second
	// repoBackedTTL bounds how long a cwd's repo-backed answer is reused. It
	// keeps the projects list subprocess-free (TS-12.R6) at the cost of a short
	// staleness window after a directory becomes (or stops being) a repository.
	repoBackedTTL = 30 * time.Second
)

type repoBackedEntry struct {
	inside  bool
	checked time.Time
}

// worktreeGit returns the Git boundary. It is a method so tests and future
// callers have one place to look, not because the value carries state.
func (s *Server) worktreeGit() worktree.Git { return worktree.Git{} }

// ---- Ownership and repo-backed lookups ----

// ownedWorktree returns the ownership row for a project, or ok=false when the
// project owns no checkout. A row's presence is the sole ownership test
// (FS-19.R4): a project whose cwd merely lies inside someone else's worktree is
// external and has no deletion path at all.
func (s *Server) ownedWorktree(projectID string) (state.ProjectWorktree, bool) {
	row, err := s.stateStore.ReadProjectWorktree(projectID)
	if err != nil {
		if !errors.Is(err, state.ErrNotFound) {
			s.log.Error("worktree: read ownership", "project", projectID, "err", err)
		}
		return state.ProjectWorktree{}, false
	}
	return row, true
}

func (s *Server) readOwnedWorktree(projectID string) (state.ProjectWorktree, bool, error) {
	row, err := s.stateStore.ReadProjectWorktree(projectID)
	if errors.Is(err, state.ErrNotFound) {
		return state.ProjectWorktree{}, false, nil
	}
	if err != nil {
		return state.ProjectWorktree{}, false, err
	}
	return row, true, nil
}

// repoBacked reports whether an expanded cwd resolves inside a Git working
// tree, memoized per path with a short TTL so the projects list never spawns a
// subprocess per project (TS-12.R6). A Git failure answers false rather than
// failing the caller: action visibility degrades, the list does not (INV §7).
func (s *Server) repoBacked(ctx context.Context, cwd string) bool {
	if cwd == "" {
		return false
	}
	s.repoBackedMu.Lock()
	entry, ok := s.repoBackedCache[cwd]
	s.repoBackedMu.Unlock()
	if ok && time.Since(entry.checked) < repoBackedTTL {
		return entry.inside
	}
	inside, err := s.worktreeGit().IsInsideWorkTree(ctx, cwd)
	if err != nil {
		inside = false
	}
	s.repoBackedMu.Lock()
	s.repoBackedCache[cwd] = repoBackedEntry{inside: inside, checked: time.Now()}
	s.repoBackedMu.Unlock()
	return inside
}

// invalidateRepoBacked drops the memoized answer for one cwd. It runs whenever
// a project definition is written, so an edited path is re-probed instead of
// answering from the previous directory's result.
func (s *Server) invalidateRepoBacked(cwd string) {
	if cwd == "" {
		return
	}
	s.repoBackedMu.Lock()
	delete(s.repoBackedCache, cwd)
	s.repoBackedMu.Unlock()
}

// enrichProjectResponse adds the read-only worktree fields to a project
// payload. Ownership comes from the database and repo-backedness from the
// memoized probe, so enriching a whole list spawns no subprocess per project
// (TS-12.R6). Ownership rows are read once by the list caller and passed in;
// a nil map makes this a single-project lookup.
func (s *Server) enrichProjectResponse(ctx context.Context, resp projectResponse, owned map[string]state.ProjectWorktree) projectResponse {
	cwd, err := config.ExpandTilde(resp.Cwd)
	if err == nil {
		resp.RepoBacked = s.repoBacked(ctx, cwd)
	}
	if owned == nil {
		if row, ok := s.ownedWorktree(resp.ProjectID); ok {
			resp.Worktree = &worktreeSummary{Owned: true, Branch: row.Branch}
		}
		return resp
	}
	if row, ok := owned[resp.ProjectID]; ok {
		resp.Worktree = &worktreeSummary{Owned: true, Branch: row.Branch}
	}
	return resp
}

// invalidateProjectRepoBacked drops the memoized answer for a project's stored
// cwd, expanding it first because the cache is keyed by the expanded path.
func (s *Server) invalidateProjectRepoBacked(cwd string) {
	if expanded, err := config.ExpandTilde(cwd); err == nil {
		s.invalidateRepoBacked(expanded)
	}
}

// ---- Shared checkout resolution (TS-01.R26, TS-12.R4) ----

// ensureWorktreeCheckout is the one step every start path takes before a
// process starts. It replaces the inline cwd stat checks that launch and
// pipeline stage start each carried (INV §2).
//
// For an ordinary project it is exactly the old check: the directory must
// exist. For a project that owns a checkout at this exact path it additionally
// re-materializes a missing checkout from the recorded branch and re-runs the
// project's setup command (FS-19.R7). It never substitutes another branch and
// never re-derives a different path — resume and switch pass their frozen
// snapshot cwd, and the helper only rebuilds the directory that path names.
func (s *Server) ensureWorktreeCheckout(ctx context.Context, projectID, cwd string) (recreated bool, warning string, ae *runtime.APIError) {
	row, owned, err := s.readOwnedWorktree(projectID)
	if err != nil {
		return false, "", apiError(runtime.CodeInternal, "read worktree ownership: "+err.Error())
	}
	if isExistingDir(cwd) {
		if owned && sameCheckoutPath(row.CheckoutPath, cwd) {
			if err := s.configStore.ValidateOwnedWorktreePath(projectID, row.CheckoutPath); err != nil {
				return false, "", apiError(runtime.CodeValidation, "unsafe owned worktree path: "+err.Error())
			}
		}
		return false, "", nil
	}
	if !owned || !sameCheckoutPath(row.CheckoutPath, cwd) {
		return false, "", apiError(runtime.CodeValidation,
			fmt.Sprintf("project directory %q does not exist — set project %q to an existing path", cwd, projectID))
	}

	// Two starts into the same project can observe the absence together. Claim
	// the recreation per project so the loser waits for the winner instead of
	// racing a second `git worktree add` at the same path (INV §5).
	unlock := s.claimWorktreeRecreate(projectID)
	defer unlock()
	if isExistingDir(cwd) {
		return false, "", nil
	}

	git := s.worktreeGit()
	exists, err := git.BranchExists(ctx, row.RepoPath, row.Branch)
	if err != nil {
		return false, "", apiError(runtime.CodeValidation,
			fmt.Sprintf("cannot read repository %q for project %q: %v", row.RepoPath, projectID, err))
	}
	if !exists {
		// Never silently substitute the base branch: the branch is the durable
		// thing this project is (FS-19.R7).
		return false, "", apiError(runtime.CodeValidation, fmt.Sprintf(
			"the checkout for project %q is missing and its branch %q no longer exists in %s — recreate the branch, or point the project at another directory",
			projectID, row.Branch, row.RepoPath))
	}
	if _, err := s.configStore.EnsureWorktreeRoot(projectID); err != nil {
		return false, "", apiError(runtime.CodeValidation, "cannot prepare the worktree directory: "+err.Error())
	}
	// The registration still names a directory that is gone; prune it so the
	// re-add at the same path is not refused.
	if err := git.PruneWorktrees(ctx, row.RepoPath); err != nil {
		s.log.Warn("worktree: prune before recreate", "project", projectID, "err", err)
	}
	if err := git.AddWorktreeExisting(ctx, row.RepoPath, row.CheckoutPath, row.Branch); err != nil {
		return false, "", apiError(runtime.CodeValidation,
			fmt.Sprintf("cannot recreate the checkout for project %q on branch %q: %v", projectID, row.Branch, err))
	}
	warning = s.runProjectSetup(ctx, projectID, row.CheckoutPath)
	return true, warning, nil
}

// claimWorktreeRecreate takes the per-project recreation claim and returns its
// release. The lock is per project, so unrelated projects still recreate in
// parallel.
func (s *Server) claimWorktreeRecreate(projectID string) func() {
	s.worktreeMu.Lock()
	lock, ok := s.worktreeLocks[projectID]
	if !ok {
		lock = &worktreeLock{}
		s.worktreeLocks[projectID] = lock
	}
	lock.refs++
	s.worktreeMu.Unlock()

	lock.mu.Lock()
	return func() {
		lock.mu.Unlock()
		s.worktreeMu.Lock()
		lock.refs--
		if lock.refs == 0 {
			delete(s.worktreeLocks, projectID)
		}
		s.worktreeMu.Unlock()
	}
}

// ---- Setup command (TS-12.R5) ----

// runProjectSetup runs the project's setup_command inside a checkout and
// records the outcome on the ownership row. It returns a human-facing warning
// when the command fails or times out; a failure never blocks the caller
// (FS-19.R3).
func (s *Server) runProjectSetup(ctx context.Context, projectID, checkout string) string {
	project, err := s.configStore.ReadProject(projectID)
	if err != nil {
		return "setup status could not be determined: " + err.Error()
	}
	if strings.TrimSpace(project.SetupCommand) == "" {
		return ""
	}
	runCtx, cancel := context.WithTimeout(ctx, setupTimeout)
	defer cancel()

	// /bin/sh -c with the server's inherited environment — the same environment
	// launched agents see. Shell-profile-dependent tooling needs an absolute path
	// (TS-12 §5).
	cmd := exec.CommandContext(runCtx, "/bin/sh", "-c", project.SetupCommand)
	cmd.Dir = checkout
	cmd.Stdin = nil
	outputTail := newSetupOutputTail(setupOutputLimit)
	cmd.Stdout = outputTail
	cmd.Stderr = outputTail
	runErr := cmd.Run()
	if runCtx.Err() != nil {
		_, _ = io.WriteString(outputTail, fmt.Sprintf("\nsetup command timed out after %s", setupTimeout))
		runErr = runCtx.Err()
	}
	output := outputTail.String()
	if err := s.stateStore.RecordProjectWorktreeSetup(projectID, runErr == nil, output); err != nil {
		s.log.Error("worktree: record setup result", "project", projectID, "err", err)
		if runErr == nil {
			return "setup completed, but its result could not be recorded: " + err.Error()
		}
		return fmt.Sprintf("setup command failed: %s; its result could not be recorded: %v", clipSetupOutput(output), err)
	}
	if runErr == nil {
		return ""
	}
	return fmt.Sprintf("setup command failed: %s", clipSetupOutput(output))
}

// setupOutputTail keeps the last n bytes written by a setup command. Stdout
// and stderr may be copied concurrently by os/exec, so writes are serialized.
// String drops an incomplete UTF-8 boundary instead of exposing invalid text
// to the database or a warning surface.
type setupOutputTail struct {
	mu    sync.Mutex
	limit int
	buf   []byte
}

func newSetupOutputTail(limit int) *setupOutputTail {
	return &setupOutputTail{limit: limit, buf: make([]byte, 0, limit)}
}

func (t *setupOutputTail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(p) >= t.limit {
		t.buf = append(t.buf[:0], p[len(p)-t.limit:]...)
		return len(p), nil
	}
	if overflow := len(t.buf) + len(p) - t.limit; overflow > 0 {
		copy(t.buf, t.buf[overflow:])
		t.buf = t.buf[:len(t.buf)-overflow]
	}
	t.buf = append(t.buf, p...)
	return len(p), nil
}

func (t *setupOutputTail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	// A writer can start or end in the middle of a multi-byte rune after the
	// byte-tail cut. Dropping invalid sequences retains the complete textual
	// tail and guarantees valid UTF-8 for SQLite and human-facing warnings.
	if utf8.Valid(t.buf) {
		return string(t.buf)
	}
	return strings.ToValidUTF8(string(t.buf), "")
}

// clipSetupOutput bounds what reaches a person. The stored tail is 64 KiB; a
// warning banner gets the last few lines of it, not a build log (INV §8).
func clipSetupOutput(out string) string {
	out = strings.TrimSpace(out)
	if out == "" {
		return "(no output)"
	}
	runes := []rune(out)
	if len(runes) <= setupWarningLimit {
		return out
	}
	return "…" + string(runes[len(runes)-setupWarningLimit:])
}

// ---- Fork (FS-19.R1/R10, TS-12.R3/R10) ----

// forkRequest is the creation form's payload: only what varies between the
// source project and the fork.
type forkRequest struct {
	Title  string `json:"title"`
	Branch string `json:"branch"`
	Base   string `json:"base"`
}

// forkResult reports what the fork created.
type forkResult struct {
	Project projectResponse `json:"project"`
	Branch  string          `json:"branch"`
	Base    string          `json:"base"`
	Warning string          `json:"warning,omitempty"`
}

// worktreeFork creates a branch, an owned checkout, and a copied project as one
// operation. Ordering is: validate → create the branch and worktree → write the
// project file → insert the ownership row → run setup. Any failure before setup
// rolls back in reverse, so no project, branch, or directory is left behind
// (FS-19.R10). The ownership row is written last, after both the checkout and
// the project it belongs to exist, so every crash residue is an unrecorded —
// therefore external and inert — checkout rather than a record authorizing the
// deletion of something no project owns (INV §15).
func (s *Server) worktreeFork(ctx context.Context, sourceID string, req forkRequest) (forkResult, *runtime.APIError) {
	source, err := s.configStore.ReadProject(sourceID)
	if err != nil {
		if errors.Is(err, config.ErrNotFound) {
			return forkResult{}, apiError(runtime.CodeNotFound, "no such project: "+sourceID)
		}
		return forkResult{}, apiError(runtime.CodeInternal, err.Error())
	}
	if source.Archived {
		return forkResult{}, apiError(runtime.CodeValidation, "project is archived; restore it first")
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		return forkResult{}, apiError(runtime.CodeValidation, "title is required")
	}
	branch := strings.TrimSpace(req.Branch)
	if ae := validateBranchName(branch); ae != nil {
		return forkResult{}, ae
	}

	sourceCwd, err := config.ExpandTilde(source.Cwd)
	if err != nil {
		return forkResult{}, apiError(runtime.CodeValidation, "bad project cwd: "+err.Error())
	}
	git := s.worktreeGit()
	inside, err := git.IsInsideWorkTree(ctx, sourceCwd)
	if err != nil {
		return forkResult{}, apiError(runtime.CodeValidation, gitUnavailableMessage(err))
	}
	if !inside {
		return forkResult{}, apiError(runtime.CodeValidation,
			fmt.Sprintf("project %q is not inside a Git working tree", sourceID))
	}
	repo, err := git.CommonDir(ctx, sourceCwd)
	if err != nil {
		return forkResult{}, apiError(runtime.CodeValidation, "cannot read the repository anchor: "+err.Error())
	}

	base := strings.TrimSpace(req.Base)
	if base == "" {
		resolved, ae := s.effectiveBase(ctx, source, sourceCwd)
		if ae != nil {
			return forkResult{}, ae
		}
		base = resolved
	}
	if ok, err := git.RevExists(ctx, repo, base); err != nil || !ok {
		return forkResult{}, apiError(runtime.CodeValidation,
			fmt.Sprintf("base %q does not resolve to a commit in %s", base, repo))
	}
	if exists, err := git.BranchExists(ctx, repo, branch); err != nil {
		return forkResult{}, apiError(runtime.CodeValidation, "cannot check branch: "+err.Error())
	} else if exists {
		return forkResult{}, apiError(runtime.CodeValidation, fmt.Sprintf("branch %q already exists", branch))
	}

	// Server-derived ids make checkout-path collisions unreachable; the path is
	// still pre-checked and fails closed (TS-12.R10).
	newID := config.GenerateProjectID(title, time.Now())
	if _, err := s.configStore.ReadProject(newID); err == nil {
		return forkResult{}, apiError(runtime.CodeConflict, "project "+newID+" already exists")
	}
	checkout, err := s.configStore.EnsureWorktreeRoot(newID)
	if err != nil {
		return forkResult{}, apiError(runtime.CodeValidation, "cannot prepare the worktree directory: "+err.Error())
	}
	if _, err := os.Lstat(checkout); err == nil {
		return forkResult{}, apiError(runtime.CodeConflict, "worktree path "+checkout+" already exists")
	}

	// First external effect and atomic claim. Once CreateBranch succeeds this
	// attempt owns the ref and may safely compensate it; a concurrent loser never
	// deletes the winner's branch (TS-12.R10, INV §5/§15).
	if err := git.CreateBranch(ctx, repo, branch, base); err != nil {
		return forkResult{}, apiError(runtime.CodeValidation, "cannot create branch: "+err.Error())
	}
	if err := git.AddWorktreeExisting(ctx, repo, checkout, branch); err != nil {
		if cleanupErr := s.rollbackFork(repo, checkout, branch, ""); cleanupErr != nil {
			return forkResult{}, apiError(runtime.CodeInternal, "cannot create the worktree: "+err.Error()+"; cleanup incomplete: "+cleanupErr.Error())
		}
		return forkResult{}, apiError(runtime.CodeValidation, "cannot create the worktree: "+err.Error())
	}

	resourcePath, err := s.configStore.ProjectResourcesPath(newID)
	if err != nil {
		return forkResult{}, apiError(runtime.CodeInternal, err.Error())
	}
	_, resourceStatErr := os.Lstat(resourcePath)
	resourceCreated := os.IsNotExist(resourceStatErr)
	if _, err := s.configStore.EnsureProjectResources(newID); err != nil {
		cleanupErr := s.rollbackFork(repo, checkout, branch, "")
		if cleanupErr != nil {
			return forkResult{}, apiError(runtime.CodeInternal, "cannot create project resources: "+err.Error()+"; cleanup incomplete: "+cleanupErr.Error())
		}
		return forkResult{}, apiError(runtime.CodeValidation, "cannot create project resources: "+err.Error())
	}
	if !resourceCreated {
		resourcePath = ""
	}

	fork := config.Project{
		Title:         title,
		Color:         source.Color,
		Cwd:           checkout,
		AddDirs:       append([]string{}, source.AddDirs...),
		ContextPrompt: source.ContextPrompt,
		BaseBranch:    source.BaseBranch,
		SetupCommand:  source.SetupCommand,
	}
	if err := s.writeProject(newID, fork); err != nil {
		cleanupErr := s.rollbackFork(repo, checkout, branch, resourcePath)
		if cleanupErr != nil {
			return forkResult{}, apiError(runtime.CodeInternal, "write project: "+err.Error()+"; cleanup incomplete: "+cleanupErr.Error())
		}
		return forkResult{}, apiError(runtime.CodeInternal, "write project: "+err.Error())
	}
	if err := s.stateStore.InsertProjectWorktree(state.ProjectWorktree{
		Project:      newID,
		RepoPath:     repo,
		Branch:       branch,
		CheckoutPath: checkout,
	}); err != nil {
		if delErr := s.configStore.DeleteProject(newID); delErr != nil {
			s.log.Error("worktree: rollback delete project", "project", newID, "err", delErr)
		}
		cleanupErr := s.rollbackFork(repo, checkout, branch, resourcePath)
		if cleanupErr != nil {
			return forkResult{}, apiError(runtime.CodeInternal, "record worktree ownership: "+err.Error()+"; cleanup incomplete: "+cleanupErr.Error())
		}
		return forkResult{}, apiError(runtime.CodeInternal, "record worktree ownership: "+err.Error())
	}

	// Setup is the only step whose failure leaves the created project in place
	// (FS-19.R3/R10).
	warning := s.runProjectSetup(ctx, newID, checkout)

	return forkResult{
		Project: s.enrichProjectResponse(ctx, s.toProjectResponse(newID, fork, nil), nil),
		Branch:  branch,
		Base:    base,
		Warning: warning,
	}, nil
}

// rollbackFork unwinds the fork's external effects in reverse. The branch is
// force-deleted because it was created moments ago at the base commit and
// carries no work.
func (s *Server) rollbackFork(repo, checkout, branch, resourcePath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), mutateCleanupTimeout)
	defer cancel()
	git := s.worktreeGit()
	var errs []error
	if _, err := os.Lstat(checkout); err == nil {
		if err := git.RemoveWorktree(ctx, repo, checkout); err != nil {
			s.log.Error("worktree: rollback remove checkout", "checkout", checkout, "err", err)
			errs = append(errs, err)
		}
	}
	if exists, err := git.BranchExists(ctx, repo, branch); err != nil {
		errs = append(errs, err)
	} else if exists {
		if err := git.DeleteBranch(ctx, repo, branch); err != nil {
			s.log.Error("worktree: rollback delete branch", "branch", branch, "err", err)
			errs = append(errs, err)
		}
	}
	if resourcePath != "" {
		if err := os.Remove(resourcePath); err != nil && !os.IsNotExist(err) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// effectiveBase resolves the base a fork branches from: the project's
// base_branch when set, else the repository's default branch (TS-12.R9). It is
// resolved at use time and never cached durably.
func (s *Server) effectiveBase(ctx context.Context, project config.Project, cwd string) (string, *runtime.APIError) {
	if b := strings.TrimSpace(project.BaseBranch); b != "" {
		return b, nil
	}
	base, err := s.worktreeGit().DefaultBase(ctx, cwd)
	if err != nil {
		if errors.Is(err, worktree.ErrDetachedHead) {
			return "", apiError(runtime.CodeValidation,
				"the repository has a detached HEAD and no origin/HEAD — set the project's base branch explicitly")
		}
		return "", apiError(runtime.CodeValidation, gitUnavailableMessage(err))
	}
	return base, nil
}

// validateBranchName rejects the shapes Git itself would refuse or that would
// read as an option, before anything is created.
func validateBranchName(branch string) *runtime.APIError {
	switch {
	case branch == "":
		return apiError(runtime.CodeValidation, "branch is required")
	case strings.HasPrefix(branch, "-"):
		return apiError(runtime.CodeValidation, "branch must not start with '-'")
	case strings.ContainsAny(branch, " \t\n~^:?*[\\"):
		return apiError(runtime.CodeValidation, "branch contains characters Git does not allow")
	case strings.Contains(branch, ".."), strings.HasSuffix(branch, "/"), strings.HasSuffix(branch, ".lock"):
		return apiError(runtime.CodeValidation, "branch is not a valid Git ref name")
	}
	return nil
}

func gitUnavailableMessage(err error) string {
	if errors.Is(err, worktree.ErrGitMissing) {
		return "Git is not installed on this machine"
	}
	return "cannot read the repository: " + err.Error()
}

// ---- Status (TS-12 §3) ----

// worktreeStatusResponse feeds the fork form and the archive dialog. It is the
// on-demand per-project endpoint, so it is allowed to spend subprocesses the
// list path is not (TS-12.R6).
type worktreeStatusResponse struct {
	Owned      bool         `json:"owned"`
	RepoBacked bool         `json:"repo_backed"`
	Branch     string       `json:"branch"`
	Base       string       `json:"base"`
	Dirty      bool         `json:"dirty"`
	DirtyKnown bool         `json:"dirty_known"`
	Setup      *setupStatus `json:"setup,omitempty"`
}

type setupStatus struct {
	OK     *bool  `json:"ok"`
	At     string `json:"at,omitempty"`
	Output string `json:"output,omitempty"`
}

func (s *Server) worktreeStatus(ctx context.Context, projectID string, project config.Project) (worktreeStatusResponse, error) {
	out := worktreeStatusResponse{}
	cwd, err := config.ExpandTilde(project.Cwd)
	if err != nil {
		return out, err
	}
	git := s.worktreeGit()
	inside, err := git.IsInsideWorkTree(ctx, cwd)
	out.RepoBacked = err == nil && inside

	row, owned, err := s.readOwnedWorktree(projectID)
	if err != nil {
		return out, err
	}
	out.Owned = owned
	if owned {
		out.Branch = row.Branch
		out.Setup = &setupStatus{OK: row.SetupOK}
		if row.SetupAt != nil {
			out.Setup.At = row.SetupAt.Format(time.RFC3339)
		}
		// The captured output is carried only when it explains something: a run
		// that succeeded has nothing to report, and a run that never happened has
		// nothing to report either.
		if row.SetupOK != nil && !*row.SetupOK {
			out.Setup.Output = clipSetupOutput(row.SetupOutput)
		}
	} else if out.RepoBacked {
		if branch, err := git.CurrentBranch(ctx, cwd); err == nil {
			out.Branch = branch
		}
	}
	if out.RepoBacked {
		if base, ae := s.effectiveBase(ctx, project, cwd); ae == nil {
			out.Base = base
		}
	}
	// Dirty state is the only thing deleting a checkout can lose, so a failed or
	// timed-out check reports "undeterminable" rather than a fabricated clean
	// state (FS-19.R8, INV §8).
	if owned && isExistingDir(row.CheckoutPath) {
		if dirty, err := git.IsDirty(ctx, row.CheckoutPath); err == nil {
			out.Dirty, out.DirtyKnown = dirty, true
		}
	} else if owned {
		// A missing checkout holds nothing to lose; that much is known.
		out.DirtyKnown = true
	}
	return out, nil
}

// ---- Consented deletion (FS-19.R8, TS-12.R7) ----

// deleteOwnedCheckout removes an AgentDeck-created checkout and its ownership
// row. It runs only inside an existing project-archive claim with every process
// stopped. Before removing anything it re-verifies that the recorded path is
// the canonical owned location, is not a symlink, and is currently registered
// to the recorded repository; any mismatch aborts rather than deletes. The row
// is dropped only after the checkout is gone, so a crash between the two leaves
// the recreation case, never an unrecorded owned checkout (INV §15).
func (s *Server) deleteOwnedCheckout(ctx context.Context, projectID string, expectedDirty *bool) (bool, error) {
	row, owned, err := s.readOwnedWorktree(projectID)
	if err != nil {
		return false, err
	}
	if !owned {
		// External checkouts have no row and therefore no deletion path at all.
		// Reporting "not deleted" is what keeps consent on an external project
		// from claiming a deletion that never happened (FS-19.R4/A5).
		return false, nil
	}
	canonical, err := s.configStore.WorktreePath(projectID)
	if err != nil {
		return false, fmt.Errorf("worktree: %w", err)
	}
	if !sameCheckoutPath(row.CheckoutPath, canonical) {
		return false, fmt.Errorf("worktree: recorded checkout %q is not the owned location %q", row.CheckoutPath, canonical)
	}
	if _, err := os.Lstat(row.CheckoutPath); err == nil {
		if err := s.configStore.ValidateOwnedWorktreePath(projectID, row.CheckoutPath); err != nil {
			return false, fmt.Errorf("worktree: %w", err)
		}
		git := s.worktreeGit()
		entries, err := git.ListWorktrees(ctx, row.RepoPath)
		if err != nil {
			return false, fmt.Errorf("worktree: read registrations for %q: %w", row.RepoPath, err)
		}
		registered := false
		for _, entry := range entries {
			if sameCheckoutPath(entry.Path, row.CheckoutPath) {
				registered = true
				break
			}
		}
		if !registered {
			return false, fmt.Errorf("worktree: %q is not registered to %q", row.CheckoutPath, row.RepoPath)
		}
		if expectedDirty != nil && !*expectedDirty {
			dirty, err := git.IsDirty(ctx, row.CheckoutPath)
			if err != nil {
				return false, fmt.Errorf("worktree: recheck uncommitted changes: %w", err)
			}
			if dirty {
				return false, fmt.Errorf("worktree: checkout gained uncommitted changes after confirmation; review them and confirm again")
			}
		}
		// --force is deliberate: the dialog already disclosed dirty state and the
		// person consented (FS-19.R8).
		if err := git.RemoveWorktree(ctx, row.RepoPath, row.CheckoutPath); err != nil {
			return false, fmt.Errorf("worktree: remove checkout: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("worktree: stat checkout: %w", err)
	}
	if err := s.stateStore.DeleteProjectWorktree(projectID); err != nil {
		return false, err
	}
	return true, nil
}

// ---- Small shared helpers ----

func isExistingDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// sameCheckoutPath compares two checkout paths without touching disk. Git and
// the config store can disagree on a trailing separator or on macOS's /private
// prefix for temporary directories, and both are cleaned before comparison.
func sameCheckoutPath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if filepath.Clean(a) == filepath.Clean(b) {
		return true
	}
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && ra == rb
}

// worktreeLock is the per-project recreation claim.
type worktreeLock struct {
	mu   sync.Mutex
	refs int
}
