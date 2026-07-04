package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

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

func TestRoundTripConfigObjects(t *testing.T) {
	s := newTestStore(t)

	role := Role{Title: "Reviewer", SystemPrompt: "Review.", SkipPermissions: boolPtr(true)}
	if err := s.WriteRole("reviewer", role); err != nil {
		t.Fatalf("WriteRole: %v", err)
	}
	if got, err := s.ReadRole("reviewer"); err != nil || !reflect.DeepEqual(got, role) {
		t.Fatalf("Role round-trip: got %+v err %v", got, err)
	}

	project := Project{
		Title:         "My App",
		Color:         [3]int{100, 180, 255},
		Cwd:           "~/Projects/my-app",
		AddDirs:       []string{"~/shared"},
		ContextPrompt: "ctx",
	}
	if err := s.WriteProject("my-app", project); err != nil {
		t.Fatalf("WriteProject: %v", err)
	}
	if got, err := s.ReadProject("my-app"); err != nil || !reflect.DeepEqual(got, project) {
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

func TestDeleteRoleAndProjectTolerateMissing(t *testing.T) {
	s := newTestStore(t)

	if err := s.WriteRole("reviewer", Role{Title: "Reviewer"}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteRole("reviewer"); err != nil {
		t.Fatalf("DeleteRole existing: %v", err)
	}
	if _, err := s.ReadRole("reviewer"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ReadRole deleted: err = %v, want ErrNotFound", err)
	}
	if err := s.DeleteRole("reviewer"); err != nil {
		t.Fatalf("DeleteRole missing: %v", err)
	}

	if err := s.WriteProject("my-app", Project{Title: "My App"}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteProject("my-app"); err != nil {
		t.Fatalf("DeleteProject existing: %v", err)
	}
	if _, err := s.ReadProject("my-app"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ReadProject deleted: err = %v, want ErrNotFound", err)
	}
	if err := s.DeleteProject("my-app"); err != nil {
		t.Fatalf("DeleteProject missing: %v", err)
	}
}

func TestReadNotFound(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.ReadRole("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ReadRole missing: err = %v, want ErrNotFound", err)
	}
	if _, err := s.ReadProject("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ReadProject missing: err = %v, want ErrNotFound", err)
	}
	if _, err := s.ReadConfig(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ReadConfig missing: err = %v, want ErrNotFound", err)
	}
}

func TestCorruptFileSurvival(t *testing.T) {
	s := newTestStore(t)

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

	if _, err := s.ReadRole("bad"); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("ReadRole corrupt: err = %v, want ErrCorrupt", err)
	}
	roles, err := s.ListRoles()
	if err != nil {
		t.Fatalf("ListRoles: %v", err)
	}
	if len(roles) != 2 {
		t.Fatalf("ListRoles len = %d, want 2: %v", len(roles), roles)
	}
	if _, ok := roles["bad"]; ok {
		t.Fatal("ListRoles included corrupt role")
	}

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

func TestListProjectsSkipsCorruptFiles(t *testing.T) {
	s := newTestStore(t)

	if err := s.WriteProject("good", Project{Title: "Good"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.projectPath("bad"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}

	projects, err := s.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("ListProjects len = %d, want 1: %v", len(projects), projects)
	}
	if _, ok := projects["bad"]; ok {
		t.Fatal("ListProjects included corrupt project")
	}
}

func TestHomeResolution(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(envHome, dir)
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if s.Home() != dir {
		t.Fatalf("Home() = %q, want %q", s.Home(), dir)
	}

	t.Setenv(envHome, "")
	s2, err := New()
	if err != nil {
		t.Fatal(err)
	}
	uh, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(uh, ".agentdeck")
	if s2.Home() != want {
		t.Fatalf("default Home() = %q, want %q", s2.Home(), want)
	}
}

func TestHomeResolutionExpandsTildeOverride(t *testing.T) {
	uh, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(envHome, "~/agentdeck-test-home")
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(uh, "agentdeck-test-home")
	if s.Home() != want {
		t.Fatalf("tilde Home() = %q, want %q", s.Home(), want)
	}
}

func TestHomeResolutionMakesRelativeOverrideAbsolute(t *testing.T) {
	t.Setenv(envHome, "relative-agentdeck-home")
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(s.Home()) {
		t.Fatalf("Home() = %q, want absolute path", s.Home())
	}
	if got := filepath.Base(s.Home()); got != "relative-agentdeck-home" {
		t.Fatalf("Home() base = %q, want relative-agentdeck-home", got)
	}
}

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
		{"~user/x", "~user/x"},
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

func TestAtomicWriteNoTempAndValidJSON(t *testing.T) {
	s := newTestStore(t)
	if err := s.WriteRole("reviewer", Role{Title: "Reviewer"}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(s.dirPath(dirRoles))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if len(e.Name()) >= 5 && e.Name()[:5] == ".tmp-" {
			t.Fatalf("leftover temp file after atomic write: %s", e.Name())
		}
	}
	data, err := os.ReadFile(s.rolePath("reviewer"))
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(data) {
		t.Fatalf("role file is invalid JSON: %s", data)
	}
	if _, err := s.ReadRole("reviewer"); err != nil {
		t.Fatalf("ReadRole after atomic write: %v", err)
	}
}

func TestEnsureLayoutCreatesOnlyConfigAndTranscriptDirs(t *testing.T) {
	s := newTestStore(t)
	if err := s.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout second call: %v", err)
	}
	for _, d := range []string{dirRoles, dirProjects, dirSessions} {
		fi, err := os.Stat(s.dirPath(d))
		if err != nil || !fi.IsDir() {
			t.Fatalf("dir %q missing after EnsureLayout: %v", d, err)
		}
	}
	for _, d := range []string{"agents", "running", "status", "messages"} {
		if _, err := os.Stat(s.dirPath(d)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("unexpected state dir %q exists or stat failed: %v", d, err)
		}
	}
	if _, err := os.Stat(s.filePath("state.db")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected state.db exists or stat failed: %v", err)
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

func TestSeedIfAbsentNoClobber(t *testing.T) {
	s := newTestStore(t)
	if err := s.SeedIfAbsent(); err != nil {
		t.Fatalf("SeedIfAbsent: %v", err)
	}

	roles, err := s.ListRoles()
	if err != nil {
		t.Fatalf("ListRoles: %v", err)
	}
	if len(roles) != 6 {
		t.Fatalf("seeded roles = %d, want 6", len(roles))
	}
	if _, err := s.ReadProject("my-app"); err != nil {
		t.Fatalf("seeded project: %v", err)
	}
	if _, err := s.ReadConfig(); err != nil {
		t.Fatalf("seeded config: %v", err)
	}
	if _, err := s.ReadBackends(); err != nil {
		t.Fatalf("seeded backends: %v", err)
	}
	if _, err := s.ReadLayout(); err != nil {
		t.Fatalf("seeded layout: %v", err)
	}

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

func TestListEmpty(t *testing.T) {
	s := newTestStore(t)
	roles, err := s.ListRoles()
	if err != nil {
		t.Fatalf("ListRoles empty: %v", err)
	}
	if len(roles) != 0 {
		t.Fatalf("ListRoles empty len = %d, want 0", len(roles))
	}
	projects, err := s.ListProjects()
	if err != nil || len(projects) != 0 {
		t.Fatalf("ListProjects empty: len %d err %v", len(projects), err)
	}
}
