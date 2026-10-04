// Package providerexec is the one resolver that selects which Claude/Codex
// provider executable a backend's next process uses (FS-09.R68/R75–R76,
// TS-04.R75). Every consumer — chat launch/resume/switch, Claude terminal,
// native login and readiness — asks this package, so the selected provider
// cannot drift between paths (INV §2).
//
// Resolution never executes anything: it only reads the environment and
// stats the filesystem. A backend's provider_mode chooses the source first:
// Installed resolves the user's override layers then discovery; Bundled uses
// only the running release's managed provider. Neither mode falls through to
// the other.
package providerexec

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/AsaphNoam/Chuck/internal/config"
)

// RuntimeRootEnv names the managed runtime root the release wrapper publishes
// (TS-06.R30). It is absent for source builds, which have no bundle.
const RuntimeRootEnv = "CHUCK_RUNTIME_ROOT"

// Sources identify which configuration layer won (TS-03.R52).
const (
	SourceDetected = "detected"
	SourceAmbient  = "ambient"
	SourceBackend  = "backend"
	SourceModel    = "model"
	SourceBundled  = "bundled"
)

// States are the bounded resolution outcomes (TS-03.R52).
const (
	StateAvailable         = "available"
	StateMissing           = "missing"
	StateNotExecutable     = "not_executable"
	StateInvalid           = "invalid"
	StateBundleUnavailable = "bundle_unavailable"
)

// Provider is one Claude/Codex provider the resolver knows.
type Provider struct {
	// ID is the provider id ("claude" | "codex").
	ID string
	// Name is the human product name.
	Name string
	// Command is the executable name searched during discovery.
	Command string
	// EnvKey is the executable-override key the managed adapter honors.
	EnvKey string
}

var providers = map[string]Provider{
	"claude-acp": {ID: "claude", Name: "Claude Code", Command: "claude", EnvKey: "CLAUDE_CODE_EXECUTABLE"},
	"codex-acp":  {ID: "codex", Name: "Codex", Command: "codex", EnvKey: "CODEX_PATH"},
}

// ForBackendType returns the provider a backend type runs, if it is one this
// resolver selects. OpenCode/OpenHands are unchanged and return false.
func ForBackendType(backendType string) (Provider, bool) {
	p, ok := providers[backendType]
	return p, ok
}

// Input is everything resolution depends on. ProcessEnv is the dashboard's
// own environment (os.Environ), BackendEnv/ModelEnv are the saved layers.
type Input struct {
	BackendType string
	Mode        string // "" | "installed" | "bundled"
	ProcessEnv  []string
	BackendEnv  map[string]string
	ModelEnv    map[string]string
	// Home is the user's home directory for the fixed discovery fallbacks.
	Home string
}

// Selection is the resolved provider for the next process start.
type Selection struct {
	Provider Provider
	Mode     string
	Source   string
	// Path is the absolute launcher path, preserved as found (a symlink is not
	// pinned to its target). Empty when nothing usable was found.
	Path  string
	State string
	// Override is the raw winning override value, when one won.
	Override string
}

// Available reports whether the selection names a usable executable.
func (s Selection) Available() bool { return s.State == StateAvailable }

// Resolve selects the provider executable. ok is false for backend types this
// resolver does not own.
func Resolve(in Input) (Selection, bool) {
	p, ok := providers[in.BackendType]
	if !ok {
		return Selection{}, false
	}
	process := envMap(in.ProcessEnv)
	root := process[RuntimeRootEnv]
	if in.Mode == "bundled" {
		sel := Selection{Provider: p, Mode: "bundled", Source: SourceBundled, State: StateBundleUnavailable}
		if path := BundledPath(root, p.ID); path != "" && isExecutableFile(path) {
			sel.Path, sel.State = path, StateAvailable
		}
		return sel, true
	}
	sel := Selection{Provider: p, Mode: "installed"}
	// Model > backend > inherited environment; an effective empty value
	// requests discovery rather than falling to a lower layer.
	value, source, set := "", "", false
	if v, ok := process[p.EnvKey]; ok {
		value, source, set = v, SourceAmbient, true
	}
	if v, ok := in.BackendEnv[p.EnvKey]; ok {
		value, source, set = v, SourceBackend, true
	}
	if v, ok := in.ModelEnv[p.EnvKey]; ok {
		value, source, set = v, SourceModel, true
	}
	pathEnv := process["PATH"]
	if v, ok := in.BackendEnv["PATH"]; ok {
		pathEnv = v
	}
	if v, ok := in.ModelEnv["PATH"]; ok {
		pathEnv = v
	}
	if set && strings.TrimSpace(value) != "" {
		sel.Source, sel.Override = source, value
		switch {
		case filepath.IsAbs(value):
			sel.Path = value
			sel.State = statExecutable(value)
		case strings.ContainsRune(value, filepath.Separator):
			sel.State = StateInvalid
		default:
			sel.Path = lookPath(value, searchDirs(pathEnv, ""), root)
			if sel.Path == "" {
				sel.State = StateMissing
			} else {
				sel.State = StateAvailable
			}
		}
		return sel, true
	}
	sel.Source = SourceDetected
	sel.Path = lookPath(p.Command, searchDirs(pathEnv, in.Home), root)
	if sel.Path == "" {
		sel.State = StateMissing
	} else {
		sel.State = StateAvailable
	}
	return sel, true
}

