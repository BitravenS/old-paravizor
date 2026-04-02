package cli

import (
	"charm.land/log/v2"
	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the recon pipeline",
		Long:  "Run the recon pipeline with the specified configuration",
		Run: func(cmd *cobra.Command, args []string) {
			log.Info("Running the recon pipeline...")
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "d", "", "Project directory to run the recon pipeline in")
	return cmd
}
