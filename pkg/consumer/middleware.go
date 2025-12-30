package consumer

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Middleware is a function that wraps a MessageHandlerFunc to add
// additional processing before or after the handler executes.
// Middleware can be used for logging, metrics, tracing, retries, etc.
type Middleware func(MessageHandlerFunc) MessageHandlerFunc

// LoggingMiddleware logs message processing with timing information.
func LoggingMiddleware(logger interface {
	Info(ctx context.Context, msg string, keysAndValues ...interface{})
	Error(ctx context.Context, msg string, keysAndValues ...interface{})
}) Middleware {
	return func(next MessageHandlerFunc) MessageHandlerFunc {
		return func(ctx context.Context, msg *Message) error {
			start := time.Now()

			logger.Info(ctx, "processing message",
				"topic", msg.Topic,
				"partition", msg.Partition,
				"offset", msg.Offset,
				"attempt", msg.Attempt)

			err := next(ctx, msg)
			duration := time.Since(start)

			if err != nil {
				logger.Error(ctx, "message processing failed",
					"topic", msg.Topic,
					"partition", msg.Partition,
					"offset", msg.Offset,
					"duration", duration.String(),
					"error", err.Error())
			} else {
				logger.Info(ctx, "message processed successfully",
					"topic", msg.Topic,
					"partition", msg.Partition,
					"offset", msg.Offset,
					"duration", duration.String())
			}

			return err
		}
	}
}

// RecoveryMiddleware recovers from panics in message handlers and converts
// them to errors. This prevents a single panic from crashing the entire consumer.
func RecoveryMiddleware(logger interface {
	Error(ctx context.Context, msg string, keysAndValues ...interface{})
}) Middleware {
	return func(next MessageHandlerFunc) MessageHandlerFunc {
		return func(ctx context.Context, msg *Message) (err error) {
			defer func() {
				if r := recover(); r != nil {
					logger.Error(ctx, "panic recovered in message handler",
						"topic", msg.Topic,
						"partition", msg.Partition,
						"offset", msg.Offset,
						"panic", r)

					err = &HandlerError{
						Handler: "unknown",
						Topic:   msg.Topic,
						Message: "panic recovered",
						Err:     fmt.Errorf("panic: %v", r),
						Retry:   false,
					}
				}
			}()

			return next(ctx, msg)
		}
	}
}

// MetricsMiddleware records metrics for message processing.
// This is a simplified example - in production, you would integrate
// with your observability system (Prometheus, OpenTelemetry, etc.).
func MetricsMiddleware(recorder interface {
	RecordDuration(ctx context.Context, name string, duration time.Duration, tags ...string)
	IncrementCounter(ctx context.Context, name string, tags ...string)
}) Middleware {
	return func(next MessageHandlerFunc) MessageHandlerFunc {
		return func(ctx context.Context, msg *Message) error {
			start := time.Now()

			err := next(ctx, msg)
			duration := time.Since(start)

			// Record processing duration
			tags := []string{
				"topic:" + msg.Topic,
				fmt.Sprintf("partition:%d", msg.Partition),
			}

			recorder.RecordDuration(ctx, "message.processing.duration", duration, tags...)

			// Increment counters
			if err != nil {
				recorder.IncrementCounter(ctx, "message.processing.errors", tags...)
			} else {
				recorder.IncrementCounter(ctx, "message.processing.success", tags...)
			}

			return err
		}
	}
}

// TracingMiddleware adds distributed tracing context to message processing.
// This is a simplified example - in production, you would integrate
// with OpenTelemetry or similar.
func TracingMiddleware(tracer interface {
	StartSpan(ctx context.Context, name string, attrs ...interface{}) (context.Context, interface{ End() })
}) Middleware {
	return func(next MessageHandlerFunc) MessageHandlerFunc {
		return func(ctx context.Context, msg *Message) error {
			// Start span
			spanCtx, span := tracer.StartSpan(ctx, "consumer.process_message",
				"topic", msg.Topic,
				"partition", msg.Partition,
				"offset", msg.Offset)
			defer span.End()

			// Process message with tracing context
			return next(spanCtx, msg)
		}
	}
}

