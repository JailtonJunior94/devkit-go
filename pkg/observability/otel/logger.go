package otel

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/trace"
)

const (
	redactedValue       = "[REDACTED]"
	maxFields           = 50   // máximo de fields por entrada de log
	maxFieldValueLength = 2048 // tamanho máximo de um valor de field
)

var defaultSensitiveKeys = []string{
	"password", "passwd", "pwd", "secret", "token", "api_key", "apikey", "api-key",
	"authorization", "auth", "credential", "credentials", "private_key", "privatekey",
	"ssn", "social_security", "credit_card", "creditcard", "card_number", "cvv", "pin",
	"access_token", "refresh_token", "bearer", "session", "cookie",
}

// sensitiveKeysLower é pré-computado na inicialização para evitar ToLower() por chamada.
var sensitiveKeysLower = initSensitiveKeysLower()

func initSensitiveKeysLower() []string {
	lower := make([]string, len(defaultSensitiveKeys))
	for i, k := range defaultSensitiveKeys {
		lower[i] = strings.ToLower(k)
	}
	return lower
}

const hexTable = "0123456789abcdef"

func encodeHex(dst, src []byte) {
	for i, b := range src {
		dst[i*2] = hexTable[b>>4]
		dst[i*2+1] = hexTable[b&0xf]
	}
}

func formatTraceID(id trace.TraceID) string {
	var buf [32]byte
	encodeHex(buf[:], id[:])
	return string(buf[:])
}

func formatSpanID(id trace.SpanID) string {
	var buf [16]byte
	encodeHex(buf[:], id[:])
	return string(buf[:])
}

// asciiContainsFold compara sem alocar — evita strings.ToLower(s) que heap-aloca nova string.
// substr deve estar em lowercase (como armazenado em sensitiveKeysLower).
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
				c += 'a' - 'A'
			}
			if c != substr[j] {
				continue outer
			}
		}
		return true
	}
	return false
}

var slogAttrPool = sync.Pool{New: func() any { s := make([]slog.Attr, 0, 16); return &s }}

// otelLogger usa o bridge oficial slog → OTel Logs.
// console=true serializa via sync.Mutex (slog.JSONHandler) — não usar em produção sob carga.
type otelLogger struct {
	bridgeLogger  *slog.Logger
	consoleLogger *slog.Logger
	serviceName   string
	environment   string
	fields        []observability.Field
	sanitize      bool
	console       bool
	level         observability.LogLevel
	format        observability.LogFormat
}

func newOtelLogger(
	level observability.LogLevel,
	format observability.LogFormat,
	serviceName string,
	environment string,
	loggerProvider *sdklog.LoggerProvider,
	sanitize bool,
	console bool,
) *otelLogger {
	return &otelLogger{
		bridgeLogger:  createBridgeLogger(serviceName, loggerProvider),
		consoleLogger: createSlogLogger(level, format, os.Stdout),
		serviceName:   serviceName,
		environment:   environment,
		fields:        nil,
		sanitize:      sanitize,
		console:       console,
		level:         level,
		format:        format,
	}
}

func createBridgeLogger(serviceName string, provider *sdklog.LoggerProvider) *slog.Logger {
	return slog.New(otelslog.NewHandler(serviceName,
		otelslog.WithLoggerProvider(provider),
	))
}

func createSlogLogger(level observability.LogLevel, format observability.LogFormat, output io.Writer) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level: convertLogLevel(level),
	}
	handler := createSlogHandler(format, output, opts)
	return slog.New(handler)
}

func createSlogHandler(format observability.LogFormat, output io.Writer, opts *slog.HandlerOptions) slog.Handler {
	if format == observability.LogFormatJSON {
		return slog.NewJSONHandler(output, opts)
	}
	return slog.NewTextHandler(output, opts)
}

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

func (l *otelLogger) Debug(ctx context.Context, msg string, fields ...observability.Field) {
	l.log(ctx, slog.LevelDebug, msg, fields...)
}

func (l *otelLogger) Info(ctx context.Context, msg string, fields ...observability.Field) {
	l.log(ctx, slog.LevelInfo, msg, fields...)
}

func (l *otelLogger) Warn(ctx context.Context, msg string, fields ...observability.Field) {
	l.log(ctx, slog.LevelWarn, msg, fields...)
}

