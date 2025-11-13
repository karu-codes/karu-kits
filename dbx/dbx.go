package dbx

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type Driver string

const (
	DriverPostgres Driver = "postgres" // via pgx stdlib (database/sql)
	DriverMySQL    Driver = "mysql"
)

// Logger là interface tối giản; bạn có thể nối với zap/logrus/zerolog
type Logger interface {
	Printf(format string, v ...any)
}

type nopLogger struct{}

func (nopLogger) Printf(string, ...any) {}

// DB là handler chính dùng được cho sqlc (qua stdlib)
type DB struct {
	std     *sql.DB
	driver  Driver
	logger  Logger
	roStd   *sql.DB // optional read-replica
	pgxPool *pgxpool.Pool
	closers []io.Closer
	cfg     Config
	started time.Time
}

func (d *DB) Driver() Driver         { return d.driver }
func (d *DB) StdDB() *sql.DB         { return d.std }
func (d *DB) ReadReplica() *sql.DB   { return d.roStd }
func (d *DB) PgxPool() *pgxpool.Pool { return d.pgxPool }

// Open khởi tạo DB
func Open(ctx context.Context, opts ...Option) (*DB, error) {
	cfg := defaultConfig()
	for _, o := range opts {
		o(&cfg)
	}
	if cfg.Logger == nil {
		cfg.Logger = nopLogger{}
	}

	if cfg.DSN == "" {
		var err error
		switch cfg.Driver {
		case DriverPostgres:
			cfg.DSN, err = buildPostgresDSN(cfg)
		case DriverMySQL:
			cfg.DSN, err = buildMySQLDSN(cfg)
		default:
			err = fmt.Errorf("unsupported driver: %s", cfg.Driver)
		}
		if err != nil {
			return nil, wrapDBError(err, "build DSN")
		}
	}

	// database/sql open
	std, err := sql.Open(string(cfg.Driver), cfg.DSN)
	if err != nil {
		return nil, wrapDBError(err, "open database connection")
	}
	applyPool(std, cfg)

	openCtx, cancel := context.WithTimeout(ctx, cfg.ConnTimeout)
	defer cancel()
	if err := std.PingContext(openCtx); err != nil {
		_ = std.Close()
		return nil, wrapDBError(err, "ping primary database")
	}

	var roStd *sql.DB
	if cfg.ReadReplicaDSN != "" {
		roStd, err = sql.Open(string(cfg.Driver), cfg.ReadReplicaDSN)
		if err != nil {
			_ = std.Close()
			return nil, wrapDBError(err, "open read replica connection")
		}
		applyPool(roStd, cfg)
		if err := roStd.PingContext(openCtx); err != nil {
			_ = roStd.Close()
			_ = std.Close()
			return nil, wrapDBError(err, "ping read replica database")
		}
	}

	var pgpool *pgxpool.Pool
	if cfg.Driver == DriverPostgres && cfg.EnablePgxPool {
		pcfg, err := pgxpool.ParseConfig(cfg.DSN)
		if err != nil {
			_ = std.Close()
			if roStd != nil {
				_ = roStd.Close()
			}
			return nil, wrapDBError(err, "parse pgxpool config")
		}
		if cfg.PgxPoolMinConns > 0 {
			pcfg.MinConns = cfg.PgxPoolMinConns
		}
		if cfg.PgxPoolMaxConns > 0 {
			pcfg.MaxConns = cfg.PgxPoolMaxConns
		}
		if cfg.PgxPoolMaxLifetime > 0 {
			pcfg.MaxConnLifetime = cfg.PgxPoolMaxLifetime
		}
		if cfg.PgxPoolMaxIdleTime > 0 {
			pcfg.MaxConnIdleTime = cfg.PgxPoolMaxIdleTime
		}
		if cfg.PgxPoolHealthCheck > 0 {
			pcfg.HealthCheckPeriod = cfg.PgxPoolHealthCheck
		}
		if cfg.PreferSimpleProto {
			pcfg.ConnConfig.DefaultQueryExecMode = 0 // SimpleProtocol
		}
		pgpool, err = pgxpool.NewWithConfig(ctx, pcfg)
		if err != nil {
			_ = std.Close()
			if roStd != nil {
				_ = roStd.Close()
			}
			return nil, wrapDBError(err, "create pgxpool")
		}
		if err := pgpool.Ping(ctx); err != nil {
			pgpool.Close()
			_ = std.Close()
			if roStd != nil {
				_ = roStd.Close()
			}
			return nil, wrapDBError(err, "ping pgxpool")
		}
	}

	db := &DB{
		std:     std,
		driver:  cfg.Driver,
		logger:  cfg.Logger,
		roStd:   roStd,
		pgxPool: pgpool,
		cfg:     cfg,
		started: time.Now(),
	}
	if cfg.HealthCheckInterval > 0 {
		go db.backgroundHealth(ctx, cfg.HealthCheckInterval)
	}
	return db, nil
}

