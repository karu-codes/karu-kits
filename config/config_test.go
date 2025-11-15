package config

import (
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestLoadYAMLWithEnvOverrides(t *testing.T) {
	type AppConfig struct {
		Server struct {
			Host string `yaml:"host"`
			Port int    `yaml:"port"`
		} `yaml:"server"`
		Database struct {
			URL            string `yaml:"url" env:"DATABASE_URL"`
			MaxConnections int    `yaml:"max_connections" envDefault:"50"`
		} `yaml:"database"`
		Features []string `yaml:"features"`
	}

	var cfg AppConfig

	t.Setenv("APP_SERVER_PORT", "9090")
	t.Setenv("DATABASE_URL", "postgres://remote")
	t.Setenv("APP_FEATURES", "trace,metrics ,debug ")

	path := filepath.Join("testdata", "basic.yaml")
	if err := Load(path, &cfg, WithEnvPrefix("APP")); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Host != "0.0.0.0" {
		t.Fatalf("expected host from file, got %q", cfg.Server.Host)
	}
	if cfg.Server.Port != 9090 {
		t.Fatalf("expected server port override, got %d", cfg.Server.Port)
	}
	if cfg.Database.URL != "postgres://remote" {
		t.Fatalf("expected database URL override, got %q", cfg.Database.URL)
	}
	if cfg.Database.MaxConnections != 50 {
		t.Fatalf("expected default max connections, got %d", cfg.Database.MaxConnections)
	}

	wantFeatures := []string{"trace", "metrics", "debug"}
	if len(cfg.Features) != len(wantFeatures) {
		t.Fatalf("unexpected features length: %v", cfg.Features)
	}
	for i, v := range wantFeatures {
		if cfg.Features[i] != v {
			t.Fatalf("feature %d mismatch: want %s got %s", i, v, cfg.Features[i])
		}
	}
}

func TestLoadJSONWithCustomFSAndDefaults(t *testing.T) {
	type TLSConfig struct {
		Enabled bool `json:"enabled"`
	}

	type JSONConfig struct {
		Name    string     `json:"name"`
		Secret  *string    `json:"secret" env:"APP_SECRET" envDefault:"top-secret"`
		Tokens  []string   `json:"tokens" envSeparator:";"`
		TLS     *TLSConfig `json:"tls"`
		Servers struct {
			Primary struct {
				Port int `json:"port"`
			} `json:"primary"`
		} `json:"servers"`
	}

	data := `{
		"name": "demo",
		"servers": {
			"primary": { "port": 8080 }
		}
	}`

	fsys := fstest.MapFS{
		"config.json": {Data: []byte(data)},
	}

	t.Setenv("SERVERS_PRIMARY_PORT", "6060")
	t.Setenv("TOKENS", "alpha;bravo;charlie")
	t.Setenv("TLS_ENABLED", "true")

	var cfg JSONConfig
	if err := Load("config.json", &cfg, WithFileSystem(fsys)); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Servers.Primary.Port != 6060 {
		t.Fatalf("expected port override, got %d", cfg.Servers.Primary.Port)
	}
	if cfg.Secret == nil || *cfg.Secret != "top-secret" {
		t.Fatalf("expected default secret value, got %+v", cfg.Secret)
	}
	wantTokens := []string{"alpha", "bravo", "charlie"}
	if len(cfg.Tokens) != len(wantTokens) {
		t.Fatalf("unexpected tokens length: %v", cfg.Tokens)
	}
	for i, v := range wantTokens {
		if cfg.Tokens[i] != v {
			t.Fatalf("token %d mismatch: want %s got %s", i, v, cfg.Tokens[i])
		}
	}
	if cfg.TLS == nil || !cfg.TLS.Enabled {
		t.Fatalf("expected TLS struct to be created from env")
	}
	if cfg.Name != "demo" {
		t.Fatalf("expected name from file, got %s", cfg.Name)
	}
}
