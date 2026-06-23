package store

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"
	"time"
)

// newTestStore returns a Store rooted at a fresh temp dir and ensures the layout
// exists. AGENTDECK_HOME is set to the temp dir so any New()-based code path in
// the same test stays isolated and never touches the real ~/.agentdeck.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(envHome, dir)
	s, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := s.Home(); got != dir {
		t.Fatalf("Home() = %q, want %q", got, dir)
	}
	if err := s.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout: %v", err)
	}
	return s
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse time %q: %v", s, err)
	}
	return tm
}

// ---- Round-trip every object ----

func TestRoundTrip(t *testing.T) {
	s := newTestStore(t)
	busy := mustTime(t, "2026-06-22T10:00:05Z")

	agent := Agent{
		AgentID: "a_8f3c12", Name: "Atlas", Role: "implementer", Project: "my-app",
		Backend: "claude", Model: "sonnet-4-6", Interface: "chat",
		CreatedAt: mustTime(t, "2026-06-22T10:00:00Z"), Group: "auth-migration",
	}
	if err := s.WriteAgent(agent); err != nil {
		t.Fatalf("WriteAgent: %v", err)
	}
	if got, err := s.ReadAgent(agent.AgentID); err != nil || !reflect.DeepEqual(got, agent) {
		t.Fatalf("Agent round-trip: got %+v err %v", got, err)
	}

	run := RunningEntry{
		AgentID: "a_8f3c12", PID: 48213, SessionID: "claude-sess-xyz",
		Interface: "chat", StartedAt: mustTime(t, "2026-06-22T10:00:01Z"),
	}
	if err := s.WriteRunning(run); err != nil {
		t.Fatalf("WriteRunning: %v", err)
	}
	if got, err := s.ReadRunning(run.AgentID); err != nil || !reflect.DeepEqual(got, run) {
		t.Fatalf("RunningEntry round-trip: got %+v err %v", got, err)
	}

	st := Status{
		AgentID: "a_8f3c12", State: "busy", Detail: "Editing src/auth.ts",
		LastTrace: "tool: edit", BusySince: &busy, ContextPct: 0.42,
	}
	if err := s.WriteStatus(st); err != nil {
		t.Fatalf("WriteStatus: %v", err)
	}
	if got, err := s.ReadStatus(st.AgentID); err != nil || !reflect.DeepEqual(got, st) {
		t.Fatalf("Status round-trip: got %+v err %v", got, err)
	}

	role := Role{Title: "Reviewer", SystemPrompt: "Review.", SkipPermissions: boolPtr(true)}
	if err := s.WriteRole("reviewer", role); err != nil {
		t.Fatalf("WriteRole: %v", err)
	}
	if got, err := s.ReadRole("reviewer"); err != nil || !reflect.DeepEqual(got, role) {
		t.Fatalf("Role round-trip: got %+v err %v", got, err)
	}

	proj := Project{
		Title: "My App", Color: [3]int{100, 180, 255}, Cwd: "~/Projects/my-app",
		AddDirs: []string{"~/shared"}, ContextPrompt: "ctx",
	}
	if err := s.WriteProject("my-app", proj); err != nil {
		t.Fatalf("WriteProject: %v", err)
	}
	if got, err := s.ReadProject("my-app"); err != nil || !reflect.DeepEqual(got, proj) {
		t.Fatalf("Project round-trip: got %+v err %v", got, err)
	}

	backends := DefaultBackends()
	if err := s.WriteBackends(backends); err != nil {
		t.Fatalf("WriteBackends: %v", err)
	}
	if got, err := s.ReadBackends(); err != nil || !reflect.DeepEqual(got, backends) {
		t.Fatalf("BackendsConfig round-trip: got %+v err %v", got, err)
	}

	layout := Layout{Order: []string{"a_8f3c12"}, Density: Density{CardsPerRow: 4, Gap: 20}}
	if err := s.WriteLayout(layout); err != nil {
		t.Fatalf("WriteLayout: %v", err)
	}
	if got, err := s.ReadLayout(); err != nil || !reflect.DeepEqual(got, layout) {
		t.Fatalf("Layout round-trip: got %+v err %v", got, err)
	}

	cfg := DefaultConfig()
	if err := s.WriteConfig(cfg); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}
	if got, err := s.ReadConfig(); err != nil || !reflect.DeepEqual(got, cfg) {
		t.Fatalf("Config round-trip: got %+v err %v", got, err)
	}
}