func applyPool(std *sql.DB, cfg Config) {
	if cfg.MaxOpenConns > 0 {
		std.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		std.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		std.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime > 0 {
		std.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}
}

// Close đóng mọi tài nguyên
func (d *DB) Close() error {
	var err error
	if d.pgxPool != nil {
		d.pgxPool.Close()
	}
	if d.roStd != nil {
		if e := d.roStd.Close(); e != nil && err == nil {
			err = e
		}
	}
	if d.std != nil {
		if e := d.std.Close(); e != nil && err == nil {
			err = e
		}
	}
	for _, c := range d.closers {
		if e := c.Close(); e != nil && err == nil {
			err = e
		}
	}
	return err
}

// Ping tiện cho healthcheck
func (d *DB) Ping(ctx context.Context) error {
	if err := d.std.PingContext(ctx); err != nil {
		return err
	}
	if d.pgxPool != nil {
		return d.pgxPool.Ping(ctx)
	}
	return nil
}

func (d *DB) backgroundHealth(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := d.Ping(ctx); err != nil {
				d.logger.Printf("[dbx] healthcheck failed: %v", err)
			}
		}
	}
}

// Exec/Query methods with error wrapping
func (d *DB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	result, err := d.std.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, wrapDBError(err, "exec query")
	}
	return result, nil
}

func (d *DB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	rows, err := d.std.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, wrapDBError(err, "query")
	}
	return rows, nil
}

func (d *DB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return d.std.QueryRowContext(ctx, query, args...)
}

// WithTx helper cho transaction + retry
type TxFunc func(ctx context.Context, tx *sql.Tx) error

type TxOptions struct {
	// Sử dụng sql.TxOptions cho isolation & readonly
	Options *sql.TxOptions
	// Override retry cho case đặc biệt
	MaxRetries int
	RetryDelay time.Duration
}

func (d *DB) WithTx(ctx context.Context, fn TxFunc, opt *TxOptions) error {
	return retryForTransaction(ctx, d.cfg, func() error {
		tx, err := d.std.BeginTx(ctx, optSafe(opt))
		if err != nil {
			return wrapDBError(err, "begin transaction")
		}

		// Execute function
		if err := fn(ctx, tx); err != nil {
			_ = tx.Rollback()
			return err
		}

		// Commit transaction
		if err := tx.Commit(); err != nil {
			return wrapDBError(err, "commit transaction")
		}

		return nil
	})
}

func optSafe(opt *TxOptions) *sql.TxOptions {
	if opt == nil {
		return nil
	}
	return opt.Options
}

// ---------- Ví dụ sử dụng ----------
//
//  // Postgres (pgx stdlib) + pgxpool:
//  ctx := context.Background()
//  db, err := dbx.Open(ctx,
//      dbx.WithDriver(dbx.DriverPostgres),
//      dbx.WithPGHostPort("localhost", 5432),
//      dbx.WithPGAuth("app", "secret"),
//      dbx.WithPGDB("appdb"),
//      dbx.WithPGSSLMode("disable"),
//      dbx.WithPreferSimpleProtocol(true),
//      dbx.WithPool(50, 25),
//      dbx.WithConnLifetime(45*time.Minute, 10*time.Minute),
//      dbx.WithAppName("keyra-api"),
//      dbx.WithPgxPool(true),
//      dbx.WithPgxPoolSize(10, 50),
//      dbx.WithPgxPoolLifetime(1*time.Hour, 15*time.Minute, 30*time.Second),
//  )
//  if err != nil { log.Fatal(err) }
//  defer db.Close()
//
//  // Dùng với sqlc (database/sql):
//  //   q := New(db.StdDB())  // nếu sqlc generate kiểu database/sql
//
//  // Transaction helper:
//  err = db.WithTx(ctx, func(ctx context.Context, tx *sql.Tx) error {
//      // qtx := New(tx) // sqlc withTx
//      // ... do queries ...
//      return nil
//  }, &dbx.TxOptions{ Options: &sql.TxOptions{Isolation: sql.LevelSerializable} })
//
//  // MySQL:
//  dbMy, err := dbx.Open(ctx,
//      dbx.WithDriver(dbx.DriverMySQL),
//      dbx.WithPGHostPort("localhost", 3306),      // tái dụng field host/port
//      dbx.WithPGAuth("root", "secret"),
//      dbx.WithPGDB("appdb"),
//      dbx.WithMySQLParseTime(true),
//  )
//
// -------------------------------------
