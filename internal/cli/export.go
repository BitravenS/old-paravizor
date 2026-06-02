package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bitravens/paravizor/v1/internal/store/db"
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
		Long: `Export collected data as plaintext files organized by type.

Outputs:
  subdomains.txt         All discovered subdomains
  live-subdomains.txt    Live subdomains only
  urls.txt               All discovered URLs
  live-urls.txt          URLs with HTTP 200/206 response
  findings/              Findings grouped by severity`,
		RunE: func(cmd *cobra.Command, args []string) error {
			outputPath, err := exportArtifacts(cmd.Context(), dir, outputDir)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Exported artifacts to %s\n", outputPath)
			return nil
		},
	}
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "Project directory to export data from")
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
			return fmt.Errorf("obsidian canvas export is not implemented yet")
		},
	}
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "Project directory to export data from")
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
			outputPath, err := exportReport(cmd.Context(), dir, outputFile)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Exported report to %s\n", outputPath)
			return nil
		},
	}
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "Project directory to export data from")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "exports/report.md", "File path to save the exported report (supports .md or .html)")
	return cmd
}

func exportArtifacts(ctx context.Context, dir, outputDir string) (string, error) {
	projectDir, st, err := openProjectStore(ctx, dir)
	if err != nil {
		return "", err
	}
	defer st.Close()

	outputPath := resolveExportPath(projectDir, outputDir)
	if err := os.MkdirAll(outputPath, 0o755); err != nil {
		return "", fmt.Errorf("create export directory %q: %w", outputPath, err)
	}

	domains, err := st.GetDomains(ctx, false, 100000, 0)
	if err != nil {
		return "", fmt.Errorf("load domains: %w", err)
	}
	if err := writeLines(filepath.Join(outputPath, "subdomains.txt"), domainNames(domains, false)); err != nil {
		return "", err
	}
	if err := writeLines(filepath.Join(outputPath, "live-subdomains.txt"), domainNames(domains, true)); err != nil {
		return "", err
	}

	urls, err := st.GetURLs(ctx, 100000, 0)
	if err != nil {
		return "", fmt.Errorf("load urls: %w", err)
	}
	if err := writeLines(filepath.Join(outputPath, "urls.txt"), urlValues(urls, false)); err != nil {
		return "", err
	}
	if err := writeLines(filepath.Join(outputPath, "live-urls.txt"), urlValues(urls, true)); err != nil {
		return "", err
	}

	findings, err := st.GetFindings(ctx, 100000, 0)
	if err != nil {
		return "", fmt.Errorf("load findings: %w", err)
	}
	if err := writeFindingFiles(filepath.Join(outputPath, "findings"), findings); err != nil {
		return "", err
	}

	return outputPath, nil
}

func exportReport(ctx context.Context, dir, outputFile string) (string, error) {
	projectDir, st, err := openProjectStore(ctx, dir)
	if err != nil {
		return "", err
	}
	defer st.Close()

	outputPath := resolveExportPath(projectDir, outputFile)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return "", fmt.Errorf("create report directory %q: %w", filepath.Dir(outputPath), err)
	}

	domains, err := st.CountDomains(ctx)
	if err != nil {
		return "", fmt.Errorf("count domains: %w", err)
	}
	urls, err := st.CountURLs(ctx)
	if err != nil {
		return "", fmt.Errorf("count urls: %w", err)
	}
	findings, err := st.CountFindings(ctx)
	if err != nil {
		return "", fmt.Errorf("count findings: %w", err)
	}

	content := renderReport(outputPath, domains, urls, findings)
	if err := os.WriteFile(outputPath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write report %q: %w", outputPath, err)
	}
	return outputPath, nil
}

func resolveExportPath(projectDir, outputPath string) string {
	if filepath.IsAbs(outputPath) {
		return outputPath
	}
	return filepath.Join(projectDir, outputPath)
}

func domainNames(domains []db.Domain, liveOnly bool) []string {
	lines := make([]string, 0, len(domains))
	for _, d := range domains {
		if liveOnly && (d.IsLive == nil || !*d.IsLive) {
			continue
		}
		lines = append(lines, d.Name)
	}
	sort.Strings(lines)
	return lines
}

func urlValues(urls []db.Url, liveOnly bool) []string {
	lines := make([]string, 0, len(urls))
	for _, u := range urls {
		if liveOnly && (u.StatusCode == nil || (*u.StatusCode != 200 && *u.StatusCode != 206)) {
			continue
		}
		lines = append(lines, u.FullUrl)
	}
	sort.Strings(lines)
	return lines
}

func writeLines(path string, lines []string) error {
	data := strings.Join(lines, "\n")
	if data != "" {
		data += "\n"
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		return fmt.Errorf("write %q: %w", path, err)
	}
	return nil
}

func writeFindingFiles(dir string, findings []db.Finding) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create findings directory %q: %w", dir, err)
	}

	bySeverity := make(map[string][]string)
	for _, finding := range findings {
		severity := "info"
		if finding.Severity != nil && *finding.Severity != "" {
			severity = *finding.Severity
		}
		bySeverity[severity] = append(bySeverity[severity], formatFinding(finding))
	}

	for _, severity := range []string{"info", "low", "medium", "high", "critical"} {
		lines := bySeverity[severity]
		sort.Strings(lines)
		if err := writeLines(filepath.Join(dir, severity+".txt"), lines); err != nil {
			return err
		}
	}
	return nil
}

func formatFinding(f db.Finding) string {
	parts := []string{f.Scanner, f.Title}
	if f.Description != nil && *f.Description != "" {
		parts = append(parts, *f.Description)
	}
	return strings.Join(parts, " | ")
}

func renderReport(outputPath string, domains, urls, findings int64) string {
	if strings.EqualFold(filepath.Ext(outputPath), ".html") {
		return fmt.Sprintf(`<!doctype html>
<html>
<head><meta charset="utf-8"><title>Paravizor Report</title></head>
<body>
<h1>Paravizor Report</h1>
<ul>
<li>Domains: %d</li>
<li>URLs: %d</li>
<li>Findings: %d</li>
</ul>
</body>
</html>
`, domains, urls, findings)
	}

	return fmt.Sprintf(`# Paravizor Report

| Metric | Count |
| --- | ---: |
| Domains | %d |
| URLs | %d |
| Findings | %d |
`, domains, urls, findings)
}
