package transactor

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func WithResult[T any](ctx context.Context, t Transactor, fn TxFnResult[T]) (T, error) {
	var result T
	err := t.Atomically(ctx, func(txCtx context.Context) error {
		var err error
		result, err = fn(txCtx)
		return err
	})
	return result, err
}

func GetTx(ctx context.Context) pgx.Tx {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}

	return nil
}
