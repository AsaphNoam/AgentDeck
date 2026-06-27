package state

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// Store owns the SQLite state database.
type Store struct {
	db *sql.DB
}

// Open opens home/state.db, enables WAL and foreign keys, applies migrations,
// and returns the typed state store. The home directory is created if absent.
func Open(home string) (*Store, error) {
	if home == "" {
		return nil, fmt.Errorf("state: home is empty")
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		return nil, fmt.Errorf("state: create home: %w", err)
	}

	dbPath := filepath.Join(home, "state.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("state: open db: %w", err)
	}
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		db.Close()
		return nil, fmt.Errorf("state: enable wal: %w", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		db.Close()
		return nil, fmt.Errorf("state: enable foreign keys: %w", err)
	}
	if _, err := db.Exec(`PRAGMA busy_timeout=5000`); err != nil {
		db.Close()
		return nil, fmt.Errorf("state: set busy timeout: %w", err)
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

// Close closes the underlying database handle.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// DB exposes the underlying database for later phases and narrow tests.
func (s *Store) DB() *sql.DB {
	return s.db
}
