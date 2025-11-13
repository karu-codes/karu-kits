package dbx

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PgxQuerier is the interface for pgx-based sqlc queries
// This is compatible with sqlc when using sql_package: "pgx/v5"
type PgxQuerier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Ensure pgxpool.Pool implements PgxQuerier
var _ PgxQuerier = (*pgxpool.Pool)(nil)
var _ PgxQuerier = (pgx.Tx)(nil)

// PgxPool returns the underlying pgxpool.Pool for use with pgx-native sqlc
// Returns nil if pgxpool is not enabled
func (d *DB) PgxQuerier() PgxQuerier {
	if d.pgxPool != nil {
		return &pgxQuerier{pool: d.pgxPool}
	}
	return nil
}

// pgxQuerier wraps pgxpool.Pool with error handling and observability
type pgxQuerier struct {
	pool *pgxpool.Pool
}

func (q *pgxQuerier) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	tag, err := q.pool.Exec(ctx, sql, arguments...)
	if err != nil {
		return tag, wrapDBError(err, "pgx exec")
	}
	return tag, nil
}

func (q *pgxQuerier) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	rows, err := q.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, wrapDBError(err, "pgx query")
	}
	return rows, nil
}

func (q *pgxQuerier) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return q.pool.QueryRow(ctx, sql, args...)
}

// PgxTxFunc is a function that runs within a pgx transaction
type PgxTxFunc func(ctx context.Context, tx pgx.Tx) error

// WithPgxTx executes a function within a pgx transaction with retry support
func (d *DB) WithPgxTx(ctx context.Context, fn PgxTxFunc, opts *TxOptions) error {
	if d.pgxPool == nil {
		return newDBError("pgxpool is not enabled")
	}

	return retryForTransaction(ctx, d.cfg, func() error {
		// Convert TxOptions to pgx.TxOptions
		var pgxOpts pgx.TxOptions
		if opts != nil && opts.Options != nil {
			pgxOpts.IsoLevel = convertIsolationLevel(int(opts.Options.Isolation))
			pgxOpts.AccessMode = convertAccessMode(opts.Options.ReadOnly)
		}

		tx, err := d.pgxPool.BeginTx(ctx, pgxOpts)
		if err != nil {
			return wrapDBError(err, "begin pgx transaction")
		}

		// Execute function
		if err := fn(ctx, tx); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}

		// Commit transaction
		if err := tx.Commit(ctx); err != nil {
			return wrapDBError(err, "commit pgx transaction")
		}

		return nil
	})
}

// ObservablePgxQuerier wraps PgxQuerier with observability
type ObservablePgxQuerier struct {
	querier PgxQuerier
	hooks   *pgxHooks
}

type pgxHooks struct {
	beforeExec  func(ctx context.Context, sql string, args []any)
	afterExec   func(ctx context.Context, sql string, args []any, err error)
	beforeQuery func(ctx context.Context, sql string, args []any)
	afterQuery  func(ctx context.Context, sql string, args []any, err error)
}

// WithPgxObservability wraps a PgxQuerier with observability
func WithPgxObservability(querier PgxQuerier, logger Logger) *ObservablePgxQuerier {
	hooks := &pgxHooks{
		beforeExec: func(ctx context.Context, sql string, args []any) {
			if logger != nil {
				logger.Printf("[dbx-pgx] exec start: %s", sql)
			}
		},
		afterExec: func(ctx context.Context, sql string, args []any, err error) {
			if logger != nil {
				if err != nil {
					logger.Printf("[dbx-pgx] exec error: %s (error: %v)", sql, err)
				} else {
					logger.Printf("[dbx-pgx] exec success: %s", sql)
				}
			}
		},
		beforeQuery: func(ctx context.Context, sql string, args []any) {
			if logger != nil {
				logger.Printf("[dbx-pgx] query start: %s", sql)
			}
		},
		afterQuery: func(ctx context.Context, sql string, args []any, err error) {
			if logger != nil {
				if err != nil {
					logger.Printf("[dbx-pgx] query error: %s (error: %v)", sql, err)
				} else {
					logger.Printf("[dbx-pgx] query success: %s", sql)
				}
			}
		},
	}

	return &ObservablePgxQuerier{
		querier: querier,
		hooks:   hooks,
	}
}

