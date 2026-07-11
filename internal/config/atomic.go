package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// Generic JSON read/write helpers backing every per-object method.

// writeJSONAtomic encodes v as indented JSON and writes it to path atomically:
// it creates the parent dir, writes to a temp file in the *same* directory
// (so os.Rename stays on one filesystem), fsyncs, closes, then renames over the
// target. On any failure the temp file is removed and no partial file is ever
// visible under the real name.
func writeJSONAtomic(path string, v any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("config: mkdir %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("config: marshal %s: %w", path, err)
	}
	// Trailing newline for POSIX-friendly files and clean diffs.
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("config: create temp in %s: %w", dir, err)
	}
	tmpName := tmp.Name()

	// From here on, ensure the temp file is cleaned up on any error path.
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("config: write temp %s: %w", tmpName, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("config: sync temp %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("config: close temp %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("config: rename %s -> %s: %w", tmpName, path, err)
	}
	// Fsync the parent directory so the rename itself is durable — otherwise a
	// hard crash right after rename can lose a just-written config even though the
	// temp file's contents were synced.
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}

// readJSON reads path and unmarshals into v. A missing file yields ErrNotFound.
// A file that exists but fails to parse is logged at WARN and yields ErrCorrupt.
func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrNotFound
		}
		return fmt.Errorf("config: read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		slog.Warn("config: corrupt file, treating as unreadable", "path", path, "err", err)
		return ErrCorrupt
	}
	return nil
}

// listJSON enumerates *.json files in dir, unmarshalling each into a fresh value
// produced by newElem and appending it via collect. Corrupt files are logged and
// skipped (never fail the whole listing). A missing directory yields an empty
// result with no error.
func listJSON(dir string, perFile func(path string) error) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil // empty layout: nothing listed yet
		}
		return fmt.Errorf("config: readdir %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if filepath.Ext(name) != ".json" {
			continue
		}
		// Skip atomic-write temp files that may briefly exist.
		if len(name) >= 5 && name[:5] == ".tmp-" {
			continue
		}
		if err := perFile(filepath.Join(dir, name)); err != nil {
			// perFile is expected to swallow ErrCorrupt itself; if it returns an
			// error it is a real (non-corrupt) failure worth surfacing.
			return err
		}
	}
	return nil
}

// idFromFilename strips the .json extension to recover the object id.
func idFromFilename(path string) string {
	base := filepath.Base(path)
	return base[:len(base)-len(filepath.Ext(base))]
}
