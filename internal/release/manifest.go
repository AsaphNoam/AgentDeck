package release

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// internalManifestName is the manifest that travels inside the archive/version
// directory and records the runtime's identity (TS-06.R15).
const internalManifestName = "manifest.json"

// requiredLayout is the exact set of files an extracted version directory must
// contain before it may be activated (TS-06.R15). Kept in one place so assembly,
// verification, and tests agree (INV §2).
var requiredLayout = []string{
	"bin/agentdeck",                              // wrapper
	"libexec/agentdeck",                          // FTS5 Go binary
	"runtime/node/bin/node",                      // private Node runtime
	"runtime/node_modules/.bin/claude-agent-acp", // official Claude ACP adapter
	"runtime/node_modules/.bin/codex-acp",        // official Codex ACP adapter
	internalManifestName,                         // internal identity manifest
}

// ReleaseManifest is the small, machine-readable file published alongside the
// archive on a GitHub Release. It tells the updater exactly what to download and
// how to verify it before unpacking (TS-06.R17).
type ReleaseManifest struct {
	Version string `json:"version"`
	Target  string `json:"target"`  // always "darwin-arm64" for the MVP
	Archive string `json:"archive"` // archive filename on the release
	Size    int64  `json:"size"`    // archive size in bytes
	SHA256  string `json:"sha256"`  // lowercase hex SHA-256 of the archive
}

// InternalManifest is manifest.json inside the archive: the runtime's own record
// of its version, target, and pinned component versions (TS-06.R14/R15).
type InternalManifest struct {
	Version    string            `json:"version"`
	Target     string            `json:"target"`
	Components map[string]string `json:"components"` // node, claude-agent-acp, codex-acp, agentdeck
}

// Validate reports whether a release manifest is internally coherent and targets
// this MVP's platform.
func (m ReleaseManifest) Validate() error {
	if m.Version == "" {
		return fmt.Errorf("release manifest: empty version")
	}
	if m.Target != Target {
		return fmt.Errorf("release manifest: target %q is not %q (this MVP is macOS arm64 only)", m.Target, Target)
	}
	if m.Archive == "" {
		return fmt.Errorf("release manifest: empty archive filename")
	}
	if m.Size <= 0 {
		return fmt.Errorf("release manifest: non-positive size %d", m.Size)
	}
	if len(m.SHA256) != 64 {
		return fmt.Errorf("release manifest: sha256 must be 64 hex chars, got %d", len(m.SHA256))
	}
	return nil
}

// ReadInternalManifest reads and parses manifest.json from an extracted version
// directory.
func ReadInternalManifest(dir string) (InternalManifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, internalManifestName))
	if err != nil {
		return InternalManifest{}, fmt.Errorf("read internal manifest: %w", err)
	}
	var m InternalManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return InternalManifest{}, fmt.Errorf("parse internal manifest: %w", err)
	}
	return m, nil
}

// WriteInternalManifest writes manifest.json into a version directory being
// assembled. Used by release assembly and tests (INV §2).
func WriteInternalManifest(dir string, m InternalManifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(filepath.Join(dir, internalManifestName), data, 0o600)
}

// VerifyLayout checks that an extracted version directory contains every required
// component before it may become current (TS-06.R15, R17).
func VerifyLayout(dir string) error {
	for _, rel := range requiredLayout {
		p := filepath.Join(dir, rel)
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("release layout: missing %s: %w", rel, err)
		}
	}
	return nil
}

// verifyInternalManifest confirms the extracted runtime's own manifest matches
// the expected version and this platform. A mismatch means a corrupt or
// mislabeled archive; the caller preserves the current runtime (TS-06.R17).
func verifyInternalManifest(dir, wantVersion string) error {
	m, err := ReadInternalManifest(dir)
	if err != nil {
		return err
	}
	if m.Target != Target {
		return fmt.Errorf("internal manifest target %q is not %q", m.Target, Target)
	}
	if wantVersion != "" && m.Version != wantVersion {
		return fmt.Errorf("internal manifest version %q does not match release %q", m.Version, wantVersion)
	}
	for _, component := range []string{"node", "claude-agent-acp", "codex-acp", "agentdeck"} {
		if m.Components[component] == "" {
			return fmt.Errorf("internal manifest is missing %s component version", component)
		}
	}
	return nil
}
