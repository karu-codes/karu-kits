package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/karu-codes/karu-kits/kdbx"
	"github.com/karu-codes/karu-kits/klog"
)

func main() {
	// Initialize logger
	zapLogger, err := klog.InitProvider(true) // debug mode
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}
	logger := klog.NewSlogBuilder(zapLogger).Build()

	// Get database URL from environment
	databaseURL := os.Getenv("MYSQL_URL")
	if databaseURL == "" {
		databaseURL = "root:password@tcp(localhost:3306)/testdb"
		logger.Warn("MYSQL_URL not set, using default", "url", "root:***@tcp(localhost:3306)/testdb")
	}

	// Create database configuration
	config := kdbx.DefaultConfig(kdbx.DriverMySQL, databaseURL)
	config.Logger = logger
	config.LogQueries = true
	config.HealthCheckInterval = 30 * time.Second
	config.MySQLParseTime = true
	config.MySQLLocation = time.UTC

	// Create metrics collector
	metrics := kdbx.NewInMemoryMetricsCollector(1 * time.Second) // slow query threshold
	config.Metrics = metrics

	// Connect to database
	logger.Info("connecting to MySQL database...")
	ctx := context.Background()
	db, err := kdbx.NewMySQL(ctx, config)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	logger.Info("connected to database successfully")

	// Run examples
	if err := runExamples(ctx, db, logger); err != nil {
		log.Fatalf("example failed: %v", err)
	}

	// Show metrics
	showMetrics(metrics, logger)

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down gracefully...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", "error", err)
	}

	logger.Info("shutdown complete")
}

func runExamples(ctx context.Context, db *kdbx.MySQLDB, logger *slog.Logger) error {
	logger.Info("=== Starting Examples ===")

	// Example 1: Basic Query
	if err := exampleBasicQuery(ctx, db, logger); err != nil {
		return fmt.Errorf("basic query example failed: %w", err)
	}

	// Example 2: Transaction
	if err := exampleTransaction(ctx, db, logger); err != nil {
		return fmt.Errorf("transaction example failed: %w", err)
	}

	// Example 3: Batch Operations
	if err := exampleBatchOperations(ctx, db, logger); err != nil {
		return fmt.Errorf("batch operations example failed: %w", err)
	}

	// Example 4: Health Check
	if err := exampleHealthCheck(ctx, db, logger); err != nil {
		return fmt.Errorf("health check example failed: %w", err)
	}

	// Example 5: Connection Pool Stats
	examplePoolStats(db, logger)

	logger.Info("=== Examples Completed Successfully ===")
	return nil
}

func exampleBasicQuery(ctx context.Context, db *kdbx.MySQLDB, logger *slog.Logger) error {
	logger.Info("--- Example 1: Basic Query ---")

	// Simple query
	var result int
	row := db.QueryRow(ctx, "SELECT 1")
	if err := row.Scan(&result); err != nil {
		return err
	}

	logger.Info("query result", "value", result)
	return nil
}

func exampleTransaction(ctx context.Context, db *kdbx.MySQLDB, logger *slog.Logger) error {
	logger.Info("--- Example 2: Transaction ---")

	// Create a temporary table for this example
	_, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS temp_users (
			id CHAR(36) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			email VARCHAR(255) UNIQUE NOT NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
	`)
	if err != nil {
		return fmt.Errorf("failed to create temp table: %w", err)
	}

	// Clean up before starting
	_, _ = db.Exec(ctx, "DELETE FROM temp_users")

	// Use transaction with automatic retry
	err = db.WithTransaction(ctx, func(tx kdbx.Tx) error {
		// Insert first user
		userID1 := uuid.New().String()
		_, err := tx.Exec(ctx,
			"INSERT INTO temp_users (id, name, email) VALUES (?, ?, ?)",
			userID1, "Alice", "alice@example.com",
		)
		if err != nil {
			return err
		}

		logger.Info("inserted user", "id", userID1, "name", "Alice")

		// Insert second user
		userID2 := uuid.New().String()
		_, err = tx.Exec(ctx,
			"INSERT INTO temp_users (id, name, email) VALUES (?, ?, ?)",
			userID2, "Bob", "bob@example.com",
		)
		if err != nil {
			return err
		}

		logger.Info("inserted user", "id", userID2, "name", "Bob")

		// If we return an error here, both inserts will be rolled back
		return nil
	})

	if err != nil {
		return fmt.Errorf("transaction failed: %w", err)
	}

	// Verify data was inserted
	var count int
	row := db.QueryRow(ctx, "SELECT COUNT(*) FROM temp_users")
	if err := row.Scan(&count); err != nil {
		return err
	}

	logger.Info("transaction committed", "users_inserted", count)

	// Clean up
	_, _ = db.Exec(ctx, "DROP TABLE IF EXISTS temp_users")

	return nil
}

func exampleBatchOperations(ctx context.Context, db *kdbx.MySQLDB, logger *slog.Logger) error {
	logger.Info("--- Example 3: Batch Operations ---")

	// Create batch executor
	batch := kdbx.NewBatchExecutor(db)

	// Create temporary table
	_, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS temp_products (
			id CHAR(36) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			price DECIMAL(10, 2) NOT NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
	`)
	if err != nil {
		return fmt.Errorf("failed to create temp table: %w", err)
	}

	// Clean up before starting
	_, _ = db.Exec(ctx, "DELETE FROM temp_products")

	// Add multiple queries to batch
	products := []struct {
		name  string
		price float64
	}{
		{"Laptop", 999.99},
		{"Mouse", 29.99},
		{"Keyboard", 79.99},
		{"Monitor", 299.99},
	}

	for _, p := range products {
		id := uuid.New().String()
		batch.Add(
			"INSERT INTO temp_products (id, name, price) VALUES (?, ?, ?)",
			id, p.name, p.price,
		)
	}

	logger.Info("executing batch", "queries", batch.Len())

	// Execute all queries in a single transaction
	if err := batch.Execute(ctx); err != nil {
		return fmt.Errorf("batch execution failed: %w", err)
	}

	logger.Info("batch execution completed")

	// Verify
	var count int
	row := db.QueryRow(ctx, "SELECT COUNT(*) FROM temp_products")
	if err := row.Scan(&count); err != nil {
		return err
	}

	logger.Info("batch result", "products_inserted", count)

	// Clean up
	_, _ = db.Exec(ctx, "DROP TABLE IF EXISTS temp_products")

	return nil
}

