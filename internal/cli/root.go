// Package cli wires the cobra command tree for the agentdeck binary.
package cli

import (
	"github.com/spf13/cobra"

	"github.com/agentdeck/agentdeck/internal/version"
)

// NewRootCmd builds the root cobra command with --version and subcommands.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "agentdeck",
		Short:         "AgentDeck — local dashboard for orchestrating coding agents",
		Version:       version.String(),
		SilenceUsage:  true,
		SilenceErrors: false,
	}
	// `agentdeck --version` prints "agentdeck version <version> (commit, date)".
	root.SetVersionTemplate("agentdeck version {{.Version}}\n")
	root.AddCommand(newDashboardCmd())
	return root
}

// Execute is the entrypoint called by cmd/agentdeck/main.go. It intercepts the
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
		return 1
	}
	return 0
}
