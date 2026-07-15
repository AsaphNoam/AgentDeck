package release

// Install is the single staged install/update transaction shared by the
// bootstrap installer and `agentdeck update` (INV §2). Under the install lock it
// verifies and stages the archive, activates it (recording the outgoing runtime
// as previous), and rewrites the stable shim. It never writes user configuration
// or credentials, and never signals a running dashboard (TS-06.R16–R19).
//
// On any failure it returns before activation with the current runtime intact,
// so an interrupted or corrupt install leaves the working command unchanged
// (FS-10.R8, TS-05.R12).
func (l *Layout) Install(archivePath string, m ReleaseManifest) (string, error) {
	lk, err := l.Lock()
	if err != nil {
		return "", err
	}
	defer lk.Release()
	return l.InstallWithLock(archivePath, m)
}

// InstallWithLock performs the install transaction while the caller holds l's
// install lock. It lets an updater claim the lock before it resolves release
// metadata or downloads an archive, and lets the bootstrap's lockf wrapper hold
// the same claim from its initial release lookup through activation.
//
// Callers must not invoke this without a held Layout lock (TS-06.R19).
func (l *Layout) InstallWithLock(archivePath string, m ReleaseManifest) (string, error) {
	name, err := l.StageArchive(archivePath, m)
	if err != nil {
		return "", err
	}
	if err := l.Activate(name); err != nil {
		return "", err
	}
	if err := l.WriteShim(); err != nil {
		return "", err
	}
	return name, nil
}
