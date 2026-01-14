package rabbitmq

import (
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

// Option é uma função que modifica a configuração do Client.
// Segue o padrão functional options para APIs flexíveis e extensíveis.
type Option func(*Client)

// WithConfig define configuração customizada.
// Sobrescreve valores padrão.
func WithConfig(cfg Config) Option {
	return func(c *Client) {
		c.config = cfg
	}
}

// WithConnectionStrategy define a estratégia de conexão.
// Se não fornecida, usa CloudStrategy como padrão.
//
// Estratégias disponíveis:
//   - PlainStrategy: Desenvolvimento local sem TLS
//   - TLSStrategy: TLS com certificados customizados
//   - CloudStrategy: Produção com TLS (padrão)
func WithConnectionStrategy(strategy ConnectionStrategy) Option {
	return func(c *Client) {
		c.strategy = strategy
	}
}

// WithURL define a URL de conexão.
// Sobrescreve a URL da config.
func WithURL(url string) Option {
	return func(c *Client) {
		c.config.URL = url
	}
}

// WithHeartbeat define o intervalo de heartbeat.
func WithHeartbeat(interval time.Duration) Option {
	return func(c *Client) {
		c.config.Heartbeat = interval
	}
}

// WithConnectionTimeout define timeout para conexão.
func WithConnectionTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.config.ConnectionTimeout = timeout
	}
}

// WithPublishTimeout define timeout para publish.
func WithPublishTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.config.PublishTimeout = timeout
	}
}

// WithReconnectConfig define configuração completa de reconexão.
func WithReconnectConfig(timeout, initialInterval, maxInterval time.Duration) Option {
	return func(c *Client) {
		c.config.ReconnectTimeout = timeout
		c.config.ReconnectInitialInterval = initialInterval
		c.config.ReconnectMaxInterval = maxInterval
	}
}

// WithAutoReconnect habilita/desabilita reconexão automática.
func WithAutoReconnect(enabled bool) Option {
	return func(c *Client) {
		c.config.EnableAutoReconnect = enabled
	}
}

// WithPublisherConfirms habilita/desabilita publisher confirms.
// Recomendado: true para produção (garante entrega).
func WithPublisherConfirms(enabled bool) Option {
	return func(c *Client) {
		c.config.EnablePublisherConfirms = enabled
	}
}

// WithDefaultPrefetchCount define prefetch count padrão para consumers.
func WithDefaultPrefetchCount(count int) Option {
	return func(c *Client) {
		c.config.DefaultPrefetchCount = count
	}
}

// WithServiceName define nome do serviço.
func WithServiceName(name string) Option {
	return func(c *Client) {
		c.config.ServiceName = name
	}
}

// WithServiceVersion define versão do serviço.
func WithServiceVersion(version string) Option {
	return func(c *Client) {
		c.config.ServiceVersion = version
	}
}

// WithEnvironment define ambiente de execução.
func WithEnvironment(env string) Option {
	return func(c *Client) {
		c.config.Environment = env
	}
}

// WithPlainConnection cria cliente com PlainStrategy.
// Atalho para desenvolvimento local.
func WithPlainConnection(host, username, password, vhost string) Option {
	return func(c *Client) {
		c.strategy = NewPlainStrategy(host, username, password, vhost)
	}
}

// WithTLSConnection cria cliente com TLSStrategy.
// Atalho para conexão TLS customizada.
func WithTLSConnection(host, username, password, vhost, caCert, clientCert, clientKey string) Option {
	return func(c *Client) {
		c.strategy = NewTLSStrategy(host, username, password, vhost, caCert, clientCert, clientKey)
	}
}

// WithCloudConnection cria cliente com CloudStrategy.
// Atalho para conexão cloud (CloudAMQP, AWS MQ, etc).
func WithCloudConnection(url string) Option {
	return func(c *Client) {
		c.strategy = NewCloudStrategy(url)
	}
}

// WithTracingEnabled enables OpenTelemetry tracing and metrics for RabbitMQ operations.
//
// Prerequisites:
//   - OpenTelemetry TracerProvider must be configured globally (via otel.SetTracerProvider)
//   - OpenTelemetry MeterProvider must be configured globally (via otel.SetMeterProvider)
//
// What it instruments:
//   - Publisher: Creates spans for each publish operation + metrics (count, duration, errors)
//   - Consumer: Creates spans for each consume operation + metrics (count, duration)
//   - Handler: Creates spans for each handler execution + metrics (duration)
//   - DLQ: Records metrics for Dead Letter Queue operations
//   - Retry: Records metrics for retry attempts
//
// Trace Context Propagation:
//   - Publisher injects W3C traceparent header into RabbitMQ message headers
//   - Consumer extracts traceparent to create child spans
//   - Enables end-to-end distributed tracing: HTTP → RabbitMQ → Consumer → Handler → Database
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
//	    // 2. Create RabbitMQ client with tracing
//	    client, err := rabbitmq.New(
//	        obs,
//	        rabbitmq.WithCloudConnection("amqps://..."),
//	        rabbitmq.WithTracingEnabled("order-service"), // ⭐ Enable tracing
//	    )
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	    defer client.Shutdown(ctx)
//
//	    // 3. Publisher and Consumer are automatically instrumented
//	    publisher := rabbitmq.NewPublisher(client)
//	    consumer := rabbitmq.NewConsumer(client, rabbitmq.WithQueue("orders"))
//
//	    // All operations automatically traced and metricsed
//	    publisher.Publish(ctx, "orders-exchange", "order.created", body)
//	}
//
// Performance Impact:
//   - Overhead: <10 microseconds per message (negligible vs 1-50ms network latency)
//   - Metrics export: Asynchronous, doesn't block message processing
//   - Recommendation: Use sampling (10-20%) in high-throughput production environments
func WithTracingEnabled(serviceName string) Option {
	return func(c *Client) {
		inst, err := NewInstrumentation(serviceName)
		if err != nil {
			// Log error but don't fail initialization
			// This allows RabbitMQ client to work even if OpenTelemetry is misconfigured
			c.observability.Logger().Error(nil, "failed to initialize RabbitMQ tracing",
				observability.String("service_name", serviceName),
				observability.Error(err),
			)
			return
		}

		c.instrumentation = inst
		c.observability.Logger().Info(nil, "RabbitMQ OpenTelemetry instrumentation enabled",
			observability.String("service_name", serviceName),
		)
	}
}
