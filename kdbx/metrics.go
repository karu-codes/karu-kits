package kdbx

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// NoOpMetricsCollector is a metrics collector that does nothing.
// Use this when metrics collection is not needed.
type NoOpMetricsCollector struct{}

func (n *NoOpMetricsCollector) RecordQuery(ctx context.Context, query string, duration time.Duration, err error) {
}

func (n *NoOpMetricsCollector) RecordExec(ctx context.Context, query string, duration time.Duration, err error) {
}

func (n *NoOpMetricsCollector) RecordTransaction(ctx context.Context, duration time.Duration, committed bool, err error) {
}

func (n *NoOpMetricsCollector) RecordPoolStats(stats PoolStats) {
}

// LoggingMetricsCollector is a metrics collector that logs metrics using slog.
type LoggingMetricsCollector struct {
	logger *slog.Logger
}

// NewLoggingMetricsCollector creates a new logging metrics collector.
func NewLoggingMetricsCollector(logger *slog.Logger) *LoggingMetricsCollector {
	return &LoggingMetricsCollector{
		logger: logger,
	}
}

func (l *LoggingMetricsCollector) RecordQuery(ctx context.Context, query string, duration time.Duration, err error) {
	if err != nil {
		l.logger.ErrorContext(ctx, "query failed",
			slog.String("query", query),
			slog.Duration("duration", duration),
			slog.Any("error", err),
		)
	} else {
		l.logger.DebugContext(ctx, "query completed",
			slog.String("query", query),
			slog.Duration("duration", duration),
		)
	}
}

func (l *LoggingMetricsCollector) RecordExec(ctx context.Context, query string, duration time.Duration, err error) {
	if err != nil {
		l.logger.ErrorContext(ctx, "exec failed",
			slog.String("query", query),
			slog.Duration("duration", duration),
			slog.Any("error", err),
		)
	} else {
		l.logger.DebugContext(ctx, "exec completed",
			slog.String("query", query),
			slog.Duration("duration", duration),
		)
	}
}

func (l *LoggingMetricsCollector) RecordTransaction(ctx context.Context, duration time.Duration, committed bool, err error) {
	if err != nil {
		l.logger.ErrorContext(ctx, "transaction failed",
			slog.Duration("duration", duration),
			slog.Bool("committed", committed),
			slog.Any("error", err),
		)
	} else {
		l.logger.DebugContext(ctx, "transaction completed",
			slog.Duration("duration", duration),
			slog.Bool("committed", committed),
		)
	}
}

func (l *LoggingMetricsCollector) RecordPoolStats(stats PoolStats) {
	l.logger.Debug("pool stats",
		slog.Int("acquired_conns", int(stats.AcquiredConns)),
		slog.Int("idle_conns", int(stats.IdleConns)),
		slog.Int("total_conns", int(stats.TotalConns)),
		slog.Int("max_conns", int(stats.MaxConns)),
	)
}

// InMemoryMetricsCollector collects metrics in memory for monitoring and debugging.
// This is useful for development and testing but should not be used in production
// for high-traffic applications due to memory usage.
type InMemoryMetricsCollector struct {
	mu sync.RWMutex

	// Query metrics
	queryCount      int64
	queryErrorCount int64
	queryDurations  []time.Duration

	// Exec metrics
	execCount      int64
	execErrorCount int64
	execDurations  []time.Duration

	// Transaction metrics
	txCount         int64
	txCommitCount   int64
	txRollbackCount int64
	txErrorCount    int64
	txDurations     []time.Duration

	// Pool stats
	lastPoolStats PoolStats
	poolStatsTime time.Time

	// Keep track of slow queries
	slowQueries     []SlowQuery
	slowQueryThreshold time.Duration
}

// SlowQuery represents a query that exceeded the slow query threshold.
type SlowQuery struct {
	Query     string
	Duration  time.Duration
	Timestamp time.Time
	Error     error
}

// NewInMemoryMetricsCollector creates a new in-memory metrics collector.
func NewInMemoryMetricsCollector(slowQueryThreshold time.Duration) *InMemoryMetricsCollector {
	return &InMemoryMetricsCollector{
		queryDurations:     make([]time.Duration, 0),
		execDurations:      make([]time.Duration, 0),
		txDurations:        make([]time.Duration, 0),
		slowQueries:        make([]SlowQuery, 0),
		slowQueryThreshold: slowQueryThreshold,
	}
}

func (m *InMemoryMetricsCollector) RecordQuery(ctx context.Context, query string, duration time.Duration, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.queryCount++
	m.queryDurations = append(m.queryDurations, duration)

	if err != nil {
		m.queryErrorCount++
	}

	// Record slow queries
	if duration >= m.slowQueryThreshold {
		m.slowQueries = append(m.slowQueries, SlowQuery{
			Query:     query,
			Duration:  duration,
			Timestamp: time.Now(),
			Error:     err,
		})
	}
}

func (m *InMemoryMetricsCollector) RecordExec(ctx context.Context, query string, duration time.Duration, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.execCount++
	m.execDurations = append(m.execDurations, duration)

	if err != nil {
		m.execErrorCount++
	}

	// Record slow queries
	if duration >= m.slowQueryThreshold {
		m.slowQueries = append(m.slowQueries, SlowQuery{
			Query:     query,
			Duration:  duration,
			Timestamp: time.Now(),
			Error:     err,
		})
	}
}

func (m *InMemoryMetricsCollector) RecordTransaction(ctx context.Context, duration time.Duration, committed bool, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.txCount++
	m.txDurations = append(m.txDurations, duration)

	if committed {
		m.txCommitCount++
	} else {
		m.txRollbackCount++
	}

	if err != nil {
		m.txErrorCount++
	}
}

