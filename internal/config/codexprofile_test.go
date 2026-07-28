package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeFile writes content to path (creating parents) for test fixtures.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// setupCodexHomes points CODEX_HOME at a fresh personal home and returns the
// personal dir plus the AgentDeck profile dir under a separate home.
func setupCodexHomes(t *testing.T) (personal, profile string) {
	t.Helper()
	personal = filepath.Join(t.TempDir(), "codex")
	if err := os.MkdirAll(personal, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_HOME", personal)
	return personal, CodexProfileDir(t.TempDir())
}

func TestRefreshCodexProfileMirrorsSetupNotHistory(t *testing.T) {
	personal, profile := setupCodexHomes(t)
	writeFile(t, filepath.Join(personal, "config.toml"), "model = \"gpt-5.6-sol\"\n")
	writeFile(t, filepath.Join(personal, "auth.json"), "{\"token\":\"secret\"}")
	writeFile(t, filepath.Join(personal, "prompts", "review.md"), "review")
	// Session/history state that must never be mirrored.
	writeFile(t, filepath.Join(personal, "sessions", "rollout.jsonl"), "{}")
	writeFile(t, filepath.Join(personal, "session_index.jsonl"), "{}")
	writeFile(t, filepath.Join(personal, "history.jsonl"), "{}")

	if err := RefreshCodexProfile(profile); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	for _, name := range []string{"config.toml", "auth.json", filepath.Join("prompts", "review.md")} {
		if _, err := os.Stat(filepath.Join(profile, name)); err != nil {
			t.Errorf("expected setup %q mirrored: %v", name, err)
		}
	}
	for _, name := range []string{"sessions", "session_index.jsonl", "history.jsonl"} {
		if _, err := os.Stat(filepath.Join(profile, name)); !os.IsNotExist(err) {
			t.Errorf("history %q must not be mirrored (err=%v)", name, err)
		}
	}

	// Owner-only modes: profile dir 0700, secret-bearing files 0600.
	if fi, _ := os.Stat(profile); fi.Mode().Perm() != 0o700 {
		t.Errorf("profile dir mode = %o, want 700", fi.Mode().Perm())
	}
	if fi, _ := os.Stat(filepath.Join(profile, "auth.json")); fi.Mode().Perm() != 0o600 {
		t.Errorf("auth.json mode = %o, want 600", fi.Mode().Perm())
	}

	m := readCodexManifest(filepath.Join(filepath.Dir(profile), dirCache, fileCodexProfileManifest))
	want := map[string]bool{"config.toml": true, "auth.json": true, "prompts": true}
	if len(m.Managed) != len(want) {
		t.Fatalf("managed = %v, want keys %v", m.Managed, want)
	}
	for _, name := range m.Managed {
		if !want[name] {
			t.Errorf("unexpected managed entry %q", name)
		}
	}
}

func TestRefreshCodexProfilePrunesRemovedSetupKeepsSessions(t *testing.T) {
	personal, profile := setupCodexHomes(t)
	writeFile(t, filepath.Join(personal, "config.toml"), "a = 1\n")
	writeFile(t, filepath.Join(personal, "plugins", "p.toml"), "x = 1\n")
	// Codex's own private session data already in the profile must survive.
	writeFile(t, filepath.Join(profile, "sessions", "own.jsonl"), "{}")

	if err := RefreshCodexProfile(profile); err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	if _, err := os.Stat(filepath.Join(profile, "plugins", "p.toml")); err != nil {
		t.Fatalf("plugins should be mirrored: %v", err)
	}

	// Remove the personal setup asset, then refresh again.
	if err := os.RemoveAll(filepath.Join(personal, "plugins")); err != nil {
		t.Fatal(err)
	}
	if err := RefreshCodexProfile(profile); err != nil {
		t.Fatalf("second refresh: %v", err)
	}
	if _, err := os.Stat(filepath.Join(profile, "plugins")); !os.IsNotExist(err) {
		t.Errorf("removed setup must be pruned from profile (err=%v)", err)
	}
	if _, err := os.Stat(filepath.Join(profile, "sessions", "own.jsonl")); err != nil {
		t.Errorf("profile's own session data must survive prune: %v", err)
	}
}

func TestRefreshCodexProfileReflectsInnerRemoval(t *testing.T) {
	personal, profile := setupCodexHomes(t)
	writeFile(t, filepath.Join(personal, "prompts", "a.md"), "a")
	writeFile(t, filepath.Join(personal, "prompts", "b.md"), "b")
	if err := RefreshCodexProfile(profile); err != nil {
		t.Fatalf("first refresh: %v", err)
	}

	if err := os.Remove(filepath.Join(personal, "prompts", "b.md")); err != nil {
		t.Fatal(err)
	}
	if err := RefreshCodexProfile(profile); err != nil {
		t.Fatalf("second refresh: %v", err)
	}
	if _, err := os.Stat(filepath.Join(profile, "prompts", "b.md")); !os.IsNotExist(err) {
		t.Errorf("inner removal must be reflected (err=%v)", err)
	}
	if _, err := os.Stat(filepath.Join(profile, "prompts", "a.md")); err != nil {
		t.Errorf("kept inner file must remain: %v", err)
	}
}

func TestRefreshCodexProfileRejectsEscapingSymlink(t *testing.T) {
	personal, profile := setupCodexHomes(t)
	writeFile(t, filepath.Join(personal, "config.toml"), "a = 1\n")
	outside := filepath.Join(t.TempDir(), "elsewhere")
	writeFile(t, outside, "secret outside the personal root")
	// The escape sits inside an allowlisted setup dir so it is actually selected.
	if err := os.MkdirAll(filepath.Join(personal, "skills"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(personal, "skills", "leak.md")); err != nil {
		t.Fatal(err)
	}
	if err := RefreshCodexProfile(profile); err == nil {
		t.Fatal("expected refresh to fail on a root-escaping symlink")
	}
	// The staged generation failed before publish, so nothing landed in the profile.
	if _, err := os.Stat(filepath.Join(profile, "config.toml")); !os.IsNotExist(err) {
		t.Errorf("failed generation must not publish partial setup (err=%v)", err)
	}
}

// FS-09.R44, TS-02.R19, TS-04.R21 (INV §12): only the setup allowlist is
// mirrored. Volatile session/history/runtime state — including databases and
// their WAL/SHM sidecars, logs, snapshots, and temp dirs — is never copied, and
// the child's own same-named private state survives a refresh untouched.
func TestRefreshCodexProfileExcludesRuntimeStateAndKeepsChildState(t *testing.T) {
	personal, profile := setupCodexHomes(t)
	writeFile(t, filepath.Join(personal, "config.toml"), "model = \"x\"\n")
	for _, name := range []string{
		"state_5.sqlite", "state_5.sqlite-wal", "state_5.sqlite-shm",
		"logs_2.sqlite", "logs_2.sqlite-wal", "memories_1.sqlite",
		"models_cache.json", "installation_id",
	} {
		writeFile(t, filepath.Join(personal, name), "personal-runtime")
	}
	for _, dir := range []string{"shell_snapshots", ".tmp", "sessions", "log", "cache"} {
		writeFile(t, filepath.Join(personal, dir, "x"), "personal-runtime")
	}
	// The child previously wrote its own state under the same names as personal
	// runtime files; a refresh must not replace them with personal copies.
	writeFile(t, filepath.Join(profile, "state_5.sqlite"), "child-private")

	if err := RefreshCodexProfile(profile); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	for _, name := range []string{
		"state_5.sqlite-wal", "state_5.sqlite-shm", "logs_2.sqlite",
		"memories_1.sqlite", "models_cache.json", "installation_id",
		"shell_snapshots", ".tmp", "sessions", "log", "cache",
	} {
		if _, err := os.Stat(filepath.Join(profile, name)); !os.IsNotExist(err) {
			t.Errorf("runtime state %q must not be mirrored (err=%v)", name, err)
		}
	}
	got, err := os.ReadFile(filepath.Join(profile, "state_5.sqlite"))
	if err != nil {
		t.Fatalf("child state must survive: %v", err)
	}
	if string(got) != "child-private" {
		t.Errorf("child state replaced by personal copy: %q", got)
	}
}

// TS-04.R21, INV §10/§14: an executable setup asset keeps its owner-execute bit
// in the profile while group/world bits are stripped.
func TestRefreshCodexProfilePreservesExecutableSetupBit(t *testing.T) {
	personal, profile := setupCodexHomes(t)
	script := filepath.Join(personal, "plugins", "hook.sh")
	writeFile(t, script, "#!/bin/sh\necho hi\n")
	if err := os.Chmod(script, 0o755); err != nil {
		t.Fatal(err)
	}
	plain := filepath.Join(personal, "plugins", "readme.md")
	writeFile(t, plain, "docs")
	if err := os.Chmod(plain, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := RefreshCodexProfile(profile); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if fi, err := os.Stat(filepath.Join(profile, "plugins", "hook.sh")); err != nil {
		t.Fatalf("stat script: %v", err)
	} else if fi.Mode().Perm() != 0o700 {
		t.Errorf("executable setup mode = %o, want 700 (owner-exec kept, group/world stripped)", fi.Mode().Perm())
	}
	if fi, err := os.Stat(filepath.Join(profile, "plugins", "readme.md")); err != nil {
		t.Fatalf("stat plain: %v", err)
	} else if fi.Mode().Perm() != 0o600 {
		t.Errorf("non-executable setup mode = %o, want 600", fi.Mode().Perm())
	}
}

// INV §5/§15: a copy failure late in the generation leaves the previously
// published profile fully intact — no partial replacement, no orphaned assets.
func TestRefreshCodexProfileRetainsPriorGenerationOnFailure(t *testing.T) {
	personal, profile := setupCodexHomes(t)
	writeFile(t, filepath.Join(personal, "config.toml"), "good = 1\n")
	writeFile(t, filepath.Join(personal, "plugins", "p.toml"), "good = 2\n")
	if err := RefreshCodexProfile(profile); err != nil {
		t.Fatalf("first refresh: %v", err)
	}

	// Introduce a root-escaping symlink inside a setup dir so the next refresh
	// fails during staging, after some entries would otherwise be published.
	outside := filepath.Join(t.TempDir(), "outside")
	writeFile(t, outside, "leak")
	if err := os.MkdirAll(filepath.Join(personal, "skills"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(personal, "skills", "evil")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(personal, "config.toml"), "changed = 1\n")

	if err := RefreshCodexProfile(profile); err == nil {
		t.Fatal("expected refresh to fail on the escaping symlink")
	}
	// The prior generation is retained byte-for-byte; the failed pass published nothing.
	if got, _ := os.ReadFile(filepath.Join(profile, "config.toml")); string(got) != "good = 1\n" {
		t.Errorf("prior config replaced by failed generation: %q", got)
	}
	if _, err := os.Stat(filepath.Join(profile, "plugins", "p.toml")); err != nil {
		t.Errorf("prior plugins lost after failed generation: %v", err)
	}
	if _, err := os.Stat(filepath.Join(profile, "skills")); !os.IsNotExist(err) {
		t.Errorf("failed generation must not publish partial new setup (err=%v)", err)
	}
}

// INV §5/§15: a failure after the new files were staged and swapped but before
// the manifest can publish rolls the entire generation back, including entries
// that would otherwise have been pruned.
func TestRefreshCodexProfileRollsBackManifestFailure(t *testing.T) {
	personal, profile := setupCodexHomes(t)
	writeFile(t, filepath.Join(personal, "config.toml"), "good = 1\n")
	writeFile(t, filepath.Join(personal, "plugins", "p.toml"), "good = 2\n")
	if err := RefreshCodexProfile(profile); err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(personal, "plugins")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(personal, "config.toml"), "changed = 1\n")

	originalWrite := writeCodexProfileManifest
	writeCodexProfileManifest = func(string, any) error { return errors.New("injected manifest failure") }
	t.Cleanup(func() { writeCodexProfileManifest = originalWrite })
	if err := RefreshCodexProfile(profile); err == nil {
		t.Fatal("expected refresh to fail while publishing manifest")
	}
	if got, _ := os.ReadFile(filepath.Join(profile, "config.toml")); string(got) != "good = 1\n" {
		t.Errorf("prior config replaced after manifest failure: %q", got)
	}
	if _, err := os.Stat(filepath.Join(profile, "plugins", "p.toml")); err != nil {
		t.Errorf("pruned setup not restored after manifest failure: %v", err)
	}
}

// INV §5/§15: concurrent refreshes never expose a partial generation. The
// serialized refresh completes for both callers and leaves a complete, consistent
// profile.
func TestRefreshCodexProfileConcurrentStartsAreConsistent(t *testing.T) {
	personal, profile := setupCodexHomes(t)
	writeFile(t, filepath.Join(personal, "config.toml"), "model = \"x\"\n")
	writeFile(t, filepath.Join(personal, "plugins", "p.toml"), "x = 1\n")

	const n = 8
	start := make(chan struct{})
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() {
			<-start
			errs <- RefreshCodexProfile(profile)
		}()
	}
	close(start)
	for i := 0; i < n; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent refresh: %v", err)
		}
	}
	for _, name := range []string{"config.toml", filepath.Join("plugins", "p.toml")} {
		if _, err := os.Stat(filepath.Join(profile, name)); err != nil {
			t.Errorf("concurrent refresh left setup %q missing: %v", name, err)
		}
	}
}

