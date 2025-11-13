// Package kdbx provides a database wrapper for PostgreSQL and MySQL with
// connection pooling, transaction management, health checks, and observability.
package kdbx

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Driver represents the database driver type.
type Driver string

const (
	// DriverPostgres represents PostgreSQL database driver.
	DriverPostgres Driver = "postgres"
	// DriverMySQL represents MySQL database driver.
	DriverMySQL Driver = "mysql"
)

// Database is the common interface for database operations.
// Both PostgreSQL and MySQL implementations satisfy this interface.
type Database interface {
	// Query executes a query that returns rows.
	Query(ctx context.Context, query string, args ...interface{}) (Rows, error)

	// QueryRow executes a query that is expected to return at most one row.
	QueryRow(ctx context.Context, query string, args ...interface{}) Row

	// Exec executes a query that doesn't return rows (INSERT, UPDATE, DELETE).
	Exec(ctx context.Context, query string, args ...interface{}) (Result, error)

	// Begin starts a new transaction.
	Begin(ctx context.Context) (Tx, error)

	// WithTransaction executes a function within a transaction with automatic
	// rollback on error and retry logic for transient failures.
	WithTransaction(ctx context.Context, fn func(tx Tx) error) error

	// Health checks if the database is reachable (liveness check).
	Health(ctx context.Context) error

	// HealthDetailed performs a more thorough health check including a test query (readiness check).
	HealthDetailed(ctx context.Context) error

	// Stats returns connection pool statistics.
	Stats() PoolStats

	// Close closes the database connection pool.
	// Should only be called during graceful shutdown.
	Close() error

	// Shutdown gracefully closes the database with a timeout.
	Shutdown(ctx context.Context) error

	// Driver returns the database driver type.
	Driver() Driver
}

// Tx represents a database transaction.
type Tx interface {
	// Query executes a query within the transaction.
	Query(ctx context.Context, query string, args ...interface{}) (Rows, error)

	// QueryRow executes a query that returns at most one row within the transaction.
	QueryRow(ctx context.Context, query string, args ...interface{}) Row

	// Exec executes a query within the transaction.
	Exec(ctx context.Context, query string, args ...interface{}) (Result, error)

	// Commit commits the transaction.
	Commit(ctx context.Context) error

	// Rollback rolls back the transaction.
	Rollback(ctx context.Context) error
}

// Rows represents the result of a query that returns multiple rows.
type Rows interface {
	// Next prepares the next row for scanning.
	Next() bool

	// Scan copies the columns from the current row into the values pointed at by dest.
	Scan(dest ...interface{}) error

	// Close closes the rows iterator.
	Close() error

	// Err returns any error that occurred during iteration.
	Err() error
}

// Row represents the result of a query that returns at most one row.
type Row interface {
	// Scan copies the columns from the row into the values pointed at by dest.
	Scan(dest ...interface{}) error
}

// Result represents the result of an Exec operation.
type Result interface {
	// LastInsertId returns the ID of the last inserted row (MySQL).
	// Returns 0 for PostgreSQL.
	LastInsertId() (int64, error)

	// RowsAffected returns the number of rows affected by the query.
	RowsAffected() (int64, error)
}

// PoolStats represents connection pool statistics.
type PoolStats struct {
	// AcquiredConns is the number of currently acquired connections.
	AcquiredConns int32

	// IdleConns is the number of idle connections in the pool.
	IdleConns int32

	// TotalConns is the total number of connections in the pool.
	TotalConns int32

	// MaxConns is the maximum number of connections allowed in the pool.
	MaxConns int32

	// NewConnsCount is the cumulative count of successful new connections opened.
	NewConnsCount int64

	// MaxLifetimeDestroyCount is the cumulative count of connections destroyed
	// because they exceeded MaxConnLifetime.
	MaxLifetimeDestroyCount int64

	// MaxIdleDestroyCount is the cumulative count of connections destroyed
	// because they exceeded MaxConnIdleTime.
	MaxIdleDestroyCount int64
}

// MetricsCollector is an interface for collecting database metrics.
type MetricsCollector interface {
	// RecordQuery records metrics for a query operation.
	RecordQuery(ctx context.Context, query string, duration time.Duration, err error)

	// RecordExec records metrics for an exec operation.
	RecordExec(ctx context.Context, query string, duration time.Duration, err error)

	// RecordTransaction records metrics for a transaction.
	RecordTransaction(ctx context.Context, duration time.Duration, committed bool, err error)

	// RecordPoolStats records connection pool statistics.
	RecordPoolStats(stats PoolStats)
}

// pgxPoolAdapter adapts pgxpool.Pool to the Database interface.
type pgxPoolAdapter struct {
	pool *pgxpool.Pool
}

// pgxTxAdapter adapts pgx.Tx to the Tx interface.
type pgxTxAdapter struct {
	tx     pgx.Tx
	logger *slog.Logger
	config *Config
}

// pgxRowsAdapter adapts pgx.Rows to the Rows interface.
type pgxRowsAdapter struct {
	rows pgx.Rows
}

// pgxRowAdapter adapts pgx.Row to the Row interface.
type pgxRowAdapter struct {
	row pgx.Row
}

// sqlDBAdapter adapts *sql.DB to the Database interface.
type sqlDBAdapter struct {
	db *sql.DB
}

// sqlTxAdapter adapts *sql.Tx to the Tx interface.
type sqlTxAdapter struct {
	tx     *sql.Tx
	logger *slog.Logger
	config *Config
}

// sqlRowsAdapter adapts *sql.Rows to the Rows interface.
type sqlRowsAdapter struct {
	rows *sql.Rows
}

// sqlRowAdapter adapts *sql.Row to the Row interface.
type sqlRowAdapter struct {
	row *sql.Row
}

// sqlResultAdapter adapts sql.Result to the Result interface.
type sqlResultAdapter struct {
	result sql.Result
}

// pgxCommandTagAdapter adapts pgconn.CommandTag to the Result interface.
type pgxCommandTagAdapter struct {
	tag pgx.CommandTag
}

// Ensure interfaces are implemented at compile time.
var (
	_ Rows   = (*pgxRowsAdapter)(nil)
	_ Row    = (*pgxRowAdapter)(nil)
	_ Result = (*pgxCommandTagAdapter)(nil)
	_ Result = (*sqlResultAdapter)(nil)
	_ Rows   = (*sqlRowsAdapter)(nil)
	_ Row    = (*sqlRowAdapter)(nil)
)
