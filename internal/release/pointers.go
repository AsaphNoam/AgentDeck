package release

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrNoPrevious is returned by Rollback when no previous version is retained.
var ErrNoPrevious = errors.New("no previous version to roll back to")

// Current returns the active version directory name and true, or ("", false) when
// no current pointer exists yet.
func (l *Layout) Current() (string, bool, error) {
	return l.readPointer(l.CurrentLink())
}

// Previous returns the retained previous version directory name, or ("", false).
func (l *Layout) Previous() (string, bool, error) {
	return l.readPointer(l.PreviousLink())
}

// readPointer resolves a current/previous symlink to the bare version directory
// name it selects. A missing pointer is not an error.
func (l *Layout) readPointer(link string) (string, bool, error) {
	target, err := os.Readlink(link)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("read pointer %s: %w", link, err)
	}
	return filepath.Base(target), true, nil
}

// Activate makes name the current version, recording the previously-current
// version as previous. It verifies the version directory exists first, then
// flips previous before current so a crash mid-swap always leaves a valid
// current pointer intact (TS-06.R18). It never signals a running dashboard.
func (l *Layout) Activate(name string) error {
	if _, err := os.Stat(l.VersionDir(name)); err != nil {
		return fmt.Errorf("activate %s: version directory not present: %w", name, err)
	}
	old, hadOld, err := l.Current()
	if err != nil {
		return err
	}
	// Record the outgoing current as previous first. If this is the initial
	// install (no old) or a re-activation of the same version, there is nothing
	// to retain. current is always flipped last, so it never points at a
	// half-installed runtime.
	if hadOld && old != name {
		if err := l.swapPointer(l.PreviousLink(), old); err != nil {
			return fmt.Errorf("record previous: %w", err)
		}
	}
	if err := l.swapPointer(l.CurrentLink(), name); err != nil {
		return fmt.Errorf("set current: %w", err)
	}
	return nil
}

// Rollback atomically restores the retained previous version as current, and
// records the version it replaced as the new previous, so a rollback can itself
// be undone. current is flipped before previous so any crash leaves a valid
// current pointer (TS-06.R18).
func (l *Layout) Rollback() error {
	prev, hasPrev, err := l.Previous()
	if err != nil {
		return err
	}
	if !hasPrev {
		return ErrNoPrevious
	}
	if _, statErr := os.Stat(l.VersionDir(prev)); statErr != nil {
		return fmt.Errorf("rollback: previous version %s missing: %w", prev, statErr)
	}
	cur, hadCur, err := l.Current()
	if err != nil {
		return err
	}
	if err := l.swapPointer(l.CurrentLink(), prev); err != nil {
		return fmt.Errorf("rollback set current: %w", err)
	}
	if hadCur && cur != prev {
		if err := l.swapPointer(l.PreviousLink(), cur); err != nil {
			return fmt.Errorf("rollback record previous: %w", err)
		}
	}
	return nil
}

// swapPointer atomically points link at versions/<name>. It writes a uniquely
// named temporary symlink in the root then renames it over any existing pointer
// (rename replaces atomically on the same directory), and fsyncs the parent so
// the swap survives a crash (INV §9: atomic-write-via-rename fsyncs the dir).
func (l *Layout) swapPointer(link, name string) error {
	target := filepath.Join("versions", name) // relative to the application root
	tmp, err := os.CreateTemp(l.root, ".pointer-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	tmp.Close()
	// CreateTemp made a regular file; remove it so Symlink can take the name.
	if err := os.Remove(tmpName); err != nil {
		return err
	}
	if err := os.Symlink(target, tmpName); err != nil {
		return err
	}
	if err := os.Rename(tmpName, link); err != nil {
		os.Remove(tmpName)
		return err
	}
	return fsyncDir(l.root)
}

// fsyncDir flushes a directory entry change (a rename) to disk.
func fsyncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	// Directory fsync is not supported on every platform/filesystem; a failure
	// here does not undo the rename, so treat an unsupported call as best-effort.
	if err := d.Sync(); err != nil && !errors.Is(err, os.ErrInvalid) {
		return err
	}
	return nil
}
