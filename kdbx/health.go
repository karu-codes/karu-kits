package kdbx

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// HealthStatus represents the health status of the database.
type HealthStatus string

const (
	// HealthStatusHealthy indicates the database is healthy and ready.
	HealthStatusHealthy HealthStatus = "healthy"

	// HealthStatusDegraded indicates the database is accessible but may have issues.
	HealthStatusDegraded HealthStatus = "degraded"

	// HealthStatusUnhealthy indicates the database is not accessible.
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// HealthCheck represents the result of a health check.
type HealthCheck struct {
	// Status is the overall health status.
	Status HealthStatus `json:"status"`

	// Message provides additional context about the health status.
	Message string `json:"message,omitempty"`

	// Timestamp is when the health check was performed.
	Timestamp time.Time `json:"timestamp"`

	// Duration is how long the health check took.
	Duration time.Duration `json:"duration"`

	// Details contains additional health check details.
	Details map[string]interface{} `json:"details,omitempty"`
}

// HealthChecker provides health check functionality for databases.
type HealthChecker struct {
	db Database

	// Cache settings
	cacheDuration time.Duration

	// Mutex protects cache fields from concurrent access
	mu            sync.RWMutex
	lastCheck     *HealthCheck
	lastCheckTime time.Time
}

// NewHealthChecker creates a new health checker for a database.
func NewHealthChecker(db Database) *HealthChecker {
	return &HealthChecker{
		db:            db,
		cacheDuration: 1 * time.Second, // Cache for 1 second to prevent DoS
	}
}

// WithCacheDuration sets the cache duration for health checks.
func (h *HealthChecker) WithCacheDuration(d time.Duration) *HealthChecker {
	h.cacheDuration = d
	return h
}

// Check performs a basic health check (liveness).
// Results are cached for the configured cache duration.
func (h *HealthChecker) Check(ctx context.Context) *HealthCheck {
	// Check cache with read lock first
	h.mu.RLock()
	if h.lastCheck != nil && time.Since(h.lastCheckTime) < h.cacheDuration {
		cachedCheck := h.lastCheck
		h.mu.RUnlock()
		return cachedCheck
	}
	h.mu.RUnlock()

	// Perform health check
	start := time.Now()
	err := h.db.Health(ctx)
	duration := time.Since(start)

	check := &HealthCheck{
		Timestamp: start,
		Duration:  duration,
		Details:   make(map[string]interface{}),
	}

	if err != nil {
		check.Status = HealthStatusUnhealthy
		check.Message = fmt.Sprintf("database health check failed: %v", err)
	} else {
		check.Status = HealthStatusHealthy
		check.Message = "database is healthy"
	}

	// Add pool statistics
	stats := h.db.Stats()
	check.Details["pool"] = map[string]interface{}{
		"acquired_conns": stats.AcquiredConns,
		"idle_conns":     stats.IdleConns,
		"total_conns":    stats.TotalConns,
		"max_conns":      stats.MaxConns,
	}

	// Cache the result with write lock
	h.mu.Lock()
	h.lastCheck = check
	h.lastCheckTime = start
	h.mu.Unlock()

	return check
}

// CheckDetailed performs a detailed health check (readiness).
// This includes a test query and is not cached.
func (h *HealthChecker) CheckDetailed(ctx context.Context) *HealthCheck {
	start := time.Now()
	err := h.db.HealthDetailed(ctx)
	duration := time.Since(start)

	check := &HealthCheck{
		Timestamp: start,
		Duration:  duration,
		Details:   make(map[string]interface{}),
	}

	if err != nil {
		check.Status = HealthStatusUnhealthy
		check.Message = fmt.Sprintf("database readiness check failed: %v", err)
	} else {
		check.Status = HealthStatusHealthy
		check.Message = "database is ready"
	}

	// Add pool statistics
	stats := h.db.Stats()
	check.Details["pool"] = map[string]interface{}{
		"acquired_conns": stats.AcquiredConns,
		"idle_conns":     stats.IdleConns,
		"total_conns":    stats.TotalConns,
		"max_conns":      stats.MaxConns,
	}

	// Add connection pool health indicators
	poolHealth := h.analyzePoolHealth(stats)
	check.Details["pool_health"] = poolHealth

	// Adjust status based on pool health
	if poolHealth["status"] == "degraded" && check.Status == HealthStatusHealthy {
		check.Status = HealthStatusDegraded
		check.Message = "database is accessible but connection pool may be stressed"
	}

	return check
}

// analyzePoolHealth analyzes connection pool statistics for potential issues.
func (h *HealthChecker) analyzePoolHealth(stats PoolStats) map[string]interface{} {
	analysis := make(map[string]interface{})

	// Calculate pool utilization
	var utilization float64
	if stats.MaxConns > 0 {
		utilization = float64(stats.TotalConns) / float64(stats.MaxConns)
	}

	analysis["utilization"] = fmt.Sprintf("%.2f%%", utilization*100)

	// Check for potential issues
	issues := make([]string, 0)

	// High utilization (>80%)
	if utilization > 0.8 {
		issues = append(issues, "connection pool utilization is high (>80%)")
		analysis["status"] = "degraded"
	} else {
		analysis["status"] = "healthy"
	}

	// No idle connections while under max
	if stats.IdleConns == 0 && stats.TotalConns < stats.MaxConns {
		issues = append(issues, "no idle connections available")
	}

	// All connections are in use
	if stats.AcquiredConns == stats.TotalConns && stats.TotalConns > 0 {
		issues = append(issues, "all connections are currently in use")
	}

	if len(issues) > 0 {
		analysis["issues"] = issues
	}

	return analysis
}

// String returns a human-readable string representation of the health check.
func (h *HealthCheck) String() string {
	return fmt.Sprintf("status=%s message=%q duration=%s timestamp=%s",
		h.Status, h.Message, h.Duration, h.Timestamp.Format(time.RFC3339))
}

// JSON returns a JSON representation of the health check.
func (h *HealthCheck) JSON() ([]byte, error) {
	return json.MarshalIndent(h, "", "  ")
}

// IsHealthy returns true if the database is healthy.
func (h *HealthCheck) IsHealthy() bool {
	return h.Status == HealthStatusHealthy
}

// IsDegraded returns true if the database is degraded.
func (h *HealthCheck) IsDegraded() bool {
	return h.Status == HealthStatusDegraded
}

// IsUnhealthy returns true if the database is unhealthy.
func (h *HealthCheck) IsUnhealthy() bool {
	return h.Status == HealthStatusUnhealthy
}

// HTTPStatusCode returns an appropriate HTTP status code for the health check.
// - 200: healthy
// - 429: degraded (too many requests/connections)
// - 503: unhealthy (service unavailable)
func (h *HealthCheck) HTTPStatusCode() int {
	switch h.Status {
	case HealthStatusHealthy:
		return 200
	case HealthStatusDegraded:
		return 429
	case HealthStatusUnhealthy:
		return 503
	default:
		return 503
	}
}

// HealthCheckFunc is a function that performs a custom health check.
type HealthCheckFunc func(ctx context.Context, db Database) error

// CustomHealthChecker allows adding custom health checks.
type CustomHealthChecker struct {
	*HealthChecker
	checks map[string]HealthCheckFunc
}

// NewCustomHealthChecker creates a new custom health checker.
func NewCustomHealthChecker(db Database) *CustomHealthChecker {
	return &CustomHealthChecker{
		HealthChecker: NewHealthChecker(db),
		checks:        make(map[string]HealthCheckFunc),
	}
}

// AddCheck adds a custom health check with a name.
func (c *CustomHealthChecker) AddCheck(name string, fn HealthCheckFunc) {
	c.checks[name] = fn
}

// CheckWithCustomChecks performs health check including custom checks.
func (c *CustomHealthChecker) CheckWithCustomChecks(ctx context.Context) *HealthCheck {
	// Start with base health check
	check := c.Check(ctx)

	// Run custom checks
	customResults := make(map[string]interface{})
	hasErrors := false

	for name, fn := range c.checks {
		checkStart := time.Now()
		err := fn(ctx, c.db)
		checkDuration := time.Since(checkStart)

		result := map[string]interface{}{
			"duration": checkDuration.String(),
		}

		if err != nil {
			result["status"] = "failed"
			result["error"] = err.Error()
			hasErrors = true
		} else {
			result["status"] = "passed"
		}

		customResults[name] = result
	}

	if len(customResults) > 0 {
		check.Details["custom_checks"] = customResults
	}

	// Adjust overall status if custom checks failed
	if hasErrors && check.Status == HealthStatusHealthy {
		check.Status = HealthStatusDegraded
		check.Message = "database is accessible but some custom checks failed"
	}

	return check
}

// Example custom health checks

// CheckQueryPerformance is a custom health check that measures query performance.
func CheckQueryPerformance(threshold time.Duration) HealthCheckFunc {
	return func(ctx context.Context, db Database) error {
		start := time.Now()
		row := db.QueryRow(ctx, "SELECT 1")
		var result int
		if err := row.Scan(&result); err != nil {
			return fmt.Errorf("query failed: %w", err)
		}
		duration := time.Since(start)

		if duration > threshold {
			return fmt.Errorf("query took %s, exceeds threshold of %s", duration, threshold)
		}

		return nil
	}
}

// CheckTableExists is a custom health check that verifies a table exists.
func CheckTableExists(tableName string) HealthCheckFunc {
	return func(ctx context.Context, db Database) error {
		var query string

		switch db.Driver() {
		case DriverPostgres:
			query = "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = $1)"
		case DriverMySQL:
			query = "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = ?)"
		default:
			return fmt.Errorf("unsupported driver: %s", db.Driver())
		}

		var exists bool
		row := db.QueryRow(ctx, query, tableName)
		if err := row.Scan(&exists); err != nil {
			return fmt.Errorf("failed to check table existence: %w", err)
		}

		if !exists {
			return fmt.Errorf("table %s does not exist", tableName)
		}

		return nil
	}
}
