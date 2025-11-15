package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/knadh/koanf/v2"
)

// Load reads the config file into target and optionally overrides values using
// environment variables. The target must be a pointer to a struct.
func Load(path string, target any, opts ...Option) error {
	if target == nil {
		return fmt.Errorf("config: target cannot be nil")
	}

	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}

	data, err := o.fileReader(path)
	if err != nil {
		return fmt.Errorf("config: read %q: %w", path, err)
	}

	format, err := resolveFormat(path, o.format)
	if err != nil {
		return err
	}

	k := koanf.New(".")
	parser, err := parserFor(format)
	if err != nil {
		return err
	}

	if err := k.Load(rawbytes.Provider(data), parser); err != nil {
		return fmt.Errorf("config: parse %q: %w", path, err)
	}

	metas, err := prepareFieldMeta(target, o)
	if err != nil {
		return err
	}

	if o.envEnabled {
		if err := mergeEnv(k, metas, o); err != nil {
			return err
		}
	}

	if err := k.Unmarshal("", target); err != nil {
		return fmt.Errorf("config: unmarshal: %w", err)
	}

	if err := applyDefaults(target, metas); err != nil {
		return err
	}

	return nil
}

func resolveFormat(path string, forced Format) (Format, error) {
	switch forced {
	case FormatJSON, FormatYAML:
		return forced, nil
	case FormatAuto:
	default:
		if forced != "" {
			return "", fmt.Errorf("config: unsupported format %q", forced)
		}
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		return FormatYAML, nil
	case ".json":
		return FormatJSON, nil
	default:
		return "", fmt.Errorf("config: could not detect config format from %q", path)
	}
}

func parserFor(format Format) (koanf.Parser, error) {
	switch format {
	case FormatJSON:
		return json.Parser(), nil
	case FormatYAML:
		return yaml.Parser(), nil
	default:
		return nil, fmt.Errorf("config: unsupported format %q", format)
	}
}
