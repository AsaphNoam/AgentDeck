package release

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// wrapperScript is the private-runtime wrapper shipped inside every archive at
// bin/chuck. It publishes the release's managed runtime root and execs the
// FTS5 Go binary. It deliberately leaves PATH and every provider executable
// override untouched: managed adapters launch through absolute private
// Node/entrypoint paths, and the shared resolver chooses the user's installed
// provider or this release's bundle per backend (TS-06.R30, TS-04.R75). It
// resolves its own location, so it works both directly and through the current
// pointer (TS-06.R15). pwd -P resolves the physical version directory (not the
// current symlink), so a running process keeps using its own immutable runtime
// even if an update repoints current mid-run (FS-10.R7).
const wrapperScript = `#!/bin/sh
# Chuck private-runtime wrapper (generated; do not edit).
set -e
here="$(cd "$(dirname "$0")" && pwd -P)"
root="$(cd "$here/.." && pwd -P)"
CHUCK_RUNTIME_ROOT="$root/runtime"
export CHUCK_RUNTIME_ROOT
exec "$root/libexec/chuck" "$@"
`

// WriteWrapper writes the private-runtime wrapper into a version directory being
// assembled (versionDir/bin/chuck). Assembly and tests share it (INV §2).
func WriteWrapper(versionDir string) error {
	bin := filepath.Join(versionDir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(bin, "chuck"), []byte(wrapperScript), 0o755)
}

// WriteShim writes the stable user command shim at <appRoot>/bin/chuck. The
// shim bakes in the absolute path to the current pointer, so a fixed PATH entry
// (the one directory Chuck owns) always resolves to the active release
// (TS-06.R16, FS-10.R3). It is rewritten idempotently on every install/update.
func (l *Layout) WriteShim() error {
	if err := os.MkdirAll(l.BinDir(), 0o700); err != nil {
		return err
	}
	target := filepath.Join(l.CurrentLink(), "bin", "chuck")
	if strings.Contains(target, `"`) {
		return fmt.Errorf("application root path contains a double quote; unsupported: %q", target)
	}
	script := "#!/bin/sh\n" +
		"# Chuck command shim (generated; do not edit). Resolves the active release.\n" +
		fmt.Sprintf("exec \"%s\" \"$@\"\n", target)

	// Readers execute this path directly, so writing it in place would expose an
	// empty or partial script after truncation. Publish a fully synced sibling
	// with rename, then sync the directory entry (INV §9, TS-06.R17).
	tmp, err := os.CreateTemp(l.BinDir(), ".chuck-*")
	if err != nil {
		return fmt.Errorf("create shim temp: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if err := tmp.Chmod(0o755); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("chmod shim temp: %w", err)
	}
	if _, err := tmp.WriteString(script); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("write shim temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("sync shim temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close shim temp: %w", err)
	}
	if err := os.Rename(tmpName, l.ShimPath()); err != nil {
		cleanup()
		return fmt.Errorf("publish shim: %w", err)
	}
	if err := fsyncDir(l.BinDir()); err != nil {
		return fmt.Errorf("sync shim directory: %w", err)
	}
	return nil
}
