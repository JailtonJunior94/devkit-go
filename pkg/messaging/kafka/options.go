package kafka

import (
	"context"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging/kafka/auth"
)

// Option is a functional option for configuring the Kafka client.
type Option func(*config)

// WithBrokers sets the Kafka broker addresses.
func WithBrokers(brokers ...string) Option {
	return func(c *config) {
		if len(brokers) > 0 {
			c.brokers = brokers
		}
	}
}

// WithAuthPlain configures PLAIN SASL authentication.
func WithAuthPlain(username, password string) Option {
	return func(c *config) {
		c.authStrategy = auth.NewStrategy(auth.StrategyPlain)
		c.authConfig = &auth.Config{
			Username: username,
			Password: password,
		}
	}
}

// WithAuthScram configures SCRAM SASL authentication.
// algorithm should be either ScramSHA256 or ScramSHA512.
func WithAuthScram(username, password string, algorithm auth.ScramAlgorithm) Option {
	return func(c *config) {
		c.authStrategy = auth.NewStrategy(auth.StrategyScram)
		c.authConfig = &auth.Config{
			Username:  username,
			Password:  password,
			Algorithm: algorithm,
		}
	}
}

// WithAuthConfluent configures authentication for Confluent Cloud.
// This is the RECOMMENDED authentication method for production environments.
//
// Parâmetros:
//   - apiKey: Your Confluent Cloud API Key
//   - apiSecret: Your Confluent Cloud API Secret
//
// Por quê: Confluent Cloud exige SASL_SSL com credenciais API Key/Secret.
// Esta função configura automaticamente SCRAM-SHA-512 + TLS, garantindo
// máxima segurança sem configuração manual.
//
// Exemplo:
//
//	client, err := kafka.NewClient(
//	    kafka.WithBrokers("pkc-xxxxx.us-east-1.aws.confluent.cloud:9092"),
//	    kafka.WithAuthConfluent("YOUR_API_KEY", "YOUR_API_SECRET"),
//	)
func WithAuthConfluent(apiKey, apiSecret string) Option {
	return func(c *config) {
		c.authStrategy = auth.NewStrategy(auth.StrategyConfluent)
		c.authConfig = &auth.Config{
			Username:  apiKey,
			Password:  apiSecret,
			Algorithm: auth.ScramSHA512, // Default para Confluent
		}
	}
}

// WithAuthPlaintext configures connection without authentication.
// WARNING: Only use in development environments.
func WithAuthPlaintext() Option {
	return func(c *config) {
		c.authStrategy = auth.NewStrategy(auth.StrategyPlaintext)
		c.authConfig = &auth.Config{}
	}
}

// WithAuthConfig allows custom authentication configuration.
func WithAuthConfig(strategy auth.Strategy, authConfig *auth.Config) Option {
	return func(c *config) {
		if strategy != nil {
			c.authStrategy = strategy
		}
		if authConfig != nil {
			c.authConfig = authConfig
		}
	}
}

// WithInsecureSkipVerify disables TLS certificate verification.
// WARNING: Only use in development environments. Never use in production.
func WithInsecureSkipVerify(skip bool) Option {
	return func(c *config) {
		c.authConfig.InsecureSkipVerify = skip
	}
}

// WithLogger sets the logger for structured logging.
func WithLogger(logger Logger) Option {
	return func(c *config) {
		if logger != nil {
			c.logger = logger
		}
	}
}

// WithDialTimeout sets the timeout for establishing connections.
func WithDialTimeout(timeout time.Duration) Option {
	return func(c *config) {
		if timeout > 0 {
			c.dialTimeout = timeout
		}
	}
}

// WithConnectTimeout sets the timeout for initial connection.
func WithConnectTimeout(timeout time.Duration) Option {
	return func(c *config) {
		if timeout > 0 {
			c.connectTimeout = timeout
		}
	}
}

