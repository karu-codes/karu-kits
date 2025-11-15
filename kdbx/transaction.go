package kdbx

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"time"
)

// withRetry executes a function with exponential backoff retry logic.
// It only retries on transient errors (timeouts, deadlocks, unavailable).
func withRetry(ctx context.Context, config *Config, fn func(context.Context) error) error {
	if config.RetryAttempts <= 0 {
		// No retries configured, execute once
		return fn(ctx)
	}

	var lastErr error
	backoff := config.RetryInitialBackoff

	for attempt := 0; attempt <= config.RetryAttempts; attempt++ {
		// Check if context is already cancelled
		select {
		case <-ctx.Done():
			return WrapError(ctx.Err(), "context cancelled before retry")
		default:
		}

		// Execute the function
		err := fn(ctx)
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !IsRetryable(err) {
			return err
		}

		// Don't sleep after last attempt
		if attempt >= config.RetryAttempts {
			break
		}

		// Calculate backoff with exponential growth
		sleepDuration := calculateBackoff(backoff, attempt, config.RetryMaxBackoff)

		if config.Logger != nil {
			config.Logger.Warn("retrying transaction",
				"attempt", attempt+1,
				"max_attempts", config.RetryAttempts+1,
				"backoff", sleepDuration,
				"error", err,
			)
		}

		// Sleep with context cancellation support
		select {
		case <-time.After(sleepDuration):
			// Continue to next attempt
		case <-ctx.Done():
			return WrapError(ctx.Err(), "context cancelled during retry backoff")
		}
	}

	return WrapError(lastErr, "maximum retry attempts exceeded")
}

// calculateBackoff calculates the backoff duration using exponential backoff with jitter.
func calculateBackoff(initialBackoff time.Duration, attempt int, maxBackoff time.Duration) time.Duration {
	// Exponential backoff: initial * 2^attempt
	backoff := float64(initialBackoff) * math.Pow(2, float64(attempt))

	// Cap at max backoff
	if backoff > float64(maxBackoff) {
		backoff = float64(maxBackoff)
	}

	// Add jitter (±10%) to prevent thundering herd
	jitter := backoff * 0.1 * (2*randomFloat() - 1)
	backoff += jitter

	return time.Duration(backoff)
}

// randomFloat returns a pseudo-random float64 in [0.0, 1.0).
// Uses math/rand/v2 which provides better randomness and performance.
func randomFloat() float64 {
	return rand.Float64()
}

// TxFunc is a function that executes within a transaction.
type TxFunc func(tx Tx) error

// TxOptions holds transaction configuration options.
type TxOptions struct {
	// Isolation specifies the transaction isolation level.
	// Not used currently, but reserved for future use.
	Isolation string

	// ReadOnly marks the transaction as read-only.
	ReadOnly bool

	// MaxRetries overrides the config retry attempts for this transaction.
	// Set to -1 to use config default.
	MaxRetries int
}

// DefaultTxOptions returns default transaction options.
func DefaultTxOptions() *TxOptions {
	return &TxOptions{
		ReadOnly:   false,
		MaxRetries: -1,
	}
}

