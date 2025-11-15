package config

import (
	"io/fs"
	"os"
)

// Format describes the serialization format used by the config file.
type Format string

const (
	// FormatAuto automatically detects the format based on the file extension.
	FormatAuto Format = ""
	// FormatYAML parses YAML documents (.yaml/.yml).
	FormatYAML Format = "yaml"
	// FormatJSON parses JSON documents (.json).
	FormatJSON Format = "json"
)

type options struct {
	envEnabled     bool
	envPrefix      string
	envLookup      func(string) (string, bool)
	fileReader     func(string) ([]byte, error)
	sliceSeparator string
	format         Format
}

func defaultOptions() options {
	return options{
		envEnabled:     true,
		envLookup:      os.LookupEnv,
		fileReader:     os.ReadFile,
		sliceSeparator: ",",
		format:         FormatAuto,
	}
}

// Option configures Load behaviour.
type Option func(*options)

// WithEnv controls whether environment overrides are applied (enabled by default).
func WithEnv(enabled bool) Option {
	return func(o *options) {
		o.envEnabled = enabled
	}
}

// WithoutEnv disables environment overrides entirely.
func WithoutEnv() Option {
	return WithEnv(false)
}

// WithEnvPrefix configures a prefix that is automatically prepended to inferred
// environment variable names (e.g. APP_SERVER_PORT).
func WithEnvPrefix(prefix string) Option {
	return func(o *options) {
		o.envPrefix = prefix
	}
}

// WithEnvLookup injects a custom environment lookup function. Useful for tests.
func WithEnvLookup(fn func(string) (string, bool)) Option {
	return func(o *options) {
		if fn != nil {
			o.envLookup = fn
		}
	}
}

// WithFileSystem loads the config file from the provided filesystem instead of
// the host OS. Paths are interpreted relative to the filesystem root.
func WithFileSystem(fsys fs.FS) Option {
	return func(o *options) {
		if fsys == nil {
			return
		}
		if readFS, ok := fsys.(fs.ReadFileFS); ok {
			o.fileReader = readFS.ReadFile
			return
		}
		o.fileReader = func(name string) ([]byte, error) {
			return fs.ReadFile(fsys, name)
		}
	}
}

// WithSliceSeparator overrides the default separator (",") used when parsing
// string slice environment variables.
func WithSliceSeparator(sep string) Option {
	return func(o *options) {
		if sep != "" {
			o.sliceSeparator = sep
		}
	}
}

// WithFormat forces Load to parse the provided format instead of relying on
// file extension detection.
func WithFormat(format Format) Option {
	return func(o *options) {
		o.format = format
	}
}
