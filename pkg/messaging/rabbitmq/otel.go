package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Instrumentation holds OpenTelemetry instrumentation state for RabbitMQ.
//
// Architecture:
//   - Uses global TracerProvider and MeterProvider (configured via otel.SetTracerProvider/SetMeterProvider)
//   - Instruments are created once and reused for all operations (optimal performance)
//   - Thread-safe: safe for concurrent use
//
// Performance:
//   - Span creation overhead: ~1-5 microseconds
//   - Metric recording overhead: ~100-500 nanoseconds
//   - Total overhead per message: <10 microseconds (negligible vs 1-50ms network latency)
type Instrumentation struct {
	tracer trace.Tracer
	meter  metric.Meter

	// Metrics instruments (created once, reused)
	publishCount    metric.Int64Counter
	publishDuration metric.Float64Histogram
	publishErrors   metric.Int64Counter
	consumeCount    metric.Int64Counter
	consumeDuration metric.Float64Histogram
	handlerDuration metric.Float64Histogram
	dlqPublished    metric.Int64Counter
	retryAttempts   metric.Int64Counter
}

// NewInstrumentation creates OpenTelemetry instrumentation for RabbitMQ.
//
// Prerequisites:
//   - OpenTelemetry TracerProvider must be configured globally (via otel.SetTracerProvider)
//   - OpenTelemetry MeterProvider must be configured globally (via otel.SetMeterProvider)
//
// Usage:
//
//	// In main.go - configure OpenTelemetry first
//	obs, _ := otel.NewProvider(ctx, otelConfig)
//	defer obs.Shutdown(ctx)
//
//	// Create RabbitMQ client with tracing
//	client, _ := rabbitmq.New(
//	    obs,
//	    rabbitmq.WithCloudConnection("amqps://..."),
//	    rabbitmq.WithTracingEnabled("my-service"), // Uses global providers
//	)
func NewInstrumentation(serviceName string) (*Instrumentation, error) {
	if serviceName == "" {
		return nil, fmt.Errorf("service name cannot be empty")
	}

	tracer := otel.GetTracerProvider().Tracer(serviceName)
	meter := otel.GetMeterProvider().Meter(serviceName)

	inst := &Instrumentation{
		tracer: tracer,
		meter:  meter,
	}

	var err error

	// Create metric instruments (following OpenTelemetry Semantic Conventions)
	inst.publishCount, err = meter.Int64Counter(
		"messaging.rabbitmq.publish.count",
		metric.WithDescription("Total number of messages published"),
		metric.WithUnit("{message}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create publishCount metric: %w", err)
	}

	inst.publishDuration, err = meter.Float64Histogram(
		"messaging.rabbitmq.publish.duration",
		metric.WithDescription("Duration of message publish operations"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create publishDuration metric: %w", err)
	}

	inst.publishErrors, err = meter.Int64Counter(
		"messaging.rabbitmq.publish.errors",
		metric.WithDescription("Number of publish errors"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create publishErrors metric: %w", err)
	}

	inst.consumeCount, err = meter.Int64Counter(
		"messaging.rabbitmq.consume.count",
		metric.WithDescription("Total number of messages consumed"),
		metric.WithUnit("{message}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumeCount metric: %w", err)
	}

	inst.consumeDuration, err = meter.Float64Histogram(
		"messaging.rabbitmq.consume.duration",
		metric.WithDescription("Duration of message consumption"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumeDuration metric: %w", err)
	}

	inst.handlerDuration, err = meter.Float64Histogram(
		"messaging.rabbitmq.handler.duration",
		metric.WithDescription("Duration of message handler execution"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create handlerDuration metric: %w", err)
	}

	inst.dlqPublished, err = meter.Int64Counter(
		"messaging.rabbitmq.dlq.published",
		metric.WithDescription("Number of messages sent to DLQ"),
		metric.WithUnit("{message}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create dlqPublished metric: %w", err)
	}

	inst.retryAttempts, err = meter.Int64Counter(
		"messaging.rabbitmq.retry.attempts",
		metric.WithDescription("Number of retry attempts"),
		metric.WithUnit("{attempt}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create retryAttempts metric: %w", err)
	}

	return inst, nil
}

// InstrumentPublish wraps a publish operation with tracing and metrics.
//
// Behavior:
//   1. Creates a producer span with OpenTelemetry Semantic Conventions
//   2. Injects W3C trace context into message headers (traceparent, tracestate)
//   3. Executes the publish function
//   4. Records metrics (duration, count, errors)
//   5. Ends the span with appropriate status
//
// Trace Context Propagation:
//   - The injected traceparent header allows consumers to create child spans
//   - This enables end-to-end distributed tracing: HTTP → RabbitMQ Publish → RabbitMQ Consume → Handler
//
// Example:
//
//	err := p.instrumentation.InstrumentPublish(ctx, exchange, routingKey, headers, func(ctx context.Context) error {
//	    return p.publishInternal(ctx, exchange, routingKey, body, opts...)
//	})
func (i *Instrumentation) InstrumentPublish(
	ctx context.Context,
	exchange string,
	routingKey string,
	headers map[string]interface{},
	publishFunc func(context.Context) error,
) error {
	start := time.Now()

	// Create publish span (SpanKindProducer indicates this is a message producer)
	ctx, span := i.tracer.Start(ctx, "publish "+routingKey,
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			semconv.MessagingSystemRabbitmq,
			semconv.MessagingDestinationName(routingKey),
			attribute.String("messaging.operation.type", "publish"),
			attribute.String("messaging.rabbitmq.exchange", exchange),
			attribute.String("messaging.rabbitmq.routing_key", routingKey),
		),
	)
	defer span.End()

	// Inject trace context into headers (mutates headers map)
	// This adds "traceparent" and "tracestate" keys following W3C Trace Context spec
	InjectTraceContext(ctx, headers)

	// Execute publish
	err := publishFunc(ctx)

	// Record metrics
	duration := float64(time.Since(start).Milliseconds())
	attrs := metric.WithAttributes(
		attribute.String("messaging.system", "rabbitmq"),
		attribute.String("messaging.destination", routingKey),
		attribute.String("messaging.rabbitmq.exchange", exchange),
	)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		i.publishErrors.Add(ctx, 1, metric.WithAttributes(
			attribute.String("messaging.system", "rabbitmq"),
			attribute.String("messaging.destination", routingKey),
			attribute.String("messaging.rabbitmq.exchange", exchange),
			attribute.String("error.type", classifyError(err)),
		))
	} else {
		span.SetStatus(codes.Ok, "published")
		i.publishCount.Add(ctx, 1, attrs)
	}

	i.publishDuration.Record(ctx, duration, attrs)

	return err
}

// InstrumentConsume wraps a consume operation with tracing and metrics.
//
// Behavior:
//   1. Extracts parent trace context from message headers (if present)
//   2. Creates a consumer span as CHILD of the producer span (enables correlation)
//   3. Executes the consume function
//   4. Records metrics (duration, count)
//   5. Ends the span with appropriate status
//
// Trace Hierarchy:
//
//	Producer Span (traceparent: 00-abc123-def456-01)
//	    │
//	    └─ Consumer Span (traceparent: 00-abc123-ghi789-01) ← SAME trace_id
//	        │
//	        └─ Handler Span ← SAME trace_id
//
// All spans share the same trace_id (abc123), enabling end-to-end correlation in Jaeger/Tempo.
//
// Example:
//
//	err := c.instrumentation.InstrumentConsume(ctx, exchange, routingKey, queue, headers, func(ctx context.Context) error {
//	    return c.processMessageInternal(ctx, msg)
//	})
func (i *Instrumentation) InstrumentConsume(
	ctx context.Context,
	exchange string,
	routingKey string,
	queue string,
	headers map[string]interface{},
	consumeFunc func(context.Context) error,
) error {
	start := time.Now()

	// Extract parent trace context from message headers
	// If traceparent exists, this returns a new context with the parent span context
	// If no traceparent, returns the original context (creates root span)
	parentCtx := ExtractTraceContext(ctx, headers)

	// Create consume span (child of producer span if trace context present)
	ctx, span := i.tracer.Start(parentCtx, "consume "+routingKey,
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			semconv.MessagingSystemRabbitmq,
			semconv.MessagingDestinationName(queue),
			attribute.String("messaging.operation.type", "process"),
			attribute.String("messaging.rabbitmq.exchange", exchange),
			attribute.String("messaging.rabbitmq.routing_key", routingKey),
			attribute.String("messaging.rabbitmq.queue", queue),
		),
	)
	defer span.End()

	// Execute consumption
	err := consumeFunc(ctx)

	// Record metrics
	duration := float64(time.Since(start).Milliseconds())
	attrs := metric.WithAttributes(
		attribute.String("messaging.system", "rabbitmq"),
		attribute.String("messaging.destination", queue),
		attribute.String("messaging.rabbitmq.exchange", exchange),
		attribute.String("messaging.rabbitmq.routing_key", routingKey),
	)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "processed")
		i.consumeCount.Add(ctx, 1, attrs)
	}

	i.consumeDuration.Record(ctx, duration, attrs)

	return err
}

