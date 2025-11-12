package klog

import (
	"log/slog"

	"go.uber.org/zap"
)

type LoggerBuilder struct {
	logger      *zap.Logger
	handlerOpts *slog.HandlerOptions
	extractors  []ContextExtractor
	callerSkip  int
}

func NewSlogBuilder(z *zap.Logger) *LoggerBuilder {
	if z == nil {
		panic("klog: nil zap logger")
	}
	return &LoggerBuilder{
		logger:     z,
		callerSkip: 3,
	}
}

func (b *LoggerBuilder) WithHandlerOptions(opts *slog.HandlerOptions) *LoggerBuilder {
	if opts != nil {
		b.handlerOpts = opts
	}
	return b
}

func (b *LoggerBuilder) WithExtractor(ex ContextExtractor) *LoggerBuilder {
	if ex == nil {
		panic("klog: nil context extractor")
	}
	b.extractors = append(b.extractors, ex)
	return b
}

func (b *LoggerBuilder) WithContextValue(key any, attrKey string) *LoggerBuilder {
	return b.WithExtractor(ContextValueExtractor(key, attrKey))
}

func (b *LoggerBuilder) WithCallerSkip(skip int) *LoggerBuilder {
	if skip > 0 {
		b.callerSkip = skip
	}
	return b
}

func (b *LoggerBuilder) Build() *slog.Logger {
	handler := &zapSlogHandler{
		logger:     b.logger.WithOptions(zap.AddCallerSkip(b.callerSkip)),
		opts:       normalizeHandlerOptions(b.handlerOpts),
		extractors: append([]ContextExtractor(nil), b.extractors...),
	}
	return slog.New(handler)
}