func exampleHealthCheck(ctx context.Context, db *kdbx.MySQLDB, logger *slog.Logger) error {
	logger.Info("--- Example 4: Health Check ---")

	// Create health checker
	health := kdbx.NewHealthChecker(db)

	// Basic health check (liveness)
	check := health.Check(ctx)
	logger.Info("health check (liveness)",
		"status", check.Status,
		"message", check.Message,
		"duration", check.Duration,
	)

	// Detailed health check (readiness)
	detailedCheck := health.CheckDetailed(ctx)
	logger.Info("health check (readiness)",
		"status", detailedCheck.Status,
		"message", detailedCheck.Message,
		"duration", detailedCheck.Duration,
	)

	// Get JSON representation
	jsonData, err := detailedCheck.JSON()
	if err != nil {
		return err
	}

	logger.Info("health check JSON", "json", string(jsonData))

	// Custom health check
	customHealth := kdbx.NewCustomHealthChecker(db)
	customHealth.AddCheck("query_performance", kdbx.CheckQueryPerformance(100*time.Millisecond))

	customCheck := customHealth.CheckWithCustomChecks(ctx)
	logger.Info("custom health check",
		"status", customCheck.Status,
		"http_status", customCheck.HTTPStatusCode(),
	)

	return nil
}

func examplePoolStats(db *kdbx.MySQLDB, logger *slog.Logger) {
	logger.Info("--- Example 5: Connection Pool Stats ---")

	stats := db.Stats()
	logger.Info("connection pool statistics",
		"acquired_conns", stats.AcquiredConns,
		"idle_conns", stats.IdleConns,
		"total_conns", stats.TotalConns,
		"max_conns", stats.MaxConns,
	)
}

func showMetrics(collector *kdbx.InMemoryMetricsCollector, logger *slog.Logger) {
	logger.Info("--- Database Metrics ---")

	metrics := collector.Metrics()

	logger.Info("query metrics",
		"count", metrics.QueryCount,
		"errors", metrics.QueryErrorCount,
		"avg_duration", metrics.QueryAvgDuration,
		"p50_duration", metrics.QueryP50Duration,
		"p95_duration", metrics.QueryP95Duration,
		"p99_duration", metrics.QueryP99Duration,
	)

	logger.Info("exec metrics",
		"count", metrics.ExecCount,
		"errors", metrics.ExecErrorCount,
		"avg_duration", metrics.ExecAvgDuration,
	)

	logger.Info("transaction metrics",
		"count", metrics.TxCount,
		"commits", metrics.TxCommitCount,
		"rollbacks", metrics.TxRollbackCount,
		"errors", metrics.TxErrorCount,
		"avg_duration", metrics.TxAvgDuration,
	)

	// Show slow queries
	slowQueries := collector.SlowQueries()
	if len(slowQueries) > 0 {
		logger.Warn("slow queries detected", "count", len(slowQueries))
		for i, sq := range slowQueries {
			logger.Warn("slow query",
				"index", i+1,
				"query", sq.Query,
				"duration", sq.Duration,
				"timestamp", sq.Timestamp,
			)
		}
	}
}
