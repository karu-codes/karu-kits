package kdbx

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// PostgresDB wraps a PostgreSQL connection pool and provides database operations.
type PostgresDB struct {
	// Primary implementation using pgxpool for performance
	pool *pgxpool.Pool

	// Optional database/sql compatibility layer
	stdDB *sql.DB

	config  *Config
	logger  *slog.Logger
	metrics MetricsCollector

	// Health check management
	healthTicker *time.Ticker
	healthCancel context.CancelFunc
	healthMu     sync.RWMutex
	lastHealth   error
	lastHealthAt time.Time

	closeOnce sync.Once
}

// NewPostgres creates a new PostgreSQL database connection using pgxpool.
// This is the recommended way for PostgreSQL-only applications due to better performance
// and PostgreSQL-specific features.
func NewPostgres(ctx context.Context, config *Config) (*PostgresDB, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	if config.Driver != DriverPostgres {
		return nil, ErrInvalidDriver
	}

	// Parse connection config
	poolConfig, err := pgxpool.ParseConfig(config.DatabaseURL)
	if err != nil {
		return nil, WrapError(err, "failed to parse PostgreSQL connection URL")
	}

	// Apply configuration
	poolConfig.MaxConns = int32(config.MaxOpenConns)
	poolConfig.MinConns = int32(config.MaxIdleConns)
	poolConfig.MaxConnLifetime = config.ConnMaxLifetime
	poolConfig.MaxConnIdleTime = config.ConnMaxIdleTime
	poolConfig.HealthCheckPeriod = config.HealthCheckInterval

	// Configure connection timeout
	if config.ConnectTimeout > 0 {
		poolConfig.ConnConfig.ConnectTimeout = config.ConnectTimeout
	}

	// Configure simple protocol if requested
	if config.PostgresPreferSimpleProtocol {
		poolConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	}

	// Configure query logging
	if config.LogQueries && config.Logger != nil {
		poolConfig.ConnConfig.Tracer = &queryTracer{
			logger: config.Logger,
		}
	}

	// Create connection pool with timeout
	connectCtx := ctx
	if config.ConnectTimeout > 0 {
		var cancel context.CancelFunc
		connectCtx, cancel = context.WithTimeout(ctx, config.ConnectTimeout)
		defer cancel()
	}

	pool, err := pgxpool.NewWithConfig(connectCtx, poolConfig)
	if err != nil {
		return nil, WrapError(err, "failed to create PostgreSQL connection pool")
	}

	// Verify connection
	if err := pool.Ping(connectCtx); err != nil {
		pool.Close()
		return nil, WrapError(err, "failed to ping PostgreSQL database")
	}

	db := &PostgresDB{
		pool:    pool,
		config:  config,
		logger:  config.Logger,
		metrics: config.Metrics,
	}

	// Start background health checks if configured
	if config.HealthCheckInterval > 0 {
		db.startHealthChecks()
	}

	if db.logger != nil {
		db.logger.Info("PostgreSQL connection established",
			slog.String("url", config.MaskedURL()),
			slog.Int("max_conns", config.MaxOpenConns),
			slog.Int("min_conns", config.MaxIdleConns),
		)
	}

	return db, nil
}

// NewPostgresStd creates a new PostgreSQL database connection using database/sql.
// This is useful when you need compatibility with libraries that require *sql.DB.
func NewPostgresStd(ctx context.Context, config *Config) (*PostgresDB, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	if config.Driver != DriverPostgres {
		return nil, ErrInvalidDriver
	}

	// Build connection string with options
	connStr := config.DatabaseURL

	// Register pgx as the driver
	db, err := sql.Open("pgx/v5", connStr)
	if err != nil {
		return nil, WrapError(err, "failed to open PostgreSQL connection")
	}

	// Apply connection pool settings
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(config.ConnMaxIdleTime)

	// Verify connection with timeout
	pingCtx := ctx
	if config.ConnectTimeout > 0 {
		var cancel context.CancelFunc
		pingCtx, cancel = context.WithTimeout(ctx, config.ConnectTimeout)
		defer cancel()
	}

	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return nil, WrapError(err, "failed to ping PostgreSQL database")
	}

	pgdb := &PostgresDB{
		stdDB:   db,
		config:  config,
		logger:  config.Logger,
		metrics: config.Metrics,
	}

	// Start background health checks if configured
	if config.HealthCheckInterval > 0 {
		pgdb.startHealthChecks()
	}

	if pgdb.logger != nil {
		pgdb.logger.Info("PostgreSQL connection established (database/sql mode)",
			slog.String("url", config.MaskedURL()),
			slog.Int("max_conns", config.MaxOpenConns),
		)
	}

	return pgdb, nil
}

