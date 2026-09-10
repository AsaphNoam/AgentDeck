package config

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
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

func TestReadBackendsRejectsIncompleteDocument(t *testing.T) {
	s := newTestStore(t)
	for name, body := range map[string]string{
		"missing backends": `{"version":2}`,
		"nil models":       `{"version":2,"backends":{"claude":{"type":"claude-acp","models":null}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(s.backendsPath(), []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := s.ReadBackends(); !errors.Is(err, ErrCorrupt) {
				t.Fatalf("ReadBackends error = %v, want ErrCorrupt", err)
			} else if !strings.Contains(err.Error(), "backends.json") {
				t.Fatalf("ReadBackends error = %q, want offending file name", err)
			}
		})
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

// A fresh home gets the current provider aliases as defaults; an existing
// catalog is not touched at all, so nobody's pinned model or edited entry is
// replaced by a newer AgentDeck (FS-09.R33/A12).
func TestSeededBackendDefaultsAreCurrentAndNeverRewritten(t *testing.T) {
	s := newTestStore(t)
	if err := s.SeedIfAbsent(); err != nil {
		t.Fatalf("SeedIfAbsent: %v", err)
	}
	fresh, err := s.ReadBackends()
	if err != nil {
		t.Fatalf("ReadBackends: %v", err)
	}
	if got := fresh.Backends["claude"].DefaultModel; got != "sonnet" {
		t.Errorf("fresh Claude default = %q, want sonnet", got)
	}
	if got := fresh.Backends["codex"].DefaultModel; got != "gpt-5.6-sol" {
		t.Errorf("fresh Codex default = %q, want gpt-5.6-sol", got)
	}
	for id, want := range map[string]string{"claude": "sonnet", "codex": "gpt-5.6-sol"} {
		if _, ok := fresh.Backends[id].Models[want]; !ok {
			t.Errorf("%s default model %q has no catalog entry", id, want)
		}
	}

	// A person pins an exact generation, then a later AgentDeck re-seeds.
	bk := fresh.Backends["claude"]
	bk.DefaultModel = "sonnet-4-6"
	bk.Models = map[string]Model{"sonnet-4-6": {Name: "Sonnet 4.6", Model: "claude-sonnet-4-6"}}
	fresh.Backends["claude"] = bk
	if err := s.WriteBackends(fresh); err != nil {
		t.Fatalf("WriteBackends: %v", err)
	}
	before, err := os.ReadFile(s.backendsPath())
	if err != nil {
		t.Fatalf("read backends: %v", err)
	}
	if err := s.SeedIfAbsent(); err != nil {
		t.Fatalf("re-seed: %v", err)
	}
	after, err := os.ReadFile(s.backendsPath())
	if err != nil {
		t.Fatalf("re-read backends: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("re-seeding rewrote an existing catalog:\nbefore=%s\nafter=%s", before, after)
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
	agentdecker, err := s.ReadRole("agentdecker")
	if err != nil || agentdecker.SystemPrompt != agentDeckerPrompt || strings.Contains(agentdecker.SystemPrompt, "propose_pipeline") {
		t.Fatalf("seeded AgentDecker prompt is not the thin role: role=%+v err=%v", agentdecker, err)
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

// supersededPromptFixture returns the previously shipped prompt bytes for a
// role. The fixtures are the independent oracle for the digest table: a digest
// is only trusted because these bytes hash to it (INV §17).
func supersededPromptFixture(t *testing.T, id string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "superseded_"+id+"_prompt.txt"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSuffix(string(data), "\n")
}

func supersededFixtureIDs() []string {
	return []string{"agentdecker", "teammate", "implementer", "reviewer", "researcher"}
}

// FS-18.A9, FS-04.A27: every shipped digest is re-derived from the fixture
// bytes rather than trusted, the table names only seeded roles, and no entry
// matches its role's current prompt — an entry that did would silently migrate
// nothing (INV §10, INV §17).
func TestSupersededDigestTableMatchesSeededRoles(t *testing.T) {
	seeded := seedRoles()
	for id, digests := range supersededRolePromptDigests {
		role, ok := seeded[id]
		if !ok {
			t.Fatalf("digest table names role %q, which seedRoles() does not seed", id)
		}
		sum := sha256.Sum256([]byte(role.SystemPrompt))
		currentDigest := hex.EncodeToString(sum[:])
		if len(digests) == 0 {
			t.Fatalf("role %q has an empty digest list", id)
		}
		for _, digest := range digests {
			if digest == currentDigest {
				t.Fatalf("role %q lists its current prompt as superseded; the entry can never migrate anything", id)
			}
		}
	}
	for _, id := range supersededFixtureIDs() {
		sum := sha256.Sum256([]byte(supersededPromptFixture(t, id)))
		got := hex.EncodeToString(sum[:])
		if !slices.Contains(supersededRolePromptDigests[id], got) {
			t.Fatalf("fixture digest for %q = %s, not in the shipped table %v", id, got, supersededRolePromptDigests[id])
		}
	}
}

// FS-18.A9: the corrected prompts no longer tell an agent to look for work on
// its own. The banned text is enumerated from the requirement rather than
// copied from the constants under test (INV §17).
func TestSeededPromptsDoNotInstructPolling(t *testing.T) {
	banned := []string{
		"check_messages",
		"get_assigned_task",
		"Start each turn by checking",
		"woken with no new instruction",
	}
	for id, role := range seedRoles() {
		for _, phrase := range banned {
			if strings.Contains(role.SystemPrompt, phrase) {
				t.Errorf("seeded role %q prompt contains %q; AgentDeck's activation names the tool a host-owned turn needs", id, phrase)
			}
		}
	}
	if teammate := seedRoles()["teammate"].SystemPrompt; !strings.Contains(teammate, "task queue") {
		t.Error("teammate lost its assignment-queue stance")
	}
}

// FS-18.A9, FS-04.A27: only an exact previously shipped prompt migrates, for
// every seeded role, and every non-prompt field survives the atomic role write.
// A digest belonging to another role never matches.
func TestMigrateSupersededRolePromptsExactOnly(t *testing.T) {
	seeded := seedRoles()
	for _, id := range supersededFixtureIDs() {
		superseded := supersededPromptFixture(t, id)
		for _, tc := range []struct {
			name    string
			prompt  string
			migrate bool
		}{
			{name: "exact", prompt: superseded, migrate: true},
			{name: "one byte edit", prompt: superseded + "!"},
			{name: "empty", prompt: ""},
			{name: "custom", prompt: "my prompt"},
			{name: "another role's superseded prompt", prompt: supersededPromptFixture(t, otherFixtureID(id))},
		} {
			t.Run(id+"/"+tc.name, func(t *testing.T) {
				s := newTestStore(t)
				if err := s.EnsureLayout(); err != nil {
					t.Fatal(err)
				}
				original := Role{Title: "Custom title", SystemPrompt: tc.prompt, SkipPermissions: boolPtr(true)}
				if err := s.WriteRole(id, original); err != nil {
					t.Fatal(err)
				}
				migrated, err := s.MigrateSupersededRolePrompts()
				if err != nil {
					t.Fatal(err)
				}
				if want := boolToInt(tc.migrate); migrated != want {
					t.Fatalf("migrated = %d, want %d", migrated, want)
				}
				got, err := s.ReadRole(id)
				if err != nil {
					t.Fatal(err)
				}
				wantPrompt := tc.prompt
				if tc.migrate {
					wantPrompt = seeded[id].SystemPrompt
				}
				if got.Title != original.Title || got.SystemPrompt != wantPrompt || got.SkipPermissions == nil || !*got.SkipPermissions {
					t.Fatalf("role fields after migration = %+v", got)
				}
			})
		}
	}
}

func otherFixtureID(id string) string {
	if id == "agentdecker" {
		return "teammate"
	}
	return "agentdecker"
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// FS-18.A9: a role the migration cannot read or rewrite remains unchanged and
// does not stop the other roles from being corrected (INV §7, INV §8).
func TestMigrateSupersededRolePromptsIsolatesPerRoleFailure(t *testing.T) {
	seeded := seedRoles()

	// writeOthers seeds every other role with its superseded prompt so the pass
	// has real work to finish after the broken role fails.
	writeOthers := func(t *testing.T, s *Store, broken string) {
		t.Helper()
		for _, id := range supersededFixtureIDs() {
			if id == broken {
				continue
			}
			if err := s.WriteRole(id, Role{Title: "Custom title", SystemPrompt: supersededPromptFixture(t, id)}); err != nil {
				t.Fatal(err)
			}
		}
	}
	assertOthersMigrated := func(t *testing.T, s *Store, broken string) {
		t.Helper()
		for _, id := range supersededFixtureIDs() {
			if id == broken {
				continue
			}
			got, err := s.ReadRole(id)
			if err != nil {
				t.Fatal(err)
			}
			if got.SystemPrompt != seeded[id].SystemPrompt {
				t.Fatalf("role %q was not migrated after %q failed", id, broken)
			}
		}
	}

	t.Run("corrupt role", func(t *testing.T) {
		s := newTestStore(t)
		if err := s.EnsureLayout(); err != nil {
			t.Fatal(err)
		}
		writeOthers(t, s, "teammate")
		path := s.rolePath("teammate")
		before := []byte("{not json")
		if err := os.WriteFile(path, before, 0o600); err != nil {
			t.Fatal(err)
		}
		migrated, err := s.MigrateSupersededRolePrompts()
		if err == nil {
			t.Fatal("want a reported decode error")
		}
		if want := len(supersededFixtureIDs()) - 1; migrated != want {
			t.Fatalf("migrated = %d, want %d", migrated, want)
		}
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) {
			t.Fatalf("role bytes changed by a failed migration:\nbefore %s\nafter  %s", before, after)
		}
		assertOthersMigrated(t, s, "teammate")
	})

	t.Run("read failure", func(t *testing.T) {
		s := newTestStore(t)
		if err := s.EnsureLayout(); err != nil {
			t.Fatal(err)
		}
		writeOthers(t, s, "reviewer")
		path := s.rolePath("reviewer")
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		migrated, err := s.MigrateSupersededRolePrompts()
		if err == nil {
			t.Fatal("want a reported read error")
		}
		if want := len(supersededFixtureIDs()) - 1; migrated != want {
			t.Fatalf("migrated = %d, want %d", migrated, want)
		}
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			t.Fatalf("unreadable role path changed: info=%v err=%v", info, err)
		}
		assertOthersMigrated(t, s, "reviewer")
	})

	t.Run("write failure", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("needs an unprivileged user: root ignores directory permissions")
		}
		s := newTestStore(t)
		if err := s.EnsureLayout(); err != nil {
			t.Fatal(err)
		}
		if err := s.WriteRole("agentdecker", Role{Title: "Custom title", SystemPrompt: supersededPromptFixture(t, "agentdecker"), SkipPermissions: boolPtr(true)}); err != nil {
			t.Fatal(err)
		}
		dir := filepath.Dir(s.rolePath("agentdecker"))
		if err := os.Chmod(dir, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
		before, err := os.ReadFile(s.rolePath("agentdecker"))
		if err != nil {
			t.Fatal(err)
		}

		migrated, err := s.MigrateSupersededRolePrompts()
		if migrated != 0 || err == nil {
			t.Fatalf("migration = %d, %v; want 0 and a reported error", migrated, err)
		}
		after, err := os.ReadFile(s.rolePath("agentdecker"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) {
			t.Fatalf("role bytes changed by a failed migration:\nbefore %s\nafter  %s", before, after)
		}
	})
}

// FS-18.A9: the pass is idempotent and a home with no role files is a no-op.
func TestMigrateSupersededRolePromptsMissingRolesAndIdempotence(t *testing.T) {
	s := newTestStore(t)
	if err := s.EnsureLayout(); err != nil {
		t.Fatal(err)
	}
	if migrated, err := s.MigrateSupersededRolePrompts(); err != nil || migrated != 0 {
		t.Fatalf("missing role migration = %d, %v", migrated, err)
	}
	if err := s.WriteRole("researcher", Role{Title: "Custom title", SystemPrompt: supersededPromptFixture(t, "researcher")}); err != nil {
		t.Fatal(err)
	}
	if migrated, err := s.MigrateSupersededRolePrompts(); err != nil || migrated != 1 {
		t.Fatalf("first pass = %d, %v", migrated, err)
	}
	if migrated, err := s.MigrateSupersededRolePrompts(); err != nil || migrated != 0 {
		t.Fatalf("second pass = %d, %v; want an idempotent no-op", migrated, err)
	}
}

// FS-04.A7: the migration never widens absent-only seeding — a role AgentDeck
// does not seed is out of scope even if its prompt matches a shipped digest.
func TestMigrateSupersededRolePromptsSkipsUnseededRole(t *testing.T) {
	s := newTestStore(t)
	if err := s.EnsureLayout(); err != nil {
		t.Fatal(err)
	}
	custom := Role{Title: "Mine", SystemPrompt: supersededPromptFixture(t, "reviewer")}
	if err := s.WriteRole("my-reviewer", custom); err != nil {
		t.Fatal(err)
	}
	if migrated, err := s.MigrateSupersededRolePrompts(); err != nil || migrated != 0 {
		t.Fatalf("unseeded role migration = %d, %v", migrated, err)
	}
	got, err := s.ReadRole("my-reviewer")
	if err != nil {
		t.Fatal(err)
	}
	if got.SystemPrompt != custom.SystemPrompt {
		t.Fatal("an unseeded role was rewritten by the seed-prompt migration")
	}
}

// TS-11.R13: a digest naming a role AgentDeck does not seed is a table defect,
// reported rather than silently ignored.
func TestMigrateSupersededRolePromptsReportsUnseededTableEntry(t *testing.T) {
	s := newTestStore(t)
	if err := s.EnsureLayout(); err != nil {
		t.Fatal(err)
	}
	migrated, err := s.migrateSupersededRolePrompts(map[string][]string{"not-a-seeded-role": {"deadbeef"}})
	if migrated != 0 || err == nil {
		t.Fatalf("unseeded table entry = %d, %v; want 0 and a reported error", migrated, err)
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