func (o *ObservablePgxQuerier) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	if o.hooks.beforeExec != nil {
		o.hooks.beforeExec(ctx, sql, arguments)
	}

	tag, err := o.querier.Exec(ctx, sql, arguments...)

	if o.hooks.afterExec != nil {
		o.hooks.afterExec(ctx, sql, arguments, err)
	}

	return tag, err
}

func (o *ObservablePgxQuerier) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if o.hooks.beforeQuery != nil {
		o.hooks.beforeQuery(ctx, sql, args)
	}

	rows, err := o.querier.Query(ctx, sql, args...)

	if o.hooks.afterQuery != nil {
		o.hooks.afterQuery(ctx, sql, args, err)
	}

	return rows, err
}

func (o *ObservablePgxQuerier) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if o.hooks.beforeQuery != nil {
		o.hooks.beforeQuery(ctx, sql, args)
	}

	row := o.querier.QueryRow(ctx, sql, args...)

	// Note: QueryRow doesn't return error directly
	if o.hooks.afterQuery != nil {
		o.hooks.afterQuery(ctx, sql, args, nil)
	}

	return row
}

// Helper functions to convert sql.TxOptions to pgx.TxOptions
func convertIsolationLevel(level int) pgx.TxIsoLevel {
	// database/sql isolation levels don't map directly to strings,
	// but we can use the constants
	switch level {
	case 0: // sql.LevelDefault
		return pgx.ReadCommitted
	case 1: // sql.LevelReadUncommitted
		return pgx.ReadUncommitted
	case 2: // sql.LevelReadCommitted
		return pgx.ReadCommitted
	case 4: // sql.LevelRepeatableRead
		return pgx.RepeatableRead
	case 8: // sql.LevelSerializable
		return pgx.Serializable
	default:
		return pgx.ReadCommitted
	}
}

func convertAccessMode(readOnly bool) pgx.TxAccessMode {
	if readOnly {
		return pgx.ReadOnly
	}
	return pgx.ReadWrite
}

// ---------- Usage Examples ----------
//
// Example 1: Using pgxpool with sqlc (pgx native mode)
//
//  // sqlc.yaml configuration:
//  // version: "2"
//  // sql:
//  //   - schema: "schema.sql"
//  //     queries: "queries.sql"
//  //     engine: "postgresql"
//  //     gen:
//  //       go:
//  //         package: "db"
//  //         out: "internal/db"
//  //         sql_package: "pgx/v5"  # <-- Use pgx native
//  //         emit_interface: true
//
//  ctx := context.Background()
//
//  // Open database with pgxpool enabled
//  db, err := dbx.Open(ctx,
//      dbx.WithDriver(dbx.DriverPostgres),
//      dbx.WithPGHostPort("localhost", 5432),
//      dbx.WithPGAuth("postgres", "password"),
//      dbx.WithPGDB("mydb"),
//      dbx.WithPgxPool(true),  // Enable pgxpool
//      dbx.WithPgxPoolSize(10, 50),
//  )
//
//  // Get pgx querier for sqlc
//  querier := db.PgxQuerier()
//  queries := sqlcgen.New(querier)  // sqlc-generated New() function
//
//  // Use sqlc methods
//  user, err := queries.GetUser(ctx, 1)
//  users, err := queries.ListUsers(ctx)
//
// Example 2: Transactions with pgx
//
//  err = db.WithPgxTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
//      queries := sqlcgen.New(tx)  // tx also implements PgxQuerier
//      return queries.CreateUser(ctx, sqlcgen.CreateUserParams{
//          Name:  "John",
//          Email: "john@example.com",
//      })
//  }, nil)
//
// Example 3: With observability
//
//  querier := db.PgxQuerier()
//  observableQuerier := dbx.WithPgxObservability(querier, logger)
//  queries := sqlcgen.New(observableQuerier)
//
// Example 4: Performance comparison
//
//  // database/sql mode (via pgx stdlib)
//  queries1 := sqlcgen.New(db.StdDB())  // Uses database/sql
//
//  // pgx native mode (direct pgxpool)
//  queries2 := sqlcgen.New(db.PgxQuerier())  // Uses pgxpool directly
//  // ^ This is faster for high-concurrency workloads
//
