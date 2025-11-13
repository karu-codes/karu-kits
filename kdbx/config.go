package kdbx

import (
	"log/slog"
	"time"
)

// Config holds the database configuration.
type Config struct {
	// Driver specifies the database driver (postgres or mysql).
	Driver Driver

	// DatabaseURL is the connection string for the database.
	// Format for PostgreSQL: postgresql://username:password@host:port/database?options
	// Format for MySQL: username:password@tcp(host:port)/database?options
	DatabaseURL string

	// MaxOpenConns sets the maximum number of open connections to the database.
	// Default: 25
	MaxOpenConns int

	// MaxIdleConns sets the maximum number of idle connections in the pool.
	// Default: 5
	// Note: Must be less than or equal to MaxOpenConns.
	MaxIdleConns int

	// ConnMaxLifetime sets the maximum amount of time a connection may be reused.
	// Default: 30 minutes
	// Setting this prevents long-lived connection issues (e.g., network equipment timeouts).
	ConnMaxLifetime time.Duration

	// ConnMaxIdleTime sets the maximum amount of time a connection may be idle.
	// Default: 10 minutes
	// Connections idle longer than this will be closed.
	ConnMaxIdleTime time.Duration

	// ConnectTimeout sets the timeout for establishing a new connection.
	// Default: 10 seconds
	ConnectTimeout time.Duration

	// QueryTimeout sets the default timeout for query operations.
	// Default: 30 seconds
	// Individual queries can override this with their own context timeout.
	QueryTimeout time.Duration

	// HealthCheckInterval sets how often to perform background health checks.
	// Default: 30 seconds
	// Set to 0 to disable background health checks.
	HealthCheckInterval time.Duration

	// RetryAttempts sets the maximum number of retry attempts for transient errors.
	// Default: 3
	RetryAttempts int

	// RetryInitialBackoff sets the initial backoff duration for retries.
	// Default: 100 milliseconds
	// Uses exponential backoff: 100ms, 200ms, 400ms, etc.
	RetryInitialBackoff time.Duration

	// RetryMaxBackoff sets the maximum backoff duration for retries.
	// Default: 5 seconds
	RetryMaxBackoff time.Duration

	// Logger is the structured logger for database operations.
	// If nil, logging is disabled.
	Logger *slog.Logger

	// Metrics is the metrics collector for observability.
	// If nil, metrics collection is disabled.
	Metrics MetricsCollector

	// LogQueries enables logging of all queries (sanitized).
	// Default: false
	// Warning: This can be verbose and impact performance in high-traffic applications.
	LogQueries bool

	// ReadOnly opens the database in read-only mode.
	// Default: false
	ReadOnly bool

	// PostgresPreferSimpleProtocol disables prepared statement cache for PostgreSQL.
	// Default: false
	// Set to true if you have queries with dynamic table/column names.
	PostgresPreferSimpleProtocol bool

	// MySQLParseTime changes the output type of DATE and DATETIME values to time.Time.
	// Default: true
	MySQLParseTime bool

	// MySQLLocation sets the location for parsing MySQL DATE and DATETIME values.
	// Default: UTC
	MySQLLocation *time.Location

	// MySQLMultiStatements allows multiple statements in one query (unsafe).
	// Default: false
	// Warning: Only enable if you trust the query source to prevent SQL injection.
	MySQLMultiStatements bool
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig(driver Driver, databaseURL string) *Config {
	return &Config{
		Driver:                       driver,
		DatabaseURL:                  databaseURL,
		MaxOpenConns:                 25,
		MaxIdleConns:                 5,
		ConnMaxLifetime:              30 * time.Minute,
		ConnMaxIdleTime:              10 * time.Minute,
		ConnectTimeout:               10 * time.Second,
		QueryTimeout:                 30 * time.Second,
		HealthCheckInterval:          30 * time.Second,
		RetryAttempts:                3,
		RetryInitialBackoff:          100 * time.Millisecond,
		RetryMaxBackoff:              5 * time.Second,
		Logger:                       nil,
		Metrics:                      nil,
		LogQueries:                   false,
		ReadOnly:                     false,
		PostgresPreferSimpleProtocol: false,
		MySQLParseTime:               true,
		MySQLLocation:                time.UTC,
		MySQLMultiStatements:         false,
	}
}

// Validate validates the configuration and returns an error if invalid.
func (c *Config) Validate() error {
	if c.Driver != DriverPostgres && c.Driver != DriverMySQL {
		return ErrInvalidDriver
	}

	if c.DatabaseURL == "" {
		return ErrMissingDatabaseURL
	}

	if c.MaxIdleConns > c.MaxOpenConns {
		return ErrInvalidPoolConfig
	}

	if c.MaxOpenConns < 1 {
		return ErrInvalidPoolConfig
	}

	if c.ConnMaxLifetime < 0 {
		return ErrInvalidPoolConfig
	}

	if c.ConnMaxIdleTime < 0 {
		return ErrInvalidPoolConfig
	}

	if c.ConnectTimeout < 0 {
		return ErrInvalidPoolConfig
	}

	if c.QueryTimeout < 0 {
		return ErrInvalidPoolConfig
	}

	if c.RetryAttempts < 0 {
		return ErrInvalidRetryConfig
	}

	if c.RetryInitialBackoff < 0 {
		return ErrInvalidRetryConfig
	}

	if c.RetryMaxBackoff < c.RetryInitialBackoff {
		return ErrInvalidRetryConfig
	}

	return nil
}

// Option is a function that modifies a Config.
type Option func(*Config)

// WithMaxOpenConns sets the maximum number of open connections.
func WithMaxOpenConns(n int) Option {
	return func(c *Config) {
		c.MaxOpenConns = n
	}
}

// WithMaxIdleConns sets the maximum number of idle connections.
func WithMaxIdleConns(n int) Option {
	return func(c *Config) {
		c.MaxIdleConns = n
	}
}

// WithConnMaxLifetime sets the maximum connection lifetime.
func WithConnMaxLifetime(d time.Duration) Option {
	return func(c *Config) {
		c.ConnMaxLifetime = d
	}
}

// WithConnMaxIdleTime sets the maximum connection idle time.
func WithConnMaxIdleTime(d time.Duration) Option {
	return func(c *Config) {
		c.ConnMaxIdleTime = d
	}
}

// WithConnectTimeout sets the connection timeout.
func WithConnectTimeout(d time.Duration) Option {
	return func(c *Config) {
		c.ConnectTimeout = d
	}
}

// WithQueryTimeout sets the default query timeout.
func WithQueryTimeout(d time.Duration) Option {
	return func(c *Config) {
		c.QueryTimeout = d
	}
}

// WithHealthCheckInterval sets the health check interval.
func WithHealthCheckInterval(d time.Duration) Option {
	return func(c *Config) {
		c.HealthCheckInterval = d
	}
}

// WithRetryAttempts sets the maximum retry attempts.
func WithRetryAttempts(n int) Option {
	return func(c *Config) {
		c.RetryAttempts = n
	}
}

// WithRetryBackoff sets the retry backoff configuration.
func WithRetryBackoff(initial, max time.Duration) Option {
	return func(c *Config) {
		c.RetryInitialBackoff = initial
		c.RetryMaxBackoff = max
	}
}

// WithLogger sets the structured logger.
func WithLogger(logger *slog.Logger) Option {
	return func(c *Config) {
		c.Logger = logger
	}
}

// WithMetrics sets the metrics collector.
func WithMetrics(metrics MetricsCollector) Option {
	return func(c *Config) {
		c.Metrics = metrics
	}
}

// WithLogQueries enables query logging.
func WithLogQueries(enabled bool) Option {
	return func(c *Config) {
		c.LogQueries = enabled
	}
}

// WithReadOnly enables read-only mode.
func WithReadOnly(enabled bool) Option {
	return func(c *Config) {
		c.ReadOnly = enabled
	}
}

// WithPostgresSimpleProtocol enables simple protocol for PostgreSQL.
func WithPostgresSimpleProtocol(enabled bool) Option {
	return func(c *Config) {
		c.PostgresPreferSimpleProtocol = enabled
	}
}

// WithMySQLParseTime enables parsing MySQL time values.
func WithMySQLParseTime(enabled bool) Option {
	return func(c *Config) {
		c.MySQLParseTime = enabled
	}
}

// WithMySQLLocation sets the location for MySQL time parsing.
func WithMySQLLocation(loc *time.Location) Option {
	return func(c *Config) {
		c.MySQLLocation = loc
	}
}

// WithMySQLMultiStatements enables multiple statements in one query (use with caution).
func WithMySQLMultiStatements(enabled bool) Option {
	return func(c *Config) {
		c.MySQLMultiStatements = enabled
	}
}

// ApplyOptions applies the given options to the config.
func (c *Config) ApplyOptions(opts ...Option) {
	for _, opt := range opts {
		opt(c)
	}
}

// MaskedURL returns the database URL with the password masked.
// Useful for logging without exposing credentials.
func (c *Config) MaskedURL() string {
	return maskPassword(c.DatabaseURL)
}

// maskPassword masks the password in a database URL.
func maskPassword(url string) string {
	// Simple masking: find password between : and @ or between : and /
	// postgresql://user:PASSWORD@host:5432/db -> postgresql://user:***@host:5432/db
	// user:PASSWORD@tcp(host:3306)/db -> user:***@tcp(host:3306)/db

	masked := ""
	inPassword := false

	for i := 0; i < len(url); i++ {
		if url[i] == ':' && i+1 < len(url) && !inPassword {
			// Check if this is a password separator (followed by something other than //)
			if i+2 < len(url) && url[i+1:i+2] != "/" {
				inPassword = true
				masked += ":"
				continue
			}
		}

		if inPassword && (url[i] == '@' || url[i] == '/') {
			masked += "***" + string(url[i])
			inPassword = false
			continue
		}

		if !inPassword {
			masked += string(url[i])
		}
	}

	if inPassword {
		masked += "***"
	}

	return masked
}
