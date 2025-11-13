package dbx

import "time"

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

// defaultConfig returns a config with sensible defaults
func defaultConfig() Config {
	return Config{
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
}
