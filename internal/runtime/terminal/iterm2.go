package terminal

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// iterm2Driver is the optional macOS-only TerminalDriver (§2.2): it drives real
// iTerm2 windows via AppleScript shelled out to `osascript` (no CGo/ScriptingBridge,
// so the binary stays pure-Go). It is selected only when the capability probe
// advertises it (§3.5); on any other host the server rejects an explicit
// `driver:"iterm2"` request with 422 before a launch reaches here.
//
// GATED: the AppleScript verb surface and object addressing are unverified against
// a live iTerm2 (same credentialed class as all Phase 6 terminal-CLI behavior).
// The escaping/rendering (applescript.go) and the capability gating ARE tested.
type iterm2Driver struct{}

func (iterm2Driver) Name() string { return "iterm2" }

// osascriptTimeout bounds every osascript call so a stuck AppleScript never hangs
// the request (§ error handling: "wrap every osascript call with a timeout").
const osascriptTimeout = 4 * time.Second

// StartTab renders the create-tab AppleScript (with both quoting layers applied to
// the launch command), pipes it to `osascript -`, and reads back
// "tty\twindowID\tsessionID". It then best-effort sets the title/color.
func (d iterm2Driver) StartTab(spec TabSpec) (*Tab, error) {
	if len(spec.Command) == 0 {
		return nil, fmt.Errorf("terminal: empty launch command")
	}
	// Two quoting layers (§3.6): argv → shell-quoted command line → AppleScript-escaped.
	launchCmd := shellJoin(spec.Command)
	if spec.Cwd != "" {
		launchCmd = "cd " + shellQuote(spec.Cwd) + " && " + launchCmd
	}
	script, err := renderTemplate(createTabTmpl, createTabParams{
		TabTitle:      escapeAppleScript(spec.Title),
		LaunchCommand: escapeAppleScript(launchCmd),
	})
	if err != nil {
		return nil, fmt.Errorf("terminal: render create-tab: %w", err)
	}
	out, err := runOsascript(script)
	if err != nil {
		return nil, err
	}
	fields := strings.Split(strings.TrimSpace(out), "\t")
	if len(fields) < 3 {
		return nil, fmt.Errorf("terminal: iterm2 create-tab returned %q, want tty\\twindow\\tsession", out)
	}
	tab := &Tab{
		Driver:         "iterm2",
		TTY:            strings.TrimSpace(fields[0]),
		itermWindowID:  strings.TrimSpace(fields[1]),
		itermSessionID: strings.TrimSpace(fields[2]),
		IDs: map[string]string{
			"iterm_window":  strings.TrimSpace(fields[1]),
			"iterm_session": strings.TrimSpace(fields[2]),
		},
	}
	// Title/color are cosmetic; a failure here must not fail the launch.
	_ = d.applyAppearance(tab, spec)
	return tab, nil
}

// applyAppearance sets the session title and (when the project accent is set) its
// background color. Best-effort — cosmetic only.
func (iterm2Driver) applyAppearance(tab *Tab, spec TabSpec) error {
	hasColor := spec.Color != [3]int{}
	script, err := renderTemplate(setAppearanceTmpl, appearanceParams{
		SessionID: escapeAppleScript(tab.itermSessionID),
		TabTitle:  escapeAppleScript(spec.Title),
		HasColor:  hasColor,
		R:         scaleColor8to16(spec.Color[0]),
		G:         scaleColor8to16(spec.Color[1]),
		B:         scaleColor8to16(spec.Color[2]),
	})
	if err != nil {
		return err
	}
	_, err = runOsascript(script)
	return err
}

// WriteText types the prompt into the session (iTerm2 submits it), delivering a
// SendPrompt over AppleScript.
func (iterm2Driver) WriteText(tab *Tab, text string) error {
	if tab == nil || tab.itermSessionID == "" {
		return fmt.Errorf("terminal: no iterm2 session for tab")
	}
	script, err := renderTemplate(writeTextTmpl, writeTextParams{
		SessionID: escapeAppleScript(tab.itermSessionID),
		Prompt:    escapeAppleScript(text),
	})
	if err != nil {
		return fmt.Errorf("terminal: render write-text: %w", err)
	}
	_, err = runOsascript(script)
	return err
}

// CloseTab closes the iTerm2 window hosting the session. Best-effort: the runtime
// already SIGTERM/SIGKILL'd the process group by Tab.PGID before calling this.
func (iterm2Driver) CloseTab(tab *Tab) error {
	if tab == nil || tab.itermWindowID == "" {
		return nil
	}
	// window id is a numeric literal from iTerm2's own output — addressed unquoted.
	script := fmt.Sprintf("tell application \"iTerm2\"\n  tell window id %s\n    close\n  end tell\nend tell\n", tab.itermWindowID)
	_, err := runOsascript(script)
	return err
}

// runOsascript pipes the script to `osascript -` (reads the program from stdin,
// avoiding shell-quoting the whole script on the command line) with a bounded
// timeout, returning stdout and mapping failures — including stderr — to an
// actionable error.
func runOsascript(script string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), osascriptTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "osascript", "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("terminal: iterm2 osascript timed out after %s", osascriptTimeout)
	}
	if err != nil {
		stderr := ""
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = strings.TrimSpace(string(ee.Stderr))
		}
		if stderr != "" {
			return "", fmt.Errorf("terminal: iterm2 osascript failed: %w: %s", err, stderr)
		}
		return "", fmt.Errorf("terminal: iterm2 osascript failed: %w", err)
	}
	return string(out), nil
}
