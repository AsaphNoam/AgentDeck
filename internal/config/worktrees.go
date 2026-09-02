package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// AgentDeck-owned Git worktrees: one checkout per worktree project at
// $AGENTDECK_HOME/worktrees/{project-id}/ (FS-19.R4, TS-12.R8). This file owns
// only the path rules — validation, the owner-only root, and symlink rejection.
// Git itself creates and removes the leaf; this package never writes inside one.

// dirWorktrees is the parent directory holding every owned checkout.
const dirWorktrees = "worktrees"

// WorktreePath returns the canonical absolute checkout path for a project id
// without touching disk. The id is slug-validated before it is joined into a
// path, so the result can never escape the AgentDeck home (FS-11.R8).
func (s *Store) WorktreePath(projectID string) (string, error) {
	if !ValidSlug(projectID) {
		return "", fmt.Errorf("config: invalid project id %q", projectID)
	}
	return filepath.Join(s.home, dirWorktrees, projectID), nil
}

// WorktreesRoot returns the owned worktree root. No AgentDeck code path removes
// a directory outside it (TS-12.R8).
func (s *Store) WorktreesRoot() string {
	return filepath.Join(s.home, dirWorktrees)
}

// EnsureWorktreeRoot validates the project id, ensures the owner-only worktree
// root exists, and returns the leaf path for Git to create. Unlike a project
// resource directory the leaf is deliberately NOT created here: `git worktree
// add` requires a non-existent or empty target and owns the directory it makes.
// An existing leaf that is a symlink is rejected rather than followed, so a
// planted link can never redirect a checkout — or a later deletion — outside
// the owned root (TS-12.R8, INV §14).
func (s *Store) EnsureWorktreeRoot(projectID string) (string, error) {
	leaf, err := s.WorktreePath(projectID)
	if err != nil {
		return "", err
	}
	if err := ensureOwnerDir(s.WorktreesRoot()); err != nil {
		return "", err
	}
	if err := rejectSymlink(leaf); err != nil {
		return "", err
	}
	return leaf, nil
}

// rejectSymlink refuses a path that exists as a symlink. A path that does not
// exist is fine — that is the ordinary case before a checkout is created.
func rejectSymlink(path string) error {
	fi, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("config: stat %q: %w", path, err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("config: %q is a symlink; refusing to follow", path)
	}
	return nil
}