// Query executes a query that returns rows.
func (db *PostgresDB) Query(ctx context.Context, query string, args ...interface{}) (Rows, error) {
	start := time.Now()

	if db.config.LogQueries && db.logger != nil {
		db.logger.Debug("executing query",
			slog.String("query", SanitizeQuery(query)),
		)
	}

	var rows Rows
	var err error

	if db.pool != nil {
		pgxRows, queryErr := db.pool.Query(ctx, query, args...)
		err = queryErr
		if err == nil {
			rows = &pgxRowsAdapter{rows: pgxRows}
		}
	} else {
		sqlRows, queryErr := db.stdDB.QueryContext(ctx, query, args...)
		err = queryErr
		if err == nil {
			rows = &sqlRowsAdapter{rows: sqlRows}
		}
	}

	duration := time.Since(start)

	if db.metrics != nil {
		db.metrics.RecordQuery(ctx, SanitizeQuery(query), duration, err)
	}

	if err != nil {
		return nil, WrapError(err, "query execution failed")
	}

	return rows, nil
}

// QueryRow executes a query that is expected to return at most one row.
func (db *PostgresDB) QueryRow(ctx context.Context, query string, args ...interface{}) Row {
	start := time.Now()

	if db.config.LogQueries && db.logger != nil {
		db.logger.Debug("executing query row",
			slog.String("query", SanitizeQuery(query)),
		)
	}

	var row Row

	if db.pool != nil {
		pgxRow := db.pool.QueryRow(ctx, query, args...)
		row = &pgxRowAdapter{row: pgxRow}
	} else {
		sqlRow := db.stdDB.QueryRowContext(ctx, query, args...)
		row = &sqlRowAdapter{row: sqlRow}
	}

	duration := time.Since(start)

	if db.metrics != nil {
		db.metrics.RecordQuery(ctx, SanitizeQuery(query), duration, nil)
	}

	return row
}

// Exec executes a query that doesn't return rows.
func (db *PostgresDB) Exec(ctx context.Context, query string, args ...interface{}) (Result, error) {
	start := time.Now()

	if db.config.LogQueries && db.logger != nil {
		db.logger.Debug("executing exec",
			slog.String("query", SanitizeQuery(query)),
		)
	}

	var result Result
	var err error

	if db.pool != nil {
		tag, execErr := db.pool.Exec(ctx, query, args...)
		err = execErr
		if err == nil {
			result = &pgxCommandTagAdapter{tag: tag}
		}
	} else {
		sqlResult, execErr := db.stdDB.ExecContext(ctx, query, args...)
		err = execErr
		if err == nil {
			result = &sqlResultAdapter{result: sqlResult}
		}
	}

	duration := time.Since(start)

	if db.metrics != nil {
		db.metrics.RecordExec(ctx, SanitizeQuery(query), duration, err)
	}

	if err != nil {
		return nil, WrapError(err, "exec execution failed")
	}

	return result, nil
}

// Begin starts a new transaction.
func (db *PostgresDB) Begin(ctx context.Context) (Tx, error) {
	if db.pool != nil {
		tx, err := db.pool.Begin(ctx)
		if err != nil {
			return nil, WrapError(err, "failed to begin transaction")
		}
		return &pgxTxAdapter{tx: tx, logger: db.logger, config: db.config}, nil
	}

	tx, err := db.stdDB.BeginTx(ctx, nil)
	if err != nil {
		return nil, WrapError(err, "failed to begin transaction")
	}
	return &sqlTxAdapter{tx: tx, logger: db.logger, config: db.config}, nil
}

// WithTransaction executes a function within a transaction with retry logic.
func (db *PostgresDB) WithTransaction(ctx context.Context, fn func(tx Tx) error) error {
	return withRetry(ctx, db.config, func(ctx context.Context) error {
		tx, err := db.Begin(ctx)
		if err != nil {
			return err
		}

		// Always rollback on panic or error
		defer func() {
			if p := recover(); p != nil {
				_ = tx.Rollback(ctx)
				panic(p)
			}
		}()

		if err := fn(tx); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}

		return tx.Commit(ctx)
	})
}

// Health checks if the database is reachable (liveness check).
func (db *PostgresDB) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	var err error
	if db.pool != nil {
		err = db.pool.Ping(ctx)
	} else {
		err = db.stdDB.PingContext(ctx)
	}

	if err != nil {
		return WrapError(err, "database health check failed")
	}

	return nil
}

// HealthDetailed performs a more thorough health check (readiness check).
func (db *PostgresDB) HealthDetailed(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	// First, do a basic ping
	if err := db.Health(ctx); err != nil {
		return err
	}

	// Then execute a simple query
	var result int
	row := db.QueryRow(ctx, "SELECT 1")
	if err := row.Scan(&result); err != nil {
		return WrapError(err, "database readiness check failed")
	}

	if result != 1 {
		return fmt.Errorf("unexpected result from health check: %d", result)
	}

	return nil
}

