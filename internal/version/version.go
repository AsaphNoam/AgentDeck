// Package version exposes build metadata injected at link time via -ldflags.
//
// install.sh and the Makefile set these with:
//
//	-ldflags "-X github.com/agentdeck/agentdeck/internal/version.Version=0.1.0 \
//	          -X github.com/agentdeck/agentdeck/internal/version.Commit=$(git rev-parse --short HEAD) \
//	          -X github.com/agentdeck/agentdeck/internal/version.Date=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
//
// When built without ldflags (e.g. `go build` or `go test`), the defaults below
// apply so the binary still reports a sensible, non-empty version.
package version

// These vars are overwritten at build time via -ldflags -X. Keep them as plain
// package-level strings (not const) so the linker can set them.
var (
	// Version is the semantic version of the build (e.g. "0.1.0").
	Version = "0.1.0-dev"
	// Commit is the short git SHA of the build, if available.
	Commit = "none"
	// Date is the RFC3339 UTC build timestamp, if available.
	Date = "unknown"
)

// String renders the human-readable version line used by `agentdeck --version`.
func String() string {
	return Version + " (commit " + Commit + ", built " + Date + ")"
}