// WithMaxRetries sets the maximum number of retry attempts.
func WithMaxRetries(maxRetries int) Option {
	return func(c *config) {
		if maxRetries >= 0 {
			c.maxRetries = maxRetries
		}
	}
}

// WithRetryBackoff sets the base backoff duration between retries.
func WithRetryBackoff(backoff time.Duration) Option {
	return func(c *config) {
		if backoff > 0 {
			c.retryBackoff = backoff
		}
	}
}

// WithMaxRetryBackoff sets the maximum backoff duration between retries.
func WithMaxRetryBackoff(maxBackoff time.Duration) Option {
	return func(c *config) {
		if maxBackoff > 0 {
			c.maxRetryBackoff = maxBackoff
		}
	}
}

// WithHealthCheckTimeout sets the timeout for health checks.
func WithHealthCheckTimeout(timeout time.Duration) Option {
	return func(c *config) {
		if timeout > 0 {
			c.healthCheckTimeout = timeout
		}
	}
}

// WithHealthCheckEnabled enables or disables health checks.
func WithHealthCheckEnabled(enabled bool) Option {
	return func(c *config) {
		c.healthCheckEnabled = enabled
	}
}

// WithReconnectEnabled enables or disables automatic reconnection.
func WithReconnectEnabled(enabled bool) Option {
	return func(c *config) {
		c.reconnectEnabled = enabled
	}
}

// WithReconnectInterval sets the interval between reconnection attempts.
func WithReconnectInterval(interval time.Duration) Option {
	return func(c *config) {
		if interval > 0 {
			c.reconnectInterval = interval
		}
	}
}

// WithMaxReconnectDelay sets the maximum delay between reconnection attempts.
func WithMaxReconnectDelay(delay time.Duration) Option {
	return func(c *config) {
		if delay > 0 {
			c.maxReconnectDelay = delay
		}
	}
}

// WithProducerBatchSize sets the maximum number of messages in a producer batch.
func WithProducerBatchSize(size int) Option {
	return func(c *config) {
		if size > 0 {
			c.producerBatchSize = size
		}
	}
}

// WithProducerBatchTimeout sets the maximum time to wait before sending a batch.
func WithProducerBatchTimeout(timeout time.Duration) Option {
	return func(c *config) {
		if timeout > 0 {
			c.producerBatchTimeout = timeout
		}
	}
}

// WithProducerMaxAttempts sets the maximum attempts for producing messages.
func WithProducerMaxAttempts(attempts int) Option {
	return func(c *config) {
		if attempts > 0 {
			c.producerMaxAttempts = attempts
		}
	}
}

// WithProducerAsync enables asynchronous message production.
func WithProducerAsync(async bool) Option {
	return func(c *config) {
		c.producerAsync = async
	}
}

// WithProducerCompression sets the compression codec for messages.
// 0=none, 1=gzip, 2=snappy, 3=lz4, 4=zstd.
func WithProducerCompression(codec int) Option {
	return func(c *config) {
		if codec >= 0 && codec <= 4 {
			c.producerCompression = codec
		}
	}
}

// WithProducerRequiredAcks sets the required number of acknowledgments.
// -1=all, 0=none, 1=leader.
func WithProducerRequiredAcks(acks int) Option {
	return func(c *config) {
		c.producerRequiredAcks = acks
	}
}

// WithConsumerGroupID sets the consumer group ID.
func WithConsumerGroupID(groupID string) Option {
	return func(c *config) {
		if groupID != "" {
			c.consumerGroupID = groupID
		}
	}
}

// WithConsumerTopics sets the topics for the consumer.
func WithConsumerTopics(topics ...string) Option {
	return func(c *config) {
		if len(topics) > 0 {
			c.consumerTopics = topics
		}
	}
}

// WithConsumerMinBytes sets the minimum bytes to fetch in a consumer request.
func WithConsumerMinBytes(bytes int) Option {
	return func(c *config) {
		if bytes > 0 {
			c.consumerMinBytes = bytes
		}
	}
}

