package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionFlag(t *testing.T) {
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("--version: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "agentdeck version") || strings.TrimSpace(s) == "agentdeck version" {
		t.Fatalf("--version output = %q, want non-empty version line", s)
	}
}

func TestIsLaunchArg(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"implementer@my-app", true},
		{"dashboard", false},
		{"--version", false},
		{"-x", false},
		{"plainword", false},
	}
	for _, c := range cases {
		if got := isLaunchArg(c.in); got != c.want {
			t.Errorf("isLaunchArg(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseLaunch(t *testing.T) {
	la, err := parseLaunch([]string{"implementer@my-app", "--backend", "claude", "--model", "sonnet-4-6", "--name", "Atlas", "--group", "auth"})
	if err != nil {
		t.Fatalf("parseLaunch: %v", err)
	}
	if la.Role != "implementer" || la.Project != "my-app" {
		t.Fatalf("role/project = %q/%q", la.Role, la.Project)
	}
	if la.Backend != "claude" || la.Model != "sonnet-4-6" || la.Name != "Atlas" || la.Group != "auth" {
		t.Fatalf("flags parsed wrong: %+v", la)
	}
	if la.Interface != "chat" {
		t.Fatalf("default interface = %q, want chat", la.Interface)
	}
	// Parity: the CLI body carries exactly the fields the REST launch endpoint
	// reads, so CLI and modal produce an identical agent (techspec §6.5).
	b := la.body()
	if b.Role != "implementer" || b.Project != "my-app" || b.Backend != "claude" || b.Group != "auth" {
		t.Fatalf("launch body mismatch: %+v", b)
	}
}

func TestParseLaunchErrors(t *testing.T) {
	for _, bad := range []string{"@my-app", "implementer@", "noatsign", ""} {
		if _, err := parseLaunch([]string{bad}); err == nil {
			t.Errorf("parseLaunch(%q) expected error", bad)
		}
	}
	// Last-@ split: a role with no @ and a project keeps the form unambiguous.
	la, err := parseLaunch([]string{"impl@my-app"})
	if err != nil || la.Role != "impl" || la.Project != "my-app" {
		t.Fatalf("parseLaunch impl@my-app = %+v err=%v", la, err)
	}
}

func TestPidfileRoundTrip(t *testing.T) {
	home := t.TempDir()
	if _, ok, err := readPidfile(home); ok || err != nil {
		t.Fatalf("readPidfile missing: ok=%v err=%v", ok, err)
	}
	if err := writePidfile(home, pidInfo{PID: 4242, Port: 4317}); err != nil {
		t.Fatal(err)
	}
	info, ok, err := readPidfile(home)
	if err != nil || !ok {
		t.Fatalf("readPidfile after write: ok=%v err=%v", ok, err)
	}
	if info.PID != 4242 || info.Port != 4317 {
		t.Fatalf("pidfile round-trip = %+v", info)
	}
	if err := removePidfile(home); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := readPidfile(home); ok {
		t.Fatal("pidfile still present after remove")
	}
}

func TestProcessAlive(t *testing.T) {
	// PID 1 always exists; a huge PID should not.
	if !processAlive(1) {
		t.Error("processAlive(1) = false, want true")
	}
	if processAlive(0) {
		t.Error("processAlive(0) = true, want false")
	}
	if processAlive(2147480000) {
		t.Error("processAlive(huge) = true, want false")
	}
}
