package store

// Store is the typed file-store over ~/.agentdeck/. It holds only the resolved,
// absolute home directory; all state lives on disk. A *Store is safe for
// concurrent use to the extent the filesystem is: writes are atomic
// (write-temp-then-rename) so readers never observe partial files.
type Store struct {
	home string // absolute, resolved from AGENTDECK_HOME or ~/.agentdeck
}

// New resolves the home directory (honoring AGENTDECK_HOME and leading "~") but
// does NOT create any directories. Call EnsureLayout to create the layout.
func New() (*Store, error) {
	home, err := resolveHome()
	if err != nil {
		return nil, err
	}
	return &Store{home: home}, nil
}

// NewWithHome builds a Store rooted at an explicit, already-resolved directory.
// Primarily for tests that want a fixed home without touching the environment.
func NewWithHome(home string) *Store {
	return &Store{home: home}
}

// Home returns the absolute home directory.
func (s *Store) Home() string { return s.home }
