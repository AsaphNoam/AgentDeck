package configsource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agentdeck/agentdeck/internal/config"
)

// ClaudeResolver resolves the documented, display-safe subset of Claude Code
// configuration. userHome is injected so discovery is deterministic in tests.
type ClaudeResolver struct{ userHome string }

func NewClaudeResolver(userHome string) *ClaudeResolver {
	return &ClaudeResolver{userHome: filepath.Clean(userHome)}
}

func (r *ClaudeResolver) Discover(ctx context.Context, _ config.Project) []Candidate {
	if ctx.Err() != nil {
		return []Candidate{}
	}
	root := filepath.Join(r.userHome, ".claude")
	_, err := os.Stat(filepath.Join(root, "settings.json"))
	health := "not_found"
	if err == nil {
		health = "ok"
	}
	return []Candidate{{Provider: ProviderClaude, Root: root, Found: err == nil, Health: health, Warnings: []string{}}}
}

func (r *ClaudeResolver) Preview(ctx context.Context, binding Binding, project config.Project) (Effective, Report, error) {
	preview := binding
	preview.Approved = append([]string{}, binding.Approved...)
	for _, path := range []string{binding.Root, project.Cwd} {
		if canonical, err := canonicalExisting(path); err == nil && !containsString(preview.Approved, canonical) {
			preview.Approved = append(preview.Approved, canonical)
		}
	}
	effective, report, err := r.resolve(ctx, preview, project)
	report.ApprovedRoots = append([]string{}, preview.Approved...)
	sort.Strings(report.ApprovedRoots)
	return effective, report, err
}

func (r *ClaudeResolver) Resolve(ctx context.Context, binding Binding, project config.Project) (Effective, Report, error) {
	return r.resolve(ctx, binding, project)
}

type claudeLayer struct {
	path    string
	scope   string
	managed bool
}

func (r *ClaudeResolver) resolve(ctx context.Context, binding Binding, project config.Project) (Effective, Report, error) {
	effective, report := emptyEffective(), emptyReport()
	report.ApprovedRoots = append(report.ApprovedRoots, binding.Approved...)
	sort.Strings(report.ApprovedRoots)
	if binding.Provider != ProviderClaude {
		return effective, report, fmt.Errorf("%w: provider is not Claude Code", ErrInvalidSource)
	}
	if err := ctx.Err(); err != nil {
		return effective, report, err
	}

	projectRoot := project.Cwd
	if expanded, err := config.ExpandTilde(projectRoot); err == nil {
		projectRoot = expanded
	}
	layers := []claudeLayer{
		{filepath.Join(binding.Root, "settings.json"), "user", false},
		{filepath.Join(projectRoot, ".claude", "settings.json"), "project", false},
		{filepath.Join(projectRoot, ".claude", "settings.local.json"), "local", false},
	}
	known := map[string]struct{}{
		"model": {}, "fallbackModel": {}, "effortLevel": {}, "availableModels": {}, "env": {},
		"hooks": {}, "enabledPlugins": {}, "plugins": {}, "mcpServers": {}, "mcp": {},
	}
	for _, layer := range layers {
		values, canonical, err := readClaudeSettings(ctx, layer.path, layer.scope, binding.Approved, &report)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			finalizeClaude(&effective, &report)
			return effective, report, err
		}
		recordUnknown(&report, canonical, values, known)
		applyClaudeLayer(&effective, values, layer.scope, canonical, binding)
		if hasClaim(binding, "setup") {
			inventorySettingsDeclarations(&effective, values, layer.scope, canonical, reportFingerprint(report, canonical))
		}
	}

	// AgentDeck overrides sit above ordinary native layers but never above
	// managed policy. A pointer to "" deliberately means inherit/default.
	if binding.Overrides.Model != nil {
		effective.Model = binding.Overrides.Model
		effective.Provenance["model"] = FieldProvenance{Scope: "agentdeck_override", Key: "overrides.model"}
	}
	if binding.Overrides.Effort != nil {
		effective.Effort = binding.Overrides.Effort
		effective.Provenance["effort"] = FieldProvenance{Scope: "agentdeck_override", Key: "overrides.effort"}
	}

	managedFile := filepath.Join(binding.Root, "managed-settings.json")
	values, canonical, err := readClaudeSettings(ctx, managedFile, "managed", binding.Approved, &report)
	if err == nil {
		recordUnknown(&report, canonical, values, known)
		applyClaudeLayer(&effective, values, "managed", canonical, binding)
		if hasClaim(binding, "setup") {
			inventorySettingsDeclarations(&effective, values, "managed", canonical, reportFingerprint(report, canonical))
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		finalizeClaude(&effective, &report)
		return effective, report, err
	}
	if hasClaim(binding, "setup") {
		visited := make(map[string]bool)
		for _, instruction := range []struct{ path, scope string }{
			{filepath.Join(binding.Root, "CLAUDE.md"), "user"},
			{filepath.Join(projectRoot, "CLAUDE.md"), "project"},
		} {
			if err := inventoryClaudeInstruction(ctx, instruction.path, instruction.scope, binding.Approved, &effective, &report, visited, false); err != nil && !errors.Is(err, os.ErrNotExist) {
				finalizeClaude(&effective, &report)
				return effective, report, err
			}
		}
		for _, base := range []struct{ path, scope string }{
			{binding.Root, "user"}, {filepath.Join(projectRoot, ".claude"), "project"},
		} {
			for _, dir := range []struct{ name, kind string }{{"rules", "rule"}, {"skills", "skill"}, {"agents", "agent"}} {
				if err := inventoryClaudeDir(ctx, filepath.Join(base.path, dir.name), base.scope, dir.kind, binding.Approved, &effective, &report); err != nil && !errors.Is(err, os.ErrNotExist) {
					finalizeClaude(&effective, &report)
					return effective, report, err
				}
			}
		}
	}

	finalizeClaude(&effective, &report)
	return effective, report, nil
}

