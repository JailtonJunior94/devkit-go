package kafka

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// InjectTraceContext injects W3C trace context into Kafka message headers.
//
// How it works:
//   1. Uses the global TextMapPropagator (configured via otel.SetTextMapPropagator)
//   2. Injects traceparent and tracestate headers into the map
//   3. Modifies the headers map in-place
//
// W3C Trace Context Format:
//   - traceparent: 00-{trace-id}-{span-id}-{trace-flags}
//   - tracestate: vendor-specific trace state (optional)
//
// Example:
//   Before: headers = {"event_type": "user.created"}
//   After:  headers = {
//     "event_type": "user.created",
//     "traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
//     "tracestate": "rojo=00f067aa0ba902b7"
//   }
//
// Usage:
//
//	headers := map[string]string{"event_type": "order.created"}
//	InjectTraceContext(ctx, headers)
//	// headers now contains traceparent and tracestate for propagation
func InjectTraceContext(ctx context.Context, headers map[string]string) {
	propagator := otel.GetTextMapPropagator()

	// MapCarrier adapts map[string]string to TextMapCarrier interface
	carrier := propagation.MapCarrier(headers)

	// Inject trace context (adds traceparent, tracestate keys)
	propagator.Inject(ctx, carrier)
}

// ExtractTraceContext extracts W3C trace context from Kafka message headers.
//
// How it works:
//   1. Uses the global TextMapPropagator (configured via otel.SetTextMapPropagator)
//   2. Reads traceparent and tracestate headers from the map
//   3. Returns a new context with the extracted span context
//
// Trace Correlation:
//   - If traceparent exists: Returns context with parent span context
//     → Consumer span becomes CHILD of producer span (same trace_id)
//   - If no traceparent: Returns original context
//     → Consumer span becomes ROOT span (new trace_id)
//
// Example Trace Flow:
//
//	Producer (trace_id: abc123, span_id: def456)
//	    │ Publishes message with traceparent header
//	    ▼
//	Consumer extracts traceparent
//	    │ Creates child span (trace_id: abc123, span_id: ghi789)
//	    └─ Both spans appear in the same trace in Jaeger/Tempo
//
// Usage:
//
//	headers := extractHeaders(kafkaMsg) // {"traceparent": "00-abc123-def456-01", ...}
//	ctx = ExtractTraceContext(ctx, headers)
//	// ctx now contains parent span context
//	// Next span created from this context will be a child of the producer span
func ExtractTraceContext(ctx context.Context, headers map[string]string) context.Context {
	propagator := otel.GetTextMapPropagator()

	// MapCarrier adapts map[string]string to TextMapCarrier interface
	carrier := propagation.MapCarrier(headers)

	// Extract trace context (reads traceparent, tracestate keys)
	return propagator.Extract(ctx, carrier)
}
