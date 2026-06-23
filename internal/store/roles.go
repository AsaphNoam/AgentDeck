package store

import (
	"errors"
	"fmt"
	"os"
)

// Role CRUD: roles/{role}.json. The id is the filename stem (filename-as-id).

// ReadRole reads roles/{id}.json.
func (s *Store) ReadRole(id string) (Role, error) {
	var r Role
	err := readJSON(s.rolePath(id), &r)
	return r, err
}

// WriteRole atomically writes roles/{id}.json. The id is supplied separately
// because Role has no id field (the filename is the id).
func (s *Store) WriteRole(id string, r Role) error {
	return writeJSONAtomic(s.rolePath(id), r)
}

// ListRoles returns a map of role id → Role, skipping corrupt files. The map is
// the wire shape for GET /api/roles (filename-as-id, matching PRD JSON).
func (s *Store) ListRoles() (map[string]Role, error) {
	out := map[string]Role{}
	err := listJSON(s.dirPath(dirRoles), func(path string) error {
		var r Role
		if err := readJSON(path, &r); err != nil {
			if errors.Is(err, ErrCorrupt) {
				return nil
			}
			return err
		}
		out[idFromFilename(path)] = r
		return nil
	})
	return out, err
}

// DeleteRole removes roles/{id}.json; a missing file is tolerated.
func (s *Store) DeleteRole(id string) error {
	if err := os.Remove(s.rolePath(id)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("store: delete role %s: %w", id, err)
	}
	return nil
}
