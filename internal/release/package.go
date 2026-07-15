package release

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// PackageRelease verifies an assembled runtime, writes its release archive, and
// emits the matching public manifest. Release assembly and the installer share
// the archive/layout rules through this one helper (TS-06.R15, R17; INV §2).
func PackageRelease(versionDir, outputDir, version string) (ReleaseManifest, error) {
	if version == "" {
		return ReleaseManifest{}, fmt.Errorf("package release: empty version")
	}
	if filepath.Base(versionDir) != VersionDirName(version) {
		return ReleaseManifest{}, fmt.Errorf("package release: version directory %q does not match %q", filepath.Base(versionDir), VersionDirName(version))
	}
	if err := VerifyLayout(versionDir); err != nil {
		return ReleaseManifest{}, err
	}
	if err := verifyInternalManifest(versionDir, version); err != nil {
		return ReleaseManifest{}, err
	}
	if err := os.MkdirAll(outputDir, 0o700); err != nil {
		return ReleaseManifest{}, err
	}

	m := ReleaseManifest{
		Version: version,
		Target:  Target,
		Archive: fmt.Sprintf("agentdeck-%s-%s.tar.gz", version, Target),
	}
	archivePath := filepath.Join(outputDir, m.Archive)
	if err := CreateArchive(versionDir, archivePath); err != nil {
		return ReleaseManifest{}, err
	}
	info, err := os.Stat(archivePath)
	if err != nil {
		return ReleaseManifest{}, err
	}
	m.Size = info.Size()
	m.SHA256, err = ChecksumFile(archivePath)
	if err != nil {
		return ReleaseManifest{}, err
	}
	if err := m.Validate(); err != nil {
		return ReleaseManifest{}, err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return ReleaseManifest{}, err
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(outputDir, "manifest.json"), data, 0o600); err != nil {
		return ReleaseManifest{}, err
	}
	return m, nil
}
