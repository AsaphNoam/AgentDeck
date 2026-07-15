package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/agentdeck/agentdeck/internal/release"
)

// newFetcher builds the release fetcher; tests override it to avoid network.
var newFetcher = func(repo string) release.Fetcher { return release.NewGitHubFetcher(repo) }

// newUpdateCmd builds `agentdeck update`, the only update mechanism. It contacts
// GitHub only when invoked, never in the background (FS-10.R7, TS-06.R19).
func newUpdateCmd() *cobra.Command {
	var check, yes, rollback bool
	var repo string
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update AgentDeck to the latest release, or roll back",
		RunE: func(cmd *cobra.Command, _ []string) error {
			layout, err := release.Open()
			if err != nil {
				return err
			}
			if rollback {
				if check || yes {
					return errors.New("--rollback cannot be combined with --check or --yes")
				}
				return runRollback(cmd, layout)
			}
			return runUpdate(cmd, layout, newFetcher(repo), check, yes)
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "only report whether an update is available")
	cmd.Flags().BoolVar(&yes, "yes", false, "install without prompting (non-interactive)")
	cmd.Flags().BoolVar(&rollback, "rollback", false, "restore the immediately preceding release")
	cmd.Flags().StringVar(&repo, "repo", release.DefaultRepo, "GitHub owner/repo to fetch releases from")
	_ = cmd.Flags().MarkHidden("repo")
	return cmd
}

// runRollback restores the previous release under the install lock (TS-06.R18).
func runRollback(cmd *cobra.Command, layout *release.Layout) error {
	lk, err := layout.Lock()
	if err != nil {
		return err
	}
	defer lk.Release()

	if err := layout.Rollback(); err != nil {
		if errors.Is(err, release.ErrNoPrevious) {
			fmt.Fprintln(cmd.OutOrStdout(), "no previous version to roll back to")
			return nil
		}
		return err
	}
	if v, ok, _ := layout.CurrentVersion(); ok {
		fmt.Fprintf(cmd.OutOrStdout(), "rolled back to %s\n", v)
	}
	return nil
}

// runUpdate checks availability and, unless --check, downloads and installs the
// latest release. A failed download or verification leaves the current runtime
// intact because Install stages and verifies before it activates (FS-10.R8).
func runUpdate(cmd *cobra.Command, layout *release.Layout, f release.Fetcher, check, yes bool) error {
	// Claim the install root before even resolving release metadata. Otherwise a
	// second updater can download in parallel and activate after the first one
	// releases Install's former activation-only lock (FS-10.R13, TS-06.R19).
	lk, err := layout.Lock()
	if err != nil {
		return err
	}
	defer lk.Release()

	out := cmd.OutOrStdout()
	cur, hasCur, err := layout.CurrentVersion()
	if err != nil {
		return err
	}
	if !hasCur {
		return errors.New("no AgentDeck release is installed; run the installer first")
	}

	m, err := f.Latest(cmd.Context())
	if err != nil {
		return err
	}
	if !release.UpdateAvailable(cur, m.Version) {
		fmt.Fprintf(out, "AgentDeck is up to date (%s)\n", cur)
		return nil
	}
	if check {
		fmt.Fprintf(out, "update available: %s -> %s\nrun 'agentdeck update' to install\n", cur, m.Version)
		return nil
	}
	if !yes {
		if !isInteractive(cmd) {
			return fmt.Errorf("update available %s -> %s; re-run with --yes to install non-interactively", cur, m.Version)
		}
		if !confirm(cmd, fmt.Sprintf("Update AgentDeck %s -> %s?", cur, m.Version)) {
			fmt.Fprintln(out, "update cancelled")
			return nil
		}
	}

	tmp, err := os.MkdirTemp("", "agentdeck-update-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	fmt.Fprintf(out, "downloading %s...\n", m.Version)
	archive, err := f.Download(cmd.Context(), m, tmp)
	if err != nil {
		return err
	}
	name, err := layout.InstallWithLock(archive, m)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "updated %s -> %s\n", cur, m.Version)
	fmt.Fprintf(out, "command %s\n", layout.ShimPath())
	_ = name
	return nil
}

// isInteractive reports whether the command's input is a terminal, so a bare
// `update` never blocks a script waiting for a prompt that will never be
// answered. Basing this on the command's input (not os.Stdin directly) keeps the
// decision injectable in tests.
func isInteractive(cmd *cobra.Command) bool {
	f, ok := cmd.InOrStdin().(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// confirm prompts for a yes/no answer, defaulting to no.
func confirm(cmd *cobra.Command, prompt string) bool {
	fmt.Fprintf(cmd.OutOrStdout(), "%s [y/N] ", prompt)
	reader := bufio.NewReader(cmd.InOrStdin())
	line, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}
