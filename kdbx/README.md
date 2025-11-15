# kdbx - Database Wrapper for Go

A production-ready database wrapper package for PostgreSQL and MySQL with connection pooling, transaction management, health checks, and comprehensive observability.

## Features

### Core Features
- ✅ **PostgreSQL Support** - Native `pgxpool` for high performance + `database/sql` compatibility
- ✅ **MySQL Support** - Using `go-sql-driver/mysql` (industry standard)
- ✅ **Connection Pooling** - Configurable with optimal defaults
- ✅ **Transaction Management** - Automatic retry logic with exponential backoff
- ✅ **Health Checks** - Liveness and readiness checks for Kubernetes
- ✅ **Observability** - Structured logging, metrics collection, query tracing
- ✅ **Error Handling** - Rich error classification with stack traces
- ✅ **Context-Aware** - All operations support context for timeouts and cancellation
- ✅ **sqlc Integration** - Seamless integration with sqlc-generated code
- ✅ **Graceful Shutdown** - Proper cleanup of connections

### Advanced Features
- 🔄 **Retry Logic** - Automatic retry for transient errors (deadlocks, timeouts)
- 🔍 **Query Logging** - Sanitized query logging (never logs sensitive data)
- 📊 **Metrics Collection** - In-memory metrics, logging metrics, or custom collectors
- 🏥 **Custom Health Checks** - Extensible health check system
- 💾 **Batch Operations** - Execute multiple queries in a single transaction
- 🔒 **Savepoints** - Nested transaction support with savepoints (PostgreSQL)
- 🎯 **Type-Safe** - Interface-based design for testing and mocking

## Installation

```bash
go get github.com/karu-codes/karu-kits/kdbx
```

## Quick Start

### PostgreSQL with pgxpool (Recommended)

```go
package main

import (
    "context"
    "log"

    "github.com/karu-codes/karu-kits/kdbx"
)

func main() {
    // Create configuration
    config := kdbx.DefaultConfig(
        kdbx.DriverPostgres,
        "postgresql://user:password@localhost:5432/mydb?sslmode=disable",
    )

    // Connect to database
    ctx := context.Background()
    db, err := kdbx.NewPostgres(ctx, config)
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Use the database
    var count int
    row := db.QueryRow(ctx, "SELECT COUNT(*) FROM users")
    if err := row.Scan(&count); err != nil {
        log.Fatal(err)
    }

    log.Printf("Users count: %d", count)
}
```

### MySQL

```go
package main

import (
    "context"
    "log"

    "github.com/karu-codes/karu-kits/kdbx"
)

func main() {
    // Create configuration
    config := kdbx.DefaultConfig(
        kdbx.DriverMySQL,
        "user:password@tcp(localhost:3306)/mydb",
    )

    // Connect to database
    ctx := context.Background()
    db, err := kdbx.NewMySQL(ctx, config)
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Use the database
    var count int
    row := db.QueryRow(ctx, "SELECT COUNT(*) FROM users")
    if err := row.Scan(&count); err != nil {
        log.Fatal(err)
    }

    log.Printf("Users count: %d", count)
}
```

## Configuration

### Default Configuration

The `DefaultConfig` function provides sensible defaults:

```go
config := kdbx.DefaultConfig(driver, databaseURL)

// Defaults:
// - MaxOpenConns: 25
// - MaxIdleConns: 5
// - ConnMaxLifetime: 30 minutes
// - ConnMaxIdleTime: 10 minutes
// - ConnectTimeout: 10 seconds
// - QueryTimeout: 30 seconds
// - HealthCheckInterval: 30 seconds
// - RetryAttempts: 3
// - RetryInitialBackoff: 100ms
// - RetryMaxBackoff: 5 seconds
```

### Custom Configuration

Use option functions to customize:

```go
config := kdbx.DefaultConfig(kdbx.DriverPostgres, databaseURL)

// Apply options
config.ApplyOptions(
    kdbx.WithMaxOpenConns(50),
    kdbx.WithMaxIdleConns(10),
    kdbx.WithConnMaxLifetime(1 * time.Hour),
    kdbx.WithQueryTimeout(1 * time.Minute),
    kdbx.WithLogger(logger),
    kdbx.WithMetrics(metricsCollector),
    kdbx.WithLogQueries(true),
)
```