// ForBackend resolves a saved backend/model against the dashboard's own
// environment — the entry point every process start, readiness probe and
// sign-in shares (TS-04.R71).
func ForBackend(be config.Backend, model config.Model) (Selection, bool) {
	home, _ := os.UserHomeDir()
	return Resolve(Input{
		BackendType: be.Type,
		Mode:        be.EffectiveProviderMode(),
		ProcessEnv:  os.Environ(),
		BackendEnv:  be.Env,
		ModelEnv:    model.Env,
		Home:        home,
	})
}

// BundledPath is the release-relative native provider executable inside the
// managed runtime root (TS-06.R30): the Claude Agent SDK's platform package
// binary and Codex's platform vendor binary. Empty without a managed root or
// on a platform the release does not package.
func BundledPath(root, providerID string) string {
	if root == "" {
		return ""
	}
	rel := BundledRelPath(providerID, runtime.GOOS, runtime.GOARCH)
	if rel == "" {
		return ""
	}
	return filepath.Join(root, rel)
}

// BundledRelPath is the path below the managed runtime root. Release assembly
// verifies the same paths so the layout and the resolver cannot disagree.
func BundledRelPath(providerID, goos, goarch string) string {
	if goos != "darwin" {
		return ""
	}
	var npmArch, triple string
	switch goarch {
	case "arm64":
		npmArch, triple = "arm64", "aarch64-apple-darwin"
	case "amd64":
		npmArch, triple = "x64", "x86_64-apple-darwin"
	default:
		return ""
	}
	switch providerID {
	case "claude":
		return filepath.Join("node_modules", "@anthropic-ai", "claude-agent-sdk-darwin-"+npmArch, "claude")
	case "codex":
		return filepath.Join("node_modules", "@openai", "codex-darwin-"+npmArch, "vendor", triple, "bin", "codex")
	}
	return ""
}

// SystemDirs are the common package-manager locations searched after PATH and
// ~/.local/bin. Tests outside this package clear it to stay hermetic.
var SystemDirs = []string{"/opt/homebrew/bin", "/usr/local/bin"}

// searchDirs is the discovery order: absolute PATH entries, then the common
// user install locations, deduplicated. A non-empty home adds ~/.local/bin.
func searchDirs(pathEnv, home string) []string {
	var dirs []string
	seen := map[string]bool{}
	add := func(dir string) {
		if dir == "" || !filepath.IsAbs(dir) {
			return
		}
		dir = filepath.Clean(dir)
		if !seen[dir] {
			seen[dir] = true
			dirs = append(dirs, dir)
		}
	}
	for _, dir := range filepath.SplitList(pathEnv) {
		add(dir)
	}
	if home != "" {
		add(filepath.Join(home, ".local", "bin"))
		for _, dir := range SystemDirs {
			add(dir)
		}
	}
	return dirs
}

// lookPath finds the first executable name in dirs, skipping anything inside
// the managed runtime root either lexically or through its canonical target,
// so a private dependency copy can never shadow the user's install.
func lookPath(name string, dirs []string, managedRoot string) string {
	managed := managedPrefixes(managedRoot)
	for _, dir := range dirs {
		if within(dir, managed) {
			continue
		}
		candidate := filepath.Join(dir, name)
		if !isExecutableFile(candidate) {
			continue
		}
		if target, err := filepath.EvalSymlinks(candidate); err == nil && within(target, managed) {
			continue
		}
		return candidate
	}
	return ""
}

func managedPrefixes(root string) []string {
	if root == "" {
		return nil
	}
	out := []string{filepath.Clean(root)}
	if canonical, err := filepath.EvalSymlinks(root); err == nil && canonical != out[0] {
		out = append(out, canonical)
	}
	return out
}

func within(path string, prefixes []string) bool {
	path = filepath.Clean(path)
	for _, p := range prefixes {
		if path == p || strings.HasPrefix(path, p+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func statExecutable(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return StateMissing
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return StateNotExecutable
	}
	return StateAvailable
}

func isExecutableFile(path string) bool { return statExecutable(path) == StateAvailable }

func envMap(env []string) map[string]string {
	out := make(map[string]string, len(env))
	for _, kv := range env {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			out[kv[:i]] = kv[i+1:]
		}
	}
	return out
}