func (m *InMemoryMetricsCollector) RecordPoolStats(stats PoolStats) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.lastPoolStats = stats
	m.poolStatsTime = time.Now()
}

// Metrics returns a snapshot of collected metrics.
func (m *InMemoryMetricsCollector) Metrics() *Metrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return &Metrics{
		QueryCount:         m.queryCount,
		QueryErrorCount:    m.queryErrorCount,
		QueryAvgDuration:   calculateAverage(m.queryDurations),
		QueryP50Duration:   calculatePercentile(m.queryDurations, 0.50),
		QueryP95Duration:   calculatePercentile(m.queryDurations, 0.95),
		QueryP99Duration:   calculatePercentile(m.queryDurations, 0.99),
		ExecCount:          m.execCount,
		ExecErrorCount:     m.execErrorCount,
		ExecAvgDuration:    calculateAverage(m.execDurations),
		ExecP50Duration:    calculatePercentile(m.execDurations, 0.50),
		ExecP95Duration:    calculatePercentile(m.execDurations, 0.95),
		ExecP99Duration:    calculatePercentile(m.execDurations, 0.99),
		TxCount:            m.txCount,
		TxCommitCount:      m.txCommitCount,
		TxRollbackCount:    m.txRollbackCount,
		TxErrorCount:       m.txErrorCount,
		TxAvgDuration:      calculateAverage(m.txDurations),
		TxP50Duration:      calculatePercentile(m.txDurations, 0.50),
		TxP95Duration:      calculatePercentile(m.txDurations, 0.95),
		TxP99Duration:      calculatePercentile(m.txDurations, 0.99),
		PoolStats:          m.lastPoolStats,
		PoolStatsTimestamp: m.poolStatsTime,
		SlowQueryCount:     int64(len(m.slowQueries)),
	}
}

// SlowQueries returns a list of slow queries.
func (m *InMemoryMetricsCollector) SlowQueries() []SlowQuery {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to prevent race conditions
	queries := make([]SlowQuery, len(m.slowQueries))
	copy(queries, m.slowQueries)
	return queries
}

// Reset resets all metrics.
func (m *InMemoryMetricsCollector) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.queryCount = 0
	m.queryErrorCount = 0
	m.queryDurations = m.queryDurations[:0]
	m.execCount = 0
	m.execErrorCount = 0
	m.execDurations = m.execDurations[:0]
	m.txCount = 0
	m.txCommitCount = 0
	m.txRollbackCount = 0
	m.txErrorCount = 0
	m.txDurations = m.txDurations[:0]
	m.slowQueries = m.slowQueries[:0]
}

// Metrics represents a snapshot of database metrics.
type Metrics struct {
	// Query metrics
	QueryCount       int64
	QueryErrorCount  int64
	QueryAvgDuration time.Duration
	QueryP50Duration time.Duration
	QueryP95Duration time.Duration
	QueryP99Duration time.Duration

	// Exec metrics
	ExecCount       int64
	ExecErrorCount  int64
	ExecAvgDuration time.Duration
	ExecP50Duration time.Duration
	ExecP95Duration time.Duration
	ExecP99Duration time.Duration

	// Transaction metrics
	TxCount         int64
	TxCommitCount   int64
	TxRollbackCount int64
	TxErrorCount    int64
	TxAvgDuration   time.Duration
	TxP50Duration   time.Duration
	TxP95Duration   time.Duration
	TxP99Duration   time.Duration

	// Pool stats
	PoolStats          PoolStats
	PoolStatsTimestamp time.Time

	// Slow queries
	SlowQueryCount int64
}

// calculateAverage calculates the average duration.
func calculateAverage(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	var total time.Duration
	for _, d := range durations {
		total += d
	}

	return total / time.Duration(len(durations))
}

// calculatePercentile calculates the percentile duration.
func calculatePercentile(durations []time.Duration, percentile float64) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	// Simple percentile calculation without sorting
	// For a more accurate percentile, consider using a proper statistics library
	index := int(float64(len(durations)) * percentile)
	if index >= len(durations) {
		index = len(durations) - 1
	}

	return durations[index]
}

// CompositeMetricsCollector combines multiple metrics collectors.
type CompositeMetricsCollector struct {
	collectors []MetricsCollector
}

// NewCompositeMetricsCollector creates a new composite metrics collector.
func NewCompositeMetricsCollector(collectors ...MetricsCollector) *CompositeMetricsCollector {
	return &CompositeMetricsCollector{
		collectors: collectors,
	}
}

func (c *CompositeMetricsCollector) RecordQuery(ctx context.Context, query string, duration time.Duration, err error) {
	for _, collector := range c.collectors {
		collector.RecordQuery(ctx, query, duration, err)
	}
}

func (c *CompositeMetricsCollector) RecordExec(ctx context.Context, query string, duration time.Duration, err error) {
	for _, collector := range c.collectors {
		collector.RecordExec(ctx, query, duration, err)
	}
}

func (c *CompositeMetricsCollector) RecordTransaction(ctx context.Context, duration time.Duration, committed bool, err error) {
	for _, collector := range c.collectors {
		collector.RecordTransaction(ctx, duration, committed, err)
	}
}

func (c *CompositeMetricsCollector) RecordPoolStats(stats PoolStats) {
	for _, collector := range c.collectors {
		collector.RecordPoolStats(stats)
	}
}

// Add adds a metrics collector to the composite.
func (c *CompositeMetricsCollector) Add(collector MetricsCollector) {
	c.collectors = append(c.collectors, collector)
}
