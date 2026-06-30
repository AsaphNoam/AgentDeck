// Package terminal implements the "terminal" Runtime (Phase 6 techspec §3): a
// second runtime.Runtime that drives a terminal emulator instead of the ACP
// stream and derives all status from hook POSTs (§3.3). It dispatches to a
// TerminalDriver seam (§2.1) so the cross-platform xterm.js/PTY driver, the tmux
// driver, and the optional macOS iTerm2 driver (6.7) are interchangeable behind
// one interface; the registry never sees the difference.
package terminal

import "os"

// TabSpec is the fully-resolved input to a driver's StartTab: the launch argv,
// working dir, env, and the cosmetic tab title/color. The runtime composes it
// from the LaunchSpec exactly as the chat runtime composes its spawn (§3.1).
type TabSpec struct {
	Command []string // launch argv: argv[0] is the binary, argv[1:] the args
	Cwd     string
	Env     []string // "K=V" entries (already layered, incl. AGENTDECK_* hook env)
	Title   string   // "{name} · {role}@{project}" (§3.2)
	Color   [3]int   // project accent (0–255 RGB); zero value means "unset"
}

// Tab is the live handle a driver returns for one launched terminal. It carries
// the driver-agnostic fields the runtime records in the running row (TTY, PGID,
// Driver, IDs) plus the driver-specific handles needed for I/O. Only the owning
// driver touches the private handles.
type Tab struct {
	Driver string            // "xterm" | "tmux" | "iterm2"
	TTY    string            // controlling tty path, e.g. /dev/ttys003
	PGID   int               // process group to signal for Cancel/Stop (§3.1)
	IDs    map[string]string // driver-specific ids → running.driver_ids (§3.1)

	// xterm/PTY driver handles.
	ptmx   *os.File      // PTY master, bridged to the browser over the WS (§3.4)
	exited chan struct{} // closed once the child process has exited & been reaped

	// tmux driver handles.
	tmuxName string // named tmux session, e.g. "agentdeck-a_8f3c12"
}

// TerminalDriver is the seam (§2.1). The xterm/PTY and tmux drivers are
// cross-platform; the iTerm2 driver (6.7) is an optional macOS extra. StartTab
// launches the CLI under the emulator and reads back the tty; WriteText delivers
// a prompt; ReadTTY returns the recorded tty; CloseTab releases the surface
// (close the PTY / kill the tmux session); RevealTab brings the tab forward.
//
// Process signalling for Cancel/Stop is done by the runtime against Tab.PGID
// (§3.1), not by the driver, so the signalling path is identical across drivers.
type TerminalDriver interface {
	Name() string
	StartTab(spec TabSpec) (*Tab, error)
	WriteText(tab *Tab, text string) error
	ReadTTY(tab *Tab) (string, error)
	CloseTab(tab *Tab) error
	RevealTab(tab *Tab) error
}
