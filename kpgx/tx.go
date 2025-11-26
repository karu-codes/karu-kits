package kpgx

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type txKey struct{}

// RunInTx executes the given function within a database transaction.
// If a transaction is already present in the context, it reuses it (nested transaction behavior depends on pgx, usually just reusing the same tx).
// If no transaction is present, it starts a new one.
func (db *DB) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	// Check if a transaction is already in the context
	if _, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		// Already in a transaction, just run the function
		return fn(ctx)
	}

	// Start a new transaction
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Defer rollback in case of panic or error
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// Inject transaction into context
	ctxWithTx := context.WithValue(ctx, txKey{}, tx)

	// Execute the function
	if err := fn(ctxWithTx); err != nil {
		return err
	}

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// TxFromContext returns the transaction from the context if it exists.
func TxFromContext(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(pgx.Tx)
	return tx, ok
}
