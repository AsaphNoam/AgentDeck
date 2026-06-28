package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	persistindex "github.com/agentdeck/agentdeck/internal/index"
	"github.com/agentdeck/agentdeck/internal/state"
)

// serverRunning reports whether an agentdeck server appears to be running by
// checking the pidfile and probing the process with signal 0.
func serverRunning(home string) bool {
	info, ok, err := readPidfile(home)
	if err != nil || !ok {
		return false
	}
	return processAlive(info.PID)
}

func newReindexCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reindex",
		Short: "Rebuild the archive/search index from raw transcript logs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			log := newLogger()
			cfgStore, _, err := resolveConfig(log)
			if err != nil {
				return err
			}
			if serverRunning(cfgStore.Home()) {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(),
					"warning: agentdeck server appears to be running — reindex wipes the index "+
						"and may corrupt it under a live server; stop the server first")
			}
			st, err := state.Open(cfgStore.Home())
			if err != nil {
				return err
			}
			defer st.Close()
			if err := persistindex.Reindex(cfgStore.Home(), st.DB()); err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "reindexed archive")
			return nil
		},
	}
}
