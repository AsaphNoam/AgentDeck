package store

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
)

// Agent CRUD plus stable-id minting.

// ReadAgent reads agents/{id}.json. Returns ErrNotFound if absent, ErrCorrupt if
// unparseable.
func (s *Store) ReadAgent(id string) (Agent, error) {
	var a Agent
	err := readJSON(s.agentPath(id), &a)
	return a, err
}

// WriteAgent atomically writes agents/{id}.json keyed by a.AgentID.
func (s *Store) WriteAgent(a Agent) error {
	return writeJSONAtomic(s.agentPath(a.AgentID), a)
}

// ListAgents returns every parseable agent. Corrupt files are logged and skipped;
// a single bad file never fails the whole listing.
func (s *Store) ListAgents() ([]Agent, error) {
	out := []Agent{}
	err := listJSON(s.dirPath(dirAgents), func(path string) error {
		var a Agent
		if err := readJSON(path, &a); err != nil {
			if errors.Is(err, ErrCorrupt) {
				return nil // already logged; skip
			}
			return err
		}
		out = append(out, a)
		return nil
	})
	return out, err
}

// DeleteAgent removes agents/{id}.json. A missing file is tolerated (nil).
func (s *Store) DeleteAgent(id string) error {
	if err := os.Remove(s.agentPath(id)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("store: delete agent %s: %w", id, err)
	}
	return nil
}

// NewAgentID generates "a_" + 6 lowercase hex chars from crypto/rand, retrying
// on collision against an existing agents/{id}.json (up to 10 tries).
func (s *Store) NewAgentID() (string, error) {
	for i := 0; i < 10; i++ {
		var b [3]byte // 3 bytes => 6 hex chars
		if _, err := rand.Read(b[:]); err != nil {
			return "", fmt.Errorf("store: read random: %w", err)
		}
		id := "a_" + hex.EncodeToString(b[:])
		if _, err := os.Stat(s.agentPath(id)); errors.Is(err, os.ErrNotExist) {
			return id, nil
		} else if err != nil {
			return "", fmt.Errorf("store: stat candidate id %s: %w", id, err)
		}
		// collision: retry
	}
	return "", errors.New("store: could not mint unique agent_id after 10 tries")
}
