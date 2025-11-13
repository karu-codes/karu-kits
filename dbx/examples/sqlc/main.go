package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/karu-codes/karu-kits/dbx"
)

// This example demonstrates how to use dbx with sqlc-generated code

// Example sqlc-generated interface (you would generate this with sqlc)
type DBTX interface {
	ExecContext(context.Context, string, ...interface{}) (interface{}, error)
	QueryContext(context.Context, string, ...interface{}) (interface{}, error)
	QueryRowContext(context.Context, string, ...interface{}) interface{}
}

// Example repository using sqlc pattern
type UserRepository struct {
	db dbx.DBTX
}

func NewUserRepository(db dbx.DBTX) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) GetUser(ctx context.Context, id int) error {
	// In real sqlc code, this would be generated
	row := r.db.QueryRowContext(ctx, "SELECT id, name, email FROM users WHERE id = $1", id)
	_ = row // Use the row
	return nil
}

func (r *UserRepository) CreateUser(ctx context.Context, name, email string) error {
	// In real sqlc code, this would be generated
	_, err := r.db.ExecContext(ctx, "INSERT INTO users (name, email) VALUES ($1, $2)", name, email)
	return err
}

func main() {
	ctx := context.Background()

	// Example 1: Basic setup with sqlc
	fmt.Println("=== Example 1: Using dbx with sqlc pattern ===")
	db, err := dbx.Open(ctx,
		dbx.WithDriver(dbx.DriverPostgres),
		dbx.WithPGHostPort("localhost", 5432),
		dbx.WithPGAuth("postgres", "password"),
		dbx.WithPGDB("testdb"),
		dbx.WithPGSSLMode("disable"),
	)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create repository with DB (implements DBTX)
	repo := NewUserRepository(db)
	fmt.Println("✓ Repository created with DB")

	// Example 2: Using in transactions
	fmt.Println("\n=== Example 2: Repository in Transaction ===")
	err = db.WithTx(ctx, func(ctx context.Context, tx *sql.Tx) error {
		// Create repository with TX (also implements DBTX)
		txRepo := NewUserRepository(tx)

		if err := txRepo.CreateUser(ctx, "Alice", "alice@example.com"); err != nil {
			return err
		}

		if err := txRepo.CreateUser(ctx, "Bob", "bob@example.com"); err != nil {
			return err
		}

		return nil
	}, nil)

	if err != nil {
		log.Printf("Transaction failed: %v", err)
	} else {
		fmt.Println("✓ Transaction completed successfully")
	}

	// Example 3: Using read replica
	fmt.Println("\n=== Example 3: Using Read Replica ===")
	readDB := db.WithReadReplica()
	readRepo := NewUserRepository(readDB)

	if err := readRepo.GetUser(ctx, 1); err != nil {
		log.Printf("Read failed: %v", err)
	} else {
		fmt.Println("✓ Read from replica successful")
	}

	// Example 4: Type assertion for advanced features
	fmt.Println("\n=== Example 4: Advanced DB Features ===")
	if dbxDB, ok := readDB.(*dbx.DB); ok {
		health := dbxDB.Health(ctx)
		fmt.Printf("  Health Status: %s\n", health.Status)
	}

	fmt.Println("\n=== SQLC Examples Completed ===")
}

// Example of how your sqlc.yaml would look:
//
// version: "2"
// sql:
//   - schema: "schema.sql"
//     queries: "queries.sql"
//     engine: "postgresql"
//     gen:
//       go:
//         package: "db"
//         out: "internal/db"
//         sql_package: "pgx/v5"
//         emit_interface: true
//         emit_json_tags: true
//         emit_prepared_queries: false
//         emit_exact_table_names: false

// Example queries.sql:
//
// -- name: GetUser :one
// SELECT id, name, email FROM users WHERE id = $1;
//
// -- name: ListUsers :many
// SELECT id, name, email FROM users ORDER BY id;
//
// -- name: CreateUser :one
// INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id, name, email;
//
// -- name: UpdateUser :exec
// UPDATE users SET name = $1, email = $2 WHERE id = $3;
//
// -- name: DeleteUser :exec
// DELETE FROM users WHERE id = $1;

// Usage with generated sqlc code:
//
// import "yourproject/internal/db"
//
// func main() {
//     dbx, _ := dbx.Open(ctx, ...)
//     queries := db.New(dbx)  // dbx implements DBTX
//
//     // Use generated methods
//     user, err := queries.GetUser(ctx, 1)
//     users, err := queries.ListUsers(ctx)
//
//     // In transaction
//     dbx.WithTx(ctx, func(ctx context.Context, tx *sql.Tx) error {
//         qtx := queries.WithTx(tx)
//         return qtx.CreateUser(ctx, db.CreateUserParams{...})
//     }, nil)
// }