// InstrumentHandler wraps a message handler with tracing and metrics.
//
// Creates a child span under the consume span, enabling detailed observability
// of individual handler execution time.
//
// Span Hierarchy:
//
//	Consume Span
//	    └─ Handler Span (this span)
//	        └─ Repository/Database spans (if instrumented)
//
// Example:
//
//	err := c.instrumentation.InstrumentHandler(ctx, routingKey, func(ctx context.Context) error {
//	    return handler(ctx, msg)
//	})
func (i *Instrumentation) InstrumentHandler(
	ctx context.Context,
	routingKey string,
	handlerFunc func(context.Context) error,
) error {
	start := time.Now()

	// Create handler span (child of consume span)
	ctx, span := i.tracer.Start(ctx, "handle "+routingKey,
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("routing_key", routingKey),
		),
	)
	defer span.End()

	// Execute handler
	err := handlerFunc(ctx)

	// Record metrics
	duration := float64(time.Since(start).Milliseconds())
	attrs := metric.WithAttributes(
		attribute.String("routing_key", routingKey),
	)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "handled")
	}

	i.handlerDuration.Record(ctx, duration, attrs)

	return err
}

// RecordDLQPublish records a DLQ publish event.
//
// Used when a message is sent to the Dead Letter Queue after exceeding retry limits.
//
// Metrics recorded:
//   - messaging.rabbitmq.dlq.published counter
//   - Labels: original_queue, dlq_queue
func (i *Instrumentation) RecordDLQPublish(ctx context.Context, originalQueue, dlqQueue string) {
	i.dlqPublished.Add(ctx, 1, metric.WithAttributes(
		attribute.String("messaging.original_queue", originalQueue),
		attribute.String("messaging.dlq_queue", dlqQueue),
	))
}

// RecordRetryAttempt records a retry attempt.
//
// Used to track retry behavior and identify problematic queues/messages.
//
// Metrics recorded:
//   - messaging.rabbitmq.retry.attempts counter
//   - Labels: destination queue, retry attempt number
func (i *Instrumentation) RecordRetryAttempt(ctx context.Context, queue string, attempt int) {
	i.retryAttempts.Add(ctx, 1, metric.WithAttributes(
		attribute.String("messaging.destination", queue),
		attribute.Int("retry.attempt", attempt),
	))
}

// classifyError categorizes errors for better metrics.
//
// Error Classification Strategy:
//   - timeout: Context deadline exceeded
//   - canceled: Context canceled
//   - client_closed: Client/connection closed
//   - publish_failed: Message publish failed
//   - unknown: All other errors
//
// This enables filtering and alerting on specific error classes.
func classifyError(err error) string {
	if err == nil {
		return "none"
	}

	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, ErrClientClosed):
		return "client_closed"
	case errors.Is(err, ErrConnectionClosed):
		return "connection_closed"
	default:
		return "unknown"
	}
}