// Stats returns connection pool statistics.
func (db *PostgresDB) Stats() PoolStats {
	if db.pool != nil {
		stat := db.pool.Stat()
		return PoolStats{
			AcquiredConns:           stat.AcquiredConns(),
			IdleConns:               stat.IdleConns(),
			TotalConns:              stat.TotalConns(),
			MaxConns:                stat.MaxConns(),
			NewConnsCount:           stat.NewConnsCount(),
			MaxLifetimeDestroyCount: stat.MaxLifetimeDestroyCount(),
			MaxIdleDestroyCount:     stat.MaxIdleDestroyCount(),
		}
	}

	// For database/sql
	stat := db.stdDB.Stats()
	return PoolStats{
		AcquiredConns: int32(stat.InUse),
		IdleConns:     int32(stat.Idle),
		TotalConns:    int32(stat.OpenConnections),
		MaxConns:      int32(stat.MaxOpenConnections),
	}
}

// Close closes the database connection pool.
func (db *PostgresDB) Close() error {
	var err error
	db.closeOnce.Do(func() {
		// Stop health checks
		if db.healthCancel != nil {
			db.healthCancel()
		}
		if db.healthTicker != nil {
			db.healthTicker.Stop()
		}

		// Close connection pool
		if db.pool != nil {
			db.pool.Close()
		} else if db.stdDB != nil {
			err = db.stdDB.Close()
		}

		if db.logger != nil {
			db.logger.Info("PostgreSQL connection closed")
		}
	})
	return err
}

// Shutdown gracefully closes the database with a timeout.
func (db *PostgresDB) Shutdown(ctx context.Context) error {
	done := make(chan error, 1)

	go func() {
		done <- db.Close()
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		// Force close if timeout
		_ = db.Close()
		return ctx.Err()
	}
}

// Driver returns the database driver type.
func (db *PostgresDB) Driver() Driver {
	return DriverPostgres
}

// Pool returns the underlying pgxpool.Pool for advanced usage.
// Returns nil if using database/sql mode.
func (db *PostgresDB) Pool() *pgxpool.Pool {
	return db.pool
}

// DB returns the underlying *sql.DB for advanced usage.
// Returns nil if using pgxpool mode.
func (db *PostgresDB) DB() *sql.DB {
	return db.stdDB
}

