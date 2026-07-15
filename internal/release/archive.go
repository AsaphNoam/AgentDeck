package release

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// maxArchiveBytes bounds how much a single archive may expand to, so a malformed
// or hostile archive cannot exhaust the disk during staging (INV §9: external
// content is untrusted). 2 GiB is far above a Node+adapter runtime.
const maxArchiveBytes = 2 << 30

// ChecksumFile returns the lowercase hex SHA-256 of a file.
func ChecksumFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifyArchive checks a downloaded archive's size and SHA-256 against the
// release manifest before any extraction. A mismatch means a corrupt or
// tampered download; the caller keeps the current runtime (TS-05.R12, TS-06.R17).
func VerifyArchive(path string, m ReleaseManifest) error {
	if err := m.Validate(); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Size() != m.Size {
		return fmt.Errorf("archive size %d does not match manifest %d", info.Size(), m.Size)
	}
	sum, err := ChecksumFile(path)
	if err != nil {
		return err
	}
	if !strings.EqualFold(sum, m.SHA256) {
		return fmt.Errorf("archive checksum %s does not match manifest %s", sum, m.SHA256)
	}
	return nil
}

// CreateArchive writes srcDir (a single top-level version directory such as
// agentdeck-1.2.3-darwin-arm64) into a gzip-compressed tar at archivePath,
// preserving executable bits and a stable top-level prefix. Used by release
// assembly and tests (INV §2).
func CreateArchive(srcDir, archivePath string) error {
	out, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer out.Close()
	gz := gzip.NewWriter(out)
	tw := tar.NewWriter(gz)

	prefix := filepath.Base(srcDir)
	walkErr := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && path == srcDir {
			return nil // the tree is rooted at the version dir itself, added via prefix
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(filepath.Join(prefix, rel))
		if info.IsDir() {
			hdr.Name += "/"
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(tw, f)
			f.Close()
			if copyErr != nil {
				return copyErr
			}
		}
		return nil
	})
	if walkErr != nil {
		return walkErr
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

// ExtractArchive unpacks a gzip tar into destDir. It rejects any entry whose
// path escapes destDir (path-traversal / "zip slip") and caps total expansion
// (INV §9). It returns the single top-level directory name the archive carried.
func ExtractArchive(archivePath, destDir string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("open gzip: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)

	cleanDest := filepath.Clean(destDir)
	var topLevel string
	var written int64
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read archive: %w", err)
		}
		name := filepath.Clean(hdr.Name)
		if name == "." || name == ".." || strings.HasPrefix(name, ".."+string(os.PathSeparator)) {
			return "", fmt.Errorf("archive entry escapes destination: %q", hdr.Name)
		}
		target := filepath.Join(cleanDest, name)
		if target != cleanDest && !strings.HasPrefix(target, cleanDest+string(os.PathSeparator)) {
			return "", fmt.Errorf("archive entry escapes destination: %q", hdr.Name)
		}
		if top := topOf(name); top != "" {
			if topLevel == "" {
				topLevel = top
			} else if top != topLevel {
				return "", fmt.Errorf("archive has multiple top-level entries: %q and %q", topLevel, top)
			}
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o700); err != nil {
				return "", err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return "", err
			}
			written += hdr.Size
			if written > maxArchiveBytes {
				return "", fmt.Errorf("archive exceeds %d bytes", maxArchiveBytes)
			}
			if err := writeFile(target, tr, os.FileMode(hdr.Mode).Perm()); err != nil {
				return "", err
			}
		case tar.TypeSymlink:
			// The release layout is plain files and directories; a symlink in an
			// archive is a traversal vector, so reject it rather than honor it.
			return "", fmt.Errorf("archive contains a symlink entry %q; not permitted", hdr.Name)
		default:
			return "", fmt.Errorf("archive entry %q has unsupported type %d", hdr.Name, hdr.Typeflag)
		}
	}
	if topLevel == "" {
		return "", fmt.Errorf("archive is empty")
	}
	return topLevel, nil
}

// topOf returns the first path element of a cleaned relative name.
func topOf(name string) string {
	if i := strings.IndexRune(name, os.PathSeparator); i >= 0 {
		return name[:i]
	}
	return name
}

// writeFile streams a tar entry to disk, capping the copy so a lying header
// cannot overrun maxArchiveBytes.
func writeFile(target string, r io.Reader, mode os.FileMode) error {
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, io.LimitReader(r, maxArchiveBytes)); err != nil {
		out.Close()
		return err
	}
	if err := out.Chmod(mode); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
