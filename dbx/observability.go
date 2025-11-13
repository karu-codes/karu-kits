package dbx

import (
	"context"
	"database/sql"
	"time"
)

// QueryHook is called before and after query execution
type QueryHook interface {
	BeforeQuery(ctx context.Context, query string, args []any)
	AfterQuery(ctx context.Context, query string, args []any, duration time.Duration, err error)
}

// ExecHook is called before and after exec operations
type ExecHook interface {
	BeforeExec(ctx context.Context, query string, args []any)
	AfterExec(ctx context.Context, query string, args []any, duration time.Duration, result sql.Result, err error)
}

// TxHook is called for transaction lifecycle events
type TxHook interface {
	BeforeBegin(ctx context.Context)
	AfterBegin(ctx context.Context, duration time.Duration, err error)
	BeforeCommit(ctx context.Context)
	AfterCommit(ctx context.Context, duration time.Duration, err error)
	BeforeRollback(ctx context.Context)
	AfterRollback(ctx context.Context, duration time.Duration, err error)
}

// MetricsCollector collects database metrics
type MetricsCollector interface {
	RecordQueryDuration(operation string, duration time.Duration, err error)
	RecordConnectionPoolStats(stats sql.DBStats)
}

// ObservableDB wraps DB with observability features
type ObservableDB struct {
	*DB
	queryHook   QueryHook
	execHook    ExecHook
	txHook      TxHook
	metrics     MetricsCollector
	slowQueryMs int64 // log queries slower than this (milliseconds)
}

// WithObservability wraps a DB with observability features
func WithObservability(db *DB, opts ...ObservabilityOption) *ObservableDB {
	odb := &ObservableDB{
		DB:          db,
		slowQueryMs: 1000, // default 1s
	}
	for _, opt := range opts {
		opt(odb)
	}
	return odb
}

// ObservabilityOption configures observability
type ObservabilityOption func(*ObservableDB)

// WithQueryHook adds a query hook
func WithQueryHook(hook QueryHook) ObservabilityOption {
	return func(odb *ObservableDB) {
		odb.queryHook = hook
	}
}

// WithExecHook adds an exec hook
func WithExecHook(hook ExecHook) ObservabilityOption {
	return func(odb *ObservableDB) {
		odb.execHook = hook
	}
}

// WithTxHook adds a transaction hook
func WithTxHook(hook TxHook) ObservabilityOption {
	return func(odb *ObservableDB) {
		odb.txHook = hook
	}
}

// WithMetrics adds a metrics collector
func WithMetrics(collector MetricsCollector) ObservabilityOption {
	return func(odb *ObservableDB) {
		odb.metrics = collector
	}
}

// WithSlowQueryThreshold sets the slow query threshold in milliseconds
func WithSlowQueryThreshold(ms int64) ObservabilityOption {
	return func(odb *ObservableDB) {
		odb.slowQueryMs = ms
	}
}

// ExecContext executes a query with observability
func (odb *ObservableDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if odb.execHook != nil {
		odb.execHook.BeforeExec(ctx, query, args)
	}

	start := time.Now()
	result, err := odb.DB.ExecContext(ctx, query, args...)
	duration := time.Since(start)

	if odb.execHook != nil {
		odb.execHook.AfterExec(ctx, query, args, duration, result, err)
	}

	if odb.metrics != nil {
		odb.metrics.RecordQueryDuration("exec", duration, err)
	}

	// Log slow queries
	if duration.Milliseconds() > odb.slowQueryMs {
		odb.logger.Printf("[dbx] slow exec: %v (query: %s)", duration, query)
	}

	return result, err
}

// QueryContext executes a query with observability
func (odb *ObservableDB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if odb.queryHook != nil {
		odb.queryHook.BeforeQuery(ctx, query, args)
	}

	start := time.Now()
	rows, err := odb.DB.QueryContext(ctx, query, args...)
	duration := time.Since(start)

	if odb.queryHook != nil {
		odb.queryHook.AfterQuery(ctx, query, args, duration, err)
	}

	if odb.metrics != nil {
		odb.metrics.RecordQueryDuration("query", duration, err)
	}

	// Log slow queries
	if duration.Milliseconds() > odb.slowQueryMs {
		odb.logger.Printf("[dbx] slow query: %v (query: %s)", duration, query)
	}

	return rows, err
}

// QueryRowContext executes a query that returns a single row with observability
func (odb *ObservableDB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	if odb.queryHook != nil {
		odb.queryHook.BeforeQuery(ctx, query, args)
	}

	start := time.Now()
	row := odb.DB.QueryRowContext(ctx, query, args...)
	duration := time.Since(start)

	if odb.queryHook != nil {
		odb.queryHook.AfterQuery(ctx, query, args, duration, nil)
	}

	if odb.metrics != nil {
		odb.metrics.RecordQueryDuration("query_row", duration, nil)
	}

	// Log slow queries
	if duration.Milliseconds() > odb.slowQueryMs {
		odb.logger.Printf("[dbx] slow query_row: %v (query: %s)", duration, query)
	}

	return row
}

