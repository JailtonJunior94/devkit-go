package otel

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
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

// slogAttrPool and otlpAttrPool pool attribute slices for the logger hot path.
// Eliminates per-call heap allocation for the two parallel attribute lists.
var (
	slogAttrPool = sync.Pool{New: func() any { s := make([]slog.Attr, 0, 16); return &s }}
	otlpAttrPool = sync.Pool{New: func() any { s := make([]otellog.KeyValue, 0, 16); return &s }}
)

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
	switch level {
	case observability.LogLevelDebug:
		return slog.LevelDebug
	case observability.LogLevelInfo:
		return slog.LevelInfo
	case observability.LogLevelWarn:
		return slog.LevelWarn
	case observability.LogLevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
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

// log is the internal logging method. It performs a single pass over fields to build
// both the slog and OTLP attribute lists, using pooled slices to avoid per-call allocation.
func (l *otelLogger) log(ctx context.Context, level slog.Level, msg string, fields ...observability.Field) {
	// Fast path: skip all work if the level is disabled.
	if !l.slogLogger.Enabled(ctx, level) {
		return
	}

	if msg == "" {
		msg = "[empty message]"
	}

	fields = sanitizeFields(fields)

	// Acquire pooled slices for zero-allocation single-pass conversion.
	sp := slogAttrPool.Get().(*[]slog.Attr)
	op := otlpAttrPool.Get().(*[]otellog.KeyValue)
	slogAttrs := (*sp)[:0]
	otlpAttrs := (*op)[:0]

	// Single pass: permanent fields (from With).
	for _, f := range l.fields {
		slogAttrs = append(slogAttrs, convertFieldToSlogAttr(f))
		otlpAttrs = append(otlpAttrs, convertFieldToOTelAttr(f))
	}

	// Single pass: call-specific fields.
	for _, f := range fields {
		slogAttrs = append(slogAttrs, convertFieldToSlogAttr(f))
		otlpAttrs = append(otlpAttrs, convertFieldToOTelAttr(f))
	}

	// Service field (always a string — skip generic conversion).
	slogAttrs = append(slogAttrs, slog.String("service", l.serviceName))
	otlpAttrs = append(otlpAttrs, otellog.String("service", l.serviceName))

	// Inject trace context into slog output only.
	// The OTel SDK extracts trace context from ctx automatically during Emit,
	// so adding it manually to OTLP attrs would duplicate it in the backend.
	span := trace.SpanFromContext(ctx)
	spanCtx := span.SpanContext()
	if spanCtx.IsValid() {
		slogAttrs = append(slogAttrs,
			slog.String("trace_id", spanCtx.TraceID().String()),
			slog.String("span_id", spanCtx.SpanID().String()),
		)
	}

	l.slogLogger.LogAttrs(ctx, level, msg, slogAttrs...)

	record := otellog.Record{}
	record.SetTimestamp(time.Now())
	record.SetBody(otellog.StringValue(msg))
	record.SetSeverity(convertSlogLevelToOTel(level))
	record.SetSeverityText(level.String())
	record.AddAttributes(otlpAttrs...)
	l.otelLog.Emit(ctx, record)

	// Return slices to pool. Update pointers in case append reallocated.
	*sp = slogAttrs[:0]
	*op = otlpAttrs[:0]
	slogAttrPool.Put(sp)
	otlpAttrPool.Put(op)
}

// convertSlogLevelToOTel converts slog.Level to OTel Severity.
func convertSlogLevelToOTel(level slog.Level) otellog.Severity {
	switch level {
	case slog.LevelDebug:
		return otellog.SeverityDebug
	case slog.LevelInfo:
		return otellog.SeverityInfo
	case slog.LevelWarn:
		return otellog.SeverityWarn
	case slog.LevelError:
		return otellog.SeverityError
	default:
		return otellog.SeverityInfo
	}
}

// convertFieldToOTelAttr converts an observability.Field to an OTel log KeyValue.
// Uses the Field discriminated union — zero boxing for common types.
func convertFieldToOTelAttr(field observability.Field) otellog.KeyValue {
	switch field.Kind() {
	case observability.FieldKindString:
		return otellog.String(field.Key, field.StringValue())
	case observability.FieldKindInt:
		return otellog.Int(field.Key, int(field.Int64Value()))
	case observability.FieldKindInt64:
		return otellog.Int64(field.Key, field.Int64Value())
	case observability.FieldKindFloat64:
		return otellog.Float64(field.Key, field.Float64Value())
	case observability.FieldKindBool:
		return otellog.Bool(field.Key, field.BoolValue())
	case observability.FieldKindError:
		if err, ok := field.AnyValue().(error); ok {
			return otellog.String(field.Key, err.Error())
		}
		return otellog.String(field.Key, "")
	default:
		return otellog.String(field.Key, fmt.Sprintf("%v", field.AnyValue()))
	}
}

// With creates a child logger with additional fields.
// Creates a deep copy of fields to prevent race conditions.
func (l *otelLogger) With(fields ...observability.Field) observability.Logger {
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
// Uses the Field discriminated union — zero boxing for common types.
func convertFieldToSlogAttr(field observability.Field) slog.Attr {
	switch field.Kind() {
	case observability.FieldKindString:
		return slog.String(field.Key, field.StringValue())
	case observability.FieldKindInt:
		return slog.Int(field.Key, int(field.Int64Value()))
	case observability.FieldKindInt64:
		return slog.Int64(field.Key, field.Int64Value())
	case observability.FieldKindFloat64:
		return slog.Float64(field.Key, field.Float64Value())
	case observability.FieldKindBool:
		return slog.Bool(field.Key, field.BoolValue())
	case observability.FieldKindError:
		if err, ok := field.AnyValue().(error); ok {
			return slog.String(field.Key, err.Error())
		}
		return slog.String(field.Key, "")
	default:
		return slog.Any(field.Key, field.AnyValue())
	}
}

// sanitizeFields sanitizes, validates, and redacts sensitive data from fields.
func sanitizeFields(fields []observability.Field) []observability.Field {
	// Limit number of fields to prevent cardinality explosion.
	if len(fields) > maxFields {
		fields = fields[:maxFields]
	}

	// First pass: check if sanitization is needed to avoid unnecessary allocations.
	needsSanitization := false
	for _, field := range fields {
		if isSensitiveKey(field.Key) {
			needsSanitization = true
			break
		}
		if field.Kind() == observability.FieldKindString && len(field.StringValue()) > maxFieldValueLength {
			needsSanitization = true
			break
		}
	}

	// If no sanitization needed, return original slice (zero allocation).
	if !needsSanitization {
		return fields
	}

	// Only allocate when sanitization is actually required.
	sanitized := make([]observability.Field, len(fields))
	for i, field := range fields {
		// Redact sensitive keys.
		if isSensitiveKey(field.Key) {
			sanitized[i] = observability.String(field.Key, redactedValue)
			continue
		}

		// Truncate long string values.
		if field.Kind() == observability.FieldKindString {
			s := field.StringValue()
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