// WithTransactionOptions executes a function within a transaction with custom options.
func WithTransactionOptions(ctx context.Context, db Database, opts *TxOptions, fn TxFunc) error {
	// Extract config from database implementation
	var config *Config

	// Use type assertion with safety check
	if pgDB, ok := db.(*PostgresDB); ok {
		config = pgDB.config
	} else if mysqlDB, ok := db.(*MySQLDB); ok {
		config = mysqlDB.config
	} else {
		return fmt.Errorf("unsupported database type")
	}

	// Override retry attempts if specified
	if opts.MaxRetries >= 0 {
		tempConfig := *config
		tempConfig.RetryAttempts = opts.MaxRetries
		config = &tempConfig
	}

	return withRetry(ctx, config, func(ctx context.Context) error {
		tx, err := db.Begin(ctx)
		if err != nil {
			return err
		}

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

// SavepointTx extends Tx with savepoint support (PostgreSQL only).
type SavepointTx interface {
	Tx

	// Savepoint creates a savepoint with the given name.
	Savepoint(ctx context.Context, name string) error

	// RollbackToSavepoint rolls back to the specified savepoint.
	RollbackToSavepoint(ctx context.Context, name string) error

	// ReleaseSavepoint releases the specified savepoint.
	ReleaseSavepoint(ctx context.Context, name string) error
}

// savepointTx wraps a Tx and adds savepoint support.
type savepointTx struct {
	Tx
}

// Savepoint creates a savepoint with the given name.
// Note: Savepoint names cannot be parameterized in SQL, so we validate the name
// to prevent SQL injection.
func (tx *savepointTx) Savepoint(ctx context.Context, name string) error {
	if err := validateSavepointName(name); err != nil {
		return err
	}
	query := "SAVEPOINT " + name
	_, err := tx.Exec(ctx, query)
	return err
}

// RollbackToSavepoint rolls back to the specified savepoint.
func (tx *savepointTx) RollbackToSavepoint(ctx context.Context, name string) error {
	if err := validateSavepointName(name); err != nil {
		return err
	}
	query := "ROLLBACK TO SAVEPOINT " + name
	_, err := tx.Exec(ctx, query)
	return err
}

// ReleaseSavepoint releases the specified savepoint.
func (tx *savepointTx) ReleaseSavepoint(ctx context.Context, name string) error {
	if err := validateSavepointName(name); err != nil {
		return err
	}
	query := "RELEASE SAVEPOINT " + name
	_, err := tx.Exec(ctx, query)
	return err
}

// validateSavepointName validates that a savepoint name is safe to use in SQL.
// Savepoint names must start with a letter or underscore and contain only
// alphanumeric characters and underscores.
func validateSavepointName(name string) error {
	if name == "" {
		return fmt.Errorf("savepoint name cannot be empty")
	}

	// Check first character
	first := name[0]
	if !((first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z') || first == '_') {
		return fmt.Errorf("savepoint name must start with a letter or underscore")
	}

	// Check remaining characters
	for i := 1; i < len(name); i++ {
		c := name[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return fmt.Errorf("savepoint name can only contain alphanumeric characters and underscores")
		}
	}

	return nil
}

// WithSavepoint wraps a Tx with savepoint support.
func WithSavepoint(tx Tx) SavepointTx {
	return &savepointTx{Tx: tx}
}

// NestedTransaction executes a function within a nested transaction using savepoints.
// This is useful for implementing partial rollbacks in complex business logic.
//
// Example:
//
//	db.WithTransaction(ctx, func(tx Tx) error {
//	    // Insert user
//	    userID, err := insertUser(ctx, tx, user)
//	    if err != nil {
//	        return err
//	    }
//
//	    // Try to insert optional profile (may fail)
//	    err = NestedTransaction(ctx, tx, "profile", func(tx Tx) error {
//	        return insertProfile(ctx, tx, userID, profile)
//	    })
//	    if err != nil {
//	        // Profile insertion failed, but user is still inserted
//	        log.Warn("failed to insert profile", "error", err)
//	    }
//
//	    return nil
//	})
func NestedTransaction(ctx context.Context, tx Tx, savepointName string, fn TxFunc) error {
	stx := WithSavepoint(tx)

	// Create savepoint
	if err := stx.Savepoint(ctx, savepointName); err != nil {
		return WrapError(err, "failed to create savepoint")
	}

	// Execute function
	err := fn(tx)
	if err != nil {
		// Rollback to savepoint on error
		if rbErr := stx.RollbackToSavepoint(ctx, savepointName); rbErr != nil {
			return WrapError(rbErr, "failed to rollback to savepoint")
		}
		return err
	}

	// Release savepoint on success
	if err := stx.ReleaseSavepoint(ctx, savepointName); err != nil {
		return WrapError(err, "failed to release savepoint")
	}

	return nil
}

// BatchExecutor provides batch execution capabilities for multiple operations.
type BatchExecutor struct {
	db      Database
	queries []batchQuery
}

type batchQuery struct {
	query string
	args  []interface{}
}

// NewBatchExecutor creates a new batch executor.
func NewBatchExecutor(db Database) *BatchExecutor {
	return &BatchExecutor{
		db:      db,
		queries: make([]batchQuery, 0),
	}
}

// Add adds a query to the batch.
func (b *BatchExecutor) Add(query string, args ...interface{}) {
	b.queries = append(b.queries, batchQuery{
		query: query,
		args:  args,
	})
}

// Execute executes all queries in a single transaction.
// If any query fails, the entire batch is rolled back.
func (b *BatchExecutor) Execute(ctx context.Context) error {
	return b.db.WithTransaction(ctx, func(tx Tx) error {
		for i, q := range b.queries {
			if _, err := tx.Exec(ctx, q.query, q.args...); err != nil {
				return WrapError(err, fmt.Sprintf("batch query %d failed", i))
			}
		}
		return nil
	})
}

// ExecuteWithResults executes all queries and returns results.
func (b *BatchExecutor) ExecuteWithResults(ctx context.Context) ([]Result, error) {
	results := make([]Result, 0, len(b.queries))

	err := b.db.WithTransaction(ctx, func(tx Tx) error {
		for i, q := range b.queries {
			result, err := tx.Exec(ctx, q.query, q.args...)
			if err != nil {
				return WrapError(err, fmt.Sprintf("batch query %d failed", i))
			}
			results = append(results, result)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return results, nil
}

// Clear clears all queries from the batch.
func (b *BatchExecutor) Clear() {
	b.queries = b.queries[:0]
}

// Len returns the number of queries in the batch.
func (b *BatchExecutor) Len() int {
	return len(b.queries)
}
