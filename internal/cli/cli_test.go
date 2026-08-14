package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FS-04.A21: a foreground dashboard persists structured logs and keeps the
// interactive terminal stream useful.
func TestDashboardLoggerForegroundPersistsAndMirrors(t *testing.T) {
	home := t.TempDir()
	var terminal bytes.Buffer
	log, closeLog, err := newDashboardLogger(home, false, &terminal)
	if err != nil {
		t.Fatal(err)
	}
	log.Info("foreground record", "component", "test")
	if err := closeLog(); err != nil {
		t.Fatal(err)
	}

	contents, err := os.ReadFile(filepath.Join(home, "dashboard.log"))
	if err != nil {
		t.Fatal(err)
	}
	for name, got := range map[string]string{"terminal": terminal.String(), "file": string(contents)} {
		if !strings.Contains(got, `"msg":"foreground record"`) || !strings.Contains(got, `"component":"test"`) {
			t.Fatalf("%s output = %q, want JSON log record", name, got)
		}
	}
	if strings.Count(string(contents), `"msg":"foreground record"`) != 1 {
		t.Fatalf("file output = %q, want one record", contents)
	}
}

// FS-04.A21: repeated starts append and repair an existing overly broad mode.
func TestDashboardLoggerAppendsAndTightensPermissions(t *testing.T) {
	home := t.TempDir()
	logPath := filepath.Join(home, "dashboard.log")
	if err := os.WriteFile(logPath, []byte("prior record\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(logPath, 0o644); err != nil {
		t.Fatal(err)
	}

	log, closeLog, err := newDashboardLogger(home, false, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	log.Info("appended record")
	if err := closeLog(); err != nil {
		t.Fatal(err)
	}

	contents, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(contents), "prior record\n") || !strings.Contains(string(contents), `"msg":"appended record"`) {
		t.Fatalf("log contents = %q, want preserved and appended records", contents)
	}
	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("dashboard.log mode = %o, want 600", got)
	}
}

// FS-04.A21: the detached child uses its redirected stderr as the sole sink.
func TestDashboardLoggerDaemonUsesRedirectedStderr(t *testing.T) {
	home := t.TempDir()
	logPath := filepath.Join(home, "dashboard.log")
	redirected, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	log, closeLog, err := newDashboardLogger(home, true, redirected)
	if err != nil {
		t.Fatal(err)
	}
	log.Info("daemon record")
	if err := closeLog(); err != nil {
		t.Fatal(err)
	}
	if err := redirected.Close(); err != nil {
		t.Fatal(err)
	}

	contents, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(contents), `"msg":"daemon record"`); got != 1 {
		t.Fatalf("daemon record count = %d in %q, want 1", got, contents)
	}
}

// FS-04.A21: startup fails instead of silently losing its promised diagnostics.
func TestDashboardLoggerRejectsUnavailableLog(t *testing.T) {
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(home, "dashboard.log"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := newDashboardLogger(home, false, &bytes.Buffer{}); err == nil {
		t.Fatal("newDashboardLogger succeeded with a directory at dashboard.log")
	}
}

// Regression (review fix): reindex refuses to run against a live server. It gates
// on serverRunning, which must report true only for an actually-alive pid (a stale
// pidfile — a dead pid — must not block reindex).
func TestServerRunningDetectsLiveProcess(t *testing.T) {
	home := t.TempDir()
	if serverRunning(home) {
		t.Fatal("serverRunning with no pidfile = true")
	}
	if err := writePidfile(home, pidInfo{PID: os.Getpid(), Port: 4317}); err != nil {
		t.Fatal(err)
	}
	if !serverRunning(home) {
		t.Fatal("serverRunning with a live pid = false")
	}
	if err := writePidfile(home, pidInfo{PID: 2_000_000_000, Port: 4317}); err != nil {
		t.Fatal(err)
	}
	if serverRunning(home) {
		t.Fatal("serverRunning with a dead pid = true (a stale pidfile must not block reindex)")
	}
}

// Regression (review fix, BLOCKING): `dashboard start --detach` must not spawn a
// child (and clobber the pidfile) when a live server already holds it. Before the
// fix the detach parent bypassed the already-running guard: it overwrote the live
// pidfile with a doomed child's PID, and the child's failure cleanup then deleted
// the pidfile entirely, breaking stop/open/reindex against the still-live server.
func TestStartDetachedRefusesWhenAlreadyRunning(t *testing.T) {
	home := t.TempDir()
	// A live instance holds the pidfile (this test process's own PID is alive).
	if err := writePidfile(home, pidInfo{PID: os.Getpid(), Port: 4317}); err != nil {
		t.Fatalf("writePidfile: %v", err)
	}
	if err := startDetached(home, 4317); err != nil {
		t.Fatalf("startDetached returned error: %v", err)
	}
	info, ok, err := readPidfile(home)
	if err != nil || !ok {
		t.Fatalf("pidfile missing after refusal: ok=%v err=%v", ok, err)
	}
	if info.PID != os.Getpid() {
		t.Fatalf("pidfile PID = %d, want the live server's %d (child clobbered it)", info.PID, os.Getpid())
	}
}

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
	la, err := parseLaunch([]string{"implementer@my-app", "--backend", "claude", "--model", "sonnet-4-6", "--effort", "high", "--name", "Atlas", "--group", "auth"})
	if err != nil {
		t.Fatalf("parseLaunch: %v", err)
	}
	if la.Role != "implementer" || la.Project != "my-app" {
		t.Fatalf("role/project = %q/%q", la.Role, la.Project)
	}
	if la.Backend != "claude" || la.Model != "sonnet-4-6" || la.Effort != "high" || la.Name != "Atlas" || la.Group != "auth" {
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

func TestParseLaunchNewAndResumeFlags(t *testing.T) {
	// --new sets ForceNew, no ResumeID.
	la, err := parseLaunch([]string{"impl@my-app", "--new"})
	if err != nil {
		t.Fatalf("parseLaunch --new: %v", err)
	}
	if !la.ForceNew {
		t.Error("--new: ForceNew should be true")
	}
	if la.ResumeID != "" {
		t.Errorf("--new: ResumeID should be empty, got %q", la.ResumeID)
	}

	// --resume <id> sets ResumeID, ForceNew stays false.
	la2, err := parseLaunch([]string{"impl@my-app", "--resume", "a_abc123"})
	if err != nil {
		t.Fatalf("parseLaunch --resume: %v", err)
	}
	if la2.ResumeID != "a_abc123" {
		t.Errorf("--resume: ResumeID = %q, want a_abc123", la2.ResumeID)
	}
	if la2.ForceNew {
		t.Error("--resume: ForceNew should be false")
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

	// A value-taking flag with no operand (last, or followed by another flag) must
	// fail fast rather than silently take "" — `--resume` with no id previously
	// fell through to a fresh launch.
	for _, bad := range [][]string{
		{"impl@my-app", "--resume"},
		{"impl@my-app", "--model"},
		{"impl@my-app", "--backend", "--name", "Atlas"},
	} {
		if _, err := parseLaunch(bad); err == nil {
			t.Errorf("parseLaunch(%v) expected error for missing operand", bad)
		}
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
