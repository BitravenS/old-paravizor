package cli

import (
	"charm.land/log/v2"
	"github.com/spf13/cobra"
)

func newQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query",
		Args:  cobra.MaximumNArgs(1),
		Short: "Query the project database",
		Long: `Run custom queries against the project's SQLite database.
		Running without arguments will start an interactive SQL shell.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				log.Info("Running query: %s", args[0])
				return nil
			}
			log.Info("Starting interactive SQL shell...")
			return nil
		},
	}
	return cmd
}
