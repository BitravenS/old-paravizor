package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
)

//go:embed schema.sql
var schema string

// Migrate runs the database schema against the given connection.
// Uses IF NOT EXISTS so it is safe to call on an existing database.
func Migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}
