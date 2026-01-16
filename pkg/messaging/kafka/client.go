package kafka

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging"
	"github.com/segmentio/kafka-go"
)

// Client defines the interface for Kafka client operations.
// This is the main entry point for interacting with Apache Kafka.
//
// The client is thread-safe and designed for production use with:
//   - Automatic retry with exponential backoff
//   - Automatic reconnection on connection loss
//   - Health checks for monitoring
//   - Graceful shutdown
//
// Usage pattern:
//  1. Create client with NewClient()
//  2. Connect to brokers with Connect()
//  3. Create producers/consumers via NewProducer()/NewConsumer()
//  4. Close with Close() when done
type Client interface {
	// Connect establishes connection to Kafka brokers.
	Connect(ctx context.Context) error

	// HealthCheck verifies the connection to Kafka is healthy.
	HealthCheck(ctx context.Context) error

	// NewProducer creates a new Kafka producer.
	NewProducer(topic string) (messaging.Publisher, error)

	// NewConsumer creates a new Kafka consumer.
	NewConsumer(opts ...ConsumerOption) (messaging.Consumer, error)

	// Close gracefully closes the Kafka client and all resources.
	Close() error

	// IsConnected returns true if the client is connected to Kafka.
	IsConnected() bool
}

// client is the private implementation of the Client interface.
// É thread-safe e não deve ser copiada após criação - sempre use ponteiros.
type client struct {
	config *config
	dialer *kafka.Dialer
	conn   *kafka.Conn

	connected   atomic.Bool
	closed      atomic.Bool
	mu          sync.RWMutex
	closeOnce   sync.Once
	reconnectMu sync.Mutex

	// Reconnect goroutine control - GOROUTINE LEAK PREVENTION
	reconnectCancel context.CancelFunc
	reconnectOnce   sync.Once // Ensures only one reconnect goroutine is started
}

// NewClient creates a new Kafka client with the provided options.
// The client is NOT connected automatically - call Connect() explicitly.
//
// Parâmetros:
//   - opts: Opções funcionais para configurar brokers, autenticação, timeouts, etc.
//
// Retorna:
//   - Client interface pronta para uso
//   - error se a configuração for inválida (brokers vazios, auth strategy inválida)
//
// Por quê separar New e Connect:
// Permite configurar o cliente antes de estabelecer conexão, útil para
// dependency injection e testes. Também permite melhor controle do ciclo
// de vida em aplicações complexas.
//
// Exemplo básico (Confluent Cloud):
//
//	client, err := kafka.NewClient(
//	    kafka.WithBrokers("pkc-xxxxx.us-east-1.aws.confluent.cloud:9092"),
//	    kafka.WithAuthConfluent("YOUR_API_KEY", "YOUR_API_SECRET"),
//	    kafka.WithLogger(myLogger),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//
//	if err := client.Connect(ctx); err != nil {
//	    log.Fatal(err)
//	}
//
// Exemplo desenvolvimento local (sem autenticação):
//
//	client, err := kafka.NewClient(
//	    kafka.WithBrokers("localhost:9092"),
//	    kafka.WithAuthPlaintext(),
//	)
func NewClient(opts ...Option) (Client, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	return &client{
		config: cfg,
	}, nil
}

// Connect establishes connection to Kafka brokers with retry logic.
func (c *client) Connect(ctx context.Context) error {
	if c.closed.Load() {
		return ErrClientClosed
	}

	if c.connected.Load() {
		return ErrClientAlreadyConnected
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, c.config.connectTimeout)
	defer cancel()

	var lastErr error
	backoff := c.config.retryBackoff

	for attempt := 0; attempt <= c.config.maxRetries; attempt++ {
		if attempt > 0 {
			c.config.logger.Info(ctx, "retrying connection",
				Field{Key: "attempt", Value: attempt},
				Field{Key: "backoff", Value: backoff},
			)
			time.Sleep(backoff)
			backoff = calculateBackoff(backoff, c.config.maxRetryBackoff)
		}

		if err := c.connectInternal(ctx); err != nil {
			lastErr = err
			c.config.logger.Error(ctx, "connection attempt failed",
				Field{Key: "error", Value: err},
				Field{Key: "attempt", Value: attempt},
			)
			continue
		}

		c.connected.Store(true)
		c.config.logger.Info(ctx, "successfully connected to kafka",
			Field{Key: "brokers", Value: c.config.brokers},
		)

		// GOROUTINE LEAK FIX: Use sync.Once to ensure only one reconnect goroutine
		// Even if Connect() is called multiple times, only one goroutine is created
		if c.config.reconnectEnabled {
			c.reconnectOnce.Do(func() {
				reconnectCtx, cancel := context.WithCancel(context.Background())
				c.reconnectCancel = cancel
				go c.reconnectWorker(reconnectCtx)
			})
		}

		return nil
	}

	return fmt.Errorf("%w: %v", ErrConnectionFailed, lastErr)
}

// connectInternal performs the actual connection.
func (c *client) connectInternal(ctx context.Context) error {
	dialer, err := c.config.authStrategy.Configure(c.config.authConfig)
	if err != nil {
		return fmt.Errorf("failed to configure auth: %w", err)
	}

	dialer.Timeout = c.config.dialTimeout

	conn, err := dialer.DialContext(ctx, "tcp", c.config.brokers[0])
	if err != nil {
		return fmt.Errorf("failed to dial: %w", err)
	}

	c.dialer = dialer
	c.conn = conn

	return nil
}

