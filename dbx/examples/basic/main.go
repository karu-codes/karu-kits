package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"database/sql"

	"github.com/karu-codes/karu-kits/dbx"
)

func main() {
	ctx := context.Background()

	// Example 1: Basic PostgreSQL connection
	fmt.Println("=== Example 1: Basic PostgreSQL Connection ===")
	db, err := dbx.Open(ctx,
		dbx.WithDriver(dbx.DriverPostgres),
		dbx.WithPGHostPort("localhost", 5432),
		dbx.WithPGAuth("postgres", "password"),
		dbx.WithPGDB("testdb"),
		dbx.WithPGSSLMode("disable"),
		dbx.WithPool(25, 10),
		dbx.WithConnLifetime(30*time.Minute, 10*time.Minute),
		dbx.WithAppName("basic-example"),
	)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	fmt.Println("✓ Connected to PostgreSQL successfully")

	// Example 2: Execute a query
	fmt.Println("\n=== Example 2: Execute Query ===")
	result, err := db.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS users (id SERIAL PRIMARY KEY, name TEXT, email TEXT UNIQUE)")
	if err != nil {
		log.Printf("Failed to create table: %v", err)
	} else {
		fmt.Println("✓ Table created successfully")
	}

	// Example 3: Insert data
	fmt.Println("\n=== Example 3: Insert Data ===")
	result, err = db.ExecContext(ctx, "INSERT INTO users (name, email) VALUES ($1, $2) ON CONFLICT (email) DO NOTHING", "John Doe", "john@example.com")
	if err != nil {
		log.Printf("Failed to insert: %v", err)
	} else {
		rowsAffected, _ := result.RowsAffected()
		fmt.Printf("✓ Inserted %d row(s)\n", rowsAffected)
	}

	// Example 4: Query data
	fmt.Println("\n=== Example 4: Query Data ===")
	rows, err := db.QueryContext(ctx, "SELECT id, name, email FROM users")
	if err != nil {
		log.Printf("Failed to query: %v", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var id int
			var name, email string
			if err := rows.Scan(&id, &name, &email); err != nil {
				log.Printf("Failed to scan: %v", err)
				continue
			}
			fmt.Printf("  User: id=%d, name=%s, email=%s\n", id, name, email)
		}
	}

	// Example 5: Transaction
	fmt.Println("\n=== Example 5: Transaction ===")
	err = db.WithTx(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "INSERT INTO users (name, email) VALUES ($1, $2)", "Jane Smith", "jane@example.com")
		if err != nil {
			return err
		}

		_, err = tx.ExecContext(ctx, "UPDATE users SET name = $1 WHERE email = $2", "Jane Doe", "jane@example.com")
		return err
	}, nil)
	if err != nil {
		log.Printf("Transaction failed: %v", err)
	} else {
		fmt.Println("✓ Transaction completed successfully")
	}

	// Example 6: Health Check
	fmt.Println("\n=== Example 6: Health Check ===")
	health := db.Health(ctx)
	fmt.Printf("  Status: %s\n", health.Status)
	fmt.Printf("  Uptime: %v\n", health.Uptime)
	fmt.Printf("  Response Time: %v\n", health.ResponseTime)
	fmt.Printf("  Open Connections: %d\n", health.Connections.Open)
	fmt.Printf("  In Use: %d\n", health.Connections.InUse)
	fmt.Printf("  Utilization: %.2f%%\n", health.Connections.Utilization)

	// Example 7: Error Handling with custom errors package
	fmt.Println("\n=== Example 7: Error Handling ===")
	_, err = db.QueryContext(ctx, "SELECT * FROM non_existent_table")
	if err != nil {
		fmt.Printf("  Error occurred: %v\n", err)
		if dbx.IsRetryable(err) {
			fmt.Println("  ✓ This error is retryable")
		}
	}

	fmt.Println("\n=== All Examples Completed ===")
}