func (l *otelLogger) Error(ctx context.Context, msg string, fields ...observability.Field) {
	l.log(ctx, slog.LevelError, msg, fields...)
}

func (l *otelLogger) log(ctx context.Context, level slog.Level, msg string, fields ...observability.Field) {
	if !l.consoleLogger.Enabled(ctx, level) {
		return
	}

	if msg == "" {
		msg = "[empty message]"
	}

	if l.sanitize {
		fields = sanitizeFields(fields)
	}

	if l.console {
		l.logConsole(ctx, level, msg, fields)
		return
	}

	sp := slogAttrPool.Get().(*[]slog.Attr)
	slogAttrs := (*sp)[:0]

	for _, f := range l.fields {
		slogAttrs = append(slogAttrs, convertFieldToSlogAttr(f))
	}
	for _, f := range fields {
		slogAttrs = append(slogAttrs, convertFieldToSlogAttr(f))
	}
	slogAttrs = l.appendCorrelationAttrs(ctx, slogAttrs)

	l.bridgeLogger.LogAttrs(ctx, level, msg, slogAttrs...)

	*sp = slogAttrs[:0]
	slogAttrPool.Put(sp)
}

// logConsole é o caminho de desenvolvimento. Separado de log() para manter o fast path inlinável.
// AVISO: adquire sync.Mutex do slog.JSONHandler — não usar em produção sob carga concorrente.
func (l *otelLogger) logConsole(ctx context.Context, level slog.Level, msg string, fields []observability.Field) {
	sp := slogAttrPool.Get().(*[]slog.Attr)
	slogAttrs := (*sp)[:0]

	for _, f := range l.fields {
		slogAttrs = append(slogAttrs, convertFieldToSlogAttr(f))
	}
	for _, f := range fields {
		slogAttrs = append(slogAttrs, convertFieldToSlogAttr(f))
	}
	slogAttrs = l.appendCorrelationAttrs(ctx, slogAttrs)

	l.consoleLogger.LogAttrs(ctx, level, msg, slogAttrs...)
	l.bridgeLogger.LogAttrs(ctx, level, msg, slogAttrs...)

	*sp = slogAttrs[:0]
	slogAttrPool.Put(sp)
}

func (l *otelLogger) appendCorrelationAttrs(ctx context.Context, attrs []slog.Attr) []slog.Attr {
	correlation, _ := CorrelationFromContext(ctx)
	if spanCtx := trace.SpanContextFromContext(ctx); spanCtx.IsValid() {
		correlation.TraceID = formatTraceID(spanCtx.TraceID())
		correlation.SpanID = formatSpanID(spanCtx.SpanID())
		correlation.Sampled = spanCtx.IsSampled()
	}

	return append(attrs,
		slog.String("service", l.serviceName),
		slog.String("environment", l.environment),
		slog.String("component", "observability.logger"),
		slog.String("trace_id", correlation.TraceID),
		slog.String("span_id", correlation.SpanID),
		slog.String("request_id", correlation.RequestID),
		slog.String("correlation_id", correlation.CorrelationID),
	)
}

func (l *otelLogger) With(fields ...observability.Field) observability.Logger {
	newFields := make([]observability.Field, len(l.fields)+len(fields))
	copy(newFields, l.fields)
	copy(newFields[len(l.fields):], fields)

	return &otelLogger{
		bridgeLogger:  l.bridgeLogger,
		consoleLogger: l.consoleLogger,
		serviceName:   l.serviceName,
		environment:   l.environment,
		fields:        newFields,
		sanitize:      l.sanitize,
		console:       l.console,
		level:         l.level,
		format:        l.format,
	}
}

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

func sanitizeFields(fields []observability.Field) []observability.Field {
	if len(fields) > maxFields {
		fields = fields[:maxFields]
	}

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

	if !needsSanitization {
		return fields
	}

	sanitized := make([]observability.Field, len(fields))
	for i, field := range fields {
		if isSensitiveKey(field.Key) {
			sanitized[i] = observability.String(field.Key, redactedValue)
			continue
		}

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

// isSensitiveKey usa asciiContainsFold para comparação case-insensitive sem alocar por chamada.
func isSensitiveKey(key string) bool {
	for _, sensitive := range sensitiveKeysLower {
		if asciiContainsFold(key, sensitive) {
			return true
		}
	}
	return false
}
