package cli

import (
	"database/sql"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func newQueryCmd() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "query",
		Args:  cobra.MaximumNArgs(1),
		Short: "Query the project database",
		Long:  "Run a read-only SQL query against the project's SQLite database.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("SQL query is required")
			}
			return runReadOnlyQuery(cmd, dir, args[0])
		},
	}
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "Project directory to query")
	return cmd
}

func runReadOnlyQuery(cmd *cobra.Command, dir string, sqlText string) error {
	query := strings.TrimSpace(sqlText)
	if !isReadOnlyQuery(query) {
		return fmt.Errorf("only read-only SELECT, WITH, and PRAGMA queries are supported")
	}

	_, st, err := openProjectStore(cmd.Context(), dir)
	if err != nil {
		return err
	}
	defer st.Close()

	rows, err := st.DB().QueryContext(cmd.Context(), query)
	if err != nil {
		return fmt.Errorf("run query: %w", err)
	}
	defer rows.Close()

	return printRows(cmd, rows)
}

func isReadOnlyQuery(query string) bool {
	lower := strings.ToLower(strings.TrimSpace(query))
	return strings.HasPrefix(lower, "select ") ||
		strings.HasPrefix(lower, "select\n") ||
		strings.HasPrefix(lower, "with ") ||
		strings.HasPrefix(lower, "with\n") ||
		strings.HasPrefix(lower, "pragma ")
}

func printRows(cmd *cobra.Command, rows *sql.Rows) error {
	cols, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("read columns: %w", err)
	}

	out := cmd.OutOrStdout()
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(cols, "\t"))

	values := make([]any, len(cols))
	scanTargets := make([]any, len(cols))
	for i := range values {
		scanTargets[i] = &values[i]
	}

	for rows.Next() {
		if err := rows.Scan(scanTargets...); err != nil {
			return fmt.Errorf("scan row: %w", err)
		}
		rendered := make([]string, len(cols))
		for i, value := range values {
			rendered[i] = formatSQLValue(value)
		}
		fmt.Fprintln(tw, strings.Join(rendered, "\t"))
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read rows: %w", err)
	}
	return tw.Flush()
}

func formatSQLValue(value any) string {
	switch v := value.(type) {
	case nil:
		return "NULL"
	case []byte:
		return string(v)
	default:
		return fmt.Sprint(v)
	}
}
