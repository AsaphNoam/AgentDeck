// Package cli wires the cobra command tree for the chuck binary.
package cli

import (
	"errors"
	"os"

	"github.com/spf13/cobra"

	"github.com/AsaphNoam/Chuck/internal/version"
)

// NewRootCmd builds the root cobra command with --version and subcommands.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "chuck",
		Short:         "Chuck — local dashboard for orchestrating coding agents",
		Version:       version.String(),
		SilenceUsage:  true,
		SilenceErrors: false,
	}
	// `chuck --version` prints "chuck version <version> (commit, date)".
	root.SetVersionTemplate("chuck version {{.Version}}\n")
	root.AddCommand(newDashboardCmd())
	root.AddCommand(newReindexCmd())
	root.AddCommand(newResumeCmd())
	root.AddCommand(newReleaseCmd())
	root.AddCommand(newUpdateCmd())
	root.AddCommand(newAuthCmd())
	root.AddCommand(newPipelineCmd())
	return root
}

// newResumeCmd returns the `chuck resume <agent_id>` cobra command.
func newResumeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resume <agent_id>",
		Short: "Resume an inactive persisted session by agent_id",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			os.Exit(runResumeByID(args[0]))
		},
	}
}

// Execute is the entrypoint called by cmd/chuck/main.go. It intercepts the
// reserved `<role>@<project>` launch syntax before cobra dispatch, then runs the
// command tree. Returns the process exit code.
func Execute(args []string) int {
	// Reserved launch syntax: first positional arg of the form role@project.
	if len(args) > 0 && isLaunchArg(args[0]) {
		return runLaunch(args)
	}

	root := NewRootCmd()
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		if errors.Is(err, errProviderUnavailable) {
			return 3
		}
		return 1
	}
	return 0
}
