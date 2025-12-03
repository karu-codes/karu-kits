package transactor

import "context"

type TxFn func(ctx context.Context) error
type TxFnResult[T any] func(ctx context.Context) (T, error)

type Transactor interface {
	Atomically(ctx context.Context, fn TxFn) error
}

// func (r *UserRepository) db(ctx context.Context) *db.Queries {
// 	tx := transactor.GetTx(ctx)

// 	if tx != nil {
// 		return r.queries.WithTx(tx)
// 	}

// 	return r.queries
// }
