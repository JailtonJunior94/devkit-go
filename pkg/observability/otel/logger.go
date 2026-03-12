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

// hexTable is used by encodeHex to avoid an import of encoding/hex.
const hexTable = "0123456789abcdef"

// encodeHex encodes src bytes as lowercase hex into dst.
// dst must have length >= 2*len(src). No allocations.
func encodeHex(dst, src []byte) {
	for i, b := range src {
		dst[i*2] = hexTable[b>>4]
		dst[i*2+1] = hexTable[b&0xf]
	}
}

// asciiContainsFold reports whether s contains substr using ASCII case-insensitive
// comparison. substr must already be lowercase (as stored in sensitiveKeysLower).
// Zero allocations — avoids strings.ToLower(s) which heap-allocates a new string.
func asciiContainsFold(s, substr string) bool {
	ls, lsub := len(s), len(substr)
	if lsub == 0 {
		return true
	}
	if lsub > ls {
		return false
	}
outer:
	for i := 0; i <= ls-lsub; i++ {
		for j := 0; j < lsub; j++ {
			c := s[i+j]
			if c >= 'A' && c <= 'Z' {
				c += 'a' - 'A' // toLower for ASCII only — no alloc
			}
			if c != substr[j] {
				continue outer
			}
		}
		return true
	}
	return false
}

// slogAttrPool and otlpAttrPool pool attribute slices for the logger hot path.
// Eliminates per-call heap allocation for the two parallel attribute lists.
var (
	slogAttrPool = sync.Pool{New: func() any { s := make([]slog.Attr, 0, 16); return &s }}
	otlpAttrPool = sync.Pool{New: func() any { s := make([]otellog.KeyValue, 0, 16); return &s }}
)

// otelLogger implements observability.Logger using OTel Logger API.
//
// Field ordering is optimised for cache-line locality on 64-bit systems (8-byte words):
//
//	CL0 (bytes 0–63):  otelLog(16) + slogLogger(8) + serviceName(16) + fields(24) = 64 B
//	CL1 (bytes 64–…):  sanitize(1) + console(1) + pad(6) + level(16) + format(16)
//
// All fields touched on every log call (otelLog, slogLogger, serviceName, fields) fit in a
// single cache line, avoiding a second L1/L2 fetch on the hot path.
//
// Hot path (console=false, default):
//   - No slog mutex, no I/O, no JSON encoding on the calling goroutine
//   - Only the OTel BatchProcessor channel send (async, lock-free on the write side)
//   - OTel SDK automatically extracts TraceID/SpanID from ctx during Emit
//
// Development path (console=true):
//   - slog JSON → stdout (sync, mutex), then OTLP emit
//   - TraceID/SpanID injected into slog output (human readable)
type otelLogger struct {
	otelLog     otellog.Logger        // CL0: 0–15   (interface: type ptr + data ptr)
	slogLogger  *slog.Logger          // CL0: 16–23  (pointer; always non-nil for Enabled() check)
	serviceName string                // CL0: 24–39  (string header: ptr + len)
	fields      []observability.Field // CL0: 40–63  (slice header: ptr + len + cap)
	sanitize    bool                  // CL1: 64     (opt-in sensitive-field redaction)
	console     bool                  // CL1: 65     (enable slog stdout — development only)
	level       observability.LogLevel  // CL1: 72–87 (padded to align 8)
	format      observability.LogFormat // CL1: 88–103
}

