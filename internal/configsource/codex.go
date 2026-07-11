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
	"github.com/pelletier/go-toml/v2"
)

type CodexOptions struct {
	UserHome  string
	CodexHome string
}

type CodexResolver struct {
	userHome  string
	codexHome string
}

func NewCodexResolver(options CodexOptions) *CodexResolver {
	return &CodexResolver{userHome: options.UserHome, codexHome: options.CodexHome}
}

func (r *CodexResolver) Discover(ctx context.Context, _ config.Project) []Candidate {
	if ctx.Err() != nil {
		return []Candidate{}
	}
	root := r.codexHome
	if root == "" && r.userHome != "" {
		root = filepath.Join(r.userHome, ".codex")
	}
	found := false
	if info, err := os.Stat(filepath.Join(root, "config.toml")); err == nil && !info.IsDir() {
		found = true
	}
	return []Candidate{{Provider: ProviderCodex, Root: root, Found: found, Health: "ok", Warnings: []string{}}}
}

func (r *CodexResolver) Preview(ctx context.Context, binding Binding, project config.Project) (Effective, Report, error) {
	return r.resolve(ctx, binding, project, true)
}

func (r *CodexResolver) Resolve(ctx context.Context, binding Binding, project config.Project) (Effective, Report, error) {
	return r.resolve(ctx, binding, project, false)
}

type codexLayer struct {
	path  string
	scope string
	data  map[string]any
}

func (r *CodexResolver) resolve(ctx context.Context, binding Binding, project config.Project, preview bool) (Effective, Report, error) {
	effective := emptyEffective()
	report := emptyReport()
	if binding.Provider != ProviderCodex {
		return effective, report, fmt.Errorf("%w: provider is not Codex", ErrInvalidSource)
	}
	root, err := canonicalRoot(binding.Root)
	if err != nil {
		return effective, report, classifyPathError(err)
	}
	projectPath := project.Cwd
	if expanded, expandErr := config.ExpandTilde(projectPath); expandErr == nil {
		projectPath = expanded
	}
	projectRoot, err := canonicalRoot(projectPath)
	if err != nil {
		return effective, report, classifyPathError(err)
	}
	// Approved roots gate every read. The source root and the *currently selected*
	// project's canonical root are always approved for this resolution — a binding is
	// per backend, not per project, so it must resolve on whatever project the user
	// launches/refreshes against, not only the one it was previewed with. (Freezing
	// only the preview project's roots rejected a normal A→B project change with
	// approval_required.) The skills tree is inventoried at preview only.
	approved := append([]string{}, binding.Approved...)
	approved = append(approved, root, projectRoot)
	if preview {
		if r.userHome != "" {
			if skills, skillsErr := canonicalRoot(filepath.Join(r.userHome, ".agents", "skills")); skillsErr == nil {
				approved = append(approved, skills)
			}
		}
	}
	approved = uniqueCleanPaths(approved)
	report.ApprovedRoots = append(report.ApprovedRoots, approved...)
	if _, err := approvedPath(root, approved); err != nil {
		return effective, report, classifyPathError(err)
	}
	if _, err := approvedPath(projectRoot, approved); err != nil {
		return effective, report, classifyPathError(err)
	}

	user, found, err := readTOMLLayer(filepath.Join(root, "config.toml"), "user", approved, &report)
	if err != nil {
		finalize(&effective, &report)
		return effective, report, err
	}
	layers := make([]codexLayer, 0, 4)
	if found {
		layers = append(layers, user)
	}

	if binding.Profile != "" {
		if found {
			if profiles, ok := user.data["profiles"].(map[string]any); ok {
				if selected, ok := profiles[binding.Profile].(map[string]any); ok {
					layers = append(layers, codexLayer{path: user.path, scope: "profile:" + binding.Profile, data: selected})
				}
			}
		}
		profile, profileFound, profileErr := readTOMLLayer(filepath.Join(root, binding.Profile+".config.toml"), "profile:"+binding.Profile, approved, &report)
		if profileErr != nil {
			applyCodexLayersBestEffort(&effective, &report, layers, binding)
			applyOverrides(&effective, binding.Overrides)
			finalize(&effective, &report)
			return effective, report, profileErr
		}
		if profileFound {
			layers = append(layers, profile)
		}
	}

	projectTrusted := codexProjectTrusted(user.data, projectRoot)
	for _, dir := range []string{projectRoot} {
		configPath := filepath.Join(dir, ".codex", "config.toml")
		if _, statErr := os.Stat(configPath); errors.Is(statErr, os.ErrNotExist) {
			continue
		}
		if !projectTrusted {
			report.Skipped = append(report.Skipped, SkippedPath{Path: configPath, Reason: "project_not_trusted"})
			continue
		}
		layer, layerFound, layerErr := readTOMLLayer(configPath, "project", approved, &report)
		if layerErr != nil {
			applyCodexLayersBestEffort(&effective, &report, layers, binding)
			applyOverrides(&effective, binding.Overrides)
			finalize(&effective, &report)
			return effective, report, layerErr
		}
		if !layerFound {
			continue
		}
		for _, forbidden := range []string{"model_providers", "profiles"} {
			if _, ok := layer.data[forbidden]; ok {
				delete(layer.data, forbidden)
				report.Warnings = append(report.Warnings, "project config key "+forbidden+" is native pass-through only")
			}
		}
		layers = append(layers, layer)
	}

	for _, layer := range layers {
		if err := applyCodexLayer(&effective, &report, layer, binding); err != nil {
			finalize(&effective, &report)
			return effective, report, err
		}
		collectMCPNames(&effective, layer.data)
	}
	applyOverrides(&effective, binding.Overrides)
	if effective.Model != nil && *effective.Model != "" {
		effective.Models = append(effective.Models, Model{ID: *effective.Model, Source: effective.Provenance["model"].Path})
	}
	if hasClaim(binding, "setup") {
		if err := r.inventory(ctx, &effective, &report, projectRoot, approved); err != nil {
			finalize(&effective, &report)
			return effective, report, err
		}
	}
	finalize(&effective, &report)
	return effective, report, nil
}

