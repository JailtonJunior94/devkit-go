package otel

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/trace"
)

const (
	redactedValue       = "[REDACTED]"
	maxFields           = 50   // Maximum number of fields per log entry
	maxFieldValueLength = 2048 // Maximum length of a field value
)

// defaultSensitiveKeys contains common sensitive field names that should be redacted.
// This is private to prevent external modification and ensure thread-safety.
var defaultSensitiveKeys = []string{
	"password", "passwd", "pwd", "secret", "token", "api_key", "apikey", "api-key",
	"authorization", "auth", "credential", "credentials", "private_key", "privatekey",
	"ssn", "social_security", "credit_card", "creditcard", "card_number", "cvv", "pin",
	"access_token", "refresh_token", "bearer", "session", "cookie",
}

// sensitiveKeysLower contains lowercase versions of sensitive keys for efficient comparison.
// Pre-computed once at initialization to avoid repeated ToLower() calls.
var sensitiveKeysLower = initSensitiveKeysLower()

// initSensitiveKeysLower pre-computes lowercase versions of sensitive keys.
func initSensitiveKeysLower() []string {
	lower := make([]string, len(defaultSensitiveKeys))
	for i, k := range defaultSensitiveKeys {
		lower[i] = strings.ToLower(k)
	}
	return lower
}

// otelLogger implements observability.Logger using OTel Logger API with slog fallback.
type otelLogger struct {
	otelLog     otellog.Logger // OTel logger for OTLP export
	slogLogger  *slog.Logger   // Slog logger for console output
	level       observability.LogLevel
	format      observability.LogFormat
	serviceName string
	fields      []observability.Field
}

// newOtelLogger creates a new logger with the specified level and format.
func newOtelLogger(
	level observability.LogLevel,
	format observability.LogFormat,
	serviceName string,
	otelLog otellog.Logger,
) *otelLogger {
	return &otelLogger{
		otelLog:     otelLog,
		slogLogger:  createSlogLogger(level, format, os.Stdout),
		level:       level,
		format:      format,
		serviceName: serviceName,
		fields:      nil,
	}
}

// createSlogLogger creates a slog logger with the specified configuration.
func createSlogLogger(level observability.LogLevel, format observability.LogFormat, output io.Writer) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level: convertLogLevel(level),
	}

	handler := createSlogHandler(format, output, opts)
	return slog.New(handler)
}

// createSlogHandler creates the appropriate slog handler based on format.
func createSlogHandler(format observability.LogFormat, output io.Writer, opts *slog.HandlerOptions) slog.Handler {
	if format == observability.LogFormatJSON {
		return slog.NewJSONHandler(output, opts)
	}

	return slog.NewTextHandler(output, opts)
}

// convertLogLevel converts observability.LogLevel to slog.Level.
func convertLogLevel(level observability.LogLevel) slog.Level {
	levelMap := map[observability.LogLevel]slog.Level{
		observability.LogLevelDebug: slog.LevelDebug,
		observability.LogLevelInfo:  slog.LevelInfo,
		observability.LogLevelWarn:  slog.LevelWarn,
		observability.LogLevelError: slog.LevelError,
	}

	if slogLevel, exists := levelMap[level]; exists {
		return slogLevel
	}

	return slog.LevelInfo
}

// Debug logs a debug-level message.
func (l *otelLogger) Debug(ctx context.Context, msg string, fields ...observability.Field) {
	l.log(ctx, slog.LevelDebug, msg, fields...)
}

// Info logs an info-level message.
func (l *otelLogger) Info(ctx context.Context, msg string, fields ...observability.Field) {
	l.log(ctx, slog.LevelInfo, msg, fields...)
}

// Warn logs a warning-level message.
func (l *otelLogger) Warn(ctx context.Context, msg string, fields ...observability.Field) {
	l.log(ctx, slog.LevelWarn, msg, fields...)
}

// Error logs an error-level message.
func (l *otelLogger) Error(ctx context.Context, msg string, fields ...observability.Field) {
	l.log(ctx, slog.LevelError, msg, fields...)
}

// log is the internal logging method that adds trace context and structured fields.
func (l *otelLogger) log(ctx context.Context, level slog.Level, msg string, fields ...observability.Field) {
	// Validate message
	if msg == "" {
		msg = "[empty message]"
	}

	// Sanitize and validate fields
	fields = sanitizeFields(fields)

	// Combine permanent fields with call-specific fields
	allFields := make([]observability.Field, 0, len(l.fields)+len(fields)+3)
	allFields = append(allFields, l.fields...)
	allFields = append(allFields, fields...)

	// Extract trace context from the context
	span := trace.SpanFromContext(ctx)
	spanContext := span.SpanContext()
	if spanContext.IsValid() {
		allFields = append(allFields,
			observability.String("trace_id", spanContext.TraceID().String()),
			observability.String("span_id", spanContext.SpanID().String()),
		)
	}

	// Add service name
	allFields = append(allFields, observability.String("service", l.serviceName))

	// Convert fields to slog.Attr for console output
	attrs := make([]slog.Attr, 0, len(allFields))
	for _, field := range allFields {
		attrs = append(attrs, convertFieldToSlogAttr(field))
	}

	// Log to console with slog
	l.slogLogger.LogAttrs(ctx, level, msg, attrs...)

	// Also emit to OTLP
	l.emitOTLPLog(ctx, level, msg, allFields)
}

