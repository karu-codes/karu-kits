package dbx

import (
	"context"
	"database/sql"
	"time"
)

// HealthStatus represents the overall health status
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// HealthCheck represents a health check result
type HealthCheck struct {
	Status           HealthStatus       `json:"status"`
	Timestamp        time.Time          `json:"timestamp"`
	Uptime           time.Duration      `json:"uptime"`
	ResponseTime     time.Duration      `json:"response_time_ms"`
	Connections      ConnectionHealth   `json:"connections"`
	ReadReplica      *ConnectionHealth  `json:"read_replica,omitempty"`
	PgxPool          *PgxPoolHealth     `json:"pgx_pool,omitempty"`
	Checks           map[string]bool    `json:"checks"`
	Errors           []string           `json:"errors,omitempty"`
}

// ConnectionHealth represents connection pool health
type ConnectionHealth struct {
	Open              int           `json:"open"`
	InUse             int           `json:"in_use"`
	Idle              int           `json:"idle"`
	MaxOpen           int           `json:"max_open"`
	WaitCount         int64         `json:"wait_count"`
	WaitDuration      time.Duration `json:"wait_duration_ms"`
	MaxIdleClosed     int64         `json:"max_idle_closed"`
	MaxLifetimeClosed int64         `json:"max_lifetime_closed"`
	Utilization       float64       `json:"utilization_percent"`
}

// PgxPoolHealth represents pgxpool-specific health
type PgxPoolHealth struct {
	AcquireCount           int64 `json:"acquire_count"`
	AcquireDuration        int64 `json:"acquire_duration_ns"`
	AcquiredConns          int32 `json:"acquired_conns"`
	CanceledAcquireCount   int64 `json:"canceled_acquire_count"`
	ConstructingConns      int32 `json:"constructing_conns"`
	EmptyAcquireCount      int64 `json:"empty_acquire_count"`
	IdleConns              int32 `json:"idle_conns"`
	MaxConns               int32 `json:"max_conns"`
	TotalConns             int32 `json:"total_conns"`
	NewConnsCount          int64 `json:"new_conns_count"`
	MaxLifetimeDestroyCount int64 `json:"max_lifetime_destroy_count"`
	MaxIdleDestroyCount     int64 `json:"max_idle_destroy_count"`
}

// Health performs a comprehensive health check
func (d *DB) Health(ctx context.Context) HealthCheck {
	start := time.Now()
	hc := HealthCheck{
		Status:    HealthStatusHealthy,
		Timestamp: start,
		Uptime:    time.Since(d.started),
		Checks:    make(map[string]bool),
		Errors:    make([]string, 0),
	}

	// Check primary connection
	if err := d.std.PingContext(ctx); err != nil {
		hc.Status = HealthStatusUnhealthy
		hc.Checks["primary_ping"] = false
		hc.Errors = append(hc.Errors, "primary ping failed: "+err.Error())
	} else {
		hc.Checks["primary_ping"] = true
	}

	// Get primary connection stats
	stats := d.std.Stats()
	hc.Connections = buildConnectionHealth(stats, d.cfg.MaxOpenConns)

	// Check connection pool health
	if hc.Connections.Utilization > 90.0 {
		hc.Status = HealthStatusDegraded
		hc.Errors = append(hc.Errors, "connection pool utilization above 90%")
	}

	// Check read replica if configured
	if d.roStd != nil {
		if err := d.roStd.PingContext(ctx); err != nil {
			if hc.Status == HealthStatusHealthy {
				hc.Status = HealthStatusDegraded
			}
			hc.Checks["replica_ping"] = false
			hc.Errors = append(hc.Errors, "replica ping failed: "+err.Error())
		} else {
			hc.Checks["replica_ping"] = true
		}

		roStats := d.roStd.Stats()
		roHealth := buildConnectionHealth(roStats, d.cfg.MaxOpenConns)
		hc.ReadReplica = &roHealth
	}

	// Check pgxpool if configured
	if d.pgxPool != nil {
		if err := d.pgxPool.Ping(ctx); err != nil {
			if hc.Status == HealthStatusHealthy {
				hc.Status = HealthStatusDegraded
			}
			hc.Checks["pgxpool_ping"] = false
			hc.Errors = append(hc.Errors, "pgxpool ping failed: "+err.Error())
		} else {
			hc.Checks["pgxpool_ping"] = true
		}

		pgxStats := d.pgxPool.Stat()
		hc.PgxPool = &PgxPoolHealth{
			AcquireCount:            pgxStats.AcquireCount(),
			AcquireDuration:         pgxStats.AcquireDuration().Nanoseconds(),
			AcquiredConns:           pgxStats.AcquiredConns(),
			CanceledAcquireCount:    pgxStats.CanceledAcquireCount(),
			ConstructingConns:       pgxStats.ConstructingConns(),
			EmptyAcquireCount:       pgxStats.EmptyAcquireCount(),
			IdleConns:               pgxStats.IdleConns(),
			MaxConns:                pgxStats.MaxConns(),
			TotalConns:              pgxStats.TotalConns(),
			NewConnsCount:           pgxStats.NewConnsCount(),
			MaxLifetimeDestroyCount: pgxStats.MaxLifetimeDestroyCount(),
			MaxIdleDestroyCount:     pgxStats.MaxIdleDestroyCount(),
		}
	}

	hc.ResponseTime = time.Since(start)
	return hc
}