func applyCodexLayersBestEffort(effective *Effective, report *Report, layers []codexLayer, binding Binding) {
	for _, layer := range layers {
		if err := applyCodexLayer(effective, report, layer, binding); err != nil {
			return
		}
	}
}

func readTOMLLayer(path, scope string, approved []string, report *Report) (codexLayer, bool, error) {
	file, fp, data, err := fingerprintFile(path, scope, "config", approved)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return codexLayer{}, false, nil
		}
		return codexLayer{}, false, classifyPathError(err)
	}
	report.FilesRead = append(report.FilesRead, file)
	report.Fingerprints = append(report.Fingerprints, fp)
	values := make(map[string]any)
	if err := toml.Unmarshal(data, &values); err != nil {
		return codexLayer{}, false, fmt.Errorf("%w: malformed TOML in %s", ErrInvalidSource, file.Path)
	}
	return codexLayer{path: file.Path, scope: scope, data: values}, true, nil
}

var codexKnownKeys = map[string]struct{}{
	"model": {}, "model_provider": {}, "model_providers": {}, "model_reasoning_effort": {},
	"model_verbosity": {}, "model_catalog_json": {}, "profile": {}, "profiles": {},
	"projects": {}, "mcp_servers": {}, "rules": {}, "hooks": {}, "plugins": {},
}

func applyCodexLayer(effective *Effective, report *Report, layer codexLayer, binding Binding) error {
	setString := func(field, key string, target **string) {
		if value, ok := layer.data[key].(string); ok {
			copy := value
			*target = &copy
			effective.Provenance[field] = FieldProvenance{Scope: layer.scope, Path: layer.path, Key: key}
		}
	}
	if hasClaim(binding, "launch_defaults") {
		setString("model", "model", &effective.Model)
		setString("provider", "model_provider", &effective.Provider)
		setString("effort", "model_reasoning_effort", &effective.Effort)
		setString("verbosity", "model_verbosity", &effective.Verbosity)
	}
	if hasClaim(binding, "model_catalog") {
		if catalogPath, ok := layer.data["model_catalog_json"].(string); ok {
			if err := readCodexCatalog(effective, report, catalogPath, layer, report.ApprovedRoots); err != nil {
				return err
			}
		}
	}
	if hasClaim(binding, "setup") {
		for _, key := range []string{"mcp_servers", "rules", "hooks", "plugins"} {
			if value, ok := layer.data[key]; ok {
				appendNamedAssets(effective, layer, key, value)
			}
		}
	}
	recordUnknown(report, layer.path, layer.data, codexKnownKeys)
	return nil
}

