package release

import "context"

// Fetcher resolves and downloads release artifacts. It is an interface so the
// updater's decision logic is tested against a fake and the real GitHub path
// (github.go) stays a thin, network-only adapter (TS-06.R19).
type Fetcher interface {
	// Latest returns the release manifest for the newest published release.
	Latest(ctx context.Context) (ReleaseManifest, error)
	// Download fetches the manifest's archive into destDir and returns its path.
	// It does not verify the checksum; the shared Install transaction does.
	Download(ctx context.Context, m ReleaseManifest, destDir string) (string, error)
}

// CurrentVersion reports the version of the active runtime by reading the current
// pointer's internal manifest. Returns ("", false) when nothing is installed.
func (l *Layout) CurrentVersion() (string, bool, error) {
	name, ok, err := l.Current()
	if err != nil || !ok {
		return "", ok, err
	}
	m, err := ReadInternalManifest(l.VersionDir(name))
	if err != nil {
		return "", false, err
	}
	return m.Version, true, nil
}

// UpdateAvailable reports whether latest differs from the installed version. The
// GitHub "latest" release is authoritative, so any difference is an available
// change; this MVP does not attempt semantic-version ordering (TS-06.R19).
func UpdateAvailable(current, latest string) bool {
	return latest != "" && latest != current
}
