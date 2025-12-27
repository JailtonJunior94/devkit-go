package observability

import "context"

// LogLevel represents the severity level of a log entry.
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// LogFormat represents the output format for logs.
type LogFormat string

const (
	LogFormatText LogFormat = "text"
	LogFormatJSON LogFormat = "json"
)

// Logger provides structured logging capabilities with trace context propagation.
type Logger interface {
	// Debug logs a debug-level message with optional structured fields.
	Debug(ctx context.Context, msg string, fields ...Field)

	// Info logs an info-level message with optional structured fields.
	Info(ctx context.Context, msg string, fields ...Field)

	// Warn logs a warning-level message with optional structured fields.
	Warn(ctx context.Context, msg string, fields ...Field)

	// Error logs an error-level message with optional structured fields.
	Error(ctx context.Context, msg string, fields ...Field)

	// With creates a child logger with additional fields that will be included in all log entries.
	With(fields ...Field) Logger
}
