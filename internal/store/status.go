package store

import (
	"errors"
	"fmt"
	"os"
)

// Status CRUD: status/{agent_id}.json.

// ReadStatus reads status/{id}.json.
func (s *Store) ReadStatus(id string) (Status, error) {
	var st Status
	err := readJSON(s.statusPath(id), &st)
	return st, err
}

// WriteStatus atomically writes status/{id}.json keyed by st.AgentID.
func (s *Store) WriteStatus(st Status) error {
	return writeJSONAtomic(s.statusPath(st.AgentID), st)
}

// ListStatus returns every parseable status, skipping corrupt files.
func (s *Store) ListStatus() ([]Status, error) {
	out := []Status{}
	err := listJSON(s.dirPath(dirStatus), func(path string) error {
		var st Status
		if err := readJSON(path, &st); err != nil {
			if errors.Is(err, ErrCorrupt) {
				return nil
			}
			return err
		}
		out = append(out, st)
		return nil
	})
	return out, err
}

// DeleteStatus removes status/{id}.json; a missing file is tolerated.
func (s *Store) DeleteStatus(id string) error {
	if err := os.Remove(s.statusPath(id)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("store: delete status %s: %w", id, err)
	}
	return nil
}
