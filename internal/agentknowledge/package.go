// Package agentknowledge owns AgentDeck's embedded, release-matched operating skill.
package agentknowledge

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	ManagedRootName = "agent-skills"
	SkillName       = "operating-agentdeck"
)

//go:embed operating-agentdeck
var packageFS embed.FS

// Installation is the immutable process-local result of publishing the skill.
type Installation struct {
	Available bool
	Root      string
	SkillDir  string
}

// Install publishes two verified provider views beneath AgentDeck's owner-only cache.
func Install(home string) (Installation, error) {
	cacheDir := filepath.Join(home, "cache")
	root := filepath.Join(cacheDir, ManagedRootName)
	result := Installation{Root: root, SkillDir: filepath.Join(root, ".agents", "skills", SkillName)}
	if err := secureDir(cacheDir); err != nil {
		return result, err
	}

	stage, err := os.MkdirTemp(cacheDir, ".agent-skills-stage-")
	if err != nil {
		return result, fmt.Errorf("stage operator skill: %w", err)
	}
	defer os.RemoveAll(stage)
	if err := os.Chmod(stage, 0o700); err != nil {
		return result, fmt.Errorf("secure operator skill stage: %w", err)
	}
	for _, provider := range []string{".agents", ".claude"} {
		target := filepath.Join(stage, provider, "skills", SkillName)
		if err := projectPackage(target); err != nil {
			return result, err
		}
	}
	if err := verifyTree(stage); err != nil {
		return result, err
	}
	if err := syncTreeDirs(stage); err != nil {
		return result, fmt.Errorf("sync staged operator skill: %w", err)
	}

	backup, err := os.MkdirTemp(cacheDir, ".agent-skills-previous-")
	if err != nil {
		return result, fmt.Errorf("reserve operator skill backup: %w", err)
	}
	if err := os.Remove(backup); err != nil {
		return result, fmt.Errorf("prepare operator skill backup: %w", err)
	}
	defer os.RemoveAll(backup)
	hadRoot := false
	if _, err := os.Lstat(root); err == nil {
		hadRoot = true
		if err := os.Rename(root, backup); err != nil {
			return result, fmt.Errorf("preserve prior operator skill: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return result, fmt.Errorf("inspect operator skill root: %w", err)
	}
	if err := os.Rename(stage, root); err != nil {
		if hadRoot {
			_ = os.Rename(backup, root)
		}
		return result, fmt.Errorf("publish operator skill: %w", err)
	}
	if err := syncDir(cacheDir); err != nil {
		_ = os.RemoveAll(root)
		if hadRoot {
			_ = os.Rename(backup, root)
		}
		return result, fmt.Errorf("sync operator skill publication: %w", err)
	}
	if hadRoot {
		if err := os.RemoveAll(backup); err != nil {
			return result, fmt.Errorf("remove prior operator skill: %w", err)
		}
	}
	if err := verifyTree(root); err != nil {
		return result, err
	}
	result.Available = true
	return result, nil
}

func projectPackage(target string) error {
	return fs.WalkDir(packageFS, SkillName, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(SkillName, path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("invalid embedded operator skill path %q", path)
		}
		dst := target
		if rel != "." {
			dst = filepath.Join(target, rel)
		}
		if entry.IsDir() {
			return secureDir(dst)
		}
		if entry.Type() != 0 {
			return fmt.Errorf("embedded operator skill entry %q is not a regular file", path)
		}
		data, err := packageFS.ReadFile(path)
		if err != nil {
			return err
		}
		if err := writeOwnerFile(dst, data); err != nil {
			return fmt.Errorf("write operator skill %s: %w", dst, err)
		}
		return nil
	})
}

func verifyTree(root string) error {
	want, err := embeddedFiles()
	if err != nil {
		return err
	}
	for _, provider := range []string{".agents", ".claude"} {
		base := filepath.Join(root, provider, "skills", SkillName)
		got := []string{}
		err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("operator skill contains symlink %s", path)
			}
			if entry.IsDir() {
				if info.Mode().Perm() != 0o700 {
					return fmt.Errorf("operator skill directory %s has mode %o", path, info.Mode().Perm())
				}
				return nil
			}
			if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
				return fmt.Errorf("operator skill file %s is not owner-only regular data", path)
			}
			rel, err := filepath.Rel(base, path)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return fmt.Errorf("operator skill path escapes managed root: %s", path)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			expected, ok := want[filepath.ToSlash(rel)]
			if !ok || string(data) != string(expected) {
				return fmt.Errorf("operator skill file %s differs from embedded package", path)
			}
			got = append(got, filepath.ToSlash(rel))
			return nil
		})
		if err != nil {
			return fmt.Errorf("verify operator skill: %w", err)
		}
		sort.Strings(got)
		if len(got) != len(want) {
			return fmt.Errorf("operator skill %s has unexpected inventory", provider)
		}
	}
	return nil
}

func embeddedFiles() (map[string][]byte, error) {
	files := map[string][]byte{}
	err := fs.WalkDir(packageFS, SkillName, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		rel, err := filepath.Rel(SkillName, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)], err = packageFS.ReadFile(path)
		return err
	})
	return files, err
}

func secureDir(path string) error {
	info, err := os.Lstat(path)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("managed operator skill path %s is not a directory", path)
		}
		return os.Chmod(path, 0o700)
	}
	if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	info, err = os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("managed operator skill path %s was replaced during creation", path)
	}
	return os.Chmod(path, 0o700)
}

func writeOwnerFile(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		_ = file.Close()
		if !ok {
			_ = os.Remove(path)
		}
	}()
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Chmod(0o600); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	ok = true
	return nil
}

func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func syncTreeDirs(root string) error {
	dirs := []string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			dirs = append(dirs, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := syncDir(dirs[i]); err != nil {
			return err
		}
	}
	return nil
}
