package server

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain makes provider selection hermetic: launch tests use a fake ACP
// adapter, but the shared resolver still requires a selectable Claude/Codex
// executable (TS-04.R75). Ambient overrides point at inert stand-ins so the
// suite neither depends on nor finds a developer's installed CLIs. Tests of
// selection itself override these with t.Setenv.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "agentdeck-providers-")
	if err != nil {
		panic(err)
	}
	for key, name := range map[string]string{"CLAUDE_CODE_EXECUTABLE": "claude", "CODEX_PATH": "codex"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			panic(err)
		}
		os.Setenv(key, path)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
