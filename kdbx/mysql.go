package kdbx

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/url"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql" // MySQL driver
)

// MySQLDB wraps a MySQL connection pool and provides database operations.
type MySQLDB struct {
	db *sql.DB

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

// NewMySQL creates a new MySQL database connection.
func NewMySQL(ctx context.Context, config *Config) (*MySQLDB, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	if config.Driver != DriverMySQL {
		return nil, ErrInvalidDriver
	}

	// Build DSN with options
	dsn, err := buildMySQLDSN(config)
	if err != nil {
		return nil, WrapError(err, "failed to build MySQL DSN")
	}

	// Open database connection
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, WrapError(err, "failed to open MySQL connection")
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
		return nil, WrapError(err, "failed to ping MySQL database")
	}

	mysqlDB := &MySQLDB{
		db:      db,
		config:  config,
		logger:  config.Logger,
		metrics: config.Metrics,
	}

	// Start background health checks if configured
	if config.HealthCheckInterval > 0 {
		mysqlDB.startHealthChecks()
	}

	if mysqlDB.logger != nil {
		mysqlDB.logger.Info("MySQL connection established",
			slog.String("url", config.MaskedURL()),
			slog.Int("max_conns", config.MaxOpenConns),
			slog.Int("idle_conns", config.MaxIdleConns),
		)
	}

	return mysqlDB, nil
}

// Query executes a query that returns rows.
func (db *MySQLDB) Query(ctx context.Context, query string, args ...interface{}) (Rows, error) {
	start := time.Now()

	if db.config.LogQueries && db.logger != nil {
		db.logger.Debug("executing query",
			slog.String("query", SanitizeQuery(query)),
		)
	}

	rows, err := db.db.QueryContext(ctx, query, args...)

	duration := time.Since(start)

	if db.metrics != nil {
		db.metrics.RecordQuery(ctx, SanitizeQuery(query), duration, err)
	}

	if err != nil {
		return nil, WrapError(err, "query execution failed")
	}

	return &sqlRowsAdapter{rows: rows}, nil
}

// QueryRow executes a query that is expected to return at most one row.
func (db *MySQLDB) QueryRow(ctx context.Context, query string, args ...interface{}) Row {
	start := time.Now()

	if db.config.LogQueries && db.logger != nil {
		db.logger.Debug("executing query row",
			slog.String("query", SanitizeQuery(query)),
		)
	}

	row := db.db.QueryRowContext(ctx, query, args...)

	duration := time.Since(start)

	if db.metrics != nil {
		db.metrics.RecordQuery(ctx, SanitizeQuery(query), duration, nil)
	}

	return &sqlRowAdapter{row: row}
}

// Exec executes a query that doesn't return rows.
func (db *MySQLDB) Exec(ctx context.Context, query string, args ...interface{}) (Result, error) {
	start := time.Now()

	if db.config.LogQueries && db.logger != nil {
		db.logger.Debug("executing exec",
			slog.String("query", SanitizeQuery(query)),
		)
	}

	result, err := db.db.ExecContext(ctx, query, args...)

	duration := time.Since(start)

	if db.metrics != nil {
		db.metrics.RecordExec(ctx, SanitizeQuery(query), duration, err)
	}

	if err != nil {
		return nil, WrapError(err, "exec execution failed")
	}

	return &sqlResultAdapter{result: result}, nil
}

// Begin starts a new transaction.
func (db *MySQLDB) Begin(ctx context.Context) (Tx, error) {
	tx, err := db.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, WrapError(err, "failed to begin transaction")
	}

	return &sqlTxAdapter{tx: tx, logger: db.logger, config: db.config}, nil
}

// WithTransaction executes a function within a transaction with retry logic.
func (db *MySQLDB) WithTransaction(ctx context.Context, fn func(tx Tx) error) error {
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
func (db *MySQLDB) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := db.db.PingContext(ctx); err != nil {
		return WrapError(err, "database health check failed")
	}

	return nil
}

// HealthDetailed performs a more thorough health check (readiness check).
func (db *MySQLDB) HealthDetailed(ctx context.Context) error {
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
func (db *MySQLDB) Stats() PoolStats {
	stat := db.db.Stats()
	return PoolStats{
		AcquiredConns: int32(stat.InUse),
		IdleConns:     int32(stat.Idle),
		TotalConns:    int32(stat.OpenConnections),
		MaxConns:      int32(stat.MaxOpenConnections),
	}
}

// Close closes the database connection pool.
func (db *MySQLDB) Close() error {
	var err error
	db.closeOnce.Do(func() {
		// Stop health checks
		if db.healthCancel != nil {
			db.healthCancel()
		}
		if db.healthTicker != nil {
			db.healthTicker.Stop()
		}

		// Close database connection
		err = db.db.Close()

		if db.logger != nil {
			db.logger.Info("MySQL connection closed")
		}
	})
	return err
}

// Shutdown gracefully closes the database with a timeout.
func (db *MySQLDB) Shutdown(ctx context.Context) error {
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
func (db *MySQLDB) Driver() Driver {
	return DriverMySQL
}

// DB returns the underlying *sql.DB for advanced usage.
func (db *MySQLDB) DB() *sql.DB {
	return db.db
}

// startHealthChecks starts background health checks.
func (db *MySQLDB) startHealthChecks() {
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
func (db *MySQLDB) LastHealthCheck() (err error, at time.Time) {
	db.healthMu.RLock()
	defer db.healthMu.RUnlock()
	return db.lastHealth, db.lastHealthAt
}

// buildMySQLDSN builds a MySQL Data Source Name with proper configuration.
// Format: [username[:password]@][protocol[(address)]]/dbname[?param1=value1&...&paramN=valueN]
func buildMySQLDSN(config *Config) (string, error) {
	// Parse the provided URL
	// If it already contains all necessary parameters, use it as-is
	if config.DatabaseURL == "" {
		return "", ErrMissingDatabaseURL
	}

	// Check if URL already has query parameters
	hasParams := contains(config.DatabaseURL, "?")

	dsn := config.DatabaseURL

	// Build query parameters
	params := url.Values{}

	// parseTime converts DATE and DATETIME to time.Time
	if config.MySQLParseTime {
		params.Add("parseTime", "true")
	}

	// Set timezone location
	if config.MySQLLocation != nil {
		params.Add("loc", config.MySQLLocation.String())
	}

	// Set timeout
	if config.ConnectTimeout > 0 {
		params.Add("timeout", config.ConnectTimeout.String())
	}

	// Set read timeout
	if config.QueryTimeout > 0 {
		params.Add("readTimeout", config.QueryTimeout.String())
		params.Add("writeTimeout", config.QueryTimeout.String())
	}

	// Multi-statement queries (disabled by default for security)
	if config.MySQLMultiStatements {
		params.Add("multiStatements", "true")
	}

	// Charset
	params.Add("charset", "utf8mb4")

	// Collation
	params.Add("collation", "utf8mb4_unicode_ci")

	// SQL mode for strict behavior
	params.Add("sql_mode", "'STRICT_TRANS_TABLES,NO_ZERO_IN_DATE,NO_ZERO_DATE,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION'")

	// Interpolate parameters (better performance with prepared statements disabled)
	params.Add("interpolateParams", "true")

	// Build final DSN
	if hasParams {
		dsn += "&" + params.Encode()
	} else {
		dsn += "?" + params.Encode()
	}

	return dsn, nil
}

// contains checks if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && indexOf(s, substr) >= 0
}

// indexOf returns the index of the first instance of substr in s, or -1 if not found.
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
