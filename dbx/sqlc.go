package dbx

import (
	"context"
	"database/sql"
)

// DBTX is the interface used by sqlc-generated code
// It's compatible with both *sql.DB and *sql.Tx
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

// Ensure DB implements DBTX interface
var _ DBTX = (*DB)(nil)
var _ DBTX = (*sql.DB)(nil)
var _ DBTX = (*sql.Tx)(nil)

// PrepareContext prepares a statement for execution
func (d *DB) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	stmt, err := d.std.PrepareContext(ctx, query)
	if err != nil {
		return nil, wrapDBError(err, "prepare statement")
	}
	return stmt, nil
}

// Querier is a convenience interface for working with sqlc
// Use this when you want to pass either DB or Tx to your repository
type Querier interface {
	DBTX
	// Add any additional methods your repositories might need
}

// Ensure DB and Tx implement Querier
var _ Querier = (*DB)(nil)
var _ Querier = (*sql.Tx)(nil)

// WithReadReplica returns a DBTX that uses the read replica if available
// Falls back to primary if no replica is configured
func (d *DB) WithReadReplica() DBTX {
	if d.roStd != nil {
		return &dbWrapper{db: d.roStd}
	}
	return d
}

// dbWrapper wraps *sql.DB to implement DBTX with error wrapping
type dbWrapper struct {
	db *sql.DB
}

func (w *dbWrapper) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	result, err := w.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, wrapDBError(err, "exec query")
	}
	return result, nil
}

func (w *dbWrapper) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	stmt, err := w.db.PrepareContext(ctx, query)
	if err != nil {
		return nil, wrapDBError(err, "prepare statement")
	}
	return stmt, nil
}

func (w *dbWrapper) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	rows, err := w.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, wrapDBError(err, "query")
	}
	return rows, nil
}

func (w *dbWrapper) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return w.db.QueryRowContext(ctx, query, args...)
}

// txWrapper wraps *sql.Tx to add error wrapping
type txWrapper struct {
	tx *sql.Tx
}

func (w *txWrapper) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	result, err := w.tx.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, wrapDBError(err, "exec query in transaction")
	}
	return result, nil
}

func (w *txWrapper) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	stmt, err := w.tx.PrepareContext(ctx, query)
	if err != nil {
		return nil, wrapDBError(err, "prepare statement in transaction")
	}
	return stmt, nil
}

func (w *txWrapper) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	rows, err := w.tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, wrapDBError(err, "query in transaction")
	}
	return rows, nil
}

func (w *txWrapper) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return w.tx.QueryRowContext(ctx, query, args...)
}

// ---------- Usage Examples ----------
//
// Example 1: Using with sqlc-generated Queries
//
//  // Your sqlc-generated code will have something like:
//  type Queries struct {
//      db DBTX
//  }
//
//  func New(db DBTX) *Queries {
//      return &Queries{db: db}
//  }
//
//  // Usage with dbx:
//  db, err := dbx.Open(ctx, ...)
//  queries := sqlcgen.New(db)  // db implements DBTX
//
//  // Usage in transaction:
//  err = db.WithTx(ctx, func(ctx context.Context, tx *sql.Tx) error {
//      queries := sqlcgen.New(tx)  // tx also implements DBTX
//      return queries.CreateUser(ctx, ...)
//  }, nil)
//
// Example 2: Using read replicas with sqlc
//
//  // For read-heavy operations, use read replica
//  queries := sqlcgen.New(db.WithReadReplica())
//  users, err := queries.ListUsers(ctx)
//
// Example 3: Repository pattern
//
//  type UserRepository struct {
//      queries *sqlcgen.Queries
//  }
//
//  func NewUserRepository(db dbx.Querier) *UserRepository {
//      return &UserRepository{
//          queries: sqlcgen.New(db),
//      }
//  }
//
//  // Can be used with both DB and Tx
//  repo := NewUserRepository(db)
//  repo := NewUserRepository(tx)
//