### Available Options

```go
// Connection Pool
WithMaxOpenConns(n int)
WithMaxIdleConns(n int)
WithConnMaxLifetime(d time.Duration)
WithConnMaxIdleTime(d time.Duration)

// Timeouts
WithConnectTimeout(d time.Duration)
WithQueryTimeout(d time.Duration)

// Health Checks
WithHealthCheckInterval(d time.Duration)

// Retry Configuration
WithRetryAttempts(n int)
WithRetryBackoff(initial, max time.Duration)

// Observability
WithLogger(logger *slog.Logger)
WithMetrics(metrics MetricsCollector)
WithLogQueries(enabled bool)

// Mode
WithReadOnly(enabled bool)

// PostgreSQL Specific
WithPostgresSimpleProtocol(enabled bool)

// MySQL Specific
WithMySQLParseTime(enabled bool)
WithMySQLLocation(loc *time.Location)
WithMySQLMultiStatements(enabled bool) // Use with caution!
```

## Database Operations

### Basic Queries

```go
// Query (multiple rows)
rows, err := db.Query(ctx, "SELECT id, name FROM users WHERE active = $1", true)
if err != nil {
    return err
}
defer rows.Close()

for rows.Next() {
    var id int
    var name string
    if err := rows.Scan(&id, &name); err != nil {
        return err
    }
    fmt.Printf("User: %d - %s\n", id, name)
}

if err := rows.Err(); err != nil {
    return err
}

// QueryRow (single row)
var count int
row := db.QueryRow(ctx, "SELECT COUNT(*) FROM users")
if err := row.Scan(&count); err != nil {
    return err
}

// Exec (INSERT, UPDATE, DELETE)
result, err := db.Exec(ctx,
    "INSERT INTO users (name, email) VALUES ($1, $2)",
    "Alice", "alice@example.com",
)
if err != nil {
    return err
}

rowsAffected, _ := result.RowsAffected()
fmt.Printf("Inserted %d row(s)\n", rowsAffected)
```

### Transactions

#### Simple Transaction

```go
err := db.WithTransaction(ctx, func(tx kdbx.Tx) error {
    // Insert user
    result, err := tx.Exec(ctx,
        "INSERT INTO users (name, email) VALUES ($1, $2)",
        "Bob", "bob@example.com",
    )
    if err != nil {
        return err // Automatic rollback
    }

    // Insert profile
    _, err = tx.Exec(ctx,
        "INSERT INTO profiles (user_id, bio) VALUES ($1, $2)",
        userID, "Hello world",
    )
    if err != nil {
        return err // Automatic rollback
    }

    // Automatic commit if no error
    return nil
})
```

#### Manual Transaction Control

```go
tx, err := db.Begin(ctx)
if err != nil {
    return err
}
defer tx.Rollback(ctx) // Safe to call even after commit

// Do work...
_, err = tx.Exec(ctx, "INSERT INTO users (name) VALUES ($1)", "Charlie")
if err != nil {
    return err
}

// Commit
if err := tx.Commit(ctx); err != nil {
    return err
}
```

#### Transaction with Custom Options

```go
opts := kdbx.DefaultTxOptions()
opts.MaxRetries = 5 // Override retry attempts

err := kdbx.WithTransactionOptions(ctx, db, opts, func(tx kdbx.Tx) error {
    // Your transaction logic
    return nil
})
```

#### Nested Transactions (Savepoints)

```go
err := db.WithTransaction(ctx, func(tx kdbx.Tx) error {
    // Insert user
    userID, err := insertUser(ctx, tx, user)
    if err != nil {
        return err
    }

    // Try to insert optional profile (may fail)
    err = kdbx.NestedTransaction(ctx, tx, "profile", func(tx kdbx.Tx) error {
        return insertProfile(ctx, tx, userID, profile)
    })
    if err != nil {
        // Profile insertion failed, but user is still inserted
        log.Warn("failed to insert profile", "error", err)
    }

    return nil // Commit user even if profile failed
})
```

