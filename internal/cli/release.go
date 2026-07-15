package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/agentdeck/agentdeck/internal/release"
)

// newReleaseCmd builds the hidden `release` command group. These are the
// internal boundary the bootstrap installer hands off to after it downloads and
// checksum-verifies an archive; end users drive install via the documented
// bootstrap script and updates via `agentdeck update` (TS-06.R17–R19).
func newReleaseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "release",
		Short:  "Internal release-runtime operations",
		Hidden: true,
	}
	cmd.AddCommand(newReleaseInstallCmd())
	cmd.AddCommand(newReleaseWrapperCmd(), newReleaseManifestCmd(), newReleasePackageCmd())
	return cmd
}

func newReleaseWrapperCmd() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "wrapper",
		Short: "Write a private-runtime wrapper into an assembled version directory",
		RunE: func(_ *cobra.Command, _ []string) error {
			return release.WriteWrapper(dir)
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "assembled version directory")
	_ = cmd.MarkFlagRequired("dir")
	return cmd
}

func newReleaseManifestCmd() *cobra.Command {
	var dir, version, node, claudeACP, codexACP string
	cmd := &cobra.Command{
		Use:   "manifest",
		Short: "Write the internal identity manifest into an assembled version directory",
		RunE: func(_ *cobra.Command, _ []string) error {
			return release.WriteInternalManifest(dir, release.InternalManifest{
				Version: version,
				Target:  release.Target,
				Components: map[string]string{
					"node": node, "claude-agent-acp": claudeACP, "codex-acp": codexACP, "agentdeck": version,
				},
			})
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "assembled version directory")
	cmd.Flags().StringVar(&version, "version", "", "release version")
	cmd.Flags().StringVar(&node, "node", "", "private Node version")
	cmd.Flags().StringVar(&claudeACP, "claude-acp", "", "Claude ACP adapter version")
	cmd.Flags().StringVar(&codexACP, "codex-acp", "", "Codex ACP adapter version")
	for _, flag := range []string{"dir", "version", "node", "claude-acp", "codex-acp"} {
		_ = cmd.MarkFlagRequired(flag)
	}
	return cmd
}

func newReleasePackageCmd() *cobra.Command {
	var dir, outputDir, version string
	cmd := &cobra.Command{
		Use:   "package",
		Short: "Verify an assembled runtime and emit its archive and release manifest",
		RunE: func(cmd *cobra.Command, _ []string) error {
			m, err := release.PackageRelease(dir, outputDir, version)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "packaged %s\n", m.Archive)
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "assembled version directory")
	cmd.Flags().StringVar(&outputDir, "output-dir", "", "directory for release assets")
	cmd.Flags().StringVar(&version, "version", "", "release version")
	for _, flag := range []string{"dir", "output-dir", "version"} {
		_ = cmd.MarkFlagRequired(flag)
	}
	return cmd
}

// readReleaseManifest loads and validates a published release manifest file.
func readReleaseManifest(path string) (release.ReleaseManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return release.ReleaseManifest{}, err
	}
	var m release.ReleaseManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return release.ReleaseManifest{}, fmt.Errorf("parse release manifest: %w", err)
	}
	if err := m.Validate(); err != nil {
		return release.ReleaseManifest{}, err
	}
	return m, nil
}

func newReleaseInstallCmd() *cobra.Command {
	var archive, manifest string
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Verify and activate a downloaded release archive",
		RunE: func(cmd *cobra.Command, _ []string) error {
			m, err := readReleaseManifest(manifest)
			if err != nil {
				return err
			}
			layout, err := release.Open()
			if err != nil {
				return err
			}
			name, err := layout.Install(archive, m)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "installed %s\n", m.Version)
			fmt.Fprintf(cmd.OutOrStdout(), "runtime %s\n", layout.VersionDir(name))
			fmt.Fprintf(cmd.OutOrStdout(), "command %s\n", layout.ShimPath())
			return nil
		},
	}
	cmd.Flags().StringVar(&archive, "archive", "", "path to the downloaded release archive")
	cmd.Flags().StringVar(&manifest, "manifest", "", "path to the release manifest JSON")
	_ = cmd.MarkFlagRequired("archive")
	_ = cmd.MarkFlagRequired("manifest")
	return cmd
}
