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

```go
err := db.RunInTx(ctx, func(ctx context.Context) error {
    // Perform database operations here using ctx
    // If this function returns an error, the transaction will be rolled back.
    // If it returns nil, the transaction will be committed.
    return nil
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
	return s.db.RunInTx(ctx, func(ctx context.Context) error {
		// Get the DBTX (either Tx or Pool) from context
		dbtx := s.db.GetDBTX(ctx)
		
		// Use the queries with the correct DBTX
		q := s.queries.WithTx(dbtx) // Or however your sqlc generated code handles it, often just passing dbtx to New() or methods
        
        // If using standard sqlc "New(dbtx)":
        // repo := repository.New(dbtx)
        // return repo.CreateUser(ctx, name)
        
        return nil
	})
}
```

**Note on sqlc generation:**
Ensure your `sqlc` configuration generates the `DBTX` interface or you use the standard one that `pgx` satisfies. `kpgx.DBTX` is compatible with standard `pgx` interfaces.
