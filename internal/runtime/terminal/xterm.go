package terminal

import (
	"fmt"
	"os/exec"
	"syscall"

	"github.com/creack/pty"
)

// xtermDriver is the default, cross-platform driver: it runs the CLI under a
// server-side PTY (github.com/creack/pty) and bridges the PTY master to the
// browser's xterm.js panel over a WebSocket (§2.1, §3.4). Works identically on
// macOS and Linux with no platform scripting.
type xtermDriver struct{}

func (xtermDriver) Name() string { return "xterm" }

// StartTab launches argv under a freshly-opened PTY. The child becomes a session
// leader with the slave tty as its controlling terminal (Setsid+Setctty), so its
// pid is the process-group leader the runtime signals for Cancel/Stop, and the
// recorded tty path is the one hooks/`ps` see (§3.1 steps 4–6).
func (xtermDriver) StartTab(spec TabSpec) (*Tab, error) {
	if len(spec.Command) == 0 {
		return nil, fmt.Errorf("terminal: empty launch command")
	}
	ptmx, tty, err := pty.Open()
	if err != nil {
		return nil, fmt.Errorf("terminal: open pty: %w", err)
	}
	ttyName := tty.Name()

	cmd := exec.Command(spec.Command[0], spec.Command[1:]...)
	cmd.Dir = spec.Cwd
	cmd.Env = spec.Env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = tty, tty, tty
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true}

	if err := cmd.Start(); err != nil {
		_ = tty.Close()
		_ = ptmx.Close()
		return nil, fmt.Errorf("terminal: start %s: %w", spec.Command[0], err)
	}
	// The child holds its own copy of the slave; the parent only needs the master.
	_ = tty.Close()

	tab := &Tab{
		Driver: "xterm",
		TTY:    ttyName,
		PGID:   cmd.Process.Pid, // session leader (Setsid) → pid == pgid
		ptmx:   ptmx,
		exited: make(chan struct{}),
	}
	// One goroutine reaps the child; closing exited broadcasts to the runtime's
	// liveness watcher and to Stop's grace wait (single Wait avoids a double-reap).
	go func() {
		_ = cmd.Wait()
		close(tab.exited)
	}()
	return tab, nil
}

// WriteText writes the prompt plus a newline to the PTY master, as if typed and
// submitted at the keyboard (§3.1 SendPrompt).
func (xtermDriver) WriteText(tab *Tab, text string) error {
	if tab == nil || tab.ptmx == nil {
		return fmt.Errorf("terminal: no pty for tab")
	}
	_, err := tab.ptmx.WriteString(text + "\n")
	return err
}

func (xtermDriver) ReadTTY(tab *Tab) (string, error) {
	if tab == nil {
		return "", fmt.Errorf("terminal: nil tab")
	}
	return tab.TTY, nil
}

// CloseTab closes the PTY master. The bridged WebSocket then sees EOF and the
// pump ends; the child (already signalled by the runtime's Stop) exits.
func (xtermDriver) CloseTab(tab *Tab) error {
	if tab == nil || tab.ptmx == nil {
		return nil
	}
	return tab.ptmx.Close()
}

// RevealTab is a UI-side concern for the embedded xterm panel; nothing to do
// server-side.
func (xtermDriver) RevealTab(tab *Tab) error { return nil }
