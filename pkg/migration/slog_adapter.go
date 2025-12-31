package migration

import (
	"context"
	"log/slog"
	"os"
)

// slogAdapter adapts Go's standard slog.Logger to our Logger interface.
type slogAdapter struct {
	logger *slog.Logger
}

// NewSlogLogger creates a new Logger that writes to console using slog.
// This is the recommended logger for production use.
func NewSlogLogger(level slog.Level) Logger {
	opts := &slog.HandlerOptions{
		Level: level,
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler)

	return &slogAdapter{
		logger: logger,
	}
}

// NewSlogTextLogger creates a new Logger that writes human-readable text to console.
// This is useful for development and debugging.
func NewSlogTextLogger(level slog.Level) Logger {
	opts := &slog.HandlerOptions{
		Level: level,
	}

	handler := slog.NewTextHandler(os.Stdout, opts)
	logger := slog.New(handler)

	return &slogAdapter{
		logger: logger,
	}
}

// Debug logs a debug-level message.
func (s *slogAdapter) Debug(ctx context.Context, msg string, fields ...Field) {
	s.logger.DebugContext(ctx, msg, convertFieldsToSlogAttrs(fields)...)
}

// Info logs an info-level message.
func (s *slogAdapter) Info(ctx context.Context, msg string, fields ...Field) {
	s.logger.InfoContext(ctx, msg, convertFieldsToSlogAttrs(fields)...)
}

// Warn logs a warning-level message.
func (s *slogAdapter) Warn(ctx context.Context, msg string, fields ...Field) {
	s.logger.WarnContext(ctx, msg, convertFieldsToSlogAttrs(fields)...)
}

// Error logs an error-level message.
func (s *slogAdapter) Error(ctx context.Context, msg string, fields ...Field) {
	s.logger.ErrorContext(ctx, msg, convertFieldsToSlogAttrs(fields)...)
}

// convertFieldsToSlogAttrs converts our Field type to slog.Attr.
func convertFieldsToSlogAttrs(fields []Field) []any {
	attrs := make([]any, 0, len(fields))
	for _, f := range fields {
		attrs = append(attrs, slog.Any(f.Key, f.Value))
	}
	return attrs
}
