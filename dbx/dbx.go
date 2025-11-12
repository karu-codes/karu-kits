package dbx

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
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

// Config là cấu hình gốc; đa số set qua Option
type Config struct {
	Driver              Driver
	DSN                 string // nếu đã có DSN đầy đủ thì chỉ cần set đây
	AppName             string
	ConnTimeout         time.Duration
	MaxOpenConns        int
	MaxIdleConns        int
	ConnMaxLifetime     time.Duration
	ConnMaxIdleTime     time.Duration
	HealthCheckInterval time.Duration
	ReadOnly            bool
	PreferSimpleProto   bool // pgx: disable extended protocol (qua stdlib vẫn hưởng lợi)
	EnablePgxPool       bool // nếu true và Driver=postgres, tạo thêm pgxpool song song
	PgxPoolMinConns     int32
	PgxPoolMaxConns     int32
	PgxPoolMaxLifetime  time.Duration
	PgxPoolMaxIdleTime  time.Duration
	PgxPoolHealthCheck  time.Duration

	// Replica/Multi-endpoint
	// VD: Primary DSN + Read DSN cho read-only query (dùng database/sql)
	ReadReplicaDSN string

	// Observability
	Logger Logger

	// Retry (cho tx helper)
	MaxRetries int
	RetryDelay time.Duration

	// MySQL options
	MySQLParseTime bool // parseTime=true
	MySQLLoc       *time.Location

	// Postgres DSN builder fields (nếu bạn không đưa DSN sẵn)
	PGHost     string
	PGPort     int
	PGUser     string
	PGPassword string
	PGDatabase string
	PGSSLMode  string            // disable|require|verify-ca|verify-full
	PGParams   map[string]string // extra params: search_path, timezone, ...
}

// Option pattern
type Option func(*Config)

func WithDriver(d Driver) Option             { return func(c *Config) { c.Driver = d } }
func WithDSN(dsn string) Option              { return func(c *Config) { c.DSN = dsn } }
func WithAppName(name string) Option         { return func(c *Config) { c.AppName = name } }
func WithConnTimeout(d time.Duration) Option { return func(c *Config) { c.ConnTimeout = d } }
func WithPool(maxOpen, maxIdle int) Option {
	return func(c *Config) { c.MaxOpenConns, c.MaxIdleConns = maxOpen, maxIdle }
}
func WithConnLifetime(life, idle time.Duration) Option {
	return func(c *Config) { c.ConnMaxLifetime, c.ConnMaxIdleTime = life, idle }
}
func WithHealthCheckEvery(d time.Duration) Option {
	return func(c *Config) { c.HealthCheckInterval = d }
}
func WithReadOnly(ro bool) Option            { return func(c *Config) { c.ReadOnly = ro } }
func WithPreferSimpleProtocol(b bool) Option { return func(c *Config) { c.PreferSimpleProto = b } }
func WithLogger(l Logger) Option             { return func(c *Config) { c.Logger = l } }
func WithRetry(max int, delay time.Duration) Option {
	return func(c *Config) { c.MaxRetries, c.RetryDelay = max, delay }
}

// Postgres-only
func WithPgxPool(enable bool) Option { return func(c *Config) { c.EnablePgxPool = enable } }
func WithPgxPoolSize(min, max int32) Option {
	return func(c *Config) { c.PgxPoolMinConns, c.PgxPoolMaxConns = min, max }
}
func WithPgxPoolLifetime(life, idle, health time.Duration) Option {
	return func(c *Config) { c.PgxPoolMaxLifetime, c.PgxPoolMaxIdleTime, c.PgxPoolHealthCheck = life, idle, health }
}
func WithPGHostPort(host string, port int) Option {
	return func(c *Config) { c.PGHost, c.PGPort = host, port }
}
func WithPGAuth(user, pass string) Option {
	return func(c *Config) { c.PGUser, c.PGPassword = user, pass }
}
func WithPGDB(db string) Option           { return func(c *Config) { c.PGDatabase = db } }
func WithPGSSLMode(sslmode string) Option { return func(c *Config) { c.PGSSLMode = sslmode } }
func WithPGParam(k, v string) Option {
	return func(c *Config) {
		if c.PGParams == nil {
			c.PGParams = map[string]string{}
		}
		c.PGParams[k] = v
	}
}
func WithReadReplicaDSN(dsn string) Option { return func(c *Config) { c.ReadReplicaDSN = dsn } }