func readClaudeSettings(ctx context.Context, path, scope string, approved []string, report *Report) (map[string]any, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	file, fp, data, err := fingerprintFile(path, scope, "settings", approved)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, "", os.ErrNotExist
		}
		if errors.Is(err, ErrApprovalRequired) {
			report.Skipped = append(report.Skipped, SkippedPath{Path: path, Reason: "approval_required"})
		}
		return nil, "", err
	}
	report.FilesRead = append(report.FilesRead, file)
	report.Fingerprints = append(report.Fingerprints, fp)
	var values map[string]any
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, file.Path, fmt.Errorf("%w: malformed JSON in %s", ErrInvalidSource, file.Path)
	}
	return values, file.Path, nil
}

func applyClaudeLayer(e *Effective, values map[string]any, scope, path string, binding Binding) {
	if hasClaim(binding, "launch_defaults") {
		if model, ok := values["model"].(string); ok {
			e.Model = stringPtr(model)
			e.Provenance["model"] = FieldProvenance{Scope: scope, Path: path, Key: "model"}
		}
		if effort, ok := values["effortLevel"].(string); ok {
			e.Effort = stringPtr(effort)
			e.Provenance["effort"] = FieldProvenance{Scope: scope, Path: path, Key: "effortLevel"}
		}
		if fallback, ok := values["fallbackModel"].(string); ok {
			e.FallbackModel = stringPtr(fallback)
			e.Provenance["fallback_model"] = FieldProvenance{Scope: scope, Path: path, Key: "fallbackModel"}
		}
		appendEnvMetadata(e, scope, values["env"])
	}
	if hasClaim(binding, "model_catalog") {
		if models, ok := values["availableModels"].([]any); ok {
			e.Models = e.Models[:0]
			for _, item := range models {
				if id, ok := item.(string); ok {
					e.Models = append(e.Models, Model{ID: id, Source: path})
				}
			}
			e.Provenance["models"] = FieldProvenance{Scope: scope, Path: path, Key: "availableModels"}
		}
	}
}

func inventorySettingsDeclarations(e *Effective, values map[string]any, scope, path, hash string) {
	for _, declaration := range []struct{ key, kind string }{
		{"hooks", "hooks"}, {"enabledPlugins", "plugins"}, {"plugins", "plugins"},
		{"mcpServers", "mcp"}, {"mcp", "mcp"},
	} {
		if presentDeclaration(values[declaration.key]) {
			e.Assets = append(e.Assets, Asset{Kind: declaration.kind, Path: path, Scope: scope, SHA256: hash, Detachability: "reference_only", Status: "native_passthrough"})
		}
	}
}

