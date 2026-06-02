package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	_ "modernc.org/sqlite"

	"github.com/bitravens/paravizor/v1/internal/project"
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
		RunE: func(cmd *cobra.Command, args []string) error {
			absDir, database, closeFn, err := openProjectDB(dir)
			if err != nil {
				return err
			}
			defer closeFn()
			outDir := outputDir
			if !filepath.IsAbs(outDir) {
				outDir = filepath.Join(absDir, outDir)
			}
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return err
			}
			exports := map[string]string{
				"subdomains.txt":      `SELECT name FROM domains ORDER BY name`,
				"live-subdomains.txt": `SELECT name FROM domains WHERE is_live = 1 ORDER BY name`,
				"urls.txt":            `SELECT full_url FROM urls ORDER BY full_url`,
				"ips.txt":             `SELECT address FROM ips ORDER BY address`,
				"ports.txt":           `SELECT ips.address || ':' || ports.port || '/' || ports.protocol FROM ports JOIN ips ON ips.id = ports.ip_id ORDER BY ips.address, ports.port`,
				"findings.txt":        `SELECT COALESCE(severity, 'info') || ' - ' || title FROM findings ORDER BY severity, title`,
			}
			for name, query := range exports {
				if err := writeQueryLines(context.Background(), database, filepath.Join(outDir, name), query); err != nil {
					return err
				}
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Exported artifacts to %s\n", outDir)
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
		Short: "Export recon data as an Obsidian-friendly markdown note",
		RunE: func(cmd *cobra.Command, args []string) error {
			return exportReportLike(cmd, dir, filepath.Join(outputDir, "paravizor-recon.md"))
		},
	}
	cmd.Flags().StringVarP(&dir, "dir", "d", "", "Project directory to export data from")
	cmd.Flags().StringVarP(&outputDir, "output", "o", "exports/obsidian", "Directory to save the Obsidian markdown file")
	return cmd
}

func newExportReportCmd() *cobra.Command {
	var outputFile string
	var dir string
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Export recon data as a markdown report",
		RunE: func(cmd *cobra.Command, args []string) error {
			return exportReportLike(cmd, dir, outputFile)
		},
	}
	cmd.Flags().StringVarP(&dir, "dir", "d", "", "Project directory to export data from")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "exports/report.md", "File path to save the exported report")
	return cmd
}

func exportReportLike(cmd *cobra.Command, dir, outputFile string) error {
	absDir, database, closeFn, err := openProjectDB(dir)
	if err != nil {
		return err
	}
	defer closeFn()
	projCfg, _ := project.LoadProject(absDir)
	outPath := outputFile
	if !filepath.IsAbs(outPath) {
		outPath = filepath.Join(absDir, outPath)
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	sections := []struct{ title, query string }{
		{"Live Domains", `SELECT name FROM domains WHERE is_live = 1 ORDER BY name LIMIT 200`},
		{"Recent URLs", `SELECT full_url FROM urls ORDER BY created_at DESC LIMIT 200`},
		{"Open Ports", `SELECT ips.address || ':' || ports.port || '/' || ports.protocol FROM ports JOIN ips ON ips.id = ports.ip_id ORDER BY ips.address, ports.port LIMIT 200`},
		{"Findings", `SELECT COALESCE(severity, 'info') || ' - ' || title FROM findings ORDER BY severity, title LIMIT 200`},
	}
	var b strings.Builder
	b.WriteString("# Paravizor Recon Report\n\n")
	if projCfg.Name != "" {
		b.WriteString("Project: " + projCfg.Name + "\n\n")
	}
	for _, section := range sections {
		b.WriteString("## " + section.title + "\n\n")
		values := queryLines(context.Background(), database, section.query)
		if len(values) == 0 {
			b.WriteString("_None._\n\n")
			continue
		}
		for _, value := range values {
			b.WriteString("- " + value + "\n")
		}
		b.WriteString("\n")
	}
	if err := os.WriteFile(outPath, []byte(b.String()), 0o644); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Exported report to %s\n", outPath)
	return nil
}

func openProjectDB(dir string) (string, *sql.DB, func(), error) {
	projectDir := dir
	if projectDir == "" {
		projectDir = "."
	}
	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		return "", nil, nil, err
	}
	database, err := sql.Open("sqlite", project.DBPath(absDir))
	if err != nil {
		return "", nil, nil, err
	}
	return absDir, database, func() { _ = database.Close() }, nil
}

func writeQueryLines(ctx context.Context, database *sql.DB, path, query string) error {
	values := queryLines(ctx, database, query)
	return os.WriteFile(path, []byte(strings.Join(values, "\n")+"\n"), 0o644)
}

func queryLines(ctx context.Context, database *sql.DB, query string) []string {
	rows, err := database.QueryContext(ctx, query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var values []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err == nil && strings.TrimSpace(value) != "" {
			values = append(values, value)
		}
	}
	return values
}
