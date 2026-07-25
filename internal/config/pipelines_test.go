package config

import (
	"errors"
	"os"
	"reflect"
	"testing"
)

func TestPipelineFileStorageIsBoundedAtomicAndListed(t *testing.T) {
	s := newTestStore(t)
	doc := map[string]any{"version": 1, "title": "Quality", "stages": []any{}}
	if err := s.WritePipelineJSON("quality", doc); err != nil {
		t.Fatalf("WritePipelineJSON: %v", err)
	}
	data, err := s.ReadPipelineFile("quality", 1024)
	if err != nil {
		t.Fatalf("ReadPipelineFile: %v", err)
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Fatalf("pipeline JSON is not newline terminated: %q", data)
	}
	ids, err := s.ListPipelineIDs()
	if err != nil {
		t.Fatalf("ListPipelineIDs: %v", err)
	}
	if !reflect.DeepEqual(ids, []string{"quality"}) {
		t.Fatalf("pipeline ids = %v, want [quality]", ids)
	}
	if _, err := s.ReadPipelineFile("quality", 8); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("bounded read error = %v, want ErrTooLarge", err)
	}
	if info, err := os.Stat(s.pipelinePath("quality")); err != nil {
		t.Fatal(err)
	} else if info.Mode().Perm() != 0o600 {
		t.Fatalf("pipeline mode = %o, want 600", info.Mode().Perm())
	}
	if err := s.DeletePipelineFile("quality"); err != nil {
		t.Fatalf("DeletePipelineFile: %v", err)
	}
	if err := s.DeletePipelineFile("quality"); err != nil {
		t.Fatalf("DeletePipelineFile missing: %v", err)
	}
}

func TestPipelineFileStorageRejectsUnsafeID(t *testing.T) {
	s := newTestStore(t)
	if err := s.WritePipelineJSON("../escape", map[string]int{"version": 1}); err == nil {
		t.Fatal("WritePipelineJSON accepted unsafe id")
	}
	if _, err := s.ReadPipelineFile("../escape", 1024); err == nil {
		t.Fatal("ReadPipelineFile accepted unsafe id")
	}
}
