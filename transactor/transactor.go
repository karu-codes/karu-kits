package transactor

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/karu-codes/karu-kits/kpgx"
)

type txKey struct{}

type SQLTransactor struct {
	db *kpgx.DB
}

func NewTransactor(db *kpgx.DB) *SQLTransactor {
	return &SQLTransactor{db: db}
}

type Options struct {
}

func (t *SQLTransactor) Atomically(ctx context.Context, fn TxFn) error {
	if _, ok := ctx.Value(txKey{}).(*pgx.Tx); ok {
		return fn(ctx)
	}

	tx, err := t.db.Pool().Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		} else if err != nil {
			_ = tx.Rollback(ctx)
		} else {
			if commitErr := tx.Commit(ctx); commitErr != nil {
				err = fmt.Errorf("failed to commit transaction: %w", commitErr)
			}
		}
	}()

	txCtx := context.WithValue(ctx, txKey{}, tx)

	err = fn(txCtx)

	return err
}