// WithTx executes a function within a transaction with observability
func (odb *ObservableDB) WithTx(ctx context.Context, fn TxFunc, opt *TxOptions) error {
	if odb.txHook != nil {
		odb.txHook.BeforeBegin(ctx)
	}

	start := time.Now()
	err := odb.DB.WithTx(ctx, fn, opt)
	duration := time.Since(start)

	if err != nil {
		if odb.txHook != nil {
			odb.txHook.AfterRollback(ctx, duration, err)
		}
		if odb.metrics != nil {
			odb.metrics.RecordQueryDuration("tx_rollback", duration, err)
		}
	} else {
		if odb.txHook != nil {
			odb.txHook.AfterCommit(ctx, duration, nil)
		}
		if odb.metrics != nil {
			odb.metrics.RecordQueryDuration("tx_commit", duration, nil)
		}
	}

	return err
}

// Stats returns database statistics
func (odb *ObservableDB) Stats() sql.DBStats {
	stats := odb.std.Stats()

	// Record metrics if collector is available
	if odb.metrics != nil {
		odb.metrics.RecordConnectionPoolStats(stats)
	}

	return stats
}

// LoggingHook is a simple implementation that logs queries
type LoggingHook struct {
	logger Logger
}

// NewLoggingHook creates a new logging hook
func NewLoggingHook(logger Logger) *LoggingHook {
	return &LoggingHook{logger: logger}
}

func (h *LoggingHook) BeforeQuery(ctx context.Context, query string, args []any) {
	h.logger.Printf("[dbx] query start: %s (args: %v)", query, args)
}

func (h *LoggingHook) AfterQuery(ctx context.Context, query string, args []any, duration time.Duration, err error) {
	if err != nil {
		h.logger.Printf("[dbx] query error: %s (duration: %v, error: %v)", query, duration, err)
	} else {
		h.logger.Printf("[dbx] query success: %s (duration: %v)", query, duration)
	}
}

func (h *LoggingHook) BeforeExec(ctx context.Context, query string, args []any) {
	h.logger.Printf("[dbx] exec start: %s (args: %v)", query, args)
}

func (h *LoggingHook) AfterExec(ctx context.Context, query string, args []any, duration time.Duration, result sql.Result, err error) {
	if err != nil {
		h.logger.Printf("[dbx] exec error: %s (duration: %v, error: %v)", query, duration, err)
	} else {
		h.logger.Printf("[dbx] exec success: %s (duration: %v)", query, duration)
	}
}

// SimpleMetricsCollector is a basic in-memory metrics collector
type SimpleMetricsCollector struct {
	queryCount      map[string]int64
	errorCount      map[string]int64
	totalDuration   map[string]time.Duration
	lastPoolStats   sql.DBStats
}

// NewSimpleMetricsCollector creates a new simple metrics collector
func NewSimpleMetricsCollector() *SimpleMetricsCollector {
	return &SimpleMetricsCollector{
		queryCount:    make(map[string]int64),
		errorCount:    make(map[string]int64),
		totalDuration: make(map[string]time.Duration),
	}
}

func (c *SimpleMetricsCollector) RecordQueryDuration(operation string, duration time.Duration, err error) {
	c.queryCount[operation]++
	c.totalDuration[operation] += duration
	if err != nil {
		c.errorCount[operation]++
	}
}

func (c *SimpleMetricsCollector) RecordConnectionPoolStats(stats sql.DBStats) {
	c.lastPoolStats = stats
}

// GetMetrics returns current metrics
func (c *SimpleMetricsCollector) GetMetrics() map[string]interface{} {
	metrics := make(map[string]interface{})

	for op, count := range c.queryCount {
		metrics[op+"_count"] = count
		metrics[op+"_errors"] = c.errorCount[op]
		if count > 0 {
			avgDuration := c.totalDuration[op] / time.Duration(count)
			metrics[op+"_avg_duration_ms"] = avgDuration.Milliseconds()
		}
	}

	metrics["pool_open_connections"] = c.lastPoolStats.OpenConnections
	metrics["pool_in_use"] = c.lastPoolStats.InUse
	metrics["pool_idle"] = c.lastPoolStats.Idle
	metrics["pool_wait_count"] = c.lastPoolStats.WaitCount
	metrics["pool_wait_duration_ms"] = c.lastPoolStats.WaitDuration.Milliseconds()

	return metrics
}

// ResetMetrics resets all metrics
func (c *SimpleMetricsCollector) ResetMetrics() {
	c.queryCount = make(map[string]int64)
	c.errorCount = make(map[string]int64)
	c.totalDuration = make(map[string]time.Duration)
}
