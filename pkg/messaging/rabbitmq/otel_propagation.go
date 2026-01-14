package rabbitmq

import (
	"context"

	"go.opentelemetry.io/otel"
)

// InjectTraceContext injects W3C trace context into RabbitMQ message headers.
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
//   Before: headers = map[string]interface{}{"content_type": "application/json"}
//   After:  headers = map[string]interface{}{
//     "content_type": "application/json",
//     "traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
//     "tracestate": "rojo=00f067aa0ba902b7",
//   }
//
// Usage:
//
//	headers := map[string]interface{}{"content_type": "application/json"}
//	InjectTraceContext(ctx, headers)
//	// headers now contains traceparent and tracestate for propagation
func InjectTraceContext(ctx context.Context, headers map[string]interface{}) {
	propagator := otel.GetTextMapPropagator()

	// Create an adapter that implements TextMapCarrier for AMQP headers
	carrier := &amqpHeaderCarrier{headers: headers}

	// Inject trace context (adds traceparent, tracestate keys)
	propagator.Inject(ctx, carrier)
}

// ExtractTraceContext extracts W3C trace context from RabbitMQ message headers.
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
//	headers := extractHeaders(amqpDelivery) // map[string]interface{}{"traceparent": "00-abc123-def456-01", ...}
//	ctx = ExtractTraceContext(ctx, headers)
//	// ctx now contains parent span context
//	// Next span created from this context will be a child of the producer span
func ExtractTraceContext(ctx context.Context, headers map[string]interface{}) context.Context {
	propagator := otel.GetTextMapPropagator()

	// Create an adapter that implements TextMapCarrier for AMQP headers
	carrier := &amqpHeaderCarrier{headers: headers}

	// Extract trace context (reads traceparent, tracestate keys)
	return propagator.Extract(ctx, carrier)
}

// amqpHeaderCarrier adapts AMQP headers (map[string]interface{}) to OpenTelemetry TextMapCarrier.
//
// RabbitMQ headers are amqp.Table which is map[string]interface{}, not map[string]string.
// This adapter converts between the two formats for OpenTelemetry propagation.
// Implements the TextMapCarrier interface required by OpenTelemetry propagators.
type amqpHeaderCarrier struct {
	headers map[string]interface{}
}

// Get retrieves a header value as a string.
// Implements the Get method of TextMapCarrier interface.
func (c *amqpHeaderCarrier) Get(key string) string {
	if c.headers == nil {
		return ""
	}

	val, ok := c.headers[key]
	if !ok {
		return ""
	}

	// Convert interface{} to string
	strVal, ok := val.(string)
	if !ok {
		return ""
	}

	return strVal
}

// Set stores a header value.
// Implements the Set method of TextMapCarrier interface.
func (c *amqpHeaderCarrier) Set(key, value string) {
	if c.headers == nil {
		c.headers = make(map[string]interface{})
	}
	c.headers[key] = value
}

// Keys returns all header keys.
// Implements the Keys method of TextMapCarrier interface.
func (c *amqpHeaderCarrier) Keys() []string {
	if c.headers == nil {
		return nil
	}

	keys := make([]string, 0, len(c.headers))
	for k := range c.headers {
		keys = append(keys, k)
	}
	return keys
}
