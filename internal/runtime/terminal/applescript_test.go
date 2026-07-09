package terminal

import (
	"strings"
	"testing"
)

// TestEscapeAppleScript covers the mandatory §3.6 escaping rules: backslashes and
// double-quotes are escaped for an AppleScript string literal, and newlines become
// explicit `" & return & "` concatenation segments (never a raw newline).
func TestEscapeAppleScript(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "hello world", "hello world"},
		{"double_quote", `say "hi"`, `say \"hi\"`},
		{"backslash", `a\b\c`, `a\\b\\c`},
		{"backslash_then_quote", `path\"x"`, `path\\\"x\"`},
		{"newline", "line1\nline2", `line1" & return & "line2`},
		{"crlf_collapses", "line1\r\nline2", `line1" & return & "line2`},
		{"lone_cr_dropped", "a\rb", "ab"},
		{"multi_newline", "a\nb\nc", `a" & return & "b" & return & "c`},
		{"quote_and_newline", "x\"y\nz", `x\"y" & return & "z`},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := escapeAppleScript(tc.in); got != tc.want {
				t.Errorf("escapeAppleScript(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestShellQuoteArgv covers the shell-quoting layer applied to the launch argv
// BEFORE the AppleScript-escape layer (§3.6): quotes, backslashes, spaces, and
// newlines in an argument must be single-quote protected so the shell sees one
// argument, and an embedded single quote is broken out correctly.
func TestShellQuoteArgv(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want string
	}{
		{"simple", []string{"claude", "--model", "sonnet"}, "claude --model sonnet"},
		{"space_arg", []string{"echo", "hello world"}, "echo 'hello world'"},
		{"single_quote", []string{"echo", "it's"}, `echo 'it'\''s'`},
		{"double_quote", []string{"echo", `a"b`}, `echo 'a"b'`},
		{"backslash", []string{"echo", `a\b`}, `echo 'a\b'`},
		{"empty_arg", []string{"echo", ""}, "echo ''"},
		{"dollar_and_backtick", []string{"echo", "$HOME `id`"}, "echo '$HOME `id`'"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shellJoin(tc.in); got != tc.want {
				t.Errorf("shellJoin(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestScaleColor8to16 checks the 0–255 → 0–65535 mapping keeps the endpoints
// saturated and clamps out-of-range inputs.
func TestScaleColor8to16(t *testing.T) {
	cases := []struct{ in, want int }{
		{0, 0}, {255, 65535}, {128, 32896}, {-5, 0}, {300, 65535},
	}
	for _, c := range cases {
		if got := scaleColor8to16(c.in); got != c.want {
			t.Errorf("scaleColor8to16(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

// TestCreateTabTemplateEscapesInjection proves the two quoting layers defeat an
// injection attempt: a title/command containing quotes and newlines renders as a
// well-formed AppleScript literal with no un-escaped quote breaking out and no raw
// newline inside a "write text" string.
func TestCreateTabTemplateEscapesInjection(t *testing.T) {
	// A hostile argv element: a double-quote that would otherwise close the literal,
	// plus a newline.
	argv := []string{"claude", "--model", "eviltab\" & (do shell script \"touch /tmp/pwned\")\nx"}
	launch := escapeAppleScript(shellJoin(argv))
	title := escapeAppleScript("Nova · dev@\"proj\"")

	script, err := renderTemplate(createTabTmpl, createTabParams{TabTitle: title, LaunchCommand: launch})
	if err != nil {
		t.Fatalf("renderTemplate: %v", err)
	}
	// No raw newline may survive inside the rendered dynamic values (the template's
	// own line breaks are fine; the injected value's newline must be a concatenation).
	if strings.Contains(script, "\np0evil") { // a paranoid marker; not expected
		t.Fatalf("unexpected raw content: %s", script)
	}
	if !strings.Contains(script, `" & return & "`) {
		t.Fatalf("injected newline not converted to a return concatenation:\n%s", script)
	}
	// The hostile double-quote must appear escaped (\") in the launch command, not
	// as a bare quote that closes the write-text literal.
	if strings.Contains(script, `write text "claude --model 'eviltab" `) {
		t.Fatalf("hostile quote broke out of the literal:\n%s", script)
	}
	if !strings.Contains(script, `\"`) {
		t.Fatalf("expected escaped quotes in rendered script:\n%s", script)
	}
}

// TestWriteTextTemplateRenders is a smoke test that the write-text template
// substitutes the escaped session id and prompt.
func TestWriteTextTemplateRenders(t *testing.T) {
	script, err := renderTemplate(writeTextTmpl, writeTextParams{
		SessionID: escapeAppleScript("w0t0p0:ABC"),
		Prompt:    escapeAppleScript(`hello "there"`),
	})
	if err != nil {
		t.Fatalf("renderTemplate: %v", err)
	}
	if !strings.Contains(script, `session id "w0t0p0:ABC"`) {
		t.Fatalf("session id not rendered:\n%s", script)
	}
	if !strings.Contains(script, `write text "hello \"there\""`) {
		t.Fatalf("prompt not escaped/rendered:\n%s", script)
	}
}

// TestSetAppearanceTemplateColorGate checks the color block is emitted only when
// a project accent is set, and uses the 16-bit scaled values.
func TestSetAppearanceTemplateColorGate(t *testing.T) {
	withColor, err := renderTemplate(setAppearanceTmpl, appearanceParams{
		SessionID: "sid", TabTitle: "T", HasColor: true,
		R: scaleColor8to16(255), G: scaleColor8to16(0), B: scaleColor8to16(128),
	})
	if err != nil {
		t.Fatalf("renderTemplate: %v", err)
	}
	if !strings.Contains(withColor, "set background color to {65535, 0, 32896}") {
		t.Fatalf("scaled color block missing:\n%s", withColor)
	}
	noColor, err := renderTemplate(setAppearanceTmpl, appearanceParams{SessionID: "sid", TabTitle: "T"})
	if err != nil {
		t.Fatalf("renderTemplate: %v", err)
	}
	if strings.Contains(noColor, "background color") {
		t.Fatalf("color block emitted with no accent set:\n%s", noColor)
	}
}
