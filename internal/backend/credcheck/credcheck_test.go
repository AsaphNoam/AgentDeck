package credcheck

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentdeck/agentdeck/internal/config"
)

func TestMergeEnv(t *testing.T) {
	// Model env overrides backend env on conflict; backend keys survive.
	merged := MergeEnv(
		map[string]string{"A": "backend", "B": "backend"},
		map[string]string{"B": "model", "C": "model"},
	)
	if merged["A"] != "backend" {
		t.Errorf("A = %q, want backend", merged["A"])
	}
	if merged["B"] != "model" {
		t.Errorf("B = %q, want model (model wins)", merged["B"])
	}
	if merged["C"] != "model" {
		t.Errorf("C = %q, want model", merged["C"])
	}
}

func TestMergeEnvEmpty(t *testing.T) {
	if got := MergeEnv(nil, nil); len(got) != 0 {
		t.Errorf("nil+nil merge = %v, want empty", got)
	}
	if got := MergeEnv(map[string]string{"X": "1"}, nil); got["X"] != "1" {
		t.Errorf("X = %q, want 1", got["X"])
	}
}

// mockProber implements Prober for testing.
type mockProber struct {
	result CredResult
}

func (m mockProber) Check(_ context.Context, _ config.Backend, _ config.Model, _ map[string]string) CredResult {
	return m.result
}

func TestCheckDispatchUnknownType(t *testing.T) {
	bk := config.Backend{Type: "unknown-acp"}
	result := Check(context.Background(), bk, config.Model{}, nil)
	if result.Status != "skipped" {
		t.Errorf("unknown type status = %q, want skipped", result.Status)
	}
}

func TestCheckWithMockProber(t *testing.T) {
	// Temporarily register a mock for a fake backend type.
	orig := probers
	defer func() { probers = orig }()

	probers = map[string]Prober{
		"test-acp": mockProber{result: CredResult{Status: "ok"}},
	}
	bk := config.Backend{Type: "test-acp"}
	result := Check(context.Background(), bk, config.Model{}, nil)
	if result.Status != "ok" {
		t.Errorf("status = %q, want ok", result.Status)
	}
}

func TestClaudeProberRetriesWithoutNoColor(t *testing.T) {
	dir := t.TempDir()
	cliPath := filepath.Join(dir, "claude")
	script := `#!/bin/sh
if [ "$1" = "auth" ] && [ "$2" = "status" ] && [ "$3" = "--no-color" ]; then
  echo "error: unknown option '--no-color'" >&2
  exit 1
fi
if [ "$1" = "auth" ] && [ "$2" = "status" ]; then
  echo "logged in"
  exit 0
fi
echo "unexpected args: $@" >&2
exit 2
`
	if err := os.WriteFile(cliPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake claude: %v", err)
	}

	result := claudeProber{}.Check(
		context.Background(),
		config.Backend{},
		config.Model{},
		map[string]string{"CLAUDE_PATH": cliPath},
	)
	if result.Status != "ok" {
		t.Fatalf("status = %q, want ok (detail=%q)", result.Status, result.Detail)
	}
}