// INV §5/§15: process creation stays inside the profile lock. Once one child
// has refreshed and reached its start callback, a second start cannot publish a
// newer source generation until that first child exists to consume its own.
func TestWithRefreshedCodexProfileHoldsLockThroughStart(t *testing.T) {
	personal, profile := setupCodexHomes(t)
	writeFile(t, filepath.Join(personal, "config.toml"), "generation = 1\n")
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- WithRefreshedCodexProfile(profile, func() error {
			close(firstStarted)
			<-releaseFirst
			return nil
		})
	}()
	<-firstStarted
	writeFile(t, filepath.Join(personal, "config.toml"), "generation = 2\n")
	secondStarted := make(chan struct{})
	secondDone := make(chan error, 1)
	go func() {
		secondDone <- WithRefreshedCodexProfile(profile, func() error {
			close(secondStarted)
			return nil
		})
	}()
	select {
	case <-secondStarted:
		t.Fatal("second start entered before the first process-start callback released the profile lock")
	case <-time.After(100 * time.Millisecond):
	}
	if got, err := os.ReadFile(filepath.Join(profile, "config.toml")); err != nil || string(got) != "generation = 1\n" {
		t.Fatalf("first generation changed before its start completed: %q, err=%v", got, err)
	}
	close(releaseFirst)
	if err := <-firstDone; err != nil {
		t.Fatalf("first start: %v", err)
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("second start: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(profile, "config.toml")); err != nil || string(got) != "generation = 2\n" {
		t.Fatalf("second generation not published after first start: %q, err=%v", got, err)
	}
}

