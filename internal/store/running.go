package store

import (
	"errors"
	"fmt"
	"os"
)

// RunningEntry CRUD: running/{agent_id}.json.

// ReadRunning reads running/{id}.json.
func (s *Store) ReadRunning(id string) (RunningEntry, error) {
	var r RunningEntry
	err := readJSON(s.runningPath(id), &r)
	return r, err
}

// WriteRunning atomically writes running/{id}.json keyed by r.AgentID.
func (s *Store) WriteRunning(r RunningEntry) error {
	return writeJSONAtomic(s.runningPath(r.AgentID), r)
}

// ListRunning returns every parseable running entry, skipping corrupt files.
func (s *Store) ListRunning() ([]RunningEntry, error) {
	out := []RunningEntry{}
	err := listJSON(s.dirPath(dirRunning), func(path string) error {
		var r RunningEntry
		if err := readJSON(path, &r); err != nil {
			if errors.Is(err, ErrCorrupt) {
				return nil
			}
			return err
		}
		out = append(out, r)
		return nil
	})
	return out, err
}

// DeleteRunning removes running/{id}.json; a missing file is tolerated.
func (s *Store) DeleteRunning(id string) error {
	if err := os.Remove(s.runningPath(id)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("store: delete running %s: %w", id, err)
	}
	return nil
}
