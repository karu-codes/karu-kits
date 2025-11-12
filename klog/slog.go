package klog

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewSlogLogger(z *zap.Logger) *slog.Logger {
	return NewSlogBuilder(z).Build()
}

type handlerOptions struct {
	level       slog.Leveler
	addSource   bool
	replaceAttr func([]string, slog.Attr) slog.Attr
}

func normalizeHandlerOptions(opts *slog.HandlerOptions) handlerOptions {
	if opts == nil {
		return handlerOptions{
			level: slog.LevelInfo,
		}
	}
	level := opts.Level
	if level == nil {
		level = slog.LevelInfo
	}
	return handlerOptions{
		level:       level,
		addSource:   opts.AddSource,
		replaceAttr: opts.ReplaceAttr,
	}
}

type zapSlogHandler struct {
	logger     *zap.Logger
	opts       handlerOptions
	attrs      []slog.Attr
	groups     []string
	extractors []ContextExtractor
}

func (h *zapSlogHandler) Enabled(_ context.Context, level slog.Level) bool {
	min := slog.LevelInfo
	if h.opts.level != nil {
		min = h.opts.level.Level()
	}
	return level >= min
}

func (h *zapSlogHandler) Handle(ctx context.Context, record slog.Record) error {
	if !h.Enabled(ctx, record.Level) {
		return nil
	}

	fields := make([]zap.Field, 0, len(h.attrs)+record.NumAttrs()+4)
	h.appendAttrs(&fields, h.groups, h.attrs)
	for _, extractor := range h.extractors {
		h.appendAttrs(&fields, h.groups, extractor(ctx))
	}
	record.Attrs(func(attr slog.Attr) bool {
		h.appendAttr(&fields, h.groups, attr)
		return true
	})

	if h.opts.addSource && record.PC != 0 {
		if file, line, ok := frameFromPC(record.PC); ok {
			fields = append(fields, zap.String("source", fmt.Sprintf("%s:%d", file, line)))
		}
	}

	h.logger.Log(toZapLevel(record.Level), record.Message, fields...)
	return nil
}

func (h *zapSlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	clone := h.clone()
	clone.attrs = append(clone.attrs, attrs...)
	return clone
}

func (h *zapSlogHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	clone := h.clone()
	clone.groups = append(clone.groups, name)
	return clone
}

func (h *zapSlogHandler) clone() *zapSlogHandler {
	c := *h
	if len(h.attrs) > 0 {
		c.attrs = append([]slog.Attr(nil), h.attrs...)
	}
	if len(h.groups) > 0 {
		c.groups = append([]string(nil), h.groups...)
	}
	return &c
}

func (h *zapSlogHandler) appendAttrs(fields *[]zap.Field, groups []string, attrs []slog.Attr) {
	for _, attr := range attrs {
		h.appendAttr(fields, groups, attr)
	}
}

func (h *zapSlogHandler) appendAttr(fields *[]zap.Field, groups []string, attr slog.Attr) {
	attr.Value = attr.Value.Resolve()
	if h.opts.replaceAttr != nil {
		attr = h.opts.replaceAttr(groups, attr)
	}
	if attr.Equal(slog.Attr{}) {
		return
	}

	if attr.Value.Kind() == slog.KindGroup {
		childGroups := appendGroup(groups, attr.Key)
		for _, child := range attr.Value.Group() {
			h.appendAttr(fields, childGroups, child)
		}
		return
	}

	key := fullKey(groups, attr.Key)
	switch attr.Value.Kind() {
	case slog.KindBool:
		*fields = append(*fields, zap.Bool(key, attr.Value.Bool()))
	case slog.KindDuration:
		*fields = append(*fields, zap.Duration(key, attr.Value.Duration()))
	case slog.KindFloat64:
		*fields = append(*fields, zap.Float64(key, attr.Value.Float64()))
	case slog.KindInt64:
		*fields = append(*fields, zap.Int64(key, attr.Value.Int64()))
	case slog.KindUint64:
		*fields = append(*fields, zap.Uint64(key, attr.Value.Uint64()))
	case slog.KindString:
		*fields = append(*fields, zap.String(key, attr.Value.String()))
	case slog.KindTime:
		*fields = append(*fields, zap.Time(key, attr.Value.Time()))
	case slog.KindAny:
		*fields = append(*fields, anyToField(key, attr.Value.Any()))
	default:
		*fields = append(*fields, zap.Any(key, attr.Value.Any()))
	}
}

func frameFromPC(pc uintptr) (string, int, bool) {
	frames := runtime.CallersFrames([]uintptr{pc})
	frame, _ := frames.Next()
	if frame.File == "" {
		return "", 0, false
	}
	return filepath.Base(frame.File), frame.Line, true
}

func appendGroup(groups []string, key string) []string {
	if key == "" {
		return groups
	}
	next := append([]string(nil), groups...)
	next = append(next, key)
	return next
}

func fullKey(groups []string, key string) string {
	if key == "" {
		key = "value"
	}
	if len(groups) == 0 {
		return key
	}
	return strings.Join(groups, ".") + "." + key
}

func toZapLevel(level slog.Level) zapcore.Level {
	switch {
	case level < slog.LevelInfo:
		return zapcore.DebugLevel
	case level < slog.LevelWarn:
		return zapcore.InfoLevel
	case level < slog.LevelError:
		return zapcore.WarnLevel
	case level < slog.LevelError+4:
		return zapcore.ErrorLevel
	default:
		return zapcore.ErrorLevel
	}
}

func anyToField(key string, value any) zap.Field {
	switch v := value.(type) {
	case nil:
		return zap.Any(key, nil)
	case zap.Field:
		// respect existing field but override key để align với slog key
		v.Key = key
		return v
	case error:
		return zap.NamedError(key, v)
	case fmt.Stringer:
		return zap.Stringer(key, v)
	case []byte:
		return zap.ByteString(key, v)
	case bool:
		return zap.Bool(key, v)
	case string:
		return zap.String(key, v)
	case int:
		return zap.Int(key, v)
	case int64:
		return zap.Int64(key, v)
	case int32:
		return zap.Int32(key, v)
	case uint:
		return zap.Uint(key, v)
	case uint64:
		return zap.Uint64(key, v)
	case uint32:
		return zap.Uint32(key, v)
	case float64:
		return zap.Float64(key, v)
	case float32:
		return zap.Float32(key, v)
	default:
		return zap.Any(key, v)
	}
}
