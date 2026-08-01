package configsource

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentdeck/agentdeck/internal/config"
)

// FS-08.A11 (R34) / TS-07.R16 — a connection made with no project resolves only
// the provider's user-level source. The dangerous shape this pins is an empty
// project root canonicalizing to the *server process's* working directory: that
// would both read and approve an unrelated tree.

func TestClaudeResolverWithoutProjectReadsOnlyUserLayer(t *testing.T) {
	home, project := claudeFixture(t)
	root := filepath.Join(home, ".claude")
	writeClaudeTestFile(t, filepath.Join(root, "settings.json"), `{"model":"user-model"}`)
	// Present but out of scope for a project-free resolution.
	writeClaudeTestFile(t, filepath.Join(project, ".claude", "settings.json"), `{"model":"project-model"}`)

	binding := Binding{Provider: ProviderClaude, Mode: ModeLinked, Root: root,
		Claims: []string{"launch_defaults", "model_catalog", "setup"}, Approved: []string{root}}
	effective, report, err := NewClaudeResolver(home).Resolve(context.Background(), binding, config.Project{})
	if err != nil {
		t.Fatalf("Resolve: %v report=%+v", err, report)
	}
	if effective.Model == nil || *effective.Model != "user-model" {
		t.Fatalf("model = %v, want user-model", effective.Model)
	}
	assertNoProjectScope(t, report)
}

func TestCodexResolverWithoutProjectReadsOnlyUserLayer(t *testing.T) {
	home, root, project := codexFixture(t)
	writeCodexTestFile(t, filepath.Join(root, "config.toml"), "model = \"user-model\"\n")
	writeCodexTestFile(t, filepath.Join(project, ".codex", "config.toml"), "model = \"project-model\"\n")

	binding := Binding{Provider: ProviderCodex, Mode: ModeLinked, Root: root,
		Claims: []string{"launch_defaults", "model_catalog", "setup"}, Approved: []string{root}}
	effective, report, err := NewCodexResolver(CodexOptions{UserHome: home, CodexHome: root}).
		Resolve(context.Background(), binding, config.Project{})
	if err != nil {
		t.Fatalf("Resolve: %v report=%+v", err, report)
	}
	if effective.Model == nil || *effective.Model != "user-model" {
		t.Fatalf("model = %v, want user-model", effective.Model)
	}
	assertNoProjectScope(t, report)
}

// assertNoProjectScope fails when a project-free resolution reported a
// project-scoped read, a project-scoped skip, or an approved root outside the
// source root — the observable symptoms of an empty project root resolving to
// the process working directory.
func assertNoProjectScope(t *testing.T, report Report) {
	t.Helper()
	cwd, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	if cwd, err = filepath.EvalSymlinks(cwd); err != nil {
		t.Fatal(err)
	}
	for _, fp := range report.Fingerprints {
		if strings.HasPrefix(fp.Path, cwd) {
			t.Errorf("read %s from the process working directory", fp.Path)
		}
	}
	for _, skipped := range report.Skipped {
		if strings.HasPrefix(skipped.Path, cwd) {
			t.Errorf("inspected %s in the process working directory", skipped.Path)
		}
	}
	for _, approved := range report.ApprovedRoots {
		if approved == cwd {
			t.Errorf("approved the process working directory %s as a read root", approved)
		}
	}
}