// ---- Not found ----

func TestReadNotFound(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.ReadAgent("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ReadAgent missing: err = %v, want ErrNotFound", err)
	}
	if _, err := s.ReadConfig(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ReadConfig missing: err = %v, want ErrNotFound", err)
	}
}

// ---- Corrupt-file survival ----

func TestCorruptFileSurvival(t *testing.T) {
	s := newTestStore(t)

	// Seed a couple of good roles plus one corrupt one (from testdata).
	if err := s.WriteRole("good1", Role{Title: "Good 1"}); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteRole("good2", Role{Title: "Good 2"}); err != nil {
		t.Fatal(err)
	}
	corrupt, err := os.ReadFile(filepath.Join("testdata", "corrupt_role.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.rolePath("bad"), corrupt, 0o644); err != nil {
		t.Fatal(err)
	}

	// ReadRole on the corrupt one → ErrCorrupt (not panic, not NotFound).
	if _, err := s.ReadRole("bad"); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("ReadRole corrupt: err = %v, want ErrCorrupt", err)
	}

	// ListRoles skips the corrupt one and returns the two good ones.
	roles, err := s.ListRoles()
	if err != nil {
		t.Fatalf("ListRoles: %v", err)
	}
	if len(roles) != 2 {
		t.Fatalf("ListRoles len = %d, want 2 (corrupt skipped): %v", len(roles), roles)
	}
	if _, ok := roles["bad"]; ok {
		t.Fatal("ListRoles included corrupt role")
	}

	// Single-file corrupt backends → ErrCorrupt from the reader.
	cb, err := os.ReadFile(filepath.Join("testdata", "corrupt_backends.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.backendsPath(), cb, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadBackends(); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("ReadBackends corrupt: err = %v, want ErrCorrupt", err)
	}
}

// ---- AGENTDECK_HOME isolation + unset resolution ----

func TestHomeResolution(t *testing.T) {
	// Override path is honored exactly.
	dir := t.TempDir()
	t.Setenv(envHome, dir)
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if s.Home() != dir {
		t.Fatalf("Home() = %q, want %q", s.Home(), dir)
	}

	// Unset → resolves under the user's home dir (assert path only; no writes).
	t.Setenv(envHome, "")
	s2, err := New()
	if err != nil {
		t.Fatal(err)
	}
	uh, _ := os.UserHomeDir()
	want := filepath.Join(uh, ".agentdeck")
	if s2.Home() != want {
		t.Fatalf("default Home() = %q, want %q", s2.Home(), want)
	}
}

// ---- Tilde expansion ----

func TestExpandTilde(t *testing.T) {
	uh, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ in, want string }{
		{"~/x", filepath.Join(uh, "x")},
		{"~", uh},
		{"/abs/path", "/abs/path"},
		{"relative/path", "relative/path"},
		{"~user/x", "~user/x"}, // ~user form not supported, returned unchanged
	}
	for _, c := range cases {
		got, err := ExpandTilde(c.in)
		if err != nil {
			t.Fatalf("ExpandTilde(%q): %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("ExpandTilde(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ---- NewAgentID format + uniqueness ----

func TestNewAgentID(t *testing.T) {
	s := newTestStore(t)
	re := regexp.MustCompile(`^a_[0-9a-f]{6}$`)
	seen := make(map[string]bool, 1000)
	for i := 0; i < 1000; i++ {
		id, err := s.NewAgentID()
		if err != nil {
			t.Fatalf("NewAgentID: %v", err)
		}
		if !re.MatchString(id) {
			t.Fatalf("NewAgentID format = %q, want ^a_[0-9a-f]{6}$", id)
		}
		if seen[id] {
			t.Fatalf("NewAgentID returned duplicate %q", id)
		}
		seen[id] = true
	}
}

func TestNewAgentIDCollisionRetry(t *testing.T) {
	s := newTestStore(t)
	// Pre-create an agent, then confirm NewAgentID never returns that id.
	existing, err := s.NewAgentID()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.WriteAgent(Agent{AgentID: existing}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 50; i++ {
		id, err := s.NewAgentID()
		if err != nil {
			t.Fatal(err)
		}
		if id == existing {
			t.Fatalf("NewAgentID returned a colliding id %q", id)
		}
	}
}

// ---- Atomic write leaves no .tmp file ----

func TestAtomicWriteNoTemp(t *testing.T) {
	s := newTestStore(t)
	if err := s.WriteAgent(Agent{AgentID: "a_abc123", Name: "x"}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(s.dirPath(dirAgents))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if len(e.Name()) >= 5 && e.Name()[:5] == ".tmp-" {
			t.Fatalf("leftover temp file after atomic write: %s", e.Name())
		}
	}
	// And the real file parses back.
	if _, err := s.ReadAgent("a_abc123"); err != nil {
		t.Fatalf("ReadAgent after atomic write: %v", err)
	}
}

// ---- EnsureLayout idempotency + file-as-home failure ----

func TestEnsureLayoutIdempotent(t *testing.T) {
	s := newTestStore(t)
	if err := s.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout second call: %v", err)
	}
	for _, d := range dataDirs {
		fi, err := os.Stat(s.dirPath(d))
		if err != nil || !fi.IsDir() {
			t.Fatalf("dir %q missing after EnsureLayout: %v", d, err)
		}
	}
}

func TestEnsureLayoutHomeIsFile(t *testing.T) {
	dir := t.TempDir()
	homeFile := filepath.Join(dir, "home-as-file")
	if err := os.WriteFile(homeFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewWithHome(homeFile)
	if err := s.EnsureLayout(); err == nil {
		t.Fatal("EnsureLayout on a file home: want error, got nil")
	}
}

// ---- Seed idempotency / no-clobber ----

func TestSeedIfAbsentNoClobber(t *testing.T) {
	s := newTestStore(t)
	if err := s.SeedIfAbsent(); err != nil {
		t.Fatalf("SeedIfAbsent: %v", err)
	}

	// All 4 roles + project + single files present.
	roles, _ := s.ListRoles()
	if len(roles) != 4 {
		t.Fatalf("seeded roles = %d, want 4", len(roles))
	}
	if _, err := s.ReadProject("my-app"); err != nil {
		t.Fatalf("seeded project: %v", err)
	}
	if _, err := s.ReadConfig(); err != nil {
		t.Fatalf("seeded config: %v", err)
	}

	// Mutate reviewer, re-seed, assert preserved (no clobber).
	mutated := Role{Title: "MUTATED", SystemPrompt: "changed", SkipPermissions: boolPtr(false)}
	if err := s.WriteRole("reviewer", mutated); err != nil {
		t.Fatal(err)
	}
	if err := s.SeedIfAbsent(); err != nil {
		t.Fatalf("re-seed: %v", err)
	}
	got, err := s.ReadRole("reviewer")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, mutated) {
		t.Fatalf("reviewer clobbered by re-seed: got %+v, want %+v", got, mutated)
	}
}

// ---- List on empty/missing dir returns empty, not error ----

func TestListEmpty(t *testing.T) {
	s := newTestStore(t)
	agents, err := s.ListAgents()
	if err != nil {
		t.Fatalf("ListAgents empty: %v", err)
	}
	if len(agents) != 0 {
		t.Fatalf("ListAgents empty len = %d, want 0", len(agents))
	}
	running, err := s.ListRunning()
	if err != nil || len(running) != 0 {
		t.Fatalf("ListRunning empty: len %d err %v", len(running), err)
	}
}
