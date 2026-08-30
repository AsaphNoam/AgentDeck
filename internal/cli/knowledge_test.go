package cli

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/agentdeck/agentdeck/internal/config"
)

// FS-18.A8: a failed secure installation warns, advertises no package, and a
// later clean dashboard preparation retries successfully.
func TestPrepareAgentKnowledgeFailureThenRetry(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions vary on Windows")
	}
	home := t.TempDir()
	t.Setenv("AGENTDECK_HOME", home)
	store, err := config.New()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureLayout(); err != nil {
		t.Fatal(err)
	}
	legacyData, err := os.ReadFile(filepath.Join("..", "config", "testdata", "legacy_agentdecker_prompt.txt"))
	if err != nil {
		t.Fatal(err)
	}
	legacy := strings.TrimSuffix(string(legacyData), "\n")
	original := config.Role{Title: "Custom", SystemPrompt: legacy, SkipPermissions: nil}
	if err := store.WriteRole("agentdecker", original); err != nil {
		t.Fatal(err)
	}

	target := t.TempDir()
	if err := os.Symlink(target, filepath.Join(home, "cache")); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	failed := prepareAgentKnowledge(store, logger)
	if failed.Available || !strings.Contains(logs.String(), "continuing without it") {
		t.Fatalf("failed preparation = %+v, logs=%s", failed, logs.String())
	}
	role, err := store.ReadRole("agentdecker")
	if err != nil || role != original {
		t.Fatalf("install failure changed role: %+v, %v", role, err)
	}

	if err := os.Remove(filepath.Join(home, "cache")); err != nil {
		t.Fatal(err)
	}
	succeeded := prepareAgentKnowledge(store, logger)
	if !succeeded.Available {
		t.Fatalf("retry remained unavailable: %+v", succeeded)
	}
	if _, err := os.Stat(filepath.Join(succeeded.SkillDir, "SKILL.md")); err != nil {
		t.Fatalf("retry did not publish direct skill path: %v", err)
	}
	role, err = store.ReadRole("agentdecker")
	if err != nil || role.Title != original.Title || role.SystemPrompt == legacy || !strings.Contains(role.SystemPrompt, "AgentDeck's resident operator") {
		t.Fatalf("retry did not migrate the exact legacy role: %+v, %v", role, err)
	}
}
