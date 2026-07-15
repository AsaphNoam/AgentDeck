package release

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// ErrLocked is returned when another installer or updater already holds the
// install lock. A contender exits without changing the selected runtime
// (FS-10.R13, TS-06.R19).
var ErrLocked = errors.New("another install or update is in progress")

// Lock is a held install lock; call Release when the transaction completes.
type Lock struct{ f *os.File }

// Lock takes the exclusive, non-blocking install lock for this application root.
// The OS releases a flock automatically if the holder crashes, so a stale lock
// never wedges future installs. Returns ErrLocked when a live contender holds it.
func (l *Layout) Lock() (*Lock, error) {
	if err := l.EnsureLayout(); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(l.LockPath(), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrLocked
		}
		return nil, fmt.Errorf("acquire lock: %w", err)
	}
	return &Lock{f: f}, nil
}

// Release unlocks and closes the lock file.
func (lk *Lock) Release() error {
	if lk == nil || lk.f == nil {
		return nil
	}
	err := syscall.Flock(int(lk.f.Fd()), syscall.LOCK_UN)
	if cerr := lk.f.Close(); err == nil {
		err = cerr
	}
	lk.f = nil
	return err
}
