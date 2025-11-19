package config

import (
	"io/fs"
	"os"
)

type Format string

const (
	FormatAuto Format = ""
	FormatYAML Format = "yaml"
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

type Option func(*options)

func WithEnv(enabled bool) Option {
	return func(o *options) {
		o.envEnabled = enabled
	}
}

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

func WithEnvLookup(fn func(string) (string, bool)) Option {
	return func(o *options) {
		if fn != nil {
			o.envLookup = fn
		}
	}
}

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
