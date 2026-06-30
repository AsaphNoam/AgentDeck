package terminal

import (
	"os"
	"os/exec"
	"runtime"
)

// Capabilities reports which TerminalDriver the host can use (§3.5). The
// xterm/PTY driver is always available (pure-Go PTY on macOS and Linux), so the
// terminal interface itself is never globally disabled; tmux and iTerm2 are
// offered only when present. Surfaced verbatim at GET /api/capabilities (§8.5).
type Capabilities struct {
	Terminal TerminalCapabilities `json:"terminal"`
}

type TerminalCapabilities struct {
	Available     bool               `json:"available"`
	Drivers       DriverAvailability `json:"drivers"`
	DefaultDriver string             `json:"default_driver"`
}

type DriverAvailability struct {
	Xterm  bool          `json:"xterm"`
	Tmux   bool          `json:"tmux"`
	ITerm2 OptionalDriver `json:"iterm2"`
}

// OptionalDriver carries a reason string when an optional driver is unavailable,
// so the UI can disable the option and show why (§3.5).
type OptionalDriver struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

// Probe builds the capability report for the current host.
func Probe() Capabilities {
	return Capabilities{Terminal: TerminalCapabilities{
		Available:     true, // xterm default is always available
		DefaultDriver: "xterm",
		Drivers: DriverAvailability{
			Xterm:  true,
			Tmux:   onPath("tmux"),
			ITerm2: probeITerm2(),
		},
	}}
}

// DriverAvailable reports whether a named driver can be used on this host. Used
// to map an explicit unavailable-driver request to 422 (§3.5).
func (c Capabilities) DriverAvailable(name string) bool {
	switch name {
	case "", "xterm":
		return c.Terminal.Drivers.Xterm
	case "tmux":
		return c.Terminal.Drivers.Tmux
	case "iterm2":
		return c.Terminal.Drivers.ITerm2.Available
	default:
		return false
	}
}

func onPath(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

// probeITerm2 reports iTerm2 availability. The driver itself lands in 6.7; until
// then it is reported unavailable, but the probe path (macOS-only, app present)
// is already wired so the UI gating is correct.
func probeITerm2() OptionalDriver {
	if runtime.GOOS != "darwin" {
		return OptionalDriver{Reason: "iTerm2 is only available on macOS"}
	}
	if _, err := os.Stat("/Applications/iTerm.app"); err != nil {
		return OptionalDriver{Reason: "iTerm2 is not installed"}
	}
	// The macOS app is present, but the AppleScript driver is not wired until 6.7.
	return OptionalDriver{Reason: "iTerm2 driver not yet enabled"}
}
