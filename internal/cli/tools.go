package cli

import (
	"charm.land/log/v2"
	"github.com/spf13/cobra"
)

func newToolsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "Manage external tools used in the recon pipeline",
		RunE:  listTools,
	}
	cmd.AddCommand(newToolsListCmd())
	cmd.AddCommand(newToolsShowCmd())
	return cmd
}

func listTools(cmd *cobra.Command, args []string) error {
	log.Info("Listing all configured tools...")
	return nil
}

func newToolsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all configured tools",
		RunE:  listTools,
	}
	return cmd
}

func newToolsShowCmd() *cobra.Command {
	var toolName string
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show details for a specific tool",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Info("Showing details for tool: %s", toolName)
			return nil
		},
	}
	cmd.Flags().StringVarP(&toolName, "name", "n", "", "Name of the tool to show details for")
	cmd.MarkFlagRequired("name")
	return cmd
}