// HealthCheck verifies the connection to Kafka is healthy.
func (c *client) HealthCheck(ctx context.Context) error {
	if c.closed.Load() {
		return ErrClientClosed
	}

	if !c.connected.Load() {
		return ErrClientNotConnected
	}

	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return ErrClientNotConnected
	}

	if err := conn.SetDeadline(time.Now().Add(c.config.healthCheckTimeout)); err != nil {
		return fmt.Errorf("%w: failed to set deadline: %v", ErrHealthCheckFailed, err)
	}

	_, err := conn.ReadPartitions()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrHealthCheckFailed, err)
	}

	return nil
}

// NewProducer creates a new Kafka producer for the specified topic.
func (c *client) NewProducer(topic string) (messaging.Publisher, error) {
	if c.closed.Load() {
		return nil, ErrClientClosed
	}

	if !c.connected.Load() {
		return nil, ErrClientNotConnected
	}

	return newProducer(topic, c.config, c.dialer)
}

// NewConsumer creates a new Kafka consumer with the specified options.
func (c *client) NewConsumer(opts ...ConsumerOption) (messaging.Consumer, error) {
	if c.closed.Load() {
		return nil, ErrClientClosed
	}

	if !c.connected.Load() {
		return nil, ErrClientNotConnected
	}

	return newConsumer(c.config, c.dialer, opts...)
}

// Close gracefully closes the Kafka client and all resources.
func (c *client) Close() error {
	var closeErr error

	c.closeOnce.Do(func() {
		c.closed.Store(true)
		c.connected.Store(false)

		// GOROUTINE LEAK FIX: Cancel reconnect worker before closing
		// This ensures the reconnect goroutine exits cleanly
		if c.reconnectCancel != nil {
			c.config.logger.Debug(context.Background(), "stopping reconnect worker")
			c.reconnectCancel()

			// Give reconnect worker a moment to exit gracefully
			// This is a best-effort wait - we don't block indefinitely
			time.Sleep(100 * time.Millisecond)
		}

		c.mu.Lock()
		defer c.mu.Unlock()

		if c.conn != nil {
			if err := c.conn.Close(); err != nil {
				closeErr = fmt.Errorf("failed to close connection: %w", err)
			}
			c.conn = nil
		}

		c.config.logger.Info(context.Background(), "kafka client closed")
	})

	return closeErr
}

// IsConnected returns true if the client is connected to Kafka.
func (c *client) IsConnected() bool {
	return c.connected.Load() && !c.closed.Load()
}

// reconnectWorker monitors the connection and reconnects if necessary.
// GOROUTINE LEAK FIX: Now accepts a context for graceful shutdown.
// When Close() is called, the context is cancelled and this goroutine exits cleanly.
func (c *client) reconnectWorker(ctx context.Context) {
	ticker := time.NewTicker(c.config.reconnectInterval)
	defer ticker.Stop()

	c.config.logger.Info(context.Background(), "reconnect worker started")
	defer c.config.logger.Info(context.Background(), "reconnect worker stopped")

	for {
		select {
		case <-ctx.Done():
			// Context cancelled - graceful shutdown
			c.config.logger.Debug(context.Background(), "reconnect worker shutting down gracefully")
			return

		case <-ticker.C:
			// Check if client is closed
			if c.closed.Load() {
				return
			}

			// Skip if health check is disabled
			if !c.config.healthCheckEnabled {
				continue
			}

			// Perform health check with timeout
			healthCtx, cancel := context.WithTimeout(context.Background(), c.config.healthCheckTimeout)
			if err := c.HealthCheck(healthCtx); err != nil {
				c.config.logger.Warn(healthCtx, "health check failed, attempting reconnect",
					Field{Key: "error", Value: err},
				)
				cancel()
				c.attemptReconnect()
			} else {
				cancel()
			}
		}
	}
}

// attemptReconnect attempts to reconnect to Kafka.
func (c *client) attemptReconnect() {
	c.reconnectMu.Lock()
	defer c.reconnectMu.Unlock()

	if c.connected.Load() {
		return
	}

	c.config.logger.Info(context.Background(), "attempting to reconnect to kafka")

	backoff := c.config.retryBackoff
	for attempt := 0; attempt <= c.config.maxRetries; attempt++ {
		if c.closed.Load() {
			return
		}

		if attempt > 0 {
			time.Sleep(backoff)
			backoff = calculateBackoff(backoff, c.config.maxReconnectDelay)
		}

		ctx, cancel := context.WithTimeout(context.Background(), c.config.connectTimeout)
		if err := c.connectInternal(ctx); err != nil {
			c.config.logger.Error(ctx, "reconnection attempt failed",
				Field{Key: "error", Value: err},
				Field{Key: "attempt", Value: attempt},
			)
			cancel()
			continue
		}

		c.connected.Store(true)
		c.config.logger.Info(ctx, "successfully reconnected to kafka")
		cancel()
		return
	}

	c.config.logger.Error(context.Background(), "failed to reconnect after all attempts")
}

// validateConfig validates the client configuration.
func validateConfig(cfg *config) error {
	if len(cfg.brokers) == 0 {
		return ErrInvalidBrokers
	}

	if cfg.authStrategy == nil {
		return ErrInvalidAuthStrategy
	}

	return nil
}

// calculateBackoff calculates the next backoff duration with exponential backoff.
func calculateBackoff(current, max time.Duration) time.Duration {
	next := current * 2
	if next > max {
		return max
	}
	return next
}
