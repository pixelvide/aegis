// Package logger provides structured logging for the Aegis server.
//
// It wraps Go's stdlib log/slog with configurable log level and output format.
// Call Init() once at startup before any logging. After that, use slog.Info(),
// slog.Error(), etc. directly — the global default is set by Init().
//
// For future OTel integration, use FromContext(ctx) to get a logger that
// will automatically include trace_id and span_id when tracing is enabled.
package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

// Init creates and sets the global slog logger based on the given level and format.
// Must be called once at startup before any logging occurs.
//
// level: "debug", "info", "warn", "error" (default: "info")
// format: "text" (human-readable) or "json" (structured, for log aggregation)
func Init(level, format string) {
	lvl := parseLevel(level)
	opts := &slog.HandlerOptions{
		Level: lvl,
		// Include source file and line number in debug mode for easier tracing
		AddSource: lvl == slog.LevelDebug,
	}

	var handler slog.Handler
	if strings.EqualFold(format, "json") {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
}

// FromContext returns a logger from the given context.
// Currently returns the default logger, but is designed for future OTel
// integration where trace_id and span_id will be extracted from the
// context and added as log attributes automatically.
func FromContext(ctx context.Context) *slog.Logger {
	// Future: extract OTel trace context and add as attributes:
	//   spanCtx := trace.SpanContextFromContext(ctx)
	//   if spanCtx.IsValid() {
	//       return slog.Default().With(
	//           "trace_id", spanCtx.TraceID().String(),
	//           "span_id", spanCtx.SpanID().String(),
	//       )
	//   }
	_ = ctx // avoid unused warning until OTel is integrated
	return slog.Default()
}

// parseLevel converts a string log level to slog.Level.
// Defaults to slog.LevelInfo for unrecognized values.
func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
