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

func TestCodexResolverPrecedence(t *testing.T) {
	home, root, project := codexFixture(t)
	writeCodexTestFile(t, filepath.Join(root, "config.toml"), `
model = "user-model"
model_provider = "user-provider"
model_reasoning_effort = "low"

[profiles.work]
model = "embedded-model"
model_provider = "embedded-provider"
model_reasoning_effort = "medium"

[projects."`+project+`"]
trust_level = "trusted"
`)
	writeCodexTestFile(t, filepath.Join(root, "work.config.toml"), `
model = "file-profile-model"
model_reasoning_effort = "high"
`)
	projectConfig := filepath.Join(project, ".codex", "config.toml")
	writeCodexTestFile(t, projectConfig, `model = "project-model"`)

	binding := codexBinding(root, project)
	binding.Profile = "work"
	effective, report, err := NewCodexResolver(CodexOptions{UserHome: home, CodexHome: root}).Resolve(
		context.Background(), binding, config.Project{Cwd: project})
	if err != nil {
		t.Fatalf("Resolve: %v report=%+v", err, report)
	}
	if effective.Model == nil || *effective.Model != "project-model" {
		t.Fatalf("model = %v, want trusted project value", effective.Model)
	}
	if effective.Provider == nil || *effective.Provider != "embedded-provider" {
		t.Fatalf("provider = %v, want selected embedded profile value", effective.Provider)
	}
	if effective.Effort == nil || *effective.Effort != "high" {
		t.Fatalf("effort = %v, want selected file profile value", effective.Effort)
	}

	assertCodexProvenance(t, effective.Provenance["model"], "project", projectConfig, "model")
	assertCodexProvenance(t, effective.Provenance["provider"], "profile:work", filepath.Join(root, "config.toml"), "model_provider")
	assertCodexProvenance(t, effective.Provenance["effort"], "profile:work", filepath.Join(root, "work.config.toml"), "model_reasoning_effort")
}

func TestCodexResolverSkipsUntrustedProject(t *testing.T) {
	home, root, project := codexFixture(t)
	writeCodexTestFile(t, filepath.Join(root, "config.toml"), `model = "user-model"`)
	projectConfig := filepath.Join(project, ".codex", "config.toml")
	writeCodexTestFile(t, projectConfig, `model = "untrusted-project-model"`)

	effective, report, err := NewCodexResolver(CodexOptions{UserHome: home, CodexHome: root}).Resolve(
		context.Background(), codexBinding(root, project), config.Project{Cwd: project})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if effective.Model == nil || *effective.Model != "user-model" {
		t.Fatalf("model = %v, want user model", effective.Model)
	}
	if len(report.Skipped) != 1 || report.Skipped[0].Path != projectConfig || report.Skipped[0].Reason != "project_not_trusted" {
		t.Fatalf("skipped = %+v, want untrusted project config", report.Skipped)
	}
}

func TestCodexResolverMalformedTOMLIsSanitized(t *testing.T) {
	home, root, project := codexFixture(t)
	writeCodexTestFile(t, filepath.Join(root, "config.toml"), `
model = "user-model"
[projects."`+project+`"]
trust_level = "trusted"
`)
	const secret = "SENTINEL-MALFORMED-CODEX-SECRET"
	writeCodexTestFile(t, filepath.Join(project, ".codex", "config.toml"), `token = "`+secret+`" BROKEN`)

	effective, report, err := NewCodexResolver(CodexOptions{UserHome: home, CodexHome: root}).Resolve(
		context.Background(), codexBinding(root, project), config.Project{Cwd: project})
	if !errors.Is(err, ErrInvalidSource) {
		t.Fatalf("error = %v, want ErrInvalidSource; report=%+v", err, report)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("malformed TOML error leaked source text: %v", err)
	}
	if effective.Model == nil || *effective.Model != "user-model" {
		t.Fatalf("partial model = %v, want user-model", effective.Model)
	}
	assertCodexResultDoesNotContain(t, secret, effective, report, err)
}

