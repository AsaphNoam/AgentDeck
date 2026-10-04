package providerexec

import (
	"context"
	"os/exec"
	"regexp"
	"time"
)

// Version probe bounds (TS-04.R72).
const (
	probeTimeout   = 2 * time.Second
	probeOutputCap = 8 << 10
)

// probeSlots is the process-wide two-probe concurrency limit.
var probeSlots = make(chan struct{}, 2)

var versionPattern = regexp.MustCompile(`\b[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]{1,24})?\b`)

// ProbeVersion asks an executable for `--version` without a shell, stdin or a
// model turn, and returns only the parsed version. Timeout, non-zero exit,
// oversized or unrecognized output all yield "" — unknown, never
// incompatible, and never a launch veto (TS-04.R72, INV §12). A parsed version
// proves neither authentication nor feature support.
func ProbeVersion(ctx context.Context, path string, env []string) string {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	select {
	case probeSlots <- struct{}{}:
	case <-ctx.Done():
		return ""
	}
	defer func() { <-probeSlots }()

	out := &cappedBuffer{limit: probeOutputCap}
	cmd := exec.CommandContext(ctx, path, "--version")
	cmd.Env = env
	cmd.Stdout, cmd.Stderr = out, out
	// A timed-out child is killed; WaitDelay bounds a grandchild holding the
	// pipes open.
	cmd.WaitDelay = 250 * time.Millisecond
	if err := cmd.Run(); err != nil || ctx.Err() != nil {
		return ""
	}
	return versionPattern.FindString(string(out.buf))
}

// cappedBuffer captures at most limit bytes at the writer boundary while
// accepting (and discarding) the rest, so a chatty child never blocks.
type cappedBuffer struct {
	buf   []byte
	limit int
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if room := b.limit - len(b.buf); room > 0 {
		if len(p) < room {
			room = len(p)
		}
		b.buf = append(b.buf, p[:room]...)
	}
	return len(p), nil
}
