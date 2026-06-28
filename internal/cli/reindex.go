package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	persistindex "github.com/agentdeck/agentdeck/internal/index"
	"github.com/agentdeck/agentdeck/internal/state"
)

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