// TimeoutMiddleware enforces a timeout for message processing.
// This ensures that slow handlers don't block the consumer indefinitely.
func TimeoutMiddleware(timeout time.Duration) Middleware {
	return func(next MessageHandlerFunc) MessageHandlerFunc {
		return func(ctx context.Context, msg *Message) error {
			timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			// Process message with timeout context
			errChan := make(chan error, 1)
			go func() {
				errChan <- next(timeoutCtx, msg)
			}()

			select {
			case err := <-errChan:
				return err
			case <-timeoutCtx.Done():
				return &ProcessingError{
					Topic:     msg.Topic,
					Partition: msg.Partition,
					Offset:    msg.Offset,
					Attempt:   msg.Attempt,
					Err:       fmt.Errorf("processing timeout exceeded: %s", timeout),
				}
			}
		}
	}
}

// RetryMiddleware adds automatic retry logic with exponential backoff.
// This should typically be the outermost middleware in the chain.
func RetryMiddleware(maxRetries int, backoff time.Duration) Middleware {
	return func(next MessageHandlerFunc) MessageHandlerFunc {
		return func(ctx context.Context, msg *Message) error {
			var lastErr error

			for attempt := 0; attempt <= maxRetries; attempt++ {
				// Update attempt count
				msg.Attempt = attempt

				// Try processing
				err := next(ctx, msg)
				if err == nil {
					return nil
				}

				lastErr = err

				// Check if we should retry
				if attempt < maxRetries {
					// Calculate backoff
					waitTime := backoff * time.Duration(1<<uint(attempt))

					// Wait before retry
					timer := time.NewTimer(waitTime)
					select {
					case <-timer.C:
						// Continue to next retry
					case <-ctx.Done():
						timer.Stop()
						return ctx.Err()
					}
				}
			}

			return &ProcessingError{
				Topic:      msg.Topic,
				Partition:  msg.Partition,
				Offset:     msg.Offset,
				Attempt:    maxRetries,
				MaxRetries: maxRetries,
				Err:        lastErr,
			}
		}
	}
}

// DeduplicationMiddleware prevents duplicate message processing based on
// message key or a custom deduplication strategy.
type DeduplicationMiddleware struct {
	cache   map[string]time.Time
	cacheMu sync.RWMutex
	ttl     time.Duration
}

// NewDeduplicationMiddleware creates a new deduplication middleware.
func NewDeduplicationMiddleware(ttl time.Duration) *DeduplicationMiddleware {
	return &DeduplicationMiddleware{
		cache: make(map[string]time.Time),
		ttl:   ttl,
	}
}

// Middleware returns the middleware function.
func (d *DeduplicationMiddleware) Middleware() Middleware {
	return func(next MessageHandlerFunc) MessageHandlerFunc {
		return func(ctx context.Context, msg *Message) error {
			// Create deduplication key from topic + partition + offset
			dedupKey := fmt.Sprintf("%s-%d-%d", msg.Topic, msg.Partition, msg.Offset)

			// Check if already processed
			d.cacheMu.RLock()
			processedAt, exists := d.cache[dedupKey]
			d.cacheMu.RUnlock()

			if exists && time.Since(processedAt) < d.ttl {
				// Already processed recently, skip
				return nil
			}

			// Process message
			err := next(ctx, msg)
			if err != nil {
				return err
			}

			// Mark as processed
			d.cacheMu.Lock()
			d.cache[dedupKey] = time.Now()
			d.cacheMu.Unlock()

			// Cleanup old entries periodically
			go d.cleanup()

			return nil
		}
	}
}

// cleanup removes expired entries from the cache.
func (d *DeduplicationMiddleware) cleanup() {
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()

	now := time.Now()
	for key, processedAt := range d.cache {
		if now.Sub(processedAt) > d.ttl {
			delete(d.cache, key)
		}
	}
}
