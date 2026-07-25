package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ReadPipelineFile returns one raw pipeline-template JSON document through a
// bounded reader. Decoding and semantic validation belong to internal/pipeline;
// config owns only the validated path and owner-only storage boundary.
func (s *Store) ReadPipelineFile(id string, maxBytes int64) ([]byte, error) {
	if !ValidSlug(id) {
		return nil, fmt.Errorf("config: invalid pipeline id %q", id)
	}
	f, err := os.Open(s.pipelinePath(id))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("config: read pipeline %s: %w", id, err)
	}
	defer f.Close()

	if maxBytes <= 0 {
		return nil, fmt.Errorf("config: invalid pipeline read limit %d", maxBytes)
	}
	data, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("config: read pipeline %s: %w", id, err)
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("pipeline %s exceeds %d bytes: %w", id, maxBytes, ErrTooLarge)
	}
	return data, nil
}

// WritePipelineJSON atomically writes one template through the same durable,
// owner-only path used by the other JSON configuration families.
func (s *Store) WritePipelineJSON(id string, value any) error {
	if !ValidSlug(id) {
		return fmt.Errorf("config: invalid pipeline id %q", id)
	}
	return writeJSONAtomic(s.pipelinePath(id), value)
}

// ListPipelineIDs lists every JSON filename stem, including semantically
// invalid hand-edited templates so the pipeline service can return diagnostics.
func (s *Store) ListPipelineIDs() ([]string, error) {
	entries, err := os.ReadDir(s.dirPath(dirPipelines))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("config: list pipelines: %w", err)
	}
	out := []string{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		if len(entry.Name()) >= 5 && entry.Name()[:5] == ".tmp-" {
			continue
		}
		out = append(out, idFromFilename(entry.Name()))
	}
	return out, nil
}

// DeletePipelineFile removes one template file; a missing file is tolerated.
func (s *Store) DeletePipelineFile(id string) error {
	if !ValidSlug(id) {
		return fmt.Errorf("config: invalid pipeline id %q", id)
	}
	if err := os.Remove(s.pipelinePath(id)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("config: delete pipeline %s: %w", id, err)
	}
	return nil
}