### Batch Operations

```go
// Create batch executor
batch := kdbx.NewBatchExecutor(db)

// Add queries
for _, user := range users {
    batch.Add(
        "INSERT INTO users (id, name, email) VALUES ($1, $2, $3)",
        user.ID, user.Name, user.Email,
    )
}

// Execute all queries in a single transaction
if err := batch.Execute(ctx); err != nil {
    return err
}

// Or execute with results
results, err := batch.ExecuteWithResults(ctx)
if err != nil {
    return err
}

for i, result := range results {
    affected, _ := result.RowsAffected()
    fmt.Printf("Query %d: %d rows affected\n", i, affected)
}
```

## Health Checks

### Basic Health Check (Liveness)

```go
// Create health checker
health := kdbx.NewHealthChecker(db)

// Perform health check
check := health.Check(ctx)

if check.IsHealthy() {
    fmt.Println("Database is healthy")
} else {
    fmt.Printf("Database is unhealthy: %s\n", check.Message)
}

// Get HTTP status code for health endpoint
statusCode := check.HTTPStatusCode() // 200, 429, or 503

// Get JSON representation
jsonData, _ := check.JSON()
fmt.Println(string(jsonData))
```

### Detailed Health Check (Readiness)

```go
// More thorough check including test query
check := health.CheckDetailed(ctx)

fmt.Printf("Status: %s\n", check.Status)
fmt.Printf("Duration: %s\n", check.Duration)
fmt.Printf("Details: %+v\n", check.Details)
```

### Custom Health Checks

```go
// Create custom health checker
customHealth := kdbx.NewCustomHealthChecker(db)

// Add custom checks
customHealth.AddCheck("query_performance",
    kdbx.CheckQueryPerformance(100*time.Millisecond))

customHealth.AddCheck("users_table_exists",
    kdbx.CheckTableExists("users"))

// Add your own custom check
customHealth.AddCheck("business_logic", func(ctx context.Context, db kdbx.Database) error {
    var count int
    row := db.QueryRow(ctx, "SELECT COUNT(*) FROM critical_table")
    if err := row.Scan(&count); err != nil {
        return err
    }

    if count == 0 {
        return fmt.Errorf("critical_table is empty")
    }

    return nil
})

// Run all checks
check := customHealth.CheckWithCustomChecks(ctx)
```

### Health Check Caching

```go
// Cache health checks for 5 seconds to prevent DoS
health := kdbx.NewHealthChecker(db).WithCacheDuration(5 * time.Second)

// Subsequent calls within 5 seconds return cached result
check1 := health.Check(ctx) // Performs actual check
check2 := health.Check(ctx) // Returns cached result (fast)
```

## Observability

### Logging

kdbx integrates with your existing `klog` package:

```go
// Create logger
zapLogger, _ := klog.InitProvider(true)
logger := klog.NewSlogBuilder(zapLogger).Build()

// Configure database with logger
config := kdbx.DefaultConfig(kdbx.DriverPostgres, databaseURL)
config.Logger = logger
config.LogQueries = true // Enable query logging

db, _ := kdbx.NewPostgres(ctx, config)
```

### Metrics Collection

#### In-Memory Metrics (Development/Testing)

```go
// Create in-memory metrics collector
metrics := kdbx.NewInMemoryMetricsCollector(1 * time.Second) // slow query threshold

config := kdbx.DefaultConfig(kdbx.DriverPostgres, databaseURL)
config.Metrics = metrics

db, _ := kdbx.NewPostgres(ctx, config)

// Later, retrieve metrics
m := metrics.Metrics()

fmt.Printf("Queries: %d (errors: %d)\n", m.QueryCount, m.QueryErrorCount)
fmt.Printf("Avg duration: %s\n", m.QueryAvgDuration)
fmt.Printf("P95 duration: %s\n", m.QueryP95Duration)
fmt.Printf("P99 duration: %s\n", m.QueryP99Duration)

// Get slow queries
slowQueries := metrics.SlowQueries()
for _, sq := range slowQueries {
    fmt.Printf("Slow query: %s (took %s)\n", sq.Query, sq.Duration)
}
```

