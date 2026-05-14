package otel

import (
	"log/slog"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Option is a function that mutates a Config before provider initialization.
type Option func(*Config)

// WithExtraLogHandler appends h to Config.ExtraLogHandlers. Each extra handler
// receives every slog.Record before the OTLP bridge handler, in registration order.
// Calling WithExtraLogHandler multiple times accumulates handlers.
func WithExtraLogHandler(h slog.Handler) Option {
	return func(c *Config) {
		c.ExtraLogHandlers = append(c.ExtraLogHandlers, h)
	}
}

// WithExtraSpanProcessor appends sp to Config.ExtraSpanProcessors. Each extra
// processor is registered after the default BatchSpanProcessor in registration
// order. Calling WithExtraSpanProcessor multiple times accumulates processors.
func WithExtraSpanProcessor(sp sdktrace.SpanProcessor) Option {
	return func(c *Config) {
		c.ExtraSpanProcessors = append(c.ExtraSpanProcessors, sp)
	}
}
