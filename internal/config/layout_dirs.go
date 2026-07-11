package config

import (
	"fmt"
	"os"
)

// EnsureLayout creates every directory of the ~/.agentdeck/ layout (§3). It is
// idempotent (mkdir -p semantics) and never deletes or overwrites existing data.
// If home exists but is a regular file (not a directory) it returns a clear error.
func (s *Store) EnsureLayout() error {
	if fi, err := os.Stat(s.home); err == nil {
		if !fi.IsDir() {
			return fmt.Errorf("config: home %q exists but is not a directory", s.home)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("config: stat home %q: %w", s.home, err)
	}

	if err := os.MkdirAll(s.home, 0o700); err != nil {
		return fmt.Errorf("config: create home %q: %w", s.home, err)
	}
	// The home tree holds secrets (backend env/API keys, hook + MCP tokens,
	// transcripts), so it is owner-only. MkdirAll never re-modes an existing
	// dir, so tighten homes created by older builds explicitly.
	if err := os.Chmod(s.home, 0o700); err != nil {
		return fmt.Errorf("config: chmod home %q: %w", s.home, err)
	}
	for _, d := range dataDirs {
		p := s.dirPath(d)
		if err := os.MkdirAll(p, 0o700); err != nil {
			return fmt.Errorf("config: create dir %q: %w", p, err)
		}
	}
	return nil
}
