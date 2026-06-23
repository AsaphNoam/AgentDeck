package store

import (
	"os"
	"path/filepath"
	"strings"
)

// Home-directory resolution and per-object path builders.
//
// Resolution rules (resolveHome):
//  1. If $AGENTDECK_HOME is set and non-empty → use it (expanding a leading ~).
//  2. Else → filepath.Join(userHomeDir, ".agentdeck").
//
// A leading "~" inside stored paths (e.g. project.cwd) is expanded by
// ExpandTilde() on read by *callers*, never by the store on write — the store
// persists paths verbatim.

// envHome is the override environment variable name.
const envHome = "AGENTDECK_HOME"

// resolveHome computes the absolute home directory per the rules above. It does
// not create anything on disk.
func resolveHome() (string, error) {
	if h := os.Getenv(envHome); h != "" {
		return ExpandTilde(h)
	}
	u, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(u, ".agentdeck"), nil
}

// ExpandTilde expands a leading "~" or "~/" in p to the user's home directory.
// An absolute or already-expanded path is returned unchanged. The "~user" form
// is not supported and is returned unexpanded (documented in the tech spec §7).
func ExpandTilde(p string) (string, error) {
	if p == "~" {
		return os.UserHomeDir()
	}
	if strings.HasPrefix(p, "~/") {
		u, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(u, p[2:]), nil
	}
	// "~user" or any non-tilde path: return unchanged.
	return p, nil
}

// Subdirectory and file names under the home directory. Centralised so every
// path builder and EnsureLayout agree on the layout.
const (
	dirAgents   = "agents"
	dirRunning  = "running"
	dirStatus   = "status"
	dirRoles    = "roles"
	dirProjects = "projects"
	dirMessages = "messages"
	dirSessions = "sessions"

	fileBackends = "backends.json"
	fileConfig   = "config.json"
	fileLayout   = "layout.json"
)

// dataDirs is every directory EnsureLayout creates under home.
var dataDirs = []string{
	dirAgents,
	dirRunning,
	dirStatus,
	dirRoles,
	dirProjects,
	dirMessages,
	dirSessions,
}

// objPath returns {home}/{dir}/{id}.json for a keyed object.
func (s *Store) objPath(dir, id string) string {
	return filepath.Join(s.home, dir, id+".json")
}

// dirPath returns {home}/{dir}.
func (s *Store) dirPath(dir string) string {
	return filepath.Join(s.home, dir)
}

// filePath returns {home}/{name} for a single-file object.
func (s *Store) filePath(name string) string {
	return filepath.Join(s.home, name)
}

func (s *Store) agentPath(id string) string   { return s.objPath(dirAgents, id) }
func (s *Store) runningPath(id string) string { return s.objPath(dirRunning, id) }
func (s *Store) statusPath(id string) string  { return s.objPath(dirStatus, id) }
func (s *Store) rolePath(id string) string    { return s.objPath(dirRoles, id) }
func (s *Store) projectPath(id string) string { return s.objPath(dirProjects, id) }

func (s *Store) backendsPath() string { return s.filePath(fileBackends) }
func (s *Store) configPath() string   { return s.filePath(fileConfig) }
func (s *Store) layoutPath() string   { return s.filePath(fileLayout) }
