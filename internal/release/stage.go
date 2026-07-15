package release

import (
	"fmt"
	"os"
	"path/filepath"
)

// StageArchive verifies a downloaded archive against its release manifest,
// extracts it into a same-filesystem staging area, verifies the extracted layout
// and internal manifest, then moves the version directory into versions/ with a
// single rename. It returns the version directory name; the caller Activates it.
//
// Every failure path preserves the current and previous runtimes and cleans up
// staging, so an interrupted, corrupt, or mislabeled download never reaches the
// stable command (FS-10.R8, TS-06.R17, TS-05.R12). Re-staging a version already
// present is idempotent: immutable version directories are reused, never
// overwritten, so a re-run cannot disturb a runtime a process is executing from.
func (l *Layout) StageArchive(archivePath string, m ReleaseManifest) (string, error) {
	if err := l.EnsureLayout(); err != nil {
		return "", err
	}
	if err := VerifyArchive(archivePath, m); err != nil {
		return "", fmt.Errorf("verify archive: %w", err)
	}

	if err := os.MkdirAll(l.StagingDir(), 0o700); err != nil {
		return "", err
	}
	tmp, err := os.MkdirTemp(l.StagingDir(), "stage-*")
	if err != nil {
		return "", err
	}
	// Best-effort cleanup: on success the version dir has been renamed out of tmp,
	// so only scratch remains; on failure this removes the partial extraction.
	defer os.RemoveAll(tmp)

	top, err := ExtractArchive(archivePath, tmp)
	if err != nil {
		return "", fmt.Errorf("extract archive: %w", err)
	}
	name := VersionDirName(m.Version)
	if top != name {
		return "", fmt.Errorf("archive top-level %q does not match expected %q", top, name)
	}
	extracted := filepath.Join(tmp, top)
	if err := VerifyLayout(extracted); err != nil {
		return "", err
	}
	if err := verifyInternalManifest(extracted, m.Version); err != nil {
		return "", err
	}

	dest := l.VersionDir(name)
	if _, err := os.Stat(dest); err == nil {
		// This immutable version is already installed; reuse it as-is.
		return name, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := os.Rename(extracted, dest); err != nil {
		return "", fmt.Errorf("install version %s: %w", name, err)
	}
	if err := fsyncDir(l.VersionsDir()); err != nil {
		return "", err
	}
	return name, nil
}