func TestCodexResolverModelCatalogConfiguredIDs(t *testing.T) {
	home, root, project := codexFixture(t)
	writeCodexTestFile(t, filepath.Join(root, "config.toml"), `
model_catalog_json = "catalog.json"
[model_providers.custom]
name = "Custom"
[model_providers.enterprise]
name = "Enterprise"
`)
	writeCodexTestFile(t, filepath.Join(root, "catalog.json"), `{
  "models":[{"id":"catalog-b","name":"Catalog B"}],
  "nested":{"entries":[{"id":"catalog-a"}]}
}`)

	effective, report, err := NewCodexResolver(CodexOptions{UserHome: home, CodexHome: root}).Resolve(
		context.Background(), codexBinding(root, project), config.Project{Cwd: project})
	if err != nil {
		t.Fatalf("Resolve: %v report=%+v", err, report)
	}
	want := []string{"catalog-a", "catalog-b"}
	if got := modelIDs(effective.Models); !reflect.DeepEqual(got, want) {
		t.Fatalf("model IDs = %v, want %v", got, want)
	}
	if len(report.FilesRead) != 2 {
		t.Fatalf("files read = %+v, want config and catalog", report.FilesRead)
	}
}

func TestCodexResolverInventoriesAgentsAndSkills(t *testing.T) {
	home, root, project := codexFixture(t)
	writeCodexTestFile(t, filepath.Join(root, "config.toml"), ``)
	projectAgents := filepath.Join(project, "AGENTS.md")
	projectSkill := filepath.Join(project, ".agents", "skills", "review", "SKILL.md")
	userSkill := filepath.Join(home, ".agents", "skills", "plan", "SKILL.md")
	writeCodexTestFile(t, projectAgents, "Project instructions\n")
	writeCodexTestFile(t, projectSkill, "---\nname: review\n---\n")
	writeCodexTestFile(t, userSkill, "---\nname: plan\n---\n")

	effective, report, err := NewCodexResolver(CodexOptions{UserHome: home, CodexHome: root}).Resolve(
		context.Background(), codexBinding(root, project, home), config.Project{Cwd: project})
	if err != nil {
		t.Fatalf("Resolve: %v report=%+v", err, report)
	}
	for _, want := range []struct {
		path, kind, scope string
	}{
		{projectAgents, "instructions", "project"},
		{projectSkill, "skill", "project"},
		{userSkill, "skill", "user"},
	} {
		if !hasCodexAsset(effective.Assets, want.path, want.kind, want.scope) {
			t.Errorf("missing asset path=%s kind=%s scope=%s in %+v", want.path, want.kind, want.scope, effective.Assets)
		}
	}
}

func TestCodexResolverRejectsUnapprovedSymlinkTarget(t *testing.T) {
	home, root, project := codexFixture(t)
	outside := t.TempDir()
	outsideCatalog := filepath.Join(outside, "catalog.json")
	writeCodexTestFile(t, outsideCatalog, `[{"id":"outside-secret-model"}]`)
	if err := os.Symlink(outsideCatalog, filepath.Join(root, "catalog.json")); err != nil {
		t.Fatal(err)
	}
	writeCodexTestFile(t, filepath.Join(root, "config.toml"), `model_catalog_json = "catalog.json"`)

	effective, report, err := NewCodexResolver(CodexOptions{UserHome: home, CodexHome: root}).Resolve(
		context.Background(), codexBinding(root, project), config.Project{Cwd: project})
	if !errors.Is(err, ErrApprovalRequired) {
		t.Fatalf("error = %v, want ErrApprovalRequired; report=%+v", err, report)
	}
	if len(effective.Models) != 0 {
		t.Fatalf("unapproved catalog models escaped: %+v", effective.Models)
	}
	if len(report.Skipped) != 1 || report.Skipped[0].Reason != "approval_required" {
		t.Fatalf("skipped = %+v, want approval_required", report.Skipped)
	}
}

func TestBindingDoesNotWriteSource(t *testing.T) {
	home, root, project := codexFixture(t)
	writeCodexTestFile(t, filepath.Join(root, "config.toml"), `model = "read-only"`)
	writeCodexTestFile(t, filepath.Join(project, "AGENTS.md"), "Read only\n")
	before := snapshotCodexTrees(t, home, project)

	resolver := NewCodexResolver(CodexOptions{UserHome: home, CodexHome: root})
	binding := codexBinding(root, project, home)
	if _, _, err := resolver.Preview(context.Background(), binding, config.Project{Cwd: project}); err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if _, _, err := resolver.Resolve(context.Background(), binding, config.Project{Cwd: project}); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if after := snapshotCodexTrees(t, home, project); !reflect.DeepEqual(after, before) {
		t.Fatalf("binding mutated source trees\nbefore=%v\nafter=%v", before, after)
	}
}