func readCodexCatalog(effective *Effective, report *Report, catalogPath string, layer codexLayer, approved []string) error {
	if !filepath.IsAbs(catalogPath) {
		catalogPath = filepath.Join(filepath.Dir(layer.path), catalogPath)
	}
	file, fp, data, err := fingerprintFile(catalogPath, layer.scope, "model_catalog", approved)
	if err != nil {
		report.Skipped = append(report.Skipped, SkippedPath{Path: catalogPath, Reason: pathReason(err)})
		return classifyPathError(err)
	}
	report.FilesRead = append(report.FilesRead, file)
	report.Fingerprints = append(report.Fingerprints, fp)
	var decoded any
	if json.Unmarshal(data, &decoded) != nil {
		return fmt.Errorf("%w: malformed model catalog JSON in %s", ErrInvalidSource, file.Path)
	}
	collectCatalogModels(effective, decoded, file.Path)
	return nil
}

func collectCatalogModels(effective *Effective, value any, source string) {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			collectCatalogModels(effective, item, source)
		}
	case map[string]any:
		if id, ok := typed["id"].(string); ok {
			name, _ := typed["name"].(string)
			effective.Models = append(effective.Models, Model{ID: id, Name: name, Source: source})
			return
		}
		for _, key := range sortedKeys(typed) {
			collectCatalogModels(effective, typed[key], source)
		}
	}
}

func appendNamedAssets(effective *Effective, layer codexLayer, kind string, value any) {
	values, ok := value.(map[string]any)
	if !ok {
		return
	}
	for _, name := range sortedKeys(values) {
		effective.Assets = append(effective.Assets, Asset{
			Kind: kind, Name: name, Path: layer.path, Scope: layer.scope,
			Detachability: "reference_only", Status: "native_passthrough",
		})
	}
}

func applyOverrides(effective *Effective, overrides config.SourceOverrides) {
	if overrides.Model != nil {
		effective.Model = overrides.Model
		effective.Provenance["model"] = FieldProvenance{Scope: "agentdeck_override", Key: "model"}
	}
	if overrides.Effort != nil {
		effective.Effort = overrides.Effort
		effective.Provenance["effort"] = FieldProvenance{Scope: "agentdeck_override", Key: "effort"}
	}
}

func codexProjectTrusted(user map[string]any, projectRoot string) bool {
	projects, _ := user["projects"].(map[string]any)
	for path, raw := range projects {
		if filepath.Clean(path) != projectRoot {
			continue
		}
		settings, _ := raw.(map[string]any)
		level, _ := settings["trust_level"].(string)
		if strings.EqualFold(level, "trusted") {
			return true
		}
	}
	return false
}

func (r *CodexResolver) inventory(ctx context.Context, effective *Effective, report *Report, projectRoot string, approved []string) error {
	for _, dir := range []string{projectRoot} {
		agents := filepath.Join(dir, "AGENTS.md")
		if _, err := os.Stat(agents); err == nil {
			if err := addInventoryFile(effective, report, agents, "project", "instructions", "copyable", approved); err != nil {
				return classifyPathError(err)
			}
		}
		if err := walkInventory(ctx, effective, report, filepath.Join(dir, ".agents", "skills"), "project", "skill", "reference_only", approved); err != nil && !errors.Is(err, os.ErrNotExist) {
			return classifyPathError(err)
		}
	}
	if r.userHome != "" {
		if err := walkInventory(ctx, effective, report, filepath.Join(r.userHome, ".agents", "skills"), "user", "skill", "reference_only", approved); err != nil && !errors.Is(err, os.ErrNotExist) {
			return classifyPathError(err)
		}
	}
	return nil
}

func uniqueCleanPaths(paths []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		clean := filepath.Clean(path)
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		result = append(result, clean)
	}
	sort.Strings(result)
	return result
}

func classifyPathError(err error) error {
	if errors.Is(err, ErrApprovalRequired) {
		return ErrApprovalRequired
	}
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w: source path not found", ErrInvalidSource)
	}
	return fmt.Errorf("%w: source path unreadable", ErrInvalidSource)
}

func pathReason(err error) string {
	if errors.Is(err, ErrApprovalRequired) {
		return "approval_required"
	}
	if errors.Is(err, os.ErrNotExist) {
		return "not_found"
	}
	return "unreadable"
}
