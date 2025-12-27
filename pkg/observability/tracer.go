package observability

import "context"

// Tracer provides distributed tracing capabilities.
type Tracer interface {
	// Start creates a new span and returns a context containing the span.
	// The span should be ended by calling span.End() when the operation completes.
	Start(ctx context.Context, spanName string, opts ...SpanOption) (context.Context, Span)

	// SpanFromContext returns the current span from the context, if any.
	SpanFromContext(ctx context.Context) Span

	// ContextWithSpan returns a new context with the given span.
	ContextWithSpan(ctx context.Context, span Span) context.Context
}