#### Logging Metrics

```go
// Log metrics instead of storing in memory
metrics := kdbx.NewLoggingMetricsCollector(logger)

config.Metrics = metrics
```

#### Custom Metrics Collector

Implement the `MetricsCollector` interface:

```go
type MetricsCollector interface {
    RecordQuery(ctx context.Context, query string, duration time.Duration, err error)
    RecordExec(ctx context.Context, query string, duration time.Duration, err error)
    RecordTransaction(ctx context.Context, duration time.Duration, committed bool, err error)
    RecordPoolStats(stats PoolStats)
}

// Example: Prometheus metrics collector
type PrometheusCollector struct {
    queryDuration *prometheus.HistogramVec
    queryErrors   *prometheus.CounterVec
    // ...
}

func (p *PrometheusCollector) RecordQuery(ctx context.Context, query string, duration time.Duration, err error) {
    status := "success"
    if err != nil {
        status = "error"
        p.queryErrors.WithLabelValues(status).Inc()
    }
    p.queryDuration.WithLabelValues(status).Observe(duration.Seconds())
}
```

#### Composite Metrics (Multiple Collectors)

```go
// Combine multiple collectors
composite := kdbx.NewCompositeMetricsCollector(
    kdbx.NewLoggingMetricsCollector(logger),
    prometheusCollector,
    customCollector,
)

config.Metrics = composite
```

### Connection Pool Monitoring

```go
// Get pool statistics
stats := db.Stats()

fmt.Printf("Acquired: %d\n", stats.AcquiredConns)
fmt.Printf("Idle: %d\n", stats.IdleConns)
fmt.Printf("Total: %d\n", stats.TotalConns)
fmt.Printf("Max: %d\n", stats.MaxConns)
fmt.Printf("New connections: %d\n", stats.NewConnsCount)
fmt.Printf("Max lifetime destroys: %d\n", stats.MaxLifetimeDestroyCount)
fmt.Printf("Max idle destroys: %d\n", stats.MaxIdleDestroyCount)

// Calculate utilization
utilization := float64(stats.TotalConns) / float64(stats.MaxConns) * 100
fmt.Printf("Pool utilization: %.2f%%\n", utilization)
```

## Error Handling

### Error Classification

kdbx automatically classifies database errors:

```go
err := db.Exec(ctx, "INSERT INTO users (email) VALUES ($1)", "duplicate@example.com")
if err != nil {
    if kdbx.IsUniqueViolation(err) {
        fmt.Println("Email already exists")
        return ErrEmailAlreadyExists
    }

    if kdbx.IsForeignKeyViolation(err) {
        fmt.Println("Referenced record does not exist")
        return ErrInvalidReference
    }

    if kdbx.IsDeadlock(err) {
        fmt.Println("Deadlock detected - will retry")
        // Automatic retry happens in WithTransaction
    }

    if kdbx.IsNotFound(err) {
        fmt.Println("Record not found")
        return ErrUserNotFound
    }

    // Check if retryable
    if kdbx.IsRetryable(err) {
        fmt.Println("Transient error - safe to retry")
    }

    return err
}
```

### Error Details

All errors are wrapped with rich context:

```go
var kerr *errors.Error
if errors.As(err, &kerr) {
    fmt.Printf("Code: %s\n", kerr.Code)
    fmt.Printf("Message: %s\n", kerr.Message)
    fmt.Printf("Stack trace: %s\n", kerr.StackTrace)
    fmt.Printf("Details: %+v\n", kerr.Details)
}
```

## sqlc Integration

### PostgreSQL with sqlc

1. Create `sqlc.yaml`:

```yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "queries"
    schema: "schema"
    gen:
      go:
        package: "db"
        out: "db"
        sql_package: "pgx/v5"          # Use pgxpool
        emit_interface: true
        emit_pointers_for_null_types: true
        emit_prepared_queries: false
```

2. Generate code:

```bash
sqlc generate
```

3. Use with kdbx:

