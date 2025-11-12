package main

import (
	"context"
	"log/slog"

	"github.com/karu-codes/karu-kits/klog"
)

type ctxKey string

const requestIDKey ctxKey = "request_id"
const userIDKey ctxKey = "user_id"

func main() {
	zapLogger, err := klog.InitProvider(true)
	if err != nil {
		panic(err)
	}

	defer zapLogger.Sync()

	handlerOpts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
		// AddSource: true,
	}

	slogger := klog.NewSlogBuilder(zapLogger).
		WithHandlerOptions(handlerOpts).
		WithContextValue(requestIDKey, "request_id").
		WithContextValue(userIDKey, "user_id").
		Build().
		With("app_name", "kit").
		With("version", "v1.0.0")

	slog.SetDefault(slogger)

	ctx := context.Background()
	ctx = context.WithValue(ctx, requestIDKey, "req-123")
	ctx = context.WithValue(ctx, userIDKey, "user-456")

	slog.DebugContext(ctx, "debug log with context values")
	slog.InfoContext(ctx, "Starting API server")
	slog.ErrorContext(ctx, "this test error")
}