func TestResolverNeverReturnsSecrets(t *testing.T) {
	home, root, project := codexFixture(t)
	const secret = "SENTINEL-CODEX-NESTED-SECRET"
	writeCodexTestFile(t, filepath.Join(root, "config.toml"), `
model = "safe-model"
auth_token = "`+secret+`"
[env]
API_TOKEN = "`+secret+`"
[model_providers.private]
token = "`+secret+`"
[model_providers.private.env]
API_KEY = "`+secret+`"
[mcp_servers.private]
token = "`+secret+`"
[mcp_servers.private.headers]
Authorization = "`+secret+`"
`)

	effective, report, err := NewCodexResolver(CodexOptions{UserHome: home, CodexHome: root}).Resolve(
		context.Background(), codexBinding(root, project), config.Project{Cwd: project})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	assertCodexResultDoesNotContain(t, secret, effective, report, err)
}

// Regression (review fix, §2.4): a Codex binding is per backend and reused across
// projects. A binding whose frozen Approved holds only project A's root must still
// resolve project B's native config, or a normal A→B project change is wrongly
// rejected with approval_required.
func TestCodexResolverResolvesDifferentProjectThanPreview(t *testing.T) {
	home, root, projectA := codexFixture(t)
	projectB, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	writeCodexTestFile(t, filepath.Join(root, "config.toml"), `
model = "user-model"
[projects."`+projectB+`"]
trust_level = "trusted"
`)
	projectBConfig := filepath.Join(projectB, ".codex", "config.toml")
	writeCodexTestFile(t, projectBConfig, `model = "project-b-model"`)

	// Approved is frozen to project A only (as after a preview against A).
	binding := Binding{Provider: ProviderCodex, Mode: ModeLinked, Root: root,
		Claims: []string{"launch_defaults"}, Approved: []string{root, projectA}}

	effective, report, err := NewCodexResolver(CodexOptions{UserHome: home, CodexHome: root}).Resolve(
		context.Background(), binding, config.Project{Cwd: projectB})
	if err != nil {
		t.Fatalf("Resolve on project B: %v report=%+v", err, report)
	}
	for _, skip := range report.Skipped {
		if skip.Reason == "approval_required" {
			t.Fatalf("project B read rejected as approval_required: %+v", report.Skipped)
		}
	}
	if effective.Model == nil || *effective.Model != "project-b-model" {
		t.Fatalf("model = %v, want project-b-model (project B config applied)", effective.Model)
	}
}

func codexFixture(t *testing.T) (home, root, project string) {
	t.Helper()
	base := t.TempDir()
	home = filepath.Join(base, "home")
	root = filepath.Join(home, ".codex")
	project = filepath.Join(base, "project")
	for _, dir := range []string{root, project} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	var err error
	home, err = filepath.EvalSymlinks(home)
	if err != nil {
		t.Fatal(err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	project, err = filepath.EvalSymlinks(project)
	if err != nil {
		t.Fatal(err)
	}
	return home, root, project
}

func codexBinding(root string, approved ...string) Binding {
	return Binding{
		Provider: ProviderCodex,
		Mode:     ModeLinked,
		Root:     root,
		Claims:   []string{"launch_defaults", "model_catalog", "setup"},
		Approved: append([]string{root}, approved...),
	}
}

func writeCodexTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertCodexProvenance(t *testing.T, got FieldProvenance, scope, path, key string) {
	t.Helper()
	if got.Scope != scope || got.Path != path || got.Key != key {
		t.Errorf("provenance = %+v, want scope=%q path=%q key=%q", got, scope, path, key)
	}
}

func hasCodexAsset(assets []Asset, path, kind, scope string) bool {
	for _, asset := range assets {
		if asset.Path == path && asset.Kind == kind && asset.Scope == scope {
			return true
		}
	}
	return false
}

func snapshotCodexTrees(t *testing.T, roots ...string) map[string][]byte {
	t.Helper()
	snapshot := make(map[string][]byte)
	for _, root := range roots {
		if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			if entry.Type()&os.ModeSymlink != 0 {
				target, err := os.Readlink(path)
				if err != nil {
					return err
				}
				snapshot[path] = []byte("symlink:" + target)
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			snapshot[path] = append([]byte(nil), data...)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	return snapshot
}

func assertCodexResultDoesNotContain(t *testing.T, secret string, effective Effective, report Report, resultErr error) {
	t.Helper()
	errText := ""
	if resultErr != nil {
		errText = resultErr.Error()
	}
	encoded, err := json.Marshal(struct {
		Effective Effective `json:"effective"`
		Report    Report    `json:"report"`
		Error     string    `json:"error"`
	}{Effective: effective, Report: report, Error: errText})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), secret) {
		t.Fatalf("secret escaped resolver boundary: %s", encoded)
	}
}