```go
import "yourproject/db"

// Create kdbx database
config := kdbx.DefaultConfig(kdbx.DriverPostgres, databaseURL)
kdb, _ := kdbx.NewPostgres(ctx, config)

// Create sqlc queries (uses pgxpool)
queries := db.New(kdb.Pool())

// Use generated methods
users, err := queries.ListUsers(ctx, db.ListUsersParams{
    Limit:  10,
    Offset: 0,
})

// In transactions
err = kdb.WithTransaction(ctx, func(tx kdbx.Tx) error {
    // Get underlying pgx.Tx
    pgxTx := tx.(*kdbx.pgxTxAdapter) // Type assertion needed

    // Create queries with transaction
    qtx := queries.WithTx(pgxTx.tx)

    // Use transaction
    user, err := qtx.CreateUser(ctx, db.CreateUserParams{
        Email:        "user@example.com",
        Username:     "user",
        PasswordHash: "hash",
    })

    return err
})
```

### MySQL with sqlc

1. Create `sqlc.yaml`:

```yaml
version: "2"
sql:
  - engine: "mysql"
    queries: "queries"
    schema: "schema"
    gen:
      go:
        package: "mysqldb"
        out: "db"
        sql_package: "database/sql"    # Use standard library
        emit_interface: true
        emit_pointers_for_null_types: true
```

2. Use with kdbx:

```go
// Create kdbx database
config := kdbx.DefaultConfig(kdbx.DriverMySQL, databaseURL)
kdb, _ := kdbx.NewMySQL(ctx, config)

// Create sqlc queries (uses *sql.DB)
queries := mysqldb.New(kdb.DB())

// Use normally
users, err := queries.ListUsers(ctx, mysqldb.ListUsersParams{
    Limit:  10,
    Offset: 0,
})
```

## Best Practices

### 1. Connection Pool Sizing

```go
// Start conservative
config.MaxOpenConns = 25
config.MaxIdleConns = 5

// Monitor with metrics
stats := db.Stats()
utilization := float64(stats.TotalConns) / float64(stats.MaxConns)

// Adjust based on load
if utilization > 0.8 {
    // Consider increasing MaxOpenConns
}
```

### 2. Always Use Contexts

```go
// ✅ Good: With timeout
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
result, err := db.Query(ctx, query, args...)

// ❌ Bad: No timeout
result, err := db.Query(context.Background(), query, args...)
```

### 3. Defer Rollback in Transactions

```go
tx, err := db.Begin(ctx)
if err != nil {
    return err
}
defer tx.Rollback(ctx) // Always defer rollback (safe even after commit)

// Do work...

return tx.Commit(ctx)
```

### 4. Use WithTransaction for Automatic Retry

```go
// ✅ Good: Automatic retry for deadlocks
err := db.WithTransaction(ctx, func(tx kdbx.Tx) error {
    // Transaction logic
    return nil
})

// ❌ Less ideal: Manual transaction without retry
tx, _ := db.Begin(ctx)
// ...no retry logic...
```

### 5. Never Log Query Parameters

```go
// ✅ Good: Use sanitized query
logger.Info("executing query", "query", kdbx.SanitizeQuery(query))

// ❌ Bad: May log passwords, tokens, etc.
logger.Info("executing query", "query", query, "args", args)
```

### 6. Close Rows

```go
rows, err := db.Query(ctx, query)
if err != nil {
    return err
}
defer rows.Close() // Always close rows

for rows.Next() {
    // ...
}

return rows.Err() // Check for iteration errors
```

### 7. Graceful Shutdown

```go
// Setup shutdown handler
quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
<-quit

// Graceful shutdown with timeout
shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

if err := db.Shutdown(shutdownCtx); err != nil {
    logger.Error("shutdown error", "error", err)
}
```

## Advanced Usage

### Read Replicas (Future Enhancement)

```go
// TODO: Not yet implemented
// config.ReadReplicas = []string{
//     "postgresql://replica1:5432/db",
//     "postgresql://replica2:5432/db",
// }
```

### Custom Retry Logic

```go
// Override retry configuration per-transaction
opts := &kdbx.TxOptions{
    MaxRetries: 5,
}

err := kdbx.WithTransactionOptions(ctx, db, opts, func(tx kdbx.Tx) error {
    // Your logic
    return nil
})
```

