package klog

import (
	"context"
	"fmt"
	"log/slog"
)

type ContextExtractor func(context.Context) []slog.Attr

func ContextValueExtractor(key any, attrKey string) ContextExtractor {
	if attrKey == "" {
		attrKey = fmt.Sprint(key)
	}
	return func(ctx context.Context) []slog.Attr {
		if ctx == nil {
			return nil
		}
		val := ctx.Value(key)
		if val == nil {
			return nil
		}
		return []slog.Attr{slog.Any(attrKey, val)}
	}
}
