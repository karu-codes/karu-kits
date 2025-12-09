# Karu Kits: Transactor Package

A clean, safe abstraction for managing database transactions in Go applications. It is designed to work seamlessly with `karu-kits/kpgx` (a `pgx` wrapper) and supports context propagation, nested transaction reuse, and panic safety.

## 1. Overview

The `transactor` package decouples business logic from transaction management mechanics. It allows you to:

*   **Execute Atomically**: Run a block of code within a database transaction.
*   **Context Propagation**: Automatically injects the transaction into the `context.Context`, so downstream repositories can pick it up without passing `tx` arguments explicitly.
*   **Safety**: Automatically handles `Commit` on success and `Rollback` on error or panic.
*   **Nesting Support**: Detects existing transactions in the context and reuses them, allowing for composable business logic.
*   **Generics Support**: Helper functions to return values from transactional blocks.

## 2. Core Concepts

### 2.1 The Interface

The core abstraction is the `Transactor` interface:

```go
type Transactor interface {
    // Atomically executes the given function within a transaction.
    Atomically(ctx context.Context, fn TxFn) error
}

type TxFn func(ctx context.Context) error
```

### 2.2 SQLTransactor

The standard implementation `SQLTransactor` works with `kpgx.DB`.

```go
type SQLTransactor struct {
    db *kpgx.DB
}
```

## 3. Installation

```bash
go get github.com/karu-codes/karu-kits/transactor
```

## 4. Usage Guide

### 4.1 Initialization

Initialize the transactor with your database connection.

```go
import (
    "github.com/karu-codes/karu-kits/kpgx"
    "github.com/karu-codes/karu-kits/transactor"
)

func main() {
    // Initialize DB
    db, _ := kpgx.New(context.Background(), "postgres://...")
    
    // Create Transactor
    txManager := transactor.NewTransactor(db)
    
    // Inject into Services
    userService := NewUserService(txManager)
}
```

### 4.2 Basic Transaction (Void Return)

Use `Atomically` for operations that perform side effects but don't need to return a value.

```go
func (s *UserService) CreateUser(ctx context.Context, req CreateUserReq) error {
    return s.transactor.Atomically(ctx, func(txCtx context.Context) error {
        // Step 1: Insert User (uses txCtx)
        if err := s.userRepo.Create(txCtx, req.User); err != nil {
            return err // Triggers Rollback
        }

        // Step 2: Create Profile (uses txCtx)
        if err := s.profileRepo.Create(txCtx, req.Profile); err != nil {
            return err // Triggers Rollback
        }

        return nil // Triggers Commit
    })
}
```

### 4.3 Transaction with Result

Use the `WithResult` generic helper when you need to return data from the transaction.

```go
func (s *UserService) CreateAndReturnUser(ctx context.Context, email string) (*User, error) {
    return transactor.WithResult(ctx, s.transactor, func(txCtx context.Context) (*User, error) {
        // Create user
        user, err := s.userRepo.Create(txCtx, email)
        if err != nil {
            return nil, err
        }
        
        // Log event
        if err := s.eventRepo.Log(txCtx, "user_created"); err != nil {
            return nil, err
        }
        
        return user, nil
    })
}
```

### 4.4 Repository Implementation

Repositories should unaware of the `Transactor` logic but should be able to retrieve the `pgx.Tx` from the context if it exists.

Use `transactor.GetTx(ctx)` to extract the transaction.

```go
import (
    "github.com/karu-codes/karu-kits/transactor"
    "github.com/jackc/pgx/v5"
)

type UserRepository struct {
    db *kpgx.DB
}

// Helper to get the operational Executor (either *pgx.Pool or pgx.Tx)
func (r *UserRepository) getConn(ctx context.Context) kpgx.Executor {
    // Try to get transaction from context
    if tx := transactor.GetTx(ctx); tx != nil {
        return tx
    }
    // Fallback to db pool
    return r.db.Pool()
}

func (r *UserRepository) Create(ctx context.Context, user *User) error {
    conn := r.getConn(ctx)
    _, err := conn.Exec(ctx, "INSERT INTO users ...", user.Name)
    return err
}
```

## 5. Advanced Usage

### 5.1 Nested Transactions

The `SQLTransactor` supports nested calls to `Atomically`.
*   **Behavior**: It checks if a transaction is already present in the context.
*   **Result**: If present, it reuses the existing transaction. It does *not* create a new savepoint (simplified nesting).
*   **Implication**: If an inner transaction fails, the entire transaction is marked for rollback.

```go
// Nested Example
func (s *Service) Outer(ctx context.Context) error {
    return s.tx.Atomically(ctx, func(ctx context.Context) error {
        // Do something
        
        // Call Inner, which also calls Atomically
        return s.Inner(ctx) 
    })
}
```

### 5.2 Panic Handling

The `Atomically` method uses `defer` and `recover`. If a panic occurs within the transaction block:
1.  The transaction is rolled back.
2.  The panic is re-thrown (re-panicked) so the application can handle it (or crash) as standard Go behavior.

## 6. API Reference

### `NewTransactor(db *kpgx.DB) *SQLTransactor`
Creates a new transactor instance.

### `func (t *SQLTransactor) Atomically(ctx context.Context, fn TxFn) error`
Executes `fn` within a transaction.

### `func WithResult[T any](ctx context.Context, t Transactor, fn TxFnResult[T]) (T, error)`
Generic helper for transactions returning values.

### `func GetTx(ctx context.Context) pgx.Tx`
Retrieves the raw `pgx.Tx` object from the context, or `nil` if not in a transaction.