// emitOTLPLog emits a log record to OTLP backend.
func (l *otelLogger) emitOTLPLog(
	ctx context.Context,
	level slog.Level,
	msg string,

	fields []observability.Field,
) {
	// Convert fields to OTel log attributes
	attrs := make([]otellog.KeyValue, 0, len(fields))
	for _, field := range fields {
		attrs = append(attrs, convertFieldToOTelAttr(field))
	}

	// Create log record
	record := otellog.Record{}
	record.SetTimestamp(time.Now())
	record.SetBody(otellog.StringValue(msg))
	record.SetSeverity(convertSlogLevelToOTel(level))
	record.SetSeverityText(level.String())
	record.AddAttributes(attrs...)

	// Emit the log record (trace context is automatically extracted from ctx by the SDK)
	l.otelLog.Emit(ctx, record)
}

// convertSlogLevelToOTel converts slog.Level to OTel Severity.
func convertSlogLevelToOTel(level slog.Level) otellog.Severity {
	severityMap := map[slog.Level]otellog.Severity{
		slog.LevelDebug: otellog.SeverityDebug,
		slog.LevelInfo:  otellog.SeverityInfo,
		slog.LevelWarn:  otellog.SeverityWarn,
		slog.LevelError: otellog.SeverityError,
	}

	if severity, exists := severityMap[level]; exists {
		return severity
	}

	return otellog.SeverityInfo
}

// convertFieldToOTelAttr converts an observability.Field to an OTel log KeyValue.
func convertFieldToOTelAttr(field observability.Field) otellog.KeyValue {
	switch v := field.Value.(type) {
	case string:
		return otellog.String(field.Key, v)
	case int:
		return otellog.Int(field.Key, v)
	case int64:
		return otellog.Int64(field.Key, v)
	case float64:
		return otellog.Float64(field.Key, v)
	case bool:
		return otellog.Bool(field.Key, v)
	case error:
		return otellog.String(field.Key, v.Error())
	default:
		return otellog.String(field.Key, fmt.Sprint(field.Value))
	}
}

// With creates a child logger with additional fields.
// Creates a deep copy of fields to prevent race conditions.
func (l *otelLogger) With(fields ...observability.Field) observability.Logger {
	// Create new slice with deep copy to prevent race conditions
	// If we used append(l.fields, fields...) and l.fields had capacity,
	// it would modify the underlying array shared by other loggers
	newFields := make([]observability.Field, len(l.fields)+len(fields))
	copy(newFields, l.fields)
	copy(newFields[len(l.fields):], fields)

	return &otelLogger{
		otelLog:     l.otelLog,
		slogLogger:  l.slogLogger,
		level:       l.level,
		format:      l.format,
		serviceName: l.serviceName,
		fields:      newFields,
	}
}

// convertFieldToSlogAttr converts an observability.Field to a slog.Attr.
func convertFieldToSlogAttr(field observability.Field) slog.Attr {
	switch v := field.Value.(type) {
	case string:
		return slog.String(field.Key, v)
	case int:
		return slog.Int(field.Key, v)
	case int64:
		return slog.Int64(field.Key, v)
	case float64:
		return slog.Float64(field.Key, v)
	case bool:
		return slog.Bool(field.Key, v)
	case error:
		return slog.String(field.Key, v.Error())
	default:
		return slog.Any(field.Key, field.Value)
	}
}

// sanitizeFields sanitizes, validates, and redacts sensitive data from fields.
func sanitizeFields(fields []observability.Field) []observability.Field {
	// Limit number of fields to prevent cardinality explosion
	if len(fields) > maxFields {
		fields = fields[:maxFields]
	}

	// First pass: check if sanitization is needed to avoid unnecessary allocations
	needsSanitization := false
	for _, field := range fields {
		if isSensitiveKey(field.Key) {
			needsSanitization = true
			break
		}
		if s, ok := field.Value.(string); ok && len(s) > maxFieldValueLength {
			needsSanitization = true
			break
		}
	}

	// If no sanitization needed, return original slice (zero allocation)
	if !needsSanitization {
		return fields
	}

	// Only allocate when sanitization is actually required
	sanitized := make([]observability.Field, len(fields))
	for i, field := range fields {
		// Redact sensitive keys
		if isSensitiveKey(field.Key) {
			sanitized[i] = observability.String(field.Key, redactedValue)
			continue
		}

		// Truncate long string values
		if s, ok := field.Value.(string); ok {
			if len(s) > maxFieldValueLength {
				sanitized[i] = observability.String(field.Key, s[:maxFieldValueLength]+"...[truncated]")
				continue
			}
		}

		sanitized[i] = field
	}

	return sanitized
}

// isSensitiveKey checks if a field key matches any sensitive key pattern.
// Uses pre-computed lowercase sensitive keys for performance.
func isSensitiveKey(key string) bool {
	keyLower := strings.ToLower(key)
	for _, sensitive := range sensitiveKeysLower {
		if strings.Contains(keyLower, sensitive) {
			return true
		}
	}
	return false
}
