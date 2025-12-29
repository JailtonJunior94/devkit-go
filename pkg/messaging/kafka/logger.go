package kafka

import "context"

// Logger defines the interface for structured logging.
// This allows the Kafka client to work with any logging library (zap, slog, logrus, etc.).
type Logger interface {
	// Debug logs a debug message with optional fields.
	Debug(ctx context.Context, msg string, fields ...Field)
	// Info logs an info message with optional fields.
	Info(ctx context.Context, msg string, fields ...Field)
	// Warn logs a warning message with optional fields.
	Warn(ctx context.Context, msg string, fields ...Field)
	// Error logs an error message with optional fields.
	Error(ctx context.Context, msg string, fields ...Field)
}

// Field represents a key-value pair for structured logging.
type Field struct {
	Key   string
	Value interface{}
}

// noopLogger is a logger that does nothing.
// Used as default when no logger is provided.
type noopLogger struct{}

func (n *noopLogger) Debug(ctx context.Context, msg string, fields ...Field) {}
func (n *noopLogger) Info(ctx context.Context, msg string, fields ...Field)  {}
func (n *noopLogger) Warn(ctx context.Context, msg string, fields ...Field)  {}
func (n *noopLogger) Error(ctx context.Context, msg string, fields ...Field) {}

// NewNoopLogger returns a logger that does nothing.
func NewNoopLogger() Logger {
	return &noopLogger{}
}
