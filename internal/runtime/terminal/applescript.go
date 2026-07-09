package terminal

import (
	"bytes"
	"strings"
	"text/template"
)

// AppleScript rendering + escaping for the optional iTerm2 driver (§3.6). All
// dynamic strings that land inside an AppleScript "..." literal MUST pass through
// escapeAppleScript first; text/template performs NO escaping of its own (it is
// text/template, not html/template), so an un-escaped quote or backslash would
// break out of the literal. The LaunchCommand is additionally a shell command,
// so it is argv-joined with shellJoin (the shell-quote layer) BEFORE the
// AppleScript-escape layer — never string-interpolated (§3.6 mandatory rules).

// escapeAppleScript escapes a string for inclusion inside an AppleScript double-
// quoted literal (§3.6): backslash → `\\`, double-quote → `\"`, and newlines are
// turned into explicit `" & return & "` concatenation segments (a raw newline is
// invalid inside an AppleScript string literal). A bare CR (or the CR of a CRLF)
// is dropped so CRLF yields a single `return` rather than two.
func escapeAppleScript(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`" & return & "`)
		case '\r':
			// drop: CRLF collapses to one return; a lone CR would not submit cleanly.
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// scaleColor8to16 maps an 8-bit channel (0–255) to iTerm2's 16-bit AppleScript
// scale (0–65535). 255 → 65535 exactly (×257), so a saturated project accent
// stays saturated (§3.3 color mapping).
func scaleColor8to16(c int) int {
	if c < 0 {
		c = 0
	}
	if c > 255 {
		c = 255
	}
	return c * 257
}

// createTabParams / appearanceParams / writeTextParams carry the PRE-ESCAPED
// fields for their template. Callers must escape before constructing these.
type createTabParams struct {
	TabTitle      string // escaped
	LaunchCommand string // shell-joined then escaped
}

type appearanceParams struct {
	SessionID string // escaped
	TabTitle  string // escaped
	HasColor  bool
	R, G, B   int // 0–65535
}

type writeTextParams struct {
	SessionID string // escaped
	Prompt    string // escaped
}

// The three templates (§3.6). GATED: the AppleScript verb surface (create window,
// write text, session id addressing) is unverified against a live iTerm2 — same
// credential-gated class as the other Phase 6 terminal-CLI behavior. The escaping
// and rendering below ARE tested; the live AppleScript semantics are not.
var (
	// createTabTmpl returns "tty\twindowID\tsessionID" on stdout so StartTab can
	// record the tty in the running row and address the session/window later.
	createTabTmpl = template.Must(template.New("create-tab").Parse(
		`tell application "iTerm2"
  activate
  set newWindow to (create window with default profile)
  tell current session of newWindow
    set name to "{{.TabTitle}}"
    write text "{{.LaunchCommand}}"
    set ttyVal to tty
    set sidVal to id
  end tell
  set wid to id of newWindow
  return ttyVal & tab & wid & tab & sidVal
end tell
`))

	// setAppearanceTmpl sets the session name (title) and, when the project accent
	// is set, the background color (already scaled 0–65535).
	setAppearanceTmpl = template.Must(template.New("set-appearance").Parse(
		`tell application "iTerm2"
  tell session id "{{.SessionID}}"
    set name to "{{.TabTitle}}"
{{if .HasColor}}    set background color to {{"{"}}{{.R}}, {{.G}}, {{.B}}{{"}"}}
{{end}}  end tell
end tell
`))

	// writeTextTmpl types a prompt into the session as if at the keyboard (iTerm2
	// appends a newline / submits), delivering a SendPrompt.
	writeTextTmpl = template.Must(template.New("write-text").Parse(
		`tell application "iTerm2"
  tell session id "{{.SessionID}}"
    write text "{{.Prompt}}"
  end tell
end tell
`))
)

func renderTemplate(t *template.Template, data any) (string, error) {
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
