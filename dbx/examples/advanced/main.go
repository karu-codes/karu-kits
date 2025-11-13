package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/karu-codes/karu-kits/dbx"
)

// Custom logger implementing dbx.Logger interface
type customLogger struct{}

func (l *customLogger) Printf(format string, v ...any) {
	log.Printf("[CUSTOM] "+format, v...)
}

func main() {
	ctx := context.Background()

	// Example 1: Advanced configuration with all features
	fmt.Println("=== Example 1: Advanced Configuration ===")
	logger := &customLogger{}

	db, err := dbx.Open(ctx,
		// Driver and connection
		dbx.WithDriver(dbx.DriverPostgres),
		dbx.WithPGHostPort("localhost", 5432),
		dbx.WithPGAuth("postgres", "password"),
		dbx.WithPGDB("testdb"),
		dbx.WithPGSSLMode("disable"),

		// Connection pooling
		dbx.WithPool(50, 25),
		dbx.WithConnLifetime(45*time.Minute, 10*time.Minute),
		dbx.WithConnTimeout(10*time.Second),

		// pgx pool (native)
		dbx.WithPgxPool(true),
		dbx.WithPgxPoolSize(10, 50),
		dbx.WithPgxPoolLifetime(1*time.Hour, 15*time.Minute, 30*time.Second),

		// Read replica
		dbx.WithReadReplicaDSN("postgres://postgres:password@localhost:5433/testdb?sslmode=disable"),

		// Retry configuration
		dbx.WithRetry(3, 200*time.Millisecond),

		// Observability
		dbx.WithLogger(logger),
		dbx.WithHealthCheckEvery(30*time.Second),
		dbx.WithAppName("advanced-example"),
	)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	fmt.Println("✓ Database opened with advanced configuration")

	// Example 2: Observability with hooks
	fmt.Println("\n=== Example 2: Observable Database ===")
	loggingHook := dbx.NewLoggingHook(logger)
	metricsCollector := dbx.NewSimpleMetricsCollector()

	odb := dbx.WithObservability(db,
		dbx.WithQueryHook(loggingHook),
		dbx.WithExecHook(loggingHook),
		dbx.WithMetrics(metricsCollector),
		dbx.WithSlowQueryThreshold(500), // 500ms
	)

	// Execute queries with observability
	_, err = odb.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS products (id SERIAL PRIMARY KEY, name TEXT, price DECIMAL)")
	if err != nil {
		log.Printf("Failed to create table: %v", err)
	}

	_, err = odb.ExecContext(ctx, "INSERT INTO products (name, price) VALUES ($1, $2)", "Product A", 19.99)
	if err != nil {
		log.Printf("Failed to insert: %v", err)
	}

	rows, err := odb.QueryContext(ctx, "SELECT id, name, price FROM products")
	if err != nil {
		log.Printf("Failed to query: %v", err)
	} else {
		defer rows.Close()
		fmt.Println("  Query executed successfully")
	}

	// Get metrics
	metrics := metricsCollector.GetMetrics()
	fmt.Printf("  Metrics: %+v\n", metrics)

	// Example 3: Health Monitoring
	fmt.Println("\n=== Example 3: Health Monitoring ===")
	health := db.Health(ctx)
	fmt.Printf("  Status: %s\n", health.Status)
	fmt.Printf("  Uptime: %v\n", health.Uptime)
	fmt.Printf("  Response Time: %v\n", health.ResponseTime)
	fmt.Printf("  Connections:\n")
	fmt.Printf("    Open: %d\n", health.Connections.Open)
	fmt.Printf("    In Use: %d\n", health.Connections.InUse)
	fmt.Printf("    Idle: %d\n", health.Connections.Idle)
	fmt.Printf("    Utilization: %.2f%%\n", health.Connections.Utilization)

	if health.ReadReplica != nil {
		fmt.Printf("  Read Replica:\n")
		fmt.Printf("    Open: %d\n", health.ReadReplica.Open)
		fmt.Printf("    Utilization: %.2f%%\n", health.ReadReplica.Utilization)
	}

	if health.PgxPool != nil {
		fmt.Printf("  PgxPool:\n")
		fmt.Printf("    Total Conns: %d\n", health.PgxPool.TotalConns)
		fmt.Printf("    Idle Conns: %d\n", health.PgxPool.IdleConns)
		fmt.Printf("    Acquired Conns: %d\n", health.PgxPool.AcquiredConns)
	}

	// Example 4: Retry Logic with Custom Strategy
	fmt.Println("\n=== Example 4: Retry Logic ===")
	retryConfig := dbx.DefaultRetryConfig()
	retryConfig.MaxAttempts = 5
	retryConfig.Strategy = dbx.RetryStrategyExponentialJitter
	retryConfig.OnRetry = func(attempt int, err error, delay time.Duration) {
		fmt.Printf("  Retry attempt %d after error (waiting %v): %v\n", attempt, delay, err)
	}

	// Simulate a retryable operation
	err = db.WithTx(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "INSERT INTO products (name, price) VALUES ($1, $2)", "Product B", 29.99)
		return err
	}, nil)

	if err != nil {
		log.Printf("Transaction failed: %v", err)
	} else {
		fmt.Println("✓ Transaction completed")
	}

	// Example 5: Circuit Breaker
	fmt.Println("\n=== Example 5: Circuit Breaker ===")
	cb := dbx.NewCircuitBreaker(3, 10*time.Second)

	for i := 0; i < 5; i++ {
		err := cb.Execute(func() error {
			return db.Ping(ctx)
		})

		if err == dbx.ErrCircuitBreakerOpen {
			fmt.Printf("  Attempt %d: Circuit breaker is OPEN\n", i+1)
		} else if err != nil {
			fmt.Printf("  Attempt %d: Error - %v\n", i+1, err)
		} else {
			fmt.Printf("  Attempt %d: Success\n", i+1)
		}
	}

	// Example 6: Error Classification
	fmt.Println("\n=== Example 6: Error Classification ===")
	_, err = db.QueryContext(ctx, "SELECT * FROM non_existent_table")
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
		fmt.Printf("  Is Retryable: %v\n", dbx.IsRetryable(err))
		fmt.Printf("  Is Not Found: %v\n", dbx.IsNotFound(err))
		fmt.Printf("  Is Constraint Violation: %v\n", dbx.IsConstraintViolation(err))
	}

	// Example 7: Readiness and Liveness Probes
	fmt.Println("\n=== Example 7: K8s Health Probes ===")
	if err := db.ReadinessCheck(ctx); err != nil {
		fmt.Printf("  Readiness: NOT READY - %v\n", err)
	} else {
		fmt.Println("  Readiness: READY ✓")
	}

	if err := db.LivenessCheck(ctx); err != nil {
		fmt.Printf("  Liveness: NOT ALIVE - %v\n", err)
	} else {
		fmt.Println("  Liveness: ALIVE ✓")
	}

	// Example 8: Connection Pool Statistics
	fmt.Println("\n=== Example 8: Connection Pool Stats ===")
	stats := odb.Stats()
	fmt.Printf("  Max Open Connections: %d\n", stats.MaxOpenConnections)
	fmt.Printf("  Open Connections: %d\n", stats.OpenConnections)
	fmt.Printf("  In Use: %d\n", stats.InUse)
	fmt.Printf("  Idle: %d\n", stats.Idle)
	fmt.Printf("  Wait Count: %d\n", stats.WaitCount)
	fmt.Printf("  Wait Duration: %v\n", stats.WaitDuration)
	fmt.Printf("  Max Idle Closed: %d\n", stats.MaxIdleClosed)
	fmt.Printf("  Max Lifetime Closed: %d\n", stats.MaxLifetimeClosed)

	// Example 9: Background Health Checker
	fmt.Println("\n=== Example 9: Background Health Checker ===")
	healthChecker := dbx.NewHealthChecker(db, 5*time.Second, logger)
	go healthChecker.Start(ctx)
	fmt.Println("✓ Background health checker started")

	// Wait a bit to see health checks
	time.Sleep(12 * time.Second)
	healthChecker.Stop()
	fmt.Println("✓ Background health checker stopped")

	fmt.Println("\n=== Advanced Examples Completed ===")
}
