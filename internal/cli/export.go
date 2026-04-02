package cli

import (
	"charm.land/log/v2"
	"github.com/spf13/cobra"
)

func newExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export data to various formats",
	}
	cmd.AddCommand(newExportArtifactsCmd())
	cmd.AddCommand(newExportObsidianCmd())
	cmd.AddCommand(newExportReportCmd())
	return cmd
}

func newExportArtifactsCmd() *cobra.Command {
	var outputDir string
	var dir string

	cmd := &cobra.Command{
		Use:   "artifacts",
		Short: "Export recon data as individual text files",
		Long: `Export all collected data as plaintext files organized by type.

Outputs:
  subdomains.txt         All discovered subdomains
  live-subdomains.txt    Live subdomains only
  urls.txt               All discovered URLs
  live-urls.txt          URLs with HTTP 200/206 response
  ips.txt                All discovered IPs
  ports.txt              Open ports as IP:port pairs
  findings/              Findings grouped by severity
  params/                URLs grouped by vuln class (idor, ssrf, xss, ...)
  endpoints/             Endpoints grouped by type (admin, auth, upload, api)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Info("Exporting recon data as individual text files...")
			return nil
		},
	}
	cmd.Flags().StringVarP(&dir, "dir", "d", "", "Project directory to export data from")
	cmd.Flags().StringVarP(&outputDir, "output", "o", "exports/artifacts", "Directory to save the exported files")
	return cmd
}

func newExportObsidianCmd() *cobra.Command {
	var outputDir string
	var dir string

	cmd := &cobra.Command{
		Use:   "obsidian",
		Short: "Export recon data as an Obsidian Canvas",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Info("Exporting recon data as an Obsidian Canvas...")
			return nil
		},
	}
	cmd.Flags().StringVarP(&dir, "dir", "d", "", "Project directory to export data from")
	cmd.Flags().StringVarP(&outputDir, "output", "o", "exports/obsidian", "Directory to save the exported Obsidian Canvas")
	return cmd
}

func newExportReportCmd() *cobra.Command {
	var outputFile string
	var dir string

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Export recon data as a comprehensive report",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Info("Exporting recon data as a comprehensive report...")
			return nil
		},
	}
	cmd.Flags().StringVarP(&dir, "dir", "d", "", "Project directory to export data from")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "exports/report.md", "File path to save the exported report (supports .md or .html)")
	return cmd
}
