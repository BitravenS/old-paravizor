package cli

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	_ "modernc.org/sqlite"

	"github.com/bitravens/paravizor/v1/internal/project"
)

func newQueryCmd() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "query <sql>",
		Args:  cobra.ExactArgs(1),
		Short: "Query the project database",
		Long:  "Run a read-only SQL query against the project's SQLite database.",
		RunE: func(cmd *cobra.Command, args []string) error {
			projectDir := dir
			if projectDir == "" {
				projectDir = "."
			}
			absDir, err := filepath.Abs(projectDir)
			if err != nil {
				return err
			}
			query := strings.TrimSpace(args[0])
			if !strings.HasPrefix(strings.ToLower(query), "select") {
				return fmt.Errorf("only SELECT queries are allowed from the CLI query command")
			}
			database, err := sql.Open("sqlite", project.DBPath(absDir))
			if err != nil {
				return err
			}
			defer database.Close()
			rows, err := database.QueryContext(context.Background(), query)
			if err != nil {
				return err
			}
			defer rows.Close()
			cols, err := rows.Columns()
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), strings.Join(cols, "\t"))
			values := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range values {
				ptrs[i] = &values[i]
			}
			for rows.Next() {
				if err := rows.Scan(ptrs...); err != nil {
					return err
				}
				parts := make([]string, len(values))
				for i, value := range values {
					if value == nil {
						parts[i] = "NULL"
					} else if b, ok := value.([]byte); ok {
						parts[i] = string(b)
					} else {
						parts[i] = fmt.Sprint(value)
					}
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), strings.Join(parts, "\t"))
			}
			return rows.Err()
		},
	}
	cmd.Flags().StringVarP(&dir, "dir", "d", "", "Project directory to query")
	return cmd
}
