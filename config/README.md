# Config Package

Lightweight helper for loading YAML/JSON configuration files and overriding values with environment variables, built on top of the high-performance [koanf](https://github.com/knadh/koanf) loader.

## Features

- Auto-detects JSON or YAML based on file extension (or specify explicitly)
- Environment overrides inferred from struct/field names with optional prefixes
- `env`/`envDefault`/`envSeparator` struct tags for fine-grained control
- Supports nested structs, pointers, primitives, `time.Duration`, `time.Time`, and `[]string`
- Works with any `fs.FS` (embed, `fstest.MapFS`, etc.)

## Installation

```bash
go get github.com/karu-codes/karu-kits/config
```

## Quick Start

```go
package main

import (
    "log"
    "time"

    "github.com/karu-codes/karu-kits/config"
)

type AppConfig struct {
    Server struct {
        Host string `yaml:"host"`
        Port int    `yaml:"port"`
    } `yaml:"server"`

    Database struct {
        URL            string        `yaml:"url" env:"DATABASE_URL"`
        Timeout        time.Duration `yaml:"timeout" envDefault:"30s"`
        Replicas       []string      `yaml:"replicas" envSeparator:";"`
    } `yaml:"database"`
}

func main() {
    var cfg AppConfig
    if err := config.Load("config.yaml", &cfg, config.WithEnvPrefix("APP")); err != nil {
        log.Fatal(err)
    }
    // cfg now contains file values overridden by env (APP_SERVER_PORT, DATABASE_URL, etc.)
}
```

Example `config.yaml`:

```yaml
server:
  host: 0.0.0.0
  port: 8080
database:
  url: postgres://localhost:5432/app
  replicas: eu-1;us-1
```

Set the following to override file values:

```bash
export APP_SERVER_PORT=9090
export DATABASE_URL=postgres://prod:5432/app
export DATABASE_REPLICAS="eu-2;ap-1"
```

## Struct Tags

| Tag | Description |
|-----|-------------|
| `env:"NAME"` | Use a specific environment variable for the field (prefix is not applied). |
| `envDefault:"VALUE"` | Fallback value used when the field is still zero after file parsing and no env var is present. |
| `envSeparator:";"` | For `[]string` fields, overrides the default comma separator used when splitting env values. |

Environment names are inferred from the struct path when `env` is omitted. For example `Server.Port` becomes `SERVER_PORT`, and with `config.WithEnvPrefix("APP")` it becomes `APP_SERVER_PORT`.

## Options

```go
config.Load(
    "config.yaml",
    &cfg,
    config.WithEnvPrefix("APP"),     // prepend APP_ to inferred env names
    config.WithEnvLookup(os.LookupEnv), // custom lookup (defaults to os.LookupEnv)
    config.WithFileSystem(embedFS),  // read files from embed/fs.FS
    config.WithSliceSeparator(";"),  // default separator for []string overrides
    config.WithFormat(config.FormatYAML), // force parser when file lacks extension
)
```

Use `config.WithoutEnv()` if you just want to parse files without environment overrides.

## Supported Types

Environment overrides work for:

- `string`, `bool`
- Signed/unsigned integers (including `time.Duration`)
- `float32`, `float64`
- `[]string`
- `time.Time` (RFC3339 format)
- Structs/pointers composed of the above types

For more advanced scenarios you can parse complex values (e.g. JSON arrays) inside your own wrapper type that implements the necessary parsing logic before calling `config.Load`.
