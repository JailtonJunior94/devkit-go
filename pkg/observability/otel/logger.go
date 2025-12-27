package otel

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/jailtonjunior94/order/pkg/observability"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/trace"
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
		fields:      make([]observability.Field, 0),
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
	converters := []func(observability.Field) (otellog.KeyValue, bool){
		tryConvertString,
		tryConvertInt,
		tryConvertInt64,
		tryConvertFloat64,
		tryConvertBool,
		tryConvertError,
	}

	for _, converter := range converters {
		if kv, ok := converter(field); ok {
			return kv
		}
	}

	return otellog.String(field.Key, fmt.Sprint(field.Value))
}

func tryConvertString(field observability.Field) (otellog.KeyValue, bool) {
	if v, ok := field.Value.(string); ok {
		return otellog.String(field.Key, v), true
	}
	return otellog.KeyValue{}, false
}

func tryConvertInt(field observability.Field) (otellog.KeyValue, bool) {
	if v, ok := field.Value.(int); ok {
		return otellog.Int(field.Key, v), true
	}
	return otellog.KeyValue{}, false
}

func tryConvertInt64(field observability.Field) (otellog.KeyValue, bool) {
	if v, ok := field.Value.(int64); ok {
		return otellog.Int64(field.Key, v), true
	}
	return otellog.KeyValue{}, false
}

func tryConvertFloat64(field observability.Field) (otellog.KeyValue, bool) {
	if v, ok := field.Value.(float64); ok {
		return otellog.Float64(field.Key, v), true
	}
	return otellog.KeyValue{}, false
}

func tryConvertBool(field observability.Field) (otellog.KeyValue, bool) {
	if v, ok := field.Value.(bool); ok {
		return otellog.Bool(field.Key, v), true
	}
	return otellog.KeyValue{}, false
}

func tryConvertError(field observability.Field) (otellog.KeyValue, bool) {
	if v, ok := field.Value.(error); ok {
		return otellog.String(field.Key, v.Error()), true
	}
	return otellog.KeyValue{}, false
}

// With creates a child logger with additional fields.
func (l *otelLogger) With(fields ...observability.Field) observability.Logger {
	return &otelLogger{
		otelLog:     l.otelLog,
		slogLogger:  l.slogLogger,
		level:       l.level,
		format:      l.format,
		serviceName: l.serviceName,
		fields:      append(l.fields, fields...),
	}
}

// convertFieldToSlogAttr converts an observability.Field to a slog.Attr.
func convertFieldToSlogAttr(field observability.Field) slog.Attr {
	converters := []func(observability.Field) (slog.Attr, bool){
		tryConvertSlogString,
		tryConvertSlogInt,
		tryConvertSlogInt64,
		tryConvertSlogFloat64,
		tryConvertSlogBool,
		tryConvertSlogError,
	}

	for _, converter := range converters {
		if attr, ok := converter(field); ok {
			return attr
		}
	}

	return slog.Any(field.Key, field.Value)
}

func tryConvertSlogString(field observability.Field) (slog.Attr, bool) {
	if v, ok := field.Value.(string); ok {
		return slog.String(field.Key, v), true
	}
	return slog.Attr{}, false
}

func tryConvertSlogInt(field observability.Field) (slog.Attr, bool) {
	if v, ok := field.Value.(int); ok {
		return slog.Int(field.Key, v), true
	}
	return slog.Attr{}, false
}

func tryConvertSlogInt64(field observability.Field) (slog.Attr, bool) {
	if v, ok := field.Value.(int64); ok {
		return slog.Int64(field.Key, v), true
	}
	return slog.Attr{}, false
}

func tryConvertSlogFloat64(field observability.Field) (slog.Attr, bool) {
	if v, ok := field.Value.(float64); ok {
		return slog.Float64(field.Key, v), true
	}
	return slog.Attr{}, false
}

func tryConvertSlogBool(field observability.Field) (slog.Attr, bool) {
	if v, ok := field.Value.(bool); ok {
		return slog.Bool(field.Key, v), true
	}
	return slog.Attr{}, false
}

func tryConvertSlogError(field observability.Field) (slog.Attr, bool) {
	if v, ok := field.Value.(error); ok {
		return slog.String(field.Key, v.Error()), true
	}
	return slog.Attr{}, false
}
