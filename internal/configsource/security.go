package configsource

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var (
	ErrApprovalRequired = errors.New("config source: approval required")
	ErrInvalidSource    = errors.New("config source: invalid")
)

// approvedPath resolves symlinks for an existing path and verifies the result
// is within a canonical root the user approved during preview.
func approvedPath(path string, approved []string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("%w: canonicalize path", ErrInvalidSource)
	}
	canonical, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	canonical = filepath.Clean(canonical)
	for _, root := range approved {
		cleanRoot := filepath.Clean(root)
		rel, relErr := filepath.Rel(cleanRoot, canonical)
		if relErr == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return canonical, nil
		}
	}
	return "", fmt.Errorf("%w: path escapes approved roots", ErrApprovalRequired)
}

func fingerprintFile(path, scope, kind string, approved []string) (SourceFile, Fingerprint, []byte, error) {
	canonical, err := approvedPath(path, approved)
	if err != nil {
		return SourceFile{}, Fingerprint{}, nil, err
	}
	data, err := os.ReadFile(canonical)
	if err != nil {
		return SourceFile{}, Fingerprint{}, nil, err
	}
	sum := sha256.Sum256(data)
	return SourceFile{Path: canonical, Scope: scope, Kind: kind}, Fingerprint{
		Path: canonical, SHA256: hex.EncodeToString(sum[:]), Size: int64(len(data)),
	}, data, nil
}

func generationKey(fingerprints []Fingerprint) string {
	ordered := append([]Fingerprint{}, fingerprints...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Path < ordered[j].Path })
	h := sha256.New()
	for _, fp := range ordered {
		_, _ = h.Write([]byte(fp.Path))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(fp.SHA256))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func canonicalRoot(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("%w: canonicalize root", ErrInvalidSource)
	}
	canonical, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(canonical)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%w: root is not a directory", ErrInvalidSource)
	}
	return filepath.Clean(canonical), nil
}

func isSecretKey(key string) bool {
	normalized := strings.ToLower(strings.NewReplacer("-", "_", ".", "_").Replace(key))
	for _, part := range []string{"env", "token", "secret", "password", "credential", "header", "auth", "helper", "api_key", "apikey"} {
		if normalized == part || strings.Contains(normalized, "_"+part) || strings.Contains(normalized, part+"_") {
			return true
		}
	}
	return false
}

func sortedKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func recordUnknown(report *Report, path string, values map[string]any, known map[string]struct{}) {
	for _, key := range sortedKeys(values) {
		if _, ok := known[key]; ok || isSecretKey(key) {
			continue
		}
		report.UnknownKeys = append(report.UnknownKeys, UnknownKey{Path: path, Key: key, Disposition: "native_passthrough"})
	}
}

func appendEnvMetadata(effective *Effective, scope string, value any) {
	env, ok := value.(map[string]any)
	if !ok {
		return
	}
	for _, key := range sortedKeys(env) {
		effective.EnvKeys = append(effective.EnvKeys, EnvironmentKey{Name: key, Scope: scope, Configured: true})
	}
}
