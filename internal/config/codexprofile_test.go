package config

import (
	"os"
	"path/filepath"
	"testing"
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
	if err := os.Symlink(outside, filepath.Join(personal, "leak.toml")); err != nil {
		t.Fatal(err)
	}
	if err := RefreshCodexProfile(profile); err == nil {
		t.Fatal("expected refresh to fail on a root-escaping symlink")
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
