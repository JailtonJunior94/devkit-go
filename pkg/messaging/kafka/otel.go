package kafka

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

// Instrumentation holds OpenTelemetry instrumentation state.
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

// NewInstrumentation creates OpenTelemetry instrumentation.
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
//	// Create Kafka client with tracing
//	client, _ := kafka.NewClient(
//	    kafka.WithBrokers("localhost:9092"),
//	    kafka.WithTracingEnabled("my-service"), // Uses global providers
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
		"messaging.kafka.publish.count",
		metric.WithDescription("Total number of messages published"),
		metric.WithUnit("{message}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create publishCount metric: %w", err)
	}

	inst.publishDuration, err = meter.Float64Histogram(
		"messaging.kafka.publish.duration",
		metric.WithDescription("Duration of message publish operations"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create publishDuration metric: %w", err)
	}

	inst.publishErrors, err = meter.Int64Counter(
		"messaging.kafka.publish.errors",
		metric.WithDescription("Number of publish errors"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create publishErrors metric: %w", err)
	}

	inst.consumeCount, err = meter.Int64Counter(
		"messaging.kafka.consume.count",
		metric.WithDescription("Total number of messages consumed"),
		metric.WithUnit("{message}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumeCount metric: %w", err)
	}

	inst.consumeDuration, err = meter.Float64Histogram(
		"messaging.kafka.consume.duration",
		metric.WithDescription("Duration of message consumption"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumeDuration metric: %w", err)
	}

	inst.handlerDuration, err = meter.Float64Histogram(
		"messaging.kafka.handler.duration",
		metric.WithDescription("Duration of message handler execution"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create handlerDuration metric: %w", err)
	}

	inst.dlqPublished, err = meter.Int64Counter(
		"messaging.kafka.dlq.published",
		metric.WithDescription("Number of messages sent to DLQ"),
		metric.WithUnit("{message}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create dlqPublished metric: %w", err)
	}

	inst.retryAttempts, err = meter.Int64Counter(
		"messaging.kafka.retry.attempts",
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
//   - This enables end-to-end distributed tracing: HTTP → Kafka Publish → Kafka Consume → Handler
//
// Example:
//
//	err := p.instrumentation.InstrumentPublish(ctx, topic, key, headers, func(ctx context.Context) error {
//	    return p.publishInternal(ctx, topic, key, headers, message)
//	})
func (i *Instrumentation) InstrumentPublish(
	ctx context.Context,
	topic string,
	key string,
	headers map[string]string,
	publishFunc func(context.Context) error,
) error {
	start := time.Now()

	// Create publish span (SpanKindProducer indicates this is a message producer)
	ctx, span := i.tracer.Start(ctx, "publish "+topic,
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			semconv.MessagingSystemKafka,
			semconv.MessagingDestinationName(topic),
			attribute.String("messaging.operation.type", "publish"),
			attribute.String("messaging.kafka.message.key", key),
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
		attribute.String("messaging.system", "kafka"),
		attribute.String("messaging.destination", topic),
	)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		i.publishErrors.Add(ctx, 1, metric.WithAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.destination", topic),
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
//	err := c.instrumentation.InstrumentConsume(ctx, topic, partition, offset, key, headers, groupID, func(ctx context.Context) error {
//	    return c.processMessageInternal(ctx, msg, eventType, headers)
//	})
func (i *Instrumentation) InstrumentConsume(
	ctx context.Context,
	topic string,
	partition int,
	offset int64,
	key string,
	headers map[string]string,
	consumerGroup string,
	consumeFunc func(context.Context) error,
) error {
	start := time.Now()

	// Extract parent trace context from message headers
	// If traceparent exists, this returns a new context with the parent span context
	// If no traceparent, returns the original context (creates root span)
	parentCtx := ExtractTraceContext(ctx, headers)

	// Create consume span (child of producer span if trace context present)
	ctx, span := i.tracer.Start(parentCtx, "consume "+topic,
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			semconv.MessagingSystemKafka,
			semconv.MessagingDestinationName(topic),
			attribute.String("messaging.consumer.group.name", consumerGroup),
			attribute.String("messaging.operation.type", "process"),
			attribute.Int("messaging.kafka.source.partition", partition),
			attribute.Int64("messaging.kafka.message.offset", offset),
			attribute.String("messaging.kafka.message.key", key),
		),
	)
	defer span.End()

	// Execute consumption
	err := consumeFunc(ctx)

	// Record metrics
	duration := float64(time.Since(start).Milliseconds())
	attrs := metric.WithAttributes(
		attribute.String("messaging.system", "kafka"),
		attribute.String("messaging.destination", topic),
		attribute.String("messaging.consumer.group", consumerGroup),
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
//	err := c.instrumentation.InstrumentHandler(ctx, eventType, func(ctx context.Context) error {
//	    return handler(ctx, headers, msg.Value)
//	})
func (i *Instrumentation) InstrumentHandler(
	ctx context.Context,
	eventType string,
	handlerFunc func(context.Context) error,
) error {
	start := time.Now()

	// Create handler span (child of consume span)
	ctx, span := i.tracer.Start(ctx, "handle "+eventType,
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("event_type", eventType),
		),
	)
	defer span.End()

	// Execute handler
	err := handlerFunc(ctx)

	// Record metrics
	duration := float64(time.Since(start).Milliseconds())
	attrs := metric.WithAttributes(
		attribute.String("event_type", eventType),
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
//   - messaging.kafka.dlq.published counter
//   - Labels: original_topic, dlq_topic
func (i *Instrumentation) RecordDLQPublish(ctx context.Context, originalTopic, dlqTopic string) {
	i.dlqPublished.Add(ctx, 1, metric.WithAttributes(
		attribute.String("messaging.original_topic", originalTopic),
		attribute.String("messaging.dlq_topic", dlqTopic),
	))
}

// RecordRetryAttempt records a retry attempt.
//
// Used to track retry behavior and identify problematic topics/messages.
//
// Metrics recorded:
//   - messaging.kafka.retry.attempts counter
//   - Labels: destination topic, retry attempt number
func (i *Instrumentation) RecordRetryAttempt(ctx context.Context, topic string, attempt int) {
	i.retryAttempts.Add(ctx, 1, metric.WithAttributes(
		attribute.String("messaging.destination", topic),
		attribute.Int("retry.attempt", attempt),
	))
}

// classifyError categorizes errors for better metrics.
//
// Error Classification Strategy:
//   - timeout: Context deadline exceeded
//   - publish_failed: Message publish failed
//   - no_handler: No handler registered for event type
//   - max_retries: Exceeded maximum retry attempts
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
	case errors.Is(err, ErrPublishFailed):
		return "publish_failed"
	case errors.Is(err, ErrNoHandler):
		return "no_handler"
	case errors.Is(err, ErrMaxRetriesExceeded):
		return "max_retries"
	case errors.Is(err, ErrProducerClosed):
		return "producer_closed"
	case errors.Is(err, ErrConsumerClosed):
		return "consumer_closed"
	default:
		return "unknown"
	}
}