## Examples

Complete examples are available in the `example/` directory:

- [`example/postgres/main.go`](example/postgres/main.go) - PostgreSQL with pgxpool
- [`example/mysql/main.go`](example/mysql/main.go) - MySQL example

Sample sqlc configurations and queries:

- [`example/postgres/sqlc.yaml`](example/postgres/sqlc.yaml) - PostgreSQL sqlc config
- [`example/postgres/queries/`](example/postgres/queries/) - Sample SQL queries
- [`example/mysql/sqlc.yaml`](example/mysql/sqlc.yaml) - MySQL sqlc config
- [`example/mysql/queries/`](example/mysql/queries/) - Sample SQL queries

## Testing

### Mock Interface for Unit Tests

```go
// kdbx provides interfaces that can be mocked
type MockDB struct {
    QueryFunc    func(ctx context.Context, query string, args ...interface{}) (kdbx.Rows, error)
    QueryRowFunc func(ctx context.Context, query string, args ...interface{}) kdbx.Row
    ExecFunc     func(ctx context.Context, query string, args ...interface{}) (kdbx.Result, error)
    // ...
}

func (m *MockDB) Query(ctx context.Context, query string, args ...interface{}) (kdbx.Rows, error) {
    return m.QueryFunc(ctx, query, args...)
}

// Use in tests
func TestMyFunction(t *testing.T) {
    mockDB := &MockDB{
        QueryFunc: func(ctx context.Context, query string, args ...interface{}) (kdbx.Rows, error) {
            // Return mock data
            return mockRows, nil
        },
    }

    // Test your function with mockDB
    result, err := MyFunction(ctx, mockDB)
    // ...
}
```

### Integration Tests

```go
func TestIntegration(t *testing.T) {
    // Use test database
    config := kdbx.DefaultConfig(
        kdbx.DriverPostgres,
        "postgresql://postgres:postgres@localhost:5432/test_db",
    )

    db, err := kdbx.NewPostgres(context.Background(), config)
    if err != nil {
        t.Fatal(err)
    }
    defer db.Close()

    // Always rollback in tests
    tx, _ := db.Begin(context.Background())
    defer tx.Rollback(context.Background())

    // Run tests using tx
    // ...
}
```

## Performance Tips

1. **Use pgxpool for PostgreSQL** - 3x better performance than database/sql
2. **Enable prepared statement caching** - pgx does this automatically
3. **Batch operations** - Use `BatchExecutor` for multiple inserts/updates
4. **Monitor slow queries** - Use `InMemoryMetricsCollector` to identify bottlenecks
5. **Tune connection pool** - Start with defaults, adjust based on metrics
6. **Use connection pooling** - Never create a new database connection per request

## Troubleshooting

### "too many connections"

```go
// Reduce MaxOpenConns
config.MaxOpenConns = 10

// Ensure connections are released
defer rows.Close()
defer tx.Rollback(ctx)
```

### Slow queries

```go
// Enable slow query tracking
metrics := kdbx.NewInMemoryMetricsCollector(100 * time.Millisecond)
config.Metrics = metrics

// Check slow queries
for _, sq := range metrics.SlowQueries() {
    log.Printf("Slow query: %s (took %s)", sq.Query, sq.Duration)
}
```

### Connection leaks

```go
// Monitor pool stats
stats := db.Stats()
log.Printf("Acquired: %d, Idle: %d, Total: %d, Max: %d",
    stats.AcquiredConns, stats.IdleConns, stats.TotalConns, stats.MaxConns)

// If AcquiredConns keeps growing, you have a leak
// Always defer rows.Close() and tx.Rollback()
```

## Contributing

Contributions are welcome! Please ensure:

1. All tests pass
2. Code follows existing style
3. Add tests for new features
4. Update documentation

## License

[Your License Here]

## Credits

Built with:
- [pgx](https://github.com/jackc/pgx) - PostgreSQL driver
- [go-sql-driver/mysql](https://github.com/go-sql-driver/mysql) - MySQL driver
- [klog](../klog) - Structured logging
- [errors](../errors) - Error handling

---

**Made with ❤️ by the Karu team**
