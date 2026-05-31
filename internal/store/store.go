package store

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"github.com/bitravens/paravizor/v1/internal/store/db"
	_ "modernc.org/sqlite"
)

// Store provides access to the project database.
// Writes are serialized through a single connection; reads use a pool.
// All statements are pre-compiled at Open time for maximum throughput.
type Store struct {
	readDB  *sql.DB
	writeDB *sql.DB
	writeMu sync.Mutex

	// Pre-compiled query sets bound to their respective connections.
	// rq is used for reads (any connection in the pool is fine).
	// wq is used inside write transactions via WithTx.
	rq *db.Queries
	wq *db.Queries
}

// Open creates a new Store, applies all pragmas, and pre-compiles all statements.
func Open(ctx context.Context, dbPath string, cfg DBConfig) (*Store, error) {
	busyTimeout := cfg.BusyTimeout
	if busyTimeout <= 0 {
		busyTimeout = 5000
	}
	mmap_size := cfg.MMapSize
	if mmap_size <= 0 {
		mmap_size = 268435456 // 256 MB
	}
	cache_size := cfg.CacheSize
	if cache_size <= 0 {
		cache_size = 8000 // 8 MB per connection page cache
	}

	pragma := fmt.Sprintf(
		"?_pragma=journal_mode(WAL)"+
			"&_pragma=busy_timeout(%d)"+
			"&_pragma=foreign_keys(ON)"+
			"&_pragma=synchronous(NORMAL)"+
			"&_pragma=cache_size(-%d)"+
			"&_pragma=temp_store(MEMORY)"+
			"&_pragma=mmap_size(%d)",
		busyTimeout,
		cache_size,
		mmap_size,
	)

	writeDB, err := sql.Open("sqlite", dbPath+pragma)
	if err != nil {
		return nil, fmt.Errorf("open write db: %w", err)
	}
	writeDB.SetMaxOpenConns(1)
	writeDB.SetMaxIdleConns(1)

	readDB, err := sql.Open("sqlite", dbPath+pragma)
	if err != nil {
		writeDB.Close()
		return nil, fmt.Errorf("open read db: %w", err)
	}
	readDB.SetMaxOpenConns(4)
	readDB.SetMaxIdleConns(4)

	// Ensure schema exists before preparing statements.
	if err := Migrate(ctx, writeDB); err != nil {
		writeDB.Close()
		readDB.Close()
		return nil, fmt.Errorf("run migrations before prepare: %w", err)
	}

	// Pre-compile all statements on both connections.
	wq, err := db.Prepare(ctx, writeDB)
	if err != nil {
		writeDB.Close()
		readDB.Close()
		return nil, fmt.Errorf("prepare write statements: %w", err)
	}

	rq, err := db.Prepare(ctx, readDB)
	if err != nil {
		wq.Close()
		writeDB.Close()
		readDB.Close()
		return nil, fmt.Errorf("prepare read statements: %w", err)
	}

	return &Store{
		readDB:  readDB,
		writeDB: writeDB,
		rq:      rq,
		wq:      wq,
	}, nil
}

func (s *Store) Init(ctx context.Context) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return Migrate(ctx, s.writeDB)
}

// Close closes all prepared statements and both connections.
func (s *Store) Close() error {
	var errs []error
	if err := s.rq.Close(); err != nil {
		errs = append(errs, fmt.Errorf("close read queries: %w", err))
	}
	if err := s.wq.Close(); err != nil {
		errs = append(errs, fmt.Errorf("close write queries: %w", err))
	}
	if err := s.readDB.Close(); err != nil {
		errs = append(errs, fmt.Errorf("close read db: %w", err))
	}
	if err := s.writeDB.Close(); err != nil {
		errs = append(errs, fmt.Errorf("close write db: %w", err))
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// WriteTx executes fn inside a serialized write transaction.
// The *db.Queries passed into fn is bound to the transaction and reuses
// the pre-compiled statements.
func (s *Store) WriteTx(ctx context.Context, fn func(q *db.Queries) error) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	tx, err := s.writeDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if err := fn(s.wq.WithTx(tx)); err != nil {
		return err
	}

	return tx.Commit()
}

// Q returns the read-only pre-compiled query set.
func (s *Store) Q() *db.Queries {
	return s.rq
}

// DB returns the raw read pool for cases that need *sql.DB directly.
func (s *Store) DB() *sql.DB {
	return s.readDB
}
