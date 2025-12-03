# kpgx

`kpgx` is a wrapper around [pgx](https://github.com/jackc/pgx) that provides:
- Simple configuration and initialization of `pgxpool`.
- Application-layer transaction management using `context.Context`.
- Seamless integration with [sqlc](https://sqlc.dev/).

## Installation

```bash
go get github.com/karu-codes/karu-kits/kpgx
```

## Usage

### Initialization

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/karu-codes/karu-kits/kpgx"
)

func main() {
	ctx := context.Background()

	cfg := kpgx.Config{
		ConnString:      "postgres://user:password@localhost:5432/dbname",
		MaxConns:        10,
		MinConns:        2,
		MaxConnLifetime: time.Hour,
	}

	db, err := kpgx.New(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to connect to DB: %v", err)
	}
	defer db.Close()

	// Use db...
}
```

### Transaction Management



`kpgx` allows you to run a function within a transaction. If a transaction is already present in the context, it will be reused.

#### ExecTx (No Result)

```go
err := kpgx.ExecTx(ctx, db, func(ctx context.Context) error {
    // Perform database operations here using ctx
    // If this function returns an error, the transaction will be rolled back.
    // If it returns nil, the transaction will be committed.
    return nil
})
```

#### ExecTxWithResult (With Result)

```go
user, err := kpgx.ExecTxWithResult(ctx, db, func(ctx context.Context) (*User, error) {
    // Perform operations and return a result
    return &User{Name: "John"}, nil
})
```

### Integration with sqlc

To use `kpgx` with `sqlc`, you need to pass the `DBTX` interface to your `sqlc` queries. `kpgx` provides a helper `GetDBTX(ctx)` that returns either the transaction (if one exists in the context) or the pool.

Assuming you have generated `sqlc` code in a `repository` package:

```go
// In your repository or service layer
type Service struct {
	db      *kpgx.DB
	queries *repository.Queries
}

func (s *Service) CreateUser(ctx context.Context, name string) error {
	return kpgx.ExecTx(ctx, s.db, func(ctx context.Context) error {
		// Get the DBTX (either Tx or Pool) from context
		dbtx := s.db.GetDBTX(ctx)
		
		// Use the queries with the correct DBTX
		q := s.queries.WithTx(dbtx) 
        
        // ...
        return nil
	})
}
```

### Helpers

`kpgx` provides convenient helpers to convert Go types to `pgtype` types, handling pointers and zero values gracefully.

- `ToUUID(uuid.UUID) pgtype.UUID` / `ToUUIDPtr(*uuid.UUID) pgtype.UUID`
- `ToTimestamp(time.Time) pgtype.Timestamp` / `ToTimestampPtr(*time.Time) pgtype.Timestamp`
- `ToTimestamptz(time.Time) pgtype.Timestamptz` / `ToTimestamptzPtr(*time.Time) pgtype.Timestamptz`
- `ToDate(time.Time) pgtype.Date` / `ToDatePtr(*time.Time) pgtype.Date`
- `ToText(string) pgtype.Text` / `ToTextPtr(*string) pgtype.Text`
- `ToInt4(int32) pgtype.Int4` / `ToInt4Ptr(*int32) pgtype.Int4`
- `ToInt8(int64) pgtype.Int8` / `ToInt8Ptr(*int64) pgtype.Int8`
- `ToBool(bool) pgtype.Bool` / `ToBoolPtr(*bool) pgtype.Bool`
- `ToFloat8(float64) pgtype.Float8` / `ToFloat8Ptr(*float64) pgtype.Float8`

**Note on sqlc generation:**
Ensure your `sqlc` configuration generates the `DBTX` interface or you use the standard one that `pgx` satisfies. `kpgx.DBTX` is compatible with standard `pgx` interfaces.
