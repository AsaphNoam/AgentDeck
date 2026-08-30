package agentknowledge

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// FS-18.A1/A3/A4, TS-11.R1-R2/R10.
func TestInstallPublishesVerifiedOwnerOnlyProviderViews(t *testing.T) {
	home := t.TempDir()
	installation, err := Install(home)
	if err != nil {
		t.Fatal(err)
	}
	if !installation.Available {
		t.Fatal("installation is unavailable")
	}
	want, err := embeddedFiles()
	if err != nil {
		t.Fatal(err)
	}
	for _, provider := range []string{".agents", ".claude"} {
		base := filepath.Join(installation.Root, provider, "skills", SkillName)
		for rel, expected := range want {
			path := filepath.Join(base, filepath.FromSlash(rel))
			info, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
				t.Fatalf("%s mode = %v", path, info.Mode())
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != string(expected) {
				t.Fatalf("%s differs from embedded package", path)
			}
		}
	}
	info, err := os.Stat(installation.Root)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("managed root mode = %v", info.Mode())
	}

	unknown := filepath.Join(installation.Root, ".agents", "skills", SkillName, "unknown.md")
	if err := os.WriteFile(unknown, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := Install(home)
	if err != nil || !second.Available {
		t.Fatalf("refresh = %+v, %v", second, err)
	}
	if _, err := os.Stat(unknown); !os.IsNotExist(err) {
		t.Fatalf("refresh retained unknown entry: %v", err)
	}
}

func TestInstallRejectsSymlinkedCache(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions vary on Windows")
	}
	home := t.TempDir()
	target := t.TempDir()
	if err := os.Symlink(target, filepath.Join(home, "cache")); err != nil {
		t.Fatal(err)
	}
	installation, err := Install(home)
	if err == nil || installation.Available {
		t.Fatalf("symlinked cache install = %+v, %v", installation, err)
	}
	entries, readErr := os.ReadDir(target)
	if readErr != nil || len(entries) != 0 {
		t.Fatalf("symlink target changed: entries=%v err=%v", entries, readErr)
	}
}

func TestEmbeddedSkillUsesBoundedProgressiveReferences(t *testing.T) {
	files, err := embeddedFiles()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"SKILL.md",
		"references/build-and-run-pipelines.md",
		"references/coordinate-work.md",
		"references/operate-agents.md",
	}
	if len(files) != len(want) {
		t.Fatalf("inventory size = %d, want %d", len(files), len(want))
	}
	for _, name := range want {
		if _, ok := files[name]; !ok {
			t.Errorf("missing %s", name)
		}
	}
	core := string(files["SKILL.md"])
	for _, link := range want[1:] {
		if !strings.Contains(core, "("+link+")") {
			t.Errorf("SKILL.md does not route %s", link)
		}
	}
	for _, forbidden := range []string{"per-turn mail budget of 15", "human Continue", "propose_pipeline_template", "agentdeck release"} {
		if strings.Contains(core, forbidden) {
			t.Errorf("SKILL.md contains reference-owned detail %q", forbidden)
		}
	}
	if !strings.Contains(string(files["references/coordinate-work.md"]), "budget of 15") {
		t.Error("coordination reference is missing the messaging budget")
	}
	pipeline := string(files["references/build-and-run-pipelines.md"])
	for _, phrase := range []string{"`blocked`", "Continue", "AgentDecker"} {
		if !strings.Contains(pipeline, phrase) {
			t.Errorf("pipeline reference is missing %q", phrase)
		}
	}
}
