package configsource

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
)

const maxInventoryFiles = 512

func addInventoryFile(effective *Effective, report *Report, path, scope, kind, detachability string, approved []string) error {
	file, fp, _, err := fingerprintFile(path, scope, kind, approved)
	if err != nil {
		return err
	}
	report.FilesRead = append(report.FilesRead, file)
	report.Fingerprints = append(report.Fingerprints, fp)
	effective.Assets = append(effective.Assets, Asset{
		Kind: kind, Name: filepath.Base(file.Path), Path: file.Path, Scope: scope,
		SHA256: fp.SHA256, Detachability: detachability, Status: "enabled",
	})
	return nil
}

// walkInventory walks only a caller-selected, known setup directory. It never
// follows directory symlinks and bounds file count so a source cannot amplify a
// preview into an unbounded filesystem read.
func walkInventory(ctx context.Context, effective *Effective, report *Report, dir, scope, kind, detachability string, approved []string) error {
	canonicalDir, err := approvedPath(dir, approved)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	count := 0
	return filepath.WalkDir(canonicalDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			report.Skipped = append(report.Skipped, SkippedPath{Path: path, Reason: "unreadable"})
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path == canonicalDir {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			target, targetErr := approvedPath(path, approved)
			if targetErr != nil {
				reason := "broken_symlink"
				if errors.Is(targetErr, ErrApprovalRequired) {
					reason = "approval_required"
				}
				report.Skipped = append(report.Skipped, SkippedPath{Path: path, Reason: reason})
				return nil
			}
			info, statErr := os.Stat(target)
			if statErr != nil {
				report.Skipped = append(report.Skipped, SkippedPath{Path: path, Reason: "broken_symlink"})
				return nil
			}
			if info.IsDir() {
				report.Skipped = append(report.Skipped, SkippedPath{Path: path, Reason: "directory_symlink"})
				return nil
			}
		}
		if entry.IsDir() {
			return nil
		}
		if count >= maxInventoryFiles {
			report.Warnings = append(report.Warnings, "inventory file limit reached")
			return filepath.SkipAll
		}
		count++
		if err := addInventoryFile(effective, report, path, scope, kind, detachability, approved); err != nil {
			if errors.Is(err, ErrApprovalRequired) {
				report.Skipped = append(report.Skipped, SkippedPath{Path: path, Reason: "approval_required"})
				return nil
			}
			report.Skipped = append(report.Skipped, SkippedPath{Path: path, Reason: "unreadable"})
		}
		return nil
	})
}

func finalize(effective *Effective, report *Report) {
	sort.Strings(effective.MCPServers)
	sort.Slice(effective.Models, func(i, j int) bool {
		if effective.Models[i].ID == effective.Models[j].ID {
			return effective.Models[i].Source < effective.Models[j].Source
		}
		return effective.Models[i].ID < effective.Models[j].ID
	})
	sort.Slice(effective.Assets, func(i, j int) bool { return effective.Assets[i].Path < effective.Assets[j].Path })
	sort.Slice(effective.EnvKeys, func(i, j int) bool {
		if effective.EnvKeys[i].Name == effective.EnvKeys[j].Name {
			return effective.EnvKeys[i].Scope < effective.EnvKeys[j].Scope
		}
		return effective.EnvKeys[i].Name < effective.EnvKeys[j].Name
	})
	sort.Slice(report.FilesRead, func(i, j int) bool { return report.FilesRead[i].Path < report.FilesRead[j].Path })
	sort.Slice(report.Fingerprints, func(i, j int) bool { return report.Fingerprints[i].Path < report.Fingerprints[j].Path })
	sort.Slice(report.Skipped, func(i, j int) bool { return report.Skipped[i].Path < report.Skipped[j].Path })
	sort.Slice(report.UnknownKeys, func(i, j int) bool {
		if report.UnknownKeys[i].Path == report.UnknownKeys[j].Path {
			return report.UnknownKeys[i].Key < report.UnknownKeys[j].Key
		}
		return report.UnknownKeys[i].Path < report.UnknownKeys[j].Path
	})
	sort.Strings(report.ApprovedRoots)
	report.SourceDigest = generationKey(report.Fingerprints)
}
