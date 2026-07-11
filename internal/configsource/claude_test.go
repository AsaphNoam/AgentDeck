package configsource

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/agentdeck/agentdeck/internal/config"
)

func TestClaudeResolverPrecedence(t *testing.T) {
	home, project := claudeFixture(t)
	root := filepath.Join(home, ".claude")
	writeClaudeTestFile(t, filepath.Join(root, "settings.json"), `{
  "model":"user-model","effortLevel":"low","availableModels":["user-a"],
  "env":{"ANTHROPIC_API_KEY":"user-secret"},"futureSetting":{"nested":"do-not-return"}
}`)
	writeClaudeTestFile(t, filepath.Join(project, ".claude", "settings.json"), `{
  "model":"project-model","effortLevel":"medium","availableModels":["project-a"]
}`)
	writeClaudeTestFile(t, filepath.Join(project, ".claude", "settings.local.json"), `{
  "model":"local-model","availableModels":["local-b","local-a"],
  "hooks":{"PostToolUse":[{"command":"echo hook-secret"}]},
  "enabledPlugins":{"example@marketplace":true},"mcpServers":{"private":{"token":"mcp-secret"}}
}`)
	writeClaudeTestFile(t, filepath.Join(root, "managed-settings.json"), `{"effortLevel":"high"}`)
	writeClaudeTestFile(t, filepath.Join(root, "CLAUDE.md"), "User instructions\n")
	writeClaudeTestFile(t, filepath.Join(project, "CLAUDE.md"), "@extra.md\nProject instructions\n")
	writeClaudeTestFile(t, filepath.Join(project, "extra.md"), "Imported instructions\n")
	writeClaudeTestFile(t, filepath.Join(project, ".claude", "rules", "go.md"), "Rule\n")
	writeClaudeTestFile(t, filepath.Join(project, ".claude", "skills", "review", "SKILL.md"), "---\nname: review\n---\n")
	writeClaudeTestFile(t, filepath.Join(project, ".claude", "agents", "reviewer.md"), "Agent\n")

	overrideModel := "override-model"
	binding := Binding{Provider: ProviderClaude, Mode: ModeLinked, Root: root,
		Claims:    []string{"launch_defaults", "model_catalog", "setup"},
		Overrides: config.SourceOverrides{Model: &overrideModel}, Approved: []string{root, project}}
	effective, report, err := NewClaudeResolver(home).Resolve(context.Background(), binding, config.Project{Cwd: project})
	if err != nil {
		t.Fatalf("Resolve: %v report=%+v", err, report)
	}
	if effective.Model == nil || *effective.Model != "override-model" {
		t.Fatalf("model = %v, want AgentDeck override", effective.Model)
	}
	if effective.Effort == nil || *effective.Effort != "high" {
		t.Fatalf("effort = %v, want managed high", effective.Effort)
	}
	if got := effective.Provenance["model"].Scope; got != "agentdeck_override" {
		t.Errorf("model provenance = %q", got)
	}
	if got := effective.Provenance["effort"].Scope; got != "managed" {
		t.Errorf("effort provenance = %q", got)
	}
	if got := modelIDs(effective.Models); !reflect.DeepEqual(got, []string{"local-a", "local-b"}) {
		t.Errorf("models = %v", got)
	}
	if got := effective.Provenance["models"].Scope; got != "local" {
		t.Errorf("models provenance = %q", got)
	}
	if len(report.UnknownKeys) != 1 || report.UnknownKeys[0].Key != "futureSetting" {
		t.Errorf("unknown keys = %+v", report.UnknownKeys)
	}
	for _, kind := range []string{"instruction", "instruction_import", "rule", "skill", "agent", "hooks", "plugins", "mcp"} {
		if !hasAssetKind(effective.Assets, kind) {
			t.Errorf("missing %s inventory: %+v", kind, effective.Assets)
		}
	}
	if report.SourceDigest == "" {
		t.Error("empty deterministic source digest")
	}
	repeatEffective, repeatReport, err := NewClaudeResolver(home).Resolve(context.Background(), binding, config.Project{Cwd: project})
	if err != nil {
		t.Fatalf("repeat Resolve: %v", err)
	}
	if !reflect.DeepEqual(repeatEffective, effective) || !reflect.DeepEqual(repeatReport, report) {
		t.Fatal("unchanged source did not produce deterministic effective/report output")
	}

	// A managed value is a constraint and therefore wins even over AgentDeck.
	writeClaudeTestFile(t, filepath.Join(root, "managed-settings.json"), `{"model":"managed-model","effortLevel":"high"}`)
	effective, _, err = NewClaudeResolver(home).Resolve(context.Background(), binding, config.Project{Cwd: project})
	if err != nil {
		t.Fatalf("Resolve managed model: %v", err)
	}
	if effective.Model == nil || *effective.Model != "managed-model" || effective.Provenance["model"].Scope != "managed" {
		t.Fatalf("managed model did not win: value=%v provenance=%+v", effective.Model, effective.Provenance["model"])
	}
}