func inventoryClaudeInstruction(ctx context.Context, path, scope string, approved []string, e *Effective, report *Report, visited map[string]bool, imported bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	file, fp, data, err := fingerprintFile(path, scope, "instruction", approved)
	if err != nil {
		if errors.Is(err, ErrApprovalRequired) {
			report.Skipped = append(report.Skipped, SkippedPath{Path: path, Reason: "approval_required"})
		}
		return err
	}
	if visited[file.Path] {
		return nil
	}
	visited[file.Path] = true
	report.FilesRead = append(report.FilesRead, file)
	report.Fingerprints = append(report.Fingerprints, fp)
	kind := "instruction"
	if imported {
		kind = "instruction_import"
	}
	e.Assets = append(e.Assets, Asset{Kind: kind, Path: file.Path, Scope: scope, SHA256: fp.SHA256, Detachability: "reference_only", Status: "native_passthrough"})
	for _, line := range strings.Split(string(data), "\n") {
		candidate := strings.TrimSpace(line)
		if !strings.HasPrefix(candidate, "@") || strings.ContainsAny(candidate, " \t") {
			continue
		}
		importPath := strings.TrimPrefix(candidate, "@")
		if importPath == "" {
			continue
		}
		if !filepath.IsAbs(importPath) {
			importPath = filepath.Join(filepath.Dir(file.Path), importPath)
		}
		if err := inventoryClaudeInstruction(ctx, importPath, scope, approved, e, report, visited, true); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				report.Skipped = append(report.Skipped, SkippedPath{Path: importPath, Reason: "not_found"})
				continue
			}
			return err
		}
	}
	return nil
}

func inventoryClaudeDir(ctx context.Context, dir, scope, kind string, approved []string, e *Effective, report *Report) error {
	return walkInventory(ctx, e, report, dir, scope, kind, "reference_only", approved)
}

func finalizeClaude(e *Effective, report *Report) {
	sort.Slice(e.Models, func(i, j int) bool { return e.Models[i].ID < e.Models[j].ID })
	sort.Slice(e.Assets, func(i, j int) bool {
		if e.Assets[i].Kind != e.Assets[j].Kind {
			return e.Assets[i].Kind < e.Assets[j].Kind
		}
		if e.Assets[i].Path != e.Assets[j].Path {
			return e.Assets[i].Path < e.Assets[j].Path
		}
		return e.Assets[i].Scope < e.Assets[j].Scope
	})
	sort.Slice(e.EnvKeys, func(i, j int) bool {
		if e.EnvKeys[i].Name != e.EnvKeys[j].Name {
			return e.EnvKeys[i].Name < e.EnvKeys[j].Name
		}
		return e.EnvKeys[i].Scope < e.EnvKeys[j].Scope
	})
	sort.Slice(report.FilesRead, func(i, j int) bool { return report.FilesRead[i].Path < report.FilesRead[j].Path })
	sort.Slice(report.Fingerprints, func(i, j int) bool { return report.Fingerprints[i].Path < report.Fingerprints[j].Path })
	sort.Slice(report.UnknownKeys, func(i, j int) bool {
		if report.UnknownKeys[i].Path != report.UnknownKeys[j].Path {
			return report.UnknownKeys[i].Path < report.UnknownKeys[j].Path
		}
		return report.UnknownKeys[i].Key < report.UnknownKeys[j].Key
	})
	sort.Slice(report.Skipped, func(i, j int) bool { return report.Skipped[i].Path < report.Skipped[j].Path })
	report.SourceDigest = generationKey(report.Fingerprints)
}

func canonicalExisting(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if filepath.Clean(value) == filepath.Clean(want) {
			return true
		}
	}
	return false
}

func stringPtr(value string) *string { return &value }

func hasClaim(binding Binding, claim string) bool {
	if len(binding.Claims) == 0 {
		return true
	}
	for _, value := range binding.Claims {
		if value == claim {
			return true
		}
	}
	return false
}

func presentDeclaration(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		return len(typed) > 0
	case []any:
		return len(typed) > 0
	case string:
		return typed != ""
	case bool:
		return typed
	default:
		return value != nil
	}
}

func reportFingerprint(report Report, path string) string {
	for _, fp := range report.Fingerprints {
		if fp.Path == path {
			return fp.SHA256
		}
	}
	return ""
}
