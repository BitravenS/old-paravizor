package db

import "context"

// LastInsertRowID returns SQLite's last_insert_rowid() using the same
// connection or transaction as the query set.
func (q *Queries) LastInsertRowID(ctx context.Context) (int64, error) {
	row := q.queryRow(ctx, nil, `SELECT last_insert_rowid()`)
	var id int64
	err := row.Scan(&id)
	return id, err
}