func TestClaudeResolverReadOnlyAndRedacted(t *testing.T) {
	home, project := claudeFixture(t)
	root := filepath.Join(home, ".claude")
	const secret = "SENTINEL-CLAUDE-SECRET"
	writeClaudeTestFile(t, filepath.Join(root, "settings.json"), `{
  "model":"claude-model","env":{"API_TOKEN":"`+secret+`"},
  "auth_helper":"`+secret+`","headers":{"Authorization":"`+secret+`"}
}`)
	writeClaudeTestFile(t, filepath.Join(project, "CLAUDE.md"), "safe metadata only\n")
	before := snapshotClaudeTree(t, home, project)
	binding := Binding{Provider: ProviderClaude, Root: root, Approved: []string{root, project}, Claims: []string{"launch_defaults", "setup"}}
	resolver := NewClaudeResolver(home)
	for _, run := range []struct {
		name string
		fn   func() (Effective, Report, error)
	}{
		{"preview", func() (Effective, Report, error) {
			return resolver.Preview(context.Background(), binding, config.Project{Cwd: project})
		}},
		{"resolve", func() (Effective, Report, error) {
			return resolver.Resolve(context.Background(), binding, config.Project{Cwd: project})
		}},
	} {
		t.Run(run.name, func(t *testing.T) {
			effective, report, err := run.fn()
			if err != nil {
				t.Fatalf("%v report=%+v", err, report)
			}
			encoded, err := json.Marshal(struct {
				Effective Effective `json:"effective"`
				Report    Report    `json:"report"`
			}{effective, report})
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(encoded), secret) {
				t.Fatalf("secret escaped resolver: %s", encoded)
			}
			if len(effective.EnvKeys) != 1 || effective.EnvKeys[0].Name != "API_TOKEN" {
				t.Fatalf("env metadata = %+v", effective.EnvKeys)
			}
		})
	}
	if after := snapshotClaudeTree(t, home, project); !reflect.DeepEqual(after, before) {
		t.Fatalf("resolver mutated source tree\nbefore=%v\nafter=%v", before, after)
	}
}

func TestClaudeResolverMalformedJSONIsSanitizedAndPartial(t *testing.T) {
	home, project := claudeFixture(t)
	root := filepath.Join(home, ".claude")
	writeClaudeTestFile(t, filepath.Join(root, "settings.json"), `{"model":"user-model"}`)
	const secretSnippet = "SECRET-SHOULD-NOT-APPEAR"
	writeClaudeTestFile(t, filepath.Join(project, ".claude", "settings.json"), `{"env":{"TOKEN":"`+secretSnippet+`"}, BROKEN`)
	binding := Binding{Provider: ProviderClaude, Root: root, Approved: []string{root, project}}
	effective, report, err := NewClaudeResolver(home).Resolve(context.Background(), binding, config.Project{Cwd: project})
	if !errors.Is(err, ErrInvalidSource) {
		t.Fatalf("error = %v, want ErrInvalidSource; report=%+v", err, report)
	}
	if strings.Contains(err.Error(), secretSnippet) {
		t.Fatalf("error leaked source snippet: %v", err)
	}
	if effective.Model == nil || *effective.Model != "user-model" {
		t.Fatalf("partial effective model = %v", effective.Model)
	}
	if len(report.FilesRead) != 2 || report.SourceDigest == "" {
		t.Fatalf("partial report not preserved: %+v", report)
	}
}

func TestClaudeResolverRejectsUnapprovedSymlinkTarget(t *testing.T) {
	home, project := claudeFixture(t)
	root := filepath.Join(home, ".claude")
	writeClaudeTestFile(t, filepath.Join(root, "settings.json"), `{}`)
	outside := t.TempDir()
	writeClaudeTestFile(t, filepath.Join(outside, "secret.md"), "outside")
	writeClaudeTestFile(t, filepath.Join(project, "CLAUDE.md"), "@linked.md\n")
	if err := os.Symlink(filepath.Join(outside, "secret.md"), filepath.Join(project, "linked.md")); err != nil {
		t.Fatal(err)
	}
	binding := Binding{Provider: ProviderClaude, Root: root, Approved: []string{root, project}, Claims: []string{"setup"}}
	_, report, err := NewClaudeResolver(home).Resolve(context.Background(), binding, config.Project{Cwd: project})
	if !errors.Is(err, ErrApprovalRequired) {
		t.Fatalf("error = %v, want ErrApprovalRequired", err)
	}
	if len(report.Skipped) == 0 || report.Skipped[0].Reason != "approval_required" {
		t.Fatalf("skipped = %+v", report.Skipped)
	}
}

func claudeFixture(t *testing.T) (home, project string) {
	t.Helper()
	home, project = filepath.Join(t.TempDir(), "home"), filepath.Join(t.TempDir(), "project")
	for _, dir := range []string{filepath.Join(home, ".claude"), project} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	var err error
	home, err = filepath.EvalSymlinks(home)
	if err != nil {
		t.Fatal(err)
	}
	project, err = filepath.EvalSymlinks(project)
	if err != nil {
		t.Fatal(err)
	}
	return home, project
}

func writeClaudeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func snapshotClaudeTree(t *testing.T, roots ...string) map[string]string {
	t.Helper()
	result := make(map[string]string)
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if d.Type()&os.ModeSymlink != 0 {
				target, err := os.Readlink(path)
				if err != nil {
					return err
				}
				result[path] = "symlink:" + target
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			result[path] = string(data)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return result
}

func modelIDs(models []Model) []string {
	ids := make([]string, len(models))
	for i, model := range models {
		ids[i] = model.ID
	}
	return ids
}

func hasAssetKind(assets []Asset, kind string) bool {
	for _, asset := range assets {
		if asset.Kind == kind {
			return true
		}
	}
	return false
}
