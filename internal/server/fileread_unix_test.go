//go:build unix

package server

import (
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/runtime"
)

// nonRegularRoot returns a working directory whose path is short enough for a
// Unix socket. `t.TempDir()` on macOS exceeds the 104-byte `sun_path` limit, so
// binding there fails with `bind: invalid argument` and the non-regular case
// would silently vanish on the development platform (INV §17).
func nonRegularRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "adfr")
	if err != nil {
		t.Fatalf("short temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	return resolved
}

// TestFileReadRefusesSocket proves a Unix socket inside the root is refused as
// a non-regular file. The kind is decided before the open because opening a
// socket succeeds on macOS and fails on Linux, so the pre-open classification is
// what makes the refusal identical on both (TS-05.R21, FS-03.A37).
func TestFileReadRefusesSocket(t *testing.T) {
	root := nonRegularRoot(t)
	ln, err := net.Listen("unix", filepath.Join(root, "sock"))
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	defer ln.Close()

	_, apiErr := readWorkspaceFile(root, "sock")
	if apiErr == nil || apiErr.Code != runtime.CodeNotAFile {
		t.Fatalf("socket read = %+v, want %q", apiErr, runtime.CodeNotAFile)
	}
}

// TestFileReadRefusesFIFOWithoutBlocking is the regression that fails when the
// read goes back to opening before classifying: opening a FIFO with no writer
// blocks until one appears, so an unbounded read would hang the suite instead of
// refusing. The bounded wait turns that hang into a failure (TS-05.R21,
// FS-03.A37, INV §17).
func TestFileReadRefusesFIFOWithoutBlocking(t *testing.T) {
	root := nonRegularRoot(t)
	if err := syscall.Mkfifo(filepath.Join(root, "pipe"), 0o644); err != nil {
		t.Fatalf("mkfifo: %v", err)
	}

	type result struct{ apiErr *runtime.APIError }
	done := make(chan result, 1)
	go func() {
		_, apiErr := readWorkspaceFile(root, "pipe")
		done <- result{apiErr}
	}()

	select {
	case got := <-done:
		if got.apiErr == nil || got.apiErr.Code != runtime.CodeNotAFile {
			t.Fatalf("fifo read = %+v, want %q", got.apiErr, runtime.CodeNotAFile)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("fifo read blocked: the target is opened before its kind is classified")
	}
}
