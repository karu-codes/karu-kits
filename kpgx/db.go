package kpgx

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds the configuration for the database connection.
type Config struct {
	// ConnString is the PostgreSQL connection string.
	// e.g., "postgres://user:password@localhost:5432/dbname?sslmode=disable"
	ConnString string

	// MaxConns is the maximum number of connections in the pool.
	// Default is usually sufficient (runtime.NumCPU() * 4).
	MaxConns int32

	// MinConns is the minimum number of connections in the pool.
	MinConns int32

	// MaxConnLifetime is the maximum amount of time a connection may be reused.
	MaxConnLifetime time.Duration

	// MaxConnIdleTime is the maximum amount of time a connection may be idle.
	MaxConnIdleTime time.Duration

	// HealthCheckPeriod is the duration between health checks.
	HealthCheckPeriod time.Duration
}

// DB wraps pgxpool.Pool to provide application-specific functionality.
type DB struct {
	pool *pgxpool.Pool
}

// New creates a new DB instance.
func New(ctx context.Context, cfg Config) (*DB, error) {
	pgxCfg, err := pgxpool.ParseConfig(cfg.ConnString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	if cfg.MaxConns > 0 {
		pgxCfg.MaxConns = cfg.MaxConns
	}
	if cfg.MinConns > 0 {
		pgxCfg.MinConns = cfg.MinConns
	}
	if cfg.MaxConnLifetime > 0 {
		pgxCfg.MaxConnLifetime = cfg.MaxConnLifetime
	}
	if cfg.MaxConnIdleTime > 0 {
		pgxCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	}
	if cfg.HealthCheckPeriod > 0 {
		pgxCfg.HealthCheckPeriod = cfg.HealthCheckPeriod
	}

	pool, err := pgxpool.NewWithConfig(ctx, pgxCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Ping to ensure connection is established
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{pool: pool}, nil
}

// Close closes the database connection pool.
func (db *DB) Close() {
	db.pool.Close()
}

// Pool returns the underlying pgxpool.Pool.
func (db *DB) Pool() *pgxpool.Pool {
	return db.pool
}