// WithConsumerMaxBytes sets the maximum bytes to fetch in a consumer request.
func WithConsumerMaxBytes(bytes int) Option {
	return func(c *config) {
		if bytes > 0 {
			c.consumerMaxBytes = bytes
		}
	}
}

// WithConsumerCommitInterval sets the interval for committing offsets.
func WithConsumerCommitInterval(interval time.Duration) Option {
	return func(c *config) {
		if interval >= 0 {
			c.consumerCommitInterval = interval
		}
	}
}

// WithConsumerStartOffset sets the starting offset for the consumer.
// -1=newest, -2=oldest.
func WithConsumerStartOffset(offset int64) Option {
	return func(c *config) {
		c.consumerStartOffset = offset
	}
}

// WithConsumerMaxWait sets the maximum time to wait for messages.
func WithConsumerMaxWait(wait time.Duration) Option {
	return func(c *config) {
		if wait > 0 {
			c.consumerMaxWait = wait
		}
	}
}

// WithTracingEnabled enables OpenTelemetry tracing and metrics for Kafka operations.
//
// Prerequisites:
//   - OpenTelemetry TracerProvider must be configured globally (via otel.SetTracerProvider)
//   - OpenTelemetry MeterProvider must be configured globally (via otel.SetMeterProvider)
//
// What it instruments:
//   - Producer: Creates spans for each publish operation + metrics (count, duration, errors)
//   - Consumer: Creates spans for each consume operation + metrics (count, duration)
//   - Handler: Creates spans for each handler execution + metrics (duration)
//   - DLQ: Records metrics for Dead Letter Queue operations
//   - Retry: Records metrics for retry attempts
//
// Trace Context Propagation:
//   - Producer injects W3C traceparent header into Kafka messages
//   - Consumer extracts traceparent to create child spans
//   - Enables end-to-end distributed tracing: HTTP → Kafka → Consumer → Handler → Database
//
// Example Bootstrap:
//
//	func main() {
//	    ctx := context.Background()
//
//	    // 1. Initialize OpenTelemetry FIRST
//	    obs, err := otel.NewProvider(ctx, &otel.Config{
//	        ServiceName:     "order-service",
//	        ServiceVersion:  "1.0.0",
//	        OTLPEndpoint:    "tempo:4317",
//	        TraceSampleRate: 0.1, // 10% sampling in production
//	    })
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	    defer obs.Shutdown(ctx)
//
//	    // 2. Create Kafka client with tracing
//	    client, err := kafka.NewClient(
//	        kafka.WithBrokers("kafka:9092"),
//	        kafka.WithAuthPlaintext(),
//	        kafka.WithTracingEnabled("order-service"), // ⭐ Enable tracing
//	    )
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	    defer client.Close()
//
//	    // 3. Producer and Consumer are automatically instrumented
//	    producer, _ := client.NewProducer("orders")
//	    consumer, _ := client.NewConsumer(kafka.WithGroupID("order-processor"))
//
//	    // All operations automatically traced and metricsed
//	    producer.Publish(ctx, "orders", "123", headers, message)
//	}
//
// Performance Impact:
//   - Overhead: <10 microseconds per message (negligible vs 1-50ms network latency)
//   - Metrics export: Asynchronous, doesn't block message processing
//   - Recommendation: Use sampling (10-20%) in high-throughput production environments
func WithTracingEnabled(serviceName string) Option {
	return func(c *config) {
		inst, err := NewInstrumentation(serviceName)
		if err != nil {
			// Log error but don't fail initialization
			// This allows Kafka client to work even if OpenTelemetry is misconfigured
			c.logger.Error(context.Background(), "failed to initialize Kafka tracing",
				Field{Key: "service_name", Value: serviceName},
				Field{Key: "error", Value: err},
			)
			return
		}

		c.instrumentation = inst
		c.logger.Info(context.Background(), "Kafka OpenTelemetry instrumentation enabled",
			Field{Key: "service_name", Value: serviceName},
		)
	}
}
