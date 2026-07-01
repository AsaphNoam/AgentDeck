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
			// reindex opens its own writer and DELETEs the index tables — running
			// it against a live server violates the sole-writer invariant and can
			// corrupt/rewind live archive data. Refuse rather than warn. (A stale
			// pidfile is handled by serverRunning's signal-0 liveness probe, so this
			// only fires when the daemon is actually up.)
			if serverRunning(cfgStore.Home()) {
				return fmt.Errorf("agentdeck server is running — stop it before reindex " +
					"(reindex wipes and rebuilds the archive index and must be the sole DB writer)")
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
