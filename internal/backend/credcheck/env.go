package credcheck

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// lookPath resolves a CLI binary: an absolute/relative path is used verbatim
// (so fs-faked tests can point at a temp script), otherwise it is looked up on
// PATH. Mirrors exec.LookPath's error contract.
func lookPath(bin string) (string, error) {
	if strings.ContainsRune(bin, filepath.Separator) {
		if _, err := os.Stat(bin); err != nil {
			return "", err
		}
		return bin, nil
	}
	return exec.LookPath(bin)
}

// homeDir resolves the user's home directory, preferring an explicit HOME in the
// merged env (so credential probes are testable with a temp home) and falling
// back to os.UserHomeDir.
func homeDir(mergedEnv map[string]string) string {
	if h := mergedEnv["HOME"]; h != "" {
		return h
	}
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return ""
}

// fileExists reports whether path names an existing regular file/dir.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// hasProviderAPIKey reports whether the merged env carries any provider API key
// (any *_API_KEY entry with a non-empty value), the signal opencode uses to
// authenticate a provider from the environment.
func hasProviderAPIKey(mergedEnv map[string]string) bool {
	for k, v := range mergedEnv {
		if v != "" && strings.HasSuffix(strings.ToUpper(k), "_API_KEY") {
			return true
		}
	}
	return false
}

// buildEnv creates an os.Environ-style slice from the merged env map,
// inheriting the current process environment. Keys in mergedEnv override
// any matching key from the host env. Secret key names are never logged.
func buildEnv(mergedEnv map[string]string) []string {
	base := os.Environ()
	// Build override index.
	overrides := make(map[string]string, len(mergedEnv))
	for k, v := range mergedEnv {
		overrides[strings.ToUpper(k)] = v
	}
	out := make([]string, 0, len(base)+len(mergedEnv))
	// Pass through host env, replacing keys that are overridden.
	seen := make(map[string]bool, len(overrides))
	for _, kv := range base {
		idx := strings.IndexByte(kv, '=')
		if idx < 0 {
			out = append(out, kv)
			continue
		}
		key := strings.ToUpper(kv[:idx])
		if v, ok := overrides[key]; ok {
			out = append(out, kv[:idx]+"="+v)
			seen[key] = true
		} else {
			out = append(out, kv)
		}
	}
	// Append any override keys not already present in host env.
	for k, v := range mergedEnv {
		if !seen[strings.ToUpper(k)] {
			out = append(out, k+"="+v)
		}
	}
	return out
}

// sanitizeOutput removes lines containing secret-like tokens from CLI output
// before surfacing them in an error Detail (we never want to echo keys in logs).
func sanitizeOutput(s string) string {
	lines := strings.Split(s, "\n")
	safe := lines[:0]
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "key") || strings.Contains(lower, "token") || strings.Contains(lower, "secret") {
			continue
		}
		safe = append(safe, line)
	}
	if len(safe) == 0 {
		return "auth_failed"
	}
	return strings.Join(safe, "\n")
}