// startHealthChecks starts background health checks.
func (db *PostgresDB) startHealthChecks() {
	ctx, cancel := context.WithCancel(context.Background())
	db.healthCancel = cancel

	ticker := time.NewTicker(db.config.HealthCheckInterval)
	db.healthTicker = ticker

	go func() {
		for {
			select {
			case <-ticker.C:
				healthCtx, healthCancel := context.WithTimeout(ctx, 2*time.Second)
				err := db.Health(healthCtx)
				healthCancel()

				db.healthMu.Lock()
				db.lastHealth = err
				db.lastHealthAt = time.Now()
				db.healthMu.Unlock()

				if err != nil && db.logger != nil {
					db.logger.Error("health check failed", slog.Any("error", err))
				}

			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}

// LastHealthCheck returns the result of the last health check.
func (db *PostgresDB) LastHealthCheck() (err error, at time.Time) {
	db.healthMu.RLock()
	defer db.healthMu.RUnlock()
	return db.lastHealth, db.lastHealthAt
}

// Transaction adapter methods for pgxTxAdapter

func (t *pgxTxAdapter) Query(ctx context.Context, query string, args ...interface{}) (Rows, error) {
	if t.config.LogQueries && t.logger != nil {
		t.logger.Debug("executing query in transaction",
			slog.String("query", SanitizeQuery(query)),
		)
	}

	rows, err := t.tx.Query(ctx, query, args...)
	if err != nil {
		return nil, WrapError(err, "transaction query failed")
	}

	return &pgxRowsAdapter{rows: rows}, nil
}

func (t *pgxTxAdapter) QueryRow(ctx context.Context, query string, args ...interface{}) Row {
	if t.config.LogQueries && t.logger != nil {
		t.logger.Debug("executing query row in transaction",
			slog.String("query", SanitizeQuery(query)),
		)
	}

	row := t.tx.QueryRow(ctx, query, args...)
	return &pgxRowAdapter{row: row}
}

func (t *pgxTxAdapter) Exec(ctx context.Context, query string, args ...interface{}) (Result, error) {
	if t.config.LogQueries && t.logger != nil {
		t.logger.Debug("executing exec in transaction",
			slog.String("query", SanitizeQuery(query)),
		)
	}

	tag, err := t.tx.Exec(ctx, query, args...)
	if err != nil {
		return nil, WrapError(err, "transaction exec failed")
	}

	return &pgxCommandTagAdapter{tag: tag}, nil
}

func (t *pgxTxAdapter) Commit(ctx context.Context) error {
	if err := t.tx.Commit(ctx); err != nil {
		return WrapError(err, "failed to commit transaction")
	}
	return nil
}

func (t *pgxTxAdapter) Rollback(ctx context.Context) error {
	if err := t.tx.Rollback(ctx); err != nil && err != pgx.ErrTxClosed {
		return WrapError(err, "failed to rollback transaction")
	}
	return nil
}

// Transaction adapter methods for sqlTxAdapter

func (t *sqlTxAdapter) Query(ctx context.Context, query string, args ...interface{}) (Rows, error) {
	if t.config.LogQueries && t.logger != nil {
		t.logger.Debug("executing query in transaction",
			slog.String("query", SanitizeQuery(query)),
		)
	}

	rows, err := t.tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, WrapError(err, "transaction query failed")
	}

	return &sqlRowsAdapter{rows: rows}, nil
}

func (t *sqlTxAdapter) QueryRow(ctx context.Context, query string, args ...interface{}) Row {
	if t.config.LogQueries && t.logger != nil {
		t.logger.Debug("executing query row in transaction",
			slog.String("query", SanitizeQuery(query)),
		)
	}

	row := t.tx.QueryRowContext(ctx, query, args...)
	return &sqlRowAdapter{row: row}
}

func (t *sqlTxAdapter) Exec(ctx context.Context, query string, args ...interface{}) (Result, error) {
	if t.config.LogQueries && t.logger != nil {
		t.logger.Debug("executing exec in transaction",
			slog.String("query", SanitizeQuery(query)),
		)
	}

	result, err := t.tx.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, WrapError(err, "transaction exec failed")
	}

	return &sqlResultAdapter{result: result}, nil
}

func (t *sqlTxAdapter) Commit(ctx context.Context) error {
	if err := t.tx.Commit(); err != nil {
		return WrapError(err, "failed to commit transaction")
	}
	return nil
}

func (t *sqlTxAdapter) Rollback(ctx context.Context) error {
	if err := t.tx.Rollback(); err != nil && err != sql.ErrTxDone {
		return WrapError(err, "failed to rollback transaction")
	}
	return nil
}

// Adapter implementations

func (r *pgxRowsAdapter) Next() bool {
	return r.rows.Next()
}

func (r *pgxRowsAdapter) Scan(dest ...interface{}) error {
	if err := r.rows.Scan(dest...); err != nil {
		return WrapError(err, "failed to scan row")
	}
	return nil
}

func (r *pgxRowsAdapter) Close() error {
	r.rows.Close()
	return nil
}

func (r *pgxRowsAdapter) Err() error {
	if err := r.rows.Err(); err != nil {
		return WrapError(err, "rows iteration error")
	}
	return nil
}

func (r *pgxRowAdapter) Scan(dest ...interface{}) error {
	if err := r.row.Scan(dest...); err != nil {
		return WrapError(err, "failed to scan row")
	}
	return nil
}

func (r *pgxCommandTagAdapter) LastInsertId() (int64, error) {
	// PostgreSQL doesn't support LastInsertId, use RETURNING clause instead
	return 0, nil
}

func (r *pgxCommandTagAdapter) RowsAffected() (int64, error) {
	return r.tag.RowsAffected(), nil
}

func (r *sqlRowsAdapter) Next() bool {
	return r.rows.Next()
}

func (r *sqlRowsAdapter) Scan(dest ...interface{}) error {
	if err := r.rows.Scan(dest...); err != nil {
		return WrapError(err, "failed to scan row")
	}
	return nil
}

func (r *sqlRowsAdapter) Close() error {
	return r.rows.Close()
}

func (r *sqlRowsAdapter) Err() error {
	if err := r.rows.Err(); err != nil {
		return WrapError(err, "rows iteration error")
	}
	return nil
}

func (r *sqlRowAdapter) Scan(dest ...interface{}) error {
	if err := r.row.Scan(dest...); err != nil {
		return WrapError(err, "failed to scan row")
	}
	return nil
}

func (r *sqlResultAdapter) LastInsertId() (int64, error) {
	return r.result.LastInsertId()
}

func (r *sqlResultAdapter) RowsAffected() (int64, error) {
	return r.result.RowsAffected()
}

// queryTracer implements pgx.QueryTracer for query logging.
type queryTracer struct {
	logger *slog.Logger
}

func (t *queryTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	t.logger.Debug("query started",
		slog.String("sql", SanitizeQuery(data.SQL)),
	)
	return ctx
}

func (t *queryTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	if data.Err != nil {
		t.logger.Error("query failed",
			slog.String("sql", SanitizeQuery(data.CommandTag.String())),
			slog.Any("error", data.Err),
		)
	}
}
