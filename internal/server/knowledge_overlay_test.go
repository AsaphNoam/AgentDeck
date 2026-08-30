package server

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentdeck/agentdeck/internal/agentknowledge"
	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
)

func envValue(env []string, key string) string {
	prefix := key + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return strings.TrimPrefix(entry, prefix)
		}
	}
	return ""
}

// FS-18.A1/A8, TS-11.R4-R5: the overlay reaches process parameters once but
// remains structurally absent from the frozen session metadata.
func TestKnowledgeOverlayIsRuntimeOnlyAndConditional(t *testing.T) {
	srv := testServer(t, true)
	root := filepath.Join(srv.configStore.Home(), "cache", "agent-skills")
	skillDir := filepath.Join(root, ".agents", "skills", "operating-agentdeck")
	srv.knowledge = agentknowledge.Installation{Available: true, Root: root, SkillDir: skillDir}
	base := runtime.LaunchSpec{
		Agent:        state.Agent{AgentID: "a_test", Name: "Test", Role: "implementer", Project: "my-app"},
		AddDirs:      []string{"/user/dir"},
		SystemPrompt: "base prompt",
		Env:          []string{"BASE=value", "AGENTDECK_SKILL_DIR=shadowed"},
	}

	effective := srv.applyKnowledgeOverlay(srv.applyKnowledgeOverlay(base))
	if got := effective.StartAddDirs(); len(got) != 2 || got[0] != "/user/dir" || got[1] != root {
		t.Fatalf("effective add dirs = %v", got)
	}
	if strings.Count(effective.StartSystemPrompt(), skillDir+"/SKILL.md") != 1 {
		t.Fatalf("effective prompt = %q", effective.StartSystemPrompt())
	}
	if got := envValue(effective.StartEnv(), "AGENTDECK_SKILL_DIR"); got != skillDir {
		t.Fatalf("effective skill env = %q", got)
	}
	meta := runtime.NewSessionMeta(effective, "sess")
	if len(meta.AddDirs) != 1 || meta.AddDirs[0] != "/user/dir" || meta.SystemPrompt != "base prompt" {
		t.Fatalf("overlay leaked into frozen metadata: %+v", meta)
	}
	for _, key := range meta.EnvKeys {
		if key == "AGENTDECK_SKILL_DIR" {
			t.Fatalf("skill env leaked into frozen metadata: %v", meta.EnvKeys)
		}
	}

	srv.knowledge = agentknowledge.Installation{}
	unavailable := srv.applyKnowledgeOverlay(base)
	if got := unavailable.StartAddDirs(); len(got) != 1 || strings.Contains(unavailable.StartSystemPrompt(), "operating-agentdeck") || envValue(unavailable.StartEnv(), "AGENTDECK_SKILL_DIR") != "" {
		t.Fatalf("unavailable overlay changed base spec: %+v", unavailable)
	}
}

func TestKnowledgeOverlayComposesWithSwitchPrimer(t *testing.T) {
	srv := testServer(t, true)
	root := filepath.Join(srv.configStore.Home(), "cache", "agent-skills")
	skillDir := filepath.Join(root, ".agents", "skills", "operating-agentdeck")
	srv.knowledge = agentknowledge.Installation{Available: true, Root: root, SkillDir: skillDir}
	spec := srv.applyKnowledgeOverlay(runtime.LaunchSpec{
		SystemPrompt:        "frozen",
		RuntimeSystemPrompt: "one-shot primer",
	})
	got := spec.StartSystemPrompt()
	if !strings.HasPrefix(got, "one-shot primer\n\n") || !strings.Contains(got, skillDir+"/SKILL.md") || strings.Contains(got, "frozen") {
		t.Fatalf("primer + overlay prompt = %q", got)
	}
}
