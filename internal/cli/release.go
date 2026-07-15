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
