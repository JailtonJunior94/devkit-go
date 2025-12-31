package migration

import "context"

// Logger provides structured logging capabilities for migration operations.
// This interface is compatible with the observability.Logger interface.
type Logger interface {
	Debug(ctx context.Context, msg string, fields ...Field)
	Info(ctx context.Context, msg string, fields ...Field)
	Warn(ctx context.Context, msg string, fields ...Field)
	Error(ctx context.Context, msg string, fields ...Field)
}

// Field represents a key-value pair for structured logging.
type Field struct {
	Key   string
	Value any
}

// String creates a string field.
func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

// Int creates an integer field.
func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

// Uint creates an unsigned integer field.
func Uint(key string, value uint) Field {
	return Field{Key: key, Value: value}
}

// Bool creates a boolean field.
func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

// Error creates an error field.
func Error(err error) Field {
	return Field{Key: "error", Value: err}
}

// Any creates a field with any value type.
func Any(key string, value any) Field {
	return Field{Key: key, Value: value}
}

// noopLogger is a no-op implementation of Logger that discards all log messages.
type noopLogger struct{}

// NewNoopLogger creates a logger that discards all log messages.
// This is useful when logging is not required or during testing.
func NewNoopLogger() Logger {
	return &noopLogger{}
}

func (n *noopLogger) Debug(ctx context.Context, msg string, fields ...Field) {}
func (n *noopLogger) Info(ctx context.Context, msg string, fields ...Field)  {}
func (n *noopLogger) Warn(ctx context.Context, msg string, fields ...Field)  {}
func (n *noopLogger) Error(ctx context.Context, msg string, fields ...Field) {}
