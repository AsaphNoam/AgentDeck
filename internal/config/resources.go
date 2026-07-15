package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Project shared resources: an AgentDeck-owned, owner-only directory per project
// at $AGENTDECK_HOME/project-resources/{project-id}/ (TS-02.R13, TS-05.R13). It is
// opaque agent/person material — never JSON config, SQLite state, a cache, or an
// index — so this package only creates/validates the directory and never lists,
// reads, writes, deletes, or repairs its contents beyond a single probe file.

// dirProjectResources is the parent directory holding every project's leaf.
const dirProjectResources = "project-resources"

// ProjectResourcesPath returns the canonical absolute resource-directory path for
// a project id without touching disk. It validates the id with the same slug rule
// used for path construction (FS-11.R8), so a caller can safely use the result as
// read-only response metadata (TS-03.R12).
func (s *Store) ProjectResourcesPath(projectID string) (string, error) {
	if !ValidSlug(projectID) {
		return "", fmt.Errorf("config: invalid project id %q", projectID)
	}
	return filepath.Join(s.home, dirProjectResources, projectID), nil
}

// EnsureProjectResources validates the project id, ensures the owner-only
// project-resources parent and the per-project leaf directory exist, proves the
// leaf is writable with a single private zero-byte probe, and returns the absolute
// leaf path. A parent or leaf that already exists as a symlink or non-directory is
// rejected rather than followed (TS-02.R13, TS-05.R13). It never lists, reads,
// writes, deletes, or repairs resource contents.
func (s *Store) EnsureProjectResources(projectID string) (string, error) {
	leaf, err := s.ProjectResourcesPath(projectID)
	if err != nil {
		return "", err
	}
	root := filepath.Join(s.home, dirProjectResources)
	if err := ensureOwnerDir(root); err != nil {
		return "", err
	}
	if err := ensureOwnerDir(leaf); err != nil {
		return "", err
	}
	if err := probeWritable(leaf); err != nil {
		return "", err
	}
	return leaf, nil
}

// ensureOwnerDir makes dir an owner-only (0700) directory, creating it if absent.
// It uses Lstat so an existing symlink is rejected rather than followed, and it
// refuses a non-directory. An existing directory is tightened to 0700 so a looser
// umask or an older build cannot leave it group/world-accessible.
func ensureOwnerDir(dir string) error {
	fi, err := os.Lstat(dir)
	switch {
	case err == nil:
		if fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("config: %q is a symlink; refusing to follow", dir)
		}
		if !fi.IsDir() {
			return fmt.Errorf("config: %q exists but is not a directory", dir)
		}
	case os.IsNotExist(err):
		if mkErr := os.Mkdir(dir, 0o700); mkErr != nil {
			return fmt.Errorf("config: create %q: %w", dir, mkErr)
		}
	default:
		return fmt.Errorf("config: stat %q: %w", dir, err)
	}
	// Guarantee owner-only regardless of umask or a pre-existing looser mode.
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("config: chmod %q: %w", dir, err)
	}
	return nil
}

// probeWritable proves dir is writable by creating and removing one private
// (0600) zero-byte probe file. It is the only content this package ever writes
// under a resource directory.
func probeWritable(dir string) error {
	f, err := os.CreateTemp(dir, ".probe-*")
	if err != nil {
		return fmt.Errorf("config: project resources %q not writable: %w", dir, err)
	}
	name := f.Name()
	_ = f.Close()
	if err := os.Remove(name); err != nil {
		return fmt.Errorf("config: remove probe %q: %w", name, err)
	}
	return nil
}
