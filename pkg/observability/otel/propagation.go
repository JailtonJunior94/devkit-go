package otel

import (
	"context"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// TextMapCarrier é o contrato de carrier para propagação W3C.
type TextMapCarrier = propagation.TextMapCarrier

// CorrelationContext carrega IDs de trace e operação propagados entre fronteiras.
type CorrelationContext struct {
	TraceID       string
	SpanID        string
	RequestID     string
	CorrelationID string
	Sampled       bool
}

type correlationContextKey struct{}

type propagationRuntime struct {
	propagator propagation.TextMapPropagator
	headers    observability.PropagationHeaders
}

func newPropagationRuntime(
	propagator propagation.TextMapPropagator,
	headers observability.PropagationHeaders,
) *propagationRuntime {
	return &propagationRuntime{
		propagator: propagator,
		headers:    headers,
	}
}

func configuredPropagationHeaders(config *Config) observability.PropagationHeaders {
	if config == nil {
		return observability.DefaultPropagationHeaders()
	}
	headers := config.PropagationHeaders
	if headers.RequestIDHeader() == "" && headers.CorrelationIDHeader() == "" {
		return observability.DefaultPropagationHeaders()
	}
	return headers
}

func ContextWithCorrelation(ctx context.Context, correlation CorrelationContext) context.Context {
	return context.WithValue(ctx, correlationContextKey{}, correlation)
}

func CorrelationFromContext(ctx context.Context) (CorrelationContext, bool) {
	correlation, ok := ctx.Value(correlationContextKey{}).(CorrelationContext)
	return correlation, ok
}

func (p *propagationRuntime) Extract(ctx context.Context, carrier TextMapCarrier) (context.Context, CorrelationContext) {
	ctx = p.propagator.Extract(ctx, carrier)
	correlation := CorrelationContext{
		RequestID:     normalizeHeaderValue(carrier.Get(p.headers.RequestIDHeader())),
		CorrelationID: normalizeHeaderValue(carrier.Get(p.headers.CorrelationIDHeader())),
	}
	correlation = correlation.withSpanContext(trace.SpanContextFromContext(ctx))
	ctx = ContextWithCorrelation(ctx, correlation)
	return ctx, correlation
}

func (p *propagationRuntime) Inject(ctx context.Context, carrier TextMapCarrier) error {
	p.propagator.Inject(ctx, carrier)

	correlation, _ := CorrelationFromContext(ctx)
	correlation = correlation.withSpanContext(trace.SpanContextFromContext(ctx))
	if correlation.RequestID != "" {
		carrier.Set(p.headers.RequestIDHeader(), correlation.RequestID)
	}
	if correlation.CorrelationID != "" {
		carrier.Set(p.headers.CorrelationIDHeader(), correlation.CorrelationID)
	}
	return nil
}

func (c CorrelationContext) withSpanContext(spanCtx trace.SpanContext) CorrelationContext {
	if !spanCtx.IsValid() {
		return c
	}
	c.TraceID = spanCtx.TraceID().String()
	c.SpanID = spanCtx.SpanID().String()
	c.Sampled = spanCtx.IsSampled()
	return c
}

func normalizeHeaderValue(value string) string {
	return strings.TrimSpace(value)
}

// Extract propaga W3C trace context, baggage e headers de correlação configurados.
func (p *Provider) Extract(ctx context.Context, carrier TextMapCarrier) (context.Context, CorrelationContext) {
	if p == nil || p.runtime == nil || p.runtime.propagation == nil {
		correlation := CorrelationContext{}
		return ContextWithCorrelation(ctx, correlation), correlation
	}
	return p.runtime.propagation.Extract(ctx, carrier)
}

// Inject escreve W3C trace context, baggage e headers de correlação no carrier.
func (p *Provider) Inject(ctx context.Context, carrier TextMapCarrier) error {
	if p == nil || p.runtime == nil || p.runtime.propagation == nil {
		return nil
	}
	return p.runtime.propagation.Inject(ctx, carrier)
}
