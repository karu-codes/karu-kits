package dbx

import (
	"fmt"
	"net/url"
)

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