// newOtelLogger creates a new logger.
//
//   - sanitize: redact sensitive fields + truncate long values (Config.Sanitize)
//   - console:  also write JSON to stdout via slog (Config.ConsoleLog, development only)
//
// When console=false (default) the slog logger is still created — its Enabled() method
// provides the level check via a plain integer comparison with no lock.
func newOtelLogger(
	level observability.LogLevel,
	format observability.LogFormat,
	serviceName string,
	otelLog otellog.Logger,
	sanitize bool,
	console bool,
) *otelLogger {
	return &otelLogger{
		otelLog:     otelLog,
		slogLogger:  createSlogLogger(level, format, os.Stdout),
		serviceName: serviceName,
		fields:      nil,
		sanitize:    sanitize,
		console:     console,
		level:       level,
		format:      format,
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

// log is the internal dispatch method.
//
// Level check: slogLogger.Enabled() delegates to commonHandler.Enabled() which is
// a plain integer comparison — no mutex, no allocation, always in-cache (CL0).
//
// console=false (default, production):
//
//	Only the OTel OTLP path runs. No slog JSON encoding, no sync.Mutex, no I/O
//	on the calling goroutine. OTel SDK BatchProcessor sends to an async channel.
//	OTel SDK automatically extracts TraceID/SpanID from ctx during Emit —
//	no manual hex encoding needed here.
//
// console=true (development):
//
//	Calls logConsole(), which writes JSON to stdout (acquires slog mutex) then
//	emits to OTLP. TraceID/SpanID injected into the slog attrs for human readability.
func (l *otelLogger) log(ctx context.Context, level slog.Level, msg string, fields ...observability.Field) {
	// Level check: integer comparison, no lock.
	if !l.slogLogger.Enabled(ctx, level) {
		return
	}

	if msg == "" {
		msg = "[empty message]"
	}

	// sanitizeFields is O(n×m) — only pay the cost when explicitly opted in.
	if l.sanitize {
		fields = sanitizeFields(fields)
	}

	if l.console {
		l.logConsole(ctx, level, msg, fields)
		return
	}

	// ── Production fast path: OTLP only ─────────────────────────────────────
	// One pool acquire, one pass over fields, one async channel send.
	// No mutex, no JSON encoding, no TraceID hex, no I/O on this goroutine.
	op := otlpAttrPool.Get().(*[]otellog.KeyValue)
	otlpAttrs := (*op)[:0]

	for _, f := range l.fields {
		otlpAttrs = append(otlpAttrs, convertFieldToOTelAttr(f))
	}
	for _, f := range fields {
		otlpAttrs = append(otlpAttrs, convertFieldToOTelAttr(f))
	}
	otlpAttrs = append(otlpAttrs, otellog.String("service", l.serviceName))

	record := otellog.Record{}
	record.SetTimestamp(time.Now())
	record.SetBody(otellog.StringValue(msg))
	record.SetSeverity(convertSlogLevelToOTel(level))
	record.SetSeverityText(level.String())
	record.AddAttributes(otlpAttrs...)
	l.otelLog.Emit(ctx, record) // async: SDK BatchProcessor channel send

	*op = otlpAttrs[:0]
	otlpAttrPool.Put(op)
}

// logConsole is the development path: slog JSON stdout + OTLP emit.
// Separated from log() so the inliner can keep the production fast path lean.
// Called only when Config.ConsoleLog=true.
//
// Cost: acquires slog's sync.Mutex for JSON serialisation → not suitable for
// production workloads under concurrent load (use console=false there).
func (l *otelLogger) logConsole(ctx context.Context, level slog.Level, msg string, fields []observability.Field) {
	sp := slogAttrPool.Get().(*[]slog.Attr)
	op := otlpAttrPool.Get().(*[]otellog.KeyValue)
	slogAttrs := (*sp)[:0]
	otlpAttrs := (*op)[:0]

	// Single pass over both permanent and call-specific fields.
	for _, f := range l.fields {
		slogAttrs = append(slogAttrs, convertFieldToSlogAttr(f))
		otlpAttrs = append(otlpAttrs, convertFieldToOTelAttr(f))
	}
	for _, f := range fields {
		slogAttrs = append(slogAttrs, convertFieldToSlogAttr(f))
		otlpAttrs = append(otlpAttrs, convertFieldToOTelAttr(f))
	}

	slogAttrs = append(slogAttrs, slog.String("service", l.serviceName))
	otlpAttrs = append(otlpAttrs, otellog.String("service", l.serviceName))

	// Inject trace context into slog output only.
	// OTel SDK extracts it from ctx automatically during Emit — don't duplicate.
	span := trace.SpanFromContext(ctx)
	spanCtx := span.SpanContext()
	if spanCtx.IsValid() {
		// Stack-allocated hex buffers: 2 allocs (string copies) instead of 4
		// (hex.EncodeToString allocates []byte + string per call).
		tid := spanCtx.TraceID()
		sid := spanCtx.SpanID()
		var tidHex [32]byte
		var sidHex [16]byte
		encodeHex(tidHex[:], tid[:])
		encodeHex(sidHex[:], sid[:])
		slogAttrs = append(slogAttrs,
			slog.String("trace_id", string(tidHex[:])),
			slog.String("span_id", string(sidHex[:])),
		)
	}

	l.slogLogger.LogAttrs(ctx, level, msg, slogAttrs...) // ← acquires mutex here

	record := otellog.Record{}
	record.SetTimestamp(time.Now())
	record.SetBody(otellog.StringValue(msg))
	record.SetSeverity(convertSlogLevelToOTel(level))
	record.SetSeverityText(level.String())
	record.AddAttributes(otlpAttrs...)
	l.otelLog.Emit(ctx, record)

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
		serviceName: l.serviceName,
		fields:      newFields,
		sanitize:    l.sanitize,
		console:     l.console,
		level:       l.level,
		format:      l.format,
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
// Uses asciiContainsFold for zero-allocation case-insensitive ASCII comparison.
// Previously used strings.ToLower(key) which heap-allocated a new string on
// every call for every field; now zero allocations in the common case.
func isSensitiveKey(key string) bool {
	for _, sensitive := range sensitiveKeysLower {
		if asciiContainsFold(key, sensitive) {
			return true
		}
	}
	return false
}