// MySQL-only
func WithMySQLParseTime(b bool) Option            { return func(c *Config) { c.MySQLParseTime = b } }
func WithMySQLLocation(loc *time.Location) Option { return func(c *Config) { c.MySQLLoc = loc } }

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
	cfg := Config{
		Driver:              DriverPostgres,
		AppName:             "dbx",
		ConnTimeout:         5 * time.Second,
		MaxOpenConns:        20,
		MaxIdleConns:        10,
		ConnMaxLifetime:     30 * time.Minute,
		ConnMaxIdleTime:     10 * time.Minute,
		HealthCheckInterval: 0,
		ReadOnly:            false,
		PreferSimpleProto:   false,
		Logger:              nopLogger{},
		MaxRetries:          1,
		RetryDelay:          150 * time.Millisecond,
		PgxPoolMaxConns:     20,
	}
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
			return nil, err
		}
	}

	// database/sql open
	std, err := sql.Open(string(cfg.Driver), cfg.DSN)
	if err != nil {
		return nil, err
	}
	applyPool(std, cfg)

	openCtx, cancel := context.WithTimeout(ctx, cfg.ConnTimeout)
	defer cancel()
	if err := std.PingContext(openCtx); err != nil {
		_ = std.Close()
		return nil, fmt.Errorf("ping std db failed: %w", err)
	}

	var roStd *sql.DB
	if cfg.ReadReplicaDSN != "" {
		roStd, err = sql.Open(string(cfg.Driver), cfg.ReadReplicaDSN)
		if err != nil {
			_ = std.Close()
			return nil, err
		}
		applyPool(roStd, cfg)
		if err := roStd.PingContext(openCtx); err != nil {
			_ = roStd.Close()
			_ = std.Close()
			return nil, fmt.Errorf("ping read-replica failed: %w", err)
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
			return nil, fmt.Errorf("pgxpool parse: %w", err)
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
			return nil, fmt.Errorf("pgxpool open: %w", err)
		}
		if err := pgpool.Ping(ctx); err != nil {
			pgpool.Close()
			_ = std.Close()
			if roStd != nil {
				_ = roStd.Close()
			}
			return nil, fmt.Errorf("pgxpool ping: %w", err)
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

// Exec/Query shims nếu bạn muốn wrap để log/tracing (tuỳ chọn)
func (d *DB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return d.std.ExecContext(ctx, query, args...)
}
func (d *DB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return d.std.QueryContext(ctx, query, args...)
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
	maxRetries := d.cfg.MaxRetries
	delay := d.cfg.RetryDelay
	if opt != nil {
		if opt.MaxRetries > 0 {
			maxRetries = opt.MaxRetries
		}
		if opt.RetryDelay > 0 {
			delay = opt.RetryDelay
		}
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		tx, err := d.std.BeginTx(ctx, optSafe(opt))
		if err != nil {
			return err
		}
		if err := fn(ctx, tx); err != nil {
			_ = tx.Rollback()
			lastErr = err
			if isRetryableErr(err) && attempt < maxRetries {
				time.Sleep(delay)
				continue
			}
			return err
		}
		if err := tx.Commit(); err != nil {
			lastErr = err
			if isRetryableErr(err) && attempt < maxRetries {
				time.Sleep(delay)
				continue
			}
			return err
		}
		return nil
	}
	return lastErr
}

func optSafe(opt *TxOptions) *sql.TxOptions {
	if opt == nil {
		return nil
	}
	return opt.Options
}

func isRetryableErr(err error) bool {
	// đơn giản: network transient/ serialization
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Temporary() {
		return true
	}
	// Postgres serialization failure codes thường là 40001; ở đây demo đơn giản
	if strings.Contains(err.Error(), "serialization") || strings.Contains(err.Error(), "deadlock") {
		return true
	}
	return false
}

// ---------- DSN Builders ----------

// buildPostgresDSN tạo DSN cho pgx stdlib.
// Mặc định: scheme "postgres" dùng cho pgx stdlib ("github.com/jackc/pgx/v5/stdlib").
func buildPostgresDSN(cfg Config) (string, error) {
	if cfg.PGHost == "" {
		cfg.PGHost = "127.0.0.1"
	}
	if cfg.PGPort == 0 {
		cfg.PGPort = 5432
	}
	if cfg.PGSSLMode == "" {
		cfg.PGSSLMode = "disable"
	}

	u := &url.URL{
		Scheme: "postgres",
		Host:   fmt.Sprintf("%s:%d", cfg.PGHost, cfg.PGPort),
		Path:   "/" + cfg.PGDatabase,
	}
	if cfg.PGUser != "" {
		if cfg.PGPassword != "" {
			u.User = url.UserPassword(cfg.PGUser, cfg.PGPassword)
		} else {
			u.User = url.User(cfg.PGUser)
		}
	}
	q := u.Query()
	q.Set("sslmode", cfg.PGSSLMode)
	if cfg.AppName != "" {
		q.Set("application_name", cfg.AppName)
	}
	if cfg.PreferSimpleProto {
		q.Set("prefer_simple_protocol", "true")
	}
	for k, v := range cfg.PGParams {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// buildMySQLDSN trả về: user:pass@tcp(host:port)/db?parseTime=true&loc=...
func buildMySQLDSN(cfg Config) (string, error) {
	host := "127.0.0.1"
	port := 3306
	user := "root"
	db := ""
	pass := ""
	if cfg.PGHost != "" {
		host = cfg.PGHost
	} // tái dụng PGHost/PGPort để đơn giản hoá input
	if cfg.PGPort != 0 {
		port = cfg.PGPort
	}
	if cfg.PGUser != "" {
		user = cfg.PGUser
	}
	if cfg.PGPassword != "" {
		pass = cfg.PGPassword
	}
	if cfg.PGDatabase != "" {
		db = cfg.PGDatabase
	}

	addr := fmt.Sprintf("tcp(%s:%d)", host, port)
	qs := url.Values{}
	if cfg.MySQLParseTime {
		qs.Set("parseTime", "true")
	}
	if cfg.MySQLLoc != nil {
		qs.Set("loc", cfg.MySQLLoc.String())
	}
	if cfg.AppName != "" {
		qs.Set("application_name", cfg.AppName) // nhiều server bỏ qua; để minh họa
	}
	dsn := fmt.Sprintf("%s:%s@%s/%s", user, pass, addr, db)
	if enc := qs.Encode(); enc != "" {
		dsn += "?" + enc
	}
	return dsn, nil
}

// ---------- Helpers cho sqlc ----------

// SQLCx trả về interface cơ bản tương thích sqlc (database/sql):
// sqlc thường cần *sql.DB | *sql.Tx | interface có ExecContext/QueryContext/QueryRowContext
type SQLCx interface {
	driver.Conn // not strictly needed, but kept minimal
}

// Tuy nhiên, thực tế với sqlc, bạn chỉ cần *sql.DB hoặc *sql.Tx.
// Gợi ý: dùng db.StdDB() khi wire vào sqlc-generated Querier.

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
