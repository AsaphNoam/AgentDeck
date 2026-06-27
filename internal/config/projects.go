package config

import (
	"errors"
	"fmt"
	"os"
)

// Project CRUD: projects/{project}.json. The id is the filename stem.

// ReadProject reads projects/{id}.json.
func (s *Store) ReadProject(id string) (Project, error) {
	var p Project
	err := readJSON(s.projectPath(id), &p)
	return p, err
}

// WriteProject atomically writes projects/{id}.json. The id is supplied
// separately (the filename is the id).
func (s *Store) WriteProject(id string, p Project) error {
	return writeJSONAtomic(s.projectPath(id), p)
}

// ListProjects returns a map of project id → Project, skipping corrupt files.
// The map is the wire shape for GET /api/projects.
func (s *Store) ListProjects() (map[string]Project, error) {
	out := map[string]Project{}
	err := listJSON(s.dirPath(dirProjects), func(path string) error {
		var p Project
		if err := readJSON(path, &p); err != nil {
			if errors.Is(err, ErrCorrupt) {
				return nil
			}
			return err
		}
		out[idFromFilename(path)] = p
		return nil
	})
	return out, err
}

// DeleteProject removes projects/{id}.json; a missing file is tolerated.
func (s *Store) DeleteProject(id string) error {
	if err := os.Remove(s.projectPath(id)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("config: delete project %s: %w", id, err)
	}
	return nil
}
