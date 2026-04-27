package otel

import (
	"context"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func TestPropagationExtract(t *testing.T) {
	t.Parallel()

	traceID := trace.TraceID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10}
	spanID := trace.SpanID{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18}

	tests := []struct {
		name              string
		carrier           propagation.MapCarrier
		wantTraceID       string
		wantSpanID        string
		wantRequestID     string
		wantCorrelationID string
		wantBaggage       string
		wantSampled       bool
	}{
		{
			name: "w3c trace baggage and correlation headers present",
			carrier: propagation.MapCarrier{
				"traceparent":    "00-" + traceID.String() + "-" + spanID.String() + "-01",
				"baggage":        "tenant=acme",
				"x-request-id":   " req-123 ",
				"correlation-id": " corr-456 ",
			},
			wantTraceID:       traceID.String(),
			wantSpanID:        spanID.String(),
			wantRequestID:     "req-123",
			wantCorrelationID: "corr-456",
			wantBaggage:       "acme",
			wantSampled:       true,
		},
		{
			name:        "headers absent",
			carrier:     propagation.MapCarrier{},
			wantTraceID: "",
			wantSpanID:  "",
		},
	}

	runtime := newPropagationRuntime(
		propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}),
		observability.DefaultPropagationHeaders(),
	)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, correlation := runtime.Extract(context.Background(), tt.carrier)

			assert.Equal(t, tt.wantTraceID, correlation.TraceID)
			assert.Equal(t, tt.wantSpanID, correlation.SpanID)
			assert.Equal(t, tt.wantRequestID, correlation.RequestID)
			assert.Equal(t, tt.wantCorrelationID, correlation.CorrelationID)
			assert.Equal(t, tt.wantSampled, correlation.Sampled)

			stored, ok := CorrelationFromContext(ctx)
			require.True(t, ok)
			assert.Equal(t, correlation, stored)
			if tt.wantBaggage != "" {
				assert.Equal(t, tt.wantBaggage, baggage.FromContext(ctx).Member("tenant").Value())
			}
		})
	}
}

func TestPropagationInject(t *testing.T) {
	t.Parallel()

	traceID := trace.TraceID{0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27, 0x28, 0x29, 0x2a, 0x2b, 0x2c, 0x2d, 0x2e, 0x2f, 0x30}
	spanID := trace.SpanID{0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37, 0x38}

	tests := []struct {
		name              string
		ctx               context.Context
		wantTraceparent   bool
		wantBaggage       string
		wantRequestID     string
		wantCorrelationID string
	}{
		{
			name: "w3c trace baggage and correlation headers are injected",
			ctx: contextWithRemoteSpanAndBaggage(t, traceID, spanID, "tenant", "acme",
				CorrelationContext{RequestID: "req-123", CorrelationID: "corr-456"}),
			wantTraceparent:   true,
			wantBaggage:       "tenant=acme",
			wantRequestID:     "req-123",
			wantCorrelationID: "corr-456",
		},
		{
			name:            "correlation context absent only injects existing w3c context",
			ctx:             trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{TraceID: traceID, SpanID: spanID})),
			wantTraceparent: true,
		},
		{
			name: "empty context does not inject correlation headers",
			ctx:  context.Background(),
		},
	}

	runtime := newPropagationRuntime(
		propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}),
		observability.DefaultPropagationHeaders(),
	)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			carrier := propagation.MapCarrier{}
			require.NoError(t, runtime.Inject(tt.ctx, carrier))

			if tt.wantTraceparent {
				assert.NotEmpty(t, carrier.Get("traceparent"))
			} else {
				assert.Empty(t, carrier.Get("traceparent"))
			}
			assert.Equal(t, tt.wantBaggage, carrier.Get("baggage"))
			assert.Equal(t, tt.wantRequestID, carrier.Get("x-request-id"))
			assert.Equal(t, tt.wantCorrelationID, carrier.Get("correlation-id"))
		})
	}
}

func TestProviderPropagationUsesConfiguredHeaders(t *testing.T) {
	t.Parallel()

	headers, err := observability.NewPropagationHeaders("x-custom-request", "x-custom-correlation")
	require.NoError(t, err)

	provider := &Provider{runtime: &runtime{
		propagation: newPropagationRuntime(propagation.TraceContext{}, headers),
	}}
	ctx, correlation := provider.Extract(context.Background(), propagation.MapCarrier{
		"x-custom-request":     "req-custom",
		"x-custom-correlation": "corr-custom",
	})
	carrier := propagation.MapCarrier{}
	require.NoError(t, provider.Inject(ctx, carrier))

	assert.Equal(t, "req-custom", correlation.RequestID)
	assert.Equal(t, "corr-custom", correlation.CorrelationID)
	assert.Equal(t, "req-custom", carrier.Get("x-custom-request"))
	assert.Equal(t, "corr-custom", carrier.Get("x-custom-correlation"))
}

func contextWithRemoteSpanAndBaggage(
	t *testing.T,
	traceID trace.TraceID,
	spanID trace.SpanID,
	baggageKey string,
	baggageValue string,
	correlation CorrelationContext,
) context.Context {
	t.Helper()

	member, err := baggage.NewMember(baggageKey, baggageValue)
	require.NoError(t, err)
	bag, err := baggage.New(member)
	require.NoError(t, err)

	ctx := baggage.ContextWithBaggage(context.Background(), bag)
	ctx = trace.ContextWithSpanContext(ctx, trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	}))
	return ContextWithCorrelation(ctx, correlation)
}
