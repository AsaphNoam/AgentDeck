package terminal

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// tmuxDriver runs the CLI inside a named, detached tmux session (§2.1) so the
// user can reattach outside the browser. Prompts are delivered with `send-keys`;
// the controlling tty and pane pid are read back with `display-message`. The
// same PTY-bridge UI can attach to the session, or the user attaches manually.
type tmuxDriver struct{}

func (tmuxDriver) Name() string { return "tmux" }

// StartTab creates a detached session running argv, then reads back the pane tty
// and pid. The session name embeds the agent id for later reattach/teardown.
func (tmuxDriver) StartTab(spec TabSpec) (*Tab, error) {
	if len(spec.Command) == 0 {
		return nil, fmt.Errorf("terminal: empty launch command")
	}
	name := spec.tmuxSession()
	// Pass argv as a single shell-safe command; tmux runs it via the user's shell.
	args := []string{"new-session", "-d", "-s", name}
	if spec.Cwd != "" {
		args = append(args, "-c", spec.Cwd)
	}
	args = append(args, shellJoin(spec.Command))
	cmd := exec.Command("tmux", args...)
	cmd.Env = spec.Env
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("terminal: tmux new-session: %w: %s", err, strings.TrimSpace(string(out)))
	}

	tab := &Tab{Driver: "tmux", tmuxName: name, IDs: map[string]string{"tmux_session": name}}
	if tty, err := tmuxDisplay(name, "#{pane_tty}"); err == nil {
		tab.TTY = tty
	}
	if pidStr, err := tmuxDisplay(name, "#{pane_pid}"); err == nil {
		if pid, perr := strconv.Atoi(strings.TrimSpace(pidStr)); perr == nil {
			tab.PGID = pid
		}
	}
	if spec.Title != "" {
		_ = exec.Command("tmux", "rename-window", "-t", name, spec.Title).Run()
	}
	return tab, nil
}

func (tmuxDriver) WriteText(tab *Tab, text string) error {
	if tab == nil || tab.tmuxName == "" {
		return fmt.Errorf("terminal: no tmux session for tab")
	}
	// Two send-keys: the literal text, then Enter, so the line editor submits it.
	if err := exec.Command("tmux", "send-keys", "-t", tab.tmuxName, "-l", text).Run(); err != nil {
		return fmt.Errorf("terminal: tmux send-keys: %w", err)
	}
	return exec.Command("tmux", "send-keys", "-t", tab.tmuxName, "Enter").Run()
}

func (tmuxDriver) ReadTTY(tab *Tab) (string, error) {
	if tab == nil || tab.tmuxName == "" {
		return "", fmt.Errorf("terminal: nil tmux tab")
	}
	if tab.TTY != "" {
		return tab.TTY, nil
	}
	return tmuxDisplay(tab.tmuxName, "#{pane_tty}")
}

// CloseTab kills the tmux session, terminating the CLI inside it.
func (tmuxDriver) CloseTab(tab *Tab) error {
	if tab == nil || tab.tmuxName == "" {
		return nil
	}
	return exec.Command("tmux", "kill-session", "-t", tab.tmuxName).Run()
}

// RevealTab is a no-op server-side: the user attaches with `tmux attach`, or the
// embedded panel attaches to the pane tty.
func (tmuxDriver) RevealTab(tab *Tab) error { return nil }

func (s TabSpec) tmuxSession() string {
	// Stable, collision-free per agent: the runtime sets a unique Title, but the
	// session name must be tmux-safe, so derive it from a sanitized title.
	base := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			return r
		default:
			return '-'
		}
	}, s.Title)
	if base == "" {
		base = "agent"
	}
	return "agentdeck-" + base
}

func tmuxDisplay(session, fmtStr string) (string, error) {
	out, err := exec.Command("tmux", "display-message", "-p", "-t", session, fmtStr).Output()
	if err != nil {
		return "", fmt.Errorf("terminal: tmux display-message: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// shellJoin renders argv as a single shell-quoted command line for tmux.
func shellJoin(argv []string) string {
	parts := make([]string, len(argv))
	for i, a := range argv {
		parts[i] = shellQuote(a)
	}
	return strings.Join(parts, " ")
}

// shellQuote single-quotes a shell argument, escaping embedded single quotes.
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if !strings.ContainsAny(s, " \t\n\"'\\$`*?[]{}()|&;<>~#") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