// buildConnectionHealth builds connection health from sql.DBStats
func buildConnectionHealth(stats sql.DBStats, maxOpen int) ConnectionHealth {
	utilization := 0.0
	if maxOpen > 0 {
		utilization = float64(stats.InUse) / float64(maxOpen) * 100.0
	}

	return ConnectionHealth{
		Open:              stats.OpenConnections,
		InUse:             stats.InUse,
		Idle:              stats.Idle,
		MaxOpen:           maxOpen,
		WaitCount:         stats.WaitCount,
		WaitDuration:      stats.WaitDuration,
		MaxIdleClosed:     stats.MaxIdleClosed,
		MaxLifetimeClosed: stats.MaxLifetimeClosed,
		Utilization:       utilization,
	}
}

// IsHealthy returns true if the database is healthy
func (hc HealthCheck) IsHealthy() bool {
	return hc.Status == HealthStatusHealthy
}

// IsDegraded returns true if the database is degraded
func (hc HealthCheck) IsDegraded() bool {
	return hc.Status == HealthStatusDegraded
}

// IsUnhealthy returns true if the database is unhealthy
func (hc HealthCheck) IsUnhealthy() bool {
	return hc.Status == HealthStatusUnhealthy
}

// HealthChecker provides periodic health checking
type HealthChecker struct {
	db       *DB
	interval time.Duration
	logger   Logger
	stopCh   chan struct{}
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(db *DB, interval time.Duration, logger Logger) *HealthChecker {
	if logger == nil {
		logger = nopLogger{}
	}
	return &HealthChecker{
		db:       db,
		interval: interval,
		logger:   logger,
		stopCh:   make(chan struct{}),
	}
}

// Start starts the health checker
func (hc *HealthChecker) Start(ctx context.Context) {
	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-hc.stopCh:
			return
		case <-ticker.C:
			health := hc.db.Health(ctx)
			if !health.IsHealthy() {
				hc.logger.Printf("[dbx] health check: status=%s, errors=%v", health.Status, health.Errors)
			}
		}
	}
}

// Stop stops the health checker
func (hc *HealthChecker) Stop() {
	close(hc.stopCh)
}

// QuickPing performs a simple ping check (fast health check for load balancers)
func (d *DB) QuickPing(ctx context.Context) error {
	return d.std.PingContext(ctx)
}

// ReadinessCheck checks if the database is ready to accept traffic
func (d *DB) ReadinessCheck(ctx context.Context) error {
	// Check primary connection
	if err := d.std.PingContext(ctx); err != nil {
		return wrapDBError(err, "readiness check: primary ping failed")
	}

	// Check if we have available connections
	stats := d.std.Stats()
	if stats.InUse >= d.cfg.MaxOpenConns {
		return newDBError("readiness check: no available connections")
	}

	return nil
}

// LivenessCheck checks if the database connection is alive
func (d *DB) LivenessCheck(ctx context.Context) error {
	return d.std.PingContext(ctx)
}