// INV §14: a profile destination that is a symlink is rejected before any
// mutation, so a prior manifest cannot drive a prune/copy through the alias into
// the personal home.
func TestRefreshCodexProfileRejectsSymlinkDestination(t *testing.T) {
	personal := filepath.Join(t.TempDir(), "codex")
	if err := os.MkdirAll(personal, 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(personal, "config.toml"), "keep = 1\n")
	t.Setenv("CODEX_HOME", personal)

	home := t.TempDir()
	profile := CodexProfileDir(home)
	// The profile path is a symlink aliasing the personal home.
	if err := os.Symlink(personal, profile); err != nil {
		t.Fatal(err)
	}
	// Seed a manifest so a naive prune would try to delete through the alias.
	writeFile(t, filepath.Join(home, dirCache, fileCodexProfileManifest),
		`{"version":1,"managed":["config.toml"]}`)

	if err := RefreshCodexProfile(profile); err == nil {
		t.Fatal("expected refresh to reject a symlink profile destination")
	}
	// The personal setup behind the alias is untouched.
	if _, err := os.Stat(filepath.Join(personal, "config.toml")); err != nil {
		t.Errorf("personal setup must be untouched by a rejected alias: %v", err)
	}
}

// INV §14: even a non-symlink destination inside the personal root is rejected
// before Refresh creates or chmods it.
func TestRefreshCodexProfileRejectsOverlappingPersonalHomeBeforeMutation(t *testing.T) {
	personal := filepath.Join(t.TempDir(), "codex")
	if err := os.MkdirAll(personal, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_HOME", personal)
	profile := filepath.Join(personal, "agentdeck-child")

	if err := RefreshCodexProfile(profile); err == nil {
		t.Fatal("expected overlapping source/profile to be rejected")
	}
	if _, err := os.Stat(profile); !os.IsNotExist(err) {
		t.Errorf("overlapping profile was mutated before rejection (err=%v)", err)
	}
}

func TestRefreshCodexProfileDereferencesInsideSymlink(t *testing.T) {
	personal, profile := setupCodexHomes(t)
	writeFile(t, filepath.Join(personal, "real", "config.toml"), "a = 1\n")
	if err := os.Symlink(filepath.Join(personal, "real", "config.toml"), filepath.Join(personal, "config.toml")); err != nil {
		t.Fatal(err)
	}
	if err := RefreshCodexProfile(profile); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	fi, err := os.Lstat(filepath.Join(profile, "config.toml"))
	if err != nil {
		t.Fatalf("stat mirrored link: %v", err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		t.Error("mirror must dereference the source link, not create a symlink")
	}
}

func TestRefreshCodexProfileMissingPersonalHome(t *testing.T) {
	profile := CodexProfileDir(t.TempDir())
	t.Setenv("CODEX_HOME", filepath.Join(t.TempDir(), "does-not-exist"))
	if err := RefreshCodexProfile(profile); err != nil {
		t.Fatalf("missing personal home must be a non-fatal skip: %v", err)
	}
	if fi, err := os.Stat(profile); err != nil || !fi.IsDir() {
		t.Errorf("profile dir should still be provisioned: %v", err)
	}
}
