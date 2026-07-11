// Package configsource resolves read-only Claude Code and Codex configuration
// trees into a redacted, provider-aware view. Resolvers never mutate source
// trees and intentionally expose metadata rather than raw configuration.
package configsource

import (
	"context"

	"github.com/agentdeck/agentdeck/internal/config"
)

const (
	ProviderClaude = "claude-code"
	ProviderCodex  = "codex"

	ModeLinked   = "linked"
	ModeMirrored = "mirrored"
)

// Binding is the resolver-facing form of config.SourceBinding. Keeping the
// alias makes the persistence and resolution contracts impossible to drift.
type Binding = config.SourceBinding

type Resolver interface {
	Discover(ctx context.Context, project config.Project) []Candidate
	Preview(ctx context.Context, binding Binding, project config.Project) (Effective, Report, error)
	Resolve(ctx context.Context, binding Binding, project config.Project) (Effective, Report, error)
}

type Candidate struct {
	Provider string   `json:"provider"`
	Root     string   `json:"root"`
	Profile  string   `json:"profile,omitempty"`
	Found    bool     `json:"found"`
	Health   string   `json:"health"`
	Warnings []string `json:"warnings"`
}

type Effective struct {
	Model         *string                    `json:"model,omitempty"`
	FallbackModel *string                    `json:"fallback_model,omitempty"`
	Effort        *string                    `json:"effort,omitempty"`
	Verbosity     *string                    `json:"verbosity,omitempty"`
	Provider      *string                    `json:"provider,omitempty"`
	Models        []Model                    `json:"models"`
	Assets        []Asset                    `json:"assets"`
	EnvKeys       []EnvironmentKey           `json:"environment_keys"`
	Provenance    map[string]FieldProvenance `json:"provenance"`
}

type Model struct {
	ID     string `json:"id"`
	Name   string `json:"name,omitempty"`
	Source string `json:"source"`
}

type Asset struct {
	Kind          string `json:"kind"`
	Name          string `json:"name,omitempty"`
	Path          string `json:"path"`
	Scope         string `json:"scope"`
	SHA256        string `json:"sha256,omitempty"`
	Detachability string `json:"detachability"`
	Status        string `json:"status"`
}

type EnvironmentKey struct {
	Name       string `json:"name"`
	Scope      string `json:"scope"`
	Configured bool   `json:"configured"`
}

type FieldProvenance struct {
	Scope string `json:"scope"`
	Path  string `json:"path,omitempty"`
	Key   string `json:"key"`
}

type Report struct {
	FilesRead     []SourceFile  `json:"files_read"`
	Skipped       []SkippedPath `json:"skipped"`
	UnknownKeys   []UnknownKey  `json:"unknown_keys"`
	Warnings      []string      `json:"warnings"`
	Fingerprints  []Fingerprint `json:"fingerprints"`
	ApprovedRoots []string      `json:"approved_roots"`
	SourceDigest  string        `json:"source_digest"`
}

type SourceFile struct {
	Path  string `json:"path"`
	Scope string `json:"scope"`
	Kind  string `json:"kind"`
}

type SkippedPath struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type UnknownKey struct {
	Path        string `json:"path"`
	Key         string `json:"key"`
	Disposition string `json:"disposition"`
}

type Fingerprint struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

func emptyEffective() Effective {
	return Effective{
		Models:     make([]Model, 0),
		Assets:     make([]Asset, 0),
		EnvKeys:    make([]EnvironmentKey, 0),
		Provenance: make(map[string]FieldProvenance),
	}
}

func emptyReport() Report {
	return Report{
		FilesRead:     make([]SourceFile, 0),
		Skipped:       make([]SkippedPath, 0),
		UnknownKeys:   make([]UnknownKey, 0),
		Warnings:      make([]string, 0),
		Fingerprints:  make([]Fingerprint, 0),
		ApprovedRoots: make([]string, 0),
	}
}
