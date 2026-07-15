package release

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// wrapperScript is the private-runtime wrapper shipped inside every archive at
// bin/agentdeck. It prepends only the bundled Node runtime and ACP adapter bin
// directories to PATH — leaving the rest of the user PATH available to provider
// tooling — then execs the FTS5 Go binary. It resolves its own location, so it
// works both directly and through the current pointer (TS-06.R15).
// pwd -P resolves the physical version directory (not the current symlink), so a
// running process keeps using its own immutable runtime even if an update
// repoints current mid-run (FS-10.R7).
const wrapperScript = `#!/bin/sh
# AgentDeck private-runtime wrapper (generated; do not edit).
set -e
here="$(cd "$(dirname "$0")" && pwd -P)"
root="$(cd "$here/.." && pwd -P)"
PATH="$root/runtime/node/bin:$root/runtime/node_modules/.bin:$PATH"
export PATH
exec "$root/libexec/agentdeck" "$@"
`

// WriteWrapper writes the private-runtime wrapper into a version directory being
// assembled (versionDir/bin/agentdeck). Assembly and tests share it (INV §2).
func WriteWrapper(versionDir string) error {
	bin := filepath.Join(versionDir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(bin, "agentdeck"), []byte(wrapperScript), 0o755)
}

// WriteShim writes the stable user command shim at <appRoot>/bin/agentdeck. The
// shim bakes in the absolute path to the current pointer, so a fixed PATH entry
// (the one directory AgentDeck owns) always resolves to the active release
// (TS-06.R16, FS-10.R3). It is rewritten idempotently on every install/update.
func (l *Layout) WriteShim() error {
	if err := os.MkdirAll(l.BinDir(), 0o700); err != nil {
		return err
	}
	target := filepath.Join(l.CurrentLink(), "bin", "agentdeck")
	if strings.Contains(target, `"`) {
		return fmt.Errorf("application root path contains a double quote; unsupported: %q", target)
	}
	script := "#!/bin/sh\n" +
		"# AgentDeck command shim (generated; do not edit). Resolves the active release.\n" +
		fmt.Sprintf("exec \"%s\" \"$@\"\n", target)
	return os.WriteFile(l.ShimPath(), []byte(script), 0o755)
}
