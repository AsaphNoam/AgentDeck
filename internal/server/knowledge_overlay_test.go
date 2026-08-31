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

// FS-18.A1, TS-11.R4-R5, INV §6/§10: the helper tests above prove the seam's own
// arithmetic; this proves every lifecycle path that starts or restarts an agent process
// still routes through it. Without the matrix a future composer, wake, or pipeline path
// could stop delivering the package while the helper tests stayed green.
func TestKnowledgeOverlayReachesEveryLifecycleComposer(t *testing.T) {
	srv, ts := switchTestServer(t)
	root := filepath.Join(srv.configStore.Home(), "cache", "agent-skills")
	skillDir := filepath.Join(root, ".agents", "skills", "operating-agentdeck")

	id := launchAndWaitIdle(t, ts, "impl", "tmpproj")
	agent, err := srv.stateStore.ReadAgent(id)
	if err != nil {
		t.Fatalf("ReadAgent: %v", err)
	}
	snap, err := srv.stateStore.ReadSession(id)
	if err != nil {
		t.Fatalf("ReadSession: %v", err)
	}
	backends, err := srv.configStore.ReadBackends()
	if err != nil {
		t.Fatalf("ReadBackends: %v", err)
	}
	be := backends.Backends[agent.Backend]
	model := be.Models[agent.Model]
	terminalAgent := agent
	terminalAgent.Interface = "terminal"
	terminalSnap := snap
	terminalSnap.Interface = "terminal"

	// Wake resume and the pipeline's continuation both run the ordinary resume flow
	// (resume.go), so they are listed by the generation each one supplies rather than
	// as separate seams: the point of the entry is that its lifecycle still lands here.
	composers := []struct {
		lifecycle string
		compose   func(*testing.T) runtime.LaunchSpec
	}{
		{"fresh chat launch", func(t *testing.T) runtime.LaunchSpec {
			spec, _, ae := srv.composeLaunch(t.Context(), launchRequest{Role: "impl", Project: "tmpproj"})
			return mustCompose(t, spec, ae)
		}},
		{"fresh terminal launch", func(t *testing.T) runtime.LaunchSpec {
			spec, _, ae := srv.composeLaunch(t.Context(), launchRequest{Role: "impl", Project: "tmpproj", Interface: "terminal"})
			return mustCompose(t, spec, ae)
		}},
		{"pipeline-started launch", func(t *testing.T) runtime.LaunchSpec {
			spec, _, ae := srv.composeLaunchWithOptions(t.Context(), launchRequest{Role: "impl", Project: "tmpproj", Interface: "chat"}, launchOptions{Generation: "gen_stage"})
			return mustCompose(t, spec, ae)
		}},
		{"ordinary resume", func(t *testing.T) runtime.LaunchSpec {
			spec, ae := srv.composeResumeSpec(agent, snap, be, model)
			return mustCompose(t, spec, ae)
		}},
		{"terminal resume", func(t *testing.T) runtime.LaunchSpec {
			spec, ae := srv.composeResumeSpec(terminalAgent, terminalSnap, be, model)
			return mustCompose(t, spec, ae)
		}},
		{"wake resume", func(t *testing.T) runtime.LaunchSpec {
			spec, ae := srv.composeResumeSpecWithGeneration(agent, snap, be, model, "")
			return mustCompose(t, spec, ae)
		}},
		{"pipeline continuation resume", func(t *testing.T) runtime.LaunchSpec {
			spec, ae := srv.composeResumeSpecWithGeneration(agent, snap, be, model, "gen_continue")
			return mustCompose(t, spec, ae)
		}},
		{"runtime switch", func(t *testing.T) runtime.LaunchSpec {
			spec, ae := srv.composeSwitchSpec(agent, "")
			return mustCompose(t, spec, ae)
		}},
		{"terminal runtime switch", func(t *testing.T) runtime.LaunchSpec {
			spec, ae := srv.composeSwitchSpec(terminalAgent, "")
			return mustCompose(t, spec, ae)
		}},
	}

	for _, tc := range composers {
		t.Run(tc.lifecycle+" with the package available", func(t *testing.T) {
			srv.knowledge = agentknowledge.Installation{Available: true, Root: root, SkillDir: skillDir}
			spec := tc.compose(t)
			if got := countStr(spec.StartAddDirs(), root); got != 1 {
				t.Errorf("managed dir appears %d times in %v", got, spec.StartAddDirs())
			}
			if got := strings.Count(spec.StartSystemPrompt(), skillDir+"/SKILL.md"); got != 1 {
				t.Errorf("pointer appears %d times in %q", got, spec.StartSystemPrompt())
			}
			if got := envValue(spec.StartEnv(), "AGENTDECK_SKILL_DIR"); got != skillDir {
				t.Errorf("AGENTDECK_SKILL_DIR = %q, want %q", got, skillDir)
			}
			meta := runtime.NewSessionMeta(spec, "sess")
			if countStr(meta.AddDirs, root) != 0 || strings.Contains(meta.SystemPrompt, skillDir) {
				t.Errorf("overlay leaked into frozen metadata: %+v", meta)
			}
			for _, key := range meta.EnvKeys {
				if key == "AGENTDECK_SKILL_DIR" {
					t.Errorf("skill env leaked into frozen metadata: %v", meta.EnvKeys)
				}
			}
		})

		t.Run(tc.lifecycle+" with the package unavailable", func(t *testing.T) {
			srv.knowledge = agentknowledge.Installation{}
			spec := tc.compose(t)
			if countStr(spec.StartAddDirs(), root) != 0 ||
				strings.Contains(spec.StartSystemPrompt(), skillDir) ||
				envValue(spec.StartEnv(), "AGENTDECK_SKILL_DIR") != "" {
				t.Errorf("unavailable package still composed an overlay: %+v", spec)
			}
		})
	}
}

func mustCompose(t *testing.T, spec runtime.LaunchSpec, ae *runtime.APIError) runtime.LaunchSpec {
	t.Helper()
	if ae != nil {
		t.Fatalf("compose failed: %s", ae.Message)
	}
	return spec
}

func countStr(values []string, want string) int {
	n := 0
	for _, value := range values {
		if value == want {
			n++
		}
	}
	return n
}
