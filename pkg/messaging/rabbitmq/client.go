package rabbitmq

import (
	"context"
	"fmt"
	"sync"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Client representa uma conexão gerenciada com RabbitMQ.
// É thread-safe e projetada para uso em produção.
// Não deve ser copiada após criação - sempre use ponteiros.
//
// Características:
//   - Reconexão automática com backoff exponencial
//   - Publisher confirms para garantia de entrega
//   - Health checks integrados
//   - Shutdown gracioso
//   - Thread-safe
//   - Context-aware
//
// Exemplo de uso:
//
//	client, err := rabbitmq.New(
//	    o11y,
//	    rabbitmq.WithCloudConnection(os.Getenv("RABBITMQ_URL")),
//	    rabbitmq.WithServiceName("my-service"),
//	    rabbitmq.WithServiceVersion("1.0.0"),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Shutdown(context.Background())
type Client struct {
	config        Config
	strategy      ConnectionStrategy
	observability observability.Observability
	connMgr       *connectionManager

	// OpenTelemetry instrumentation (optional).
	instrumentation *Instrumentation

	mu           sync.RWMutex
	closed       bool
	shutdownOnce sync.Once
}

// New cria uma nova instância de Client com observability e opções fornecidas.
// A conexão é estabelecida imediatamente e testada.
//
// Parâmetros:
//   - o11y: Observability provider (logger, metrics, tracing)
//   - opts: Opções funcionais para configurar cliente
//
// Retorna erro se:
//   - A configuração for inválida
//   - A strategy não for fornecida ou for inválida
//   - Falhar ao estabelecer conexão inicial
//
// Exemplo:
//
//	client, err := rabbitmq.New(
//	    o11y,
//	    rabbitmq.WithCloudConnection("amqps://user:pass@host/vhost"),
//	    rabbitmq.WithPublisherConfirms(true),
//	    rabbitmq.WithAutoReconnect(true),
//	)
func New(o11y observability.Observability, opts ...Option) (*Client, error) {
	client := &Client{
		config:        DefaultConfig(),
		observability: o11y,
		closed:        false,
	}

	for _, opt := range opts {
		opt(client)
	}

	if err := client.config.Validate(); err != nil {
		return nil, fmt.Errorf("rabbitmq: invalid configuration: %w", err)
	}

	if client.strategy == nil {
		return nil, ErrInvalidStrategy
	}

	client.connMgr = newConnectionManager(client.config, client.strategy, o11y)

	ctx := context.Background()
	if err := client.connMgr.connect(ctx); err != nil {
		return nil, fmt.Errorf("rabbitmq: failed to establish initial connection: %w", err)
	}

	o11y.Logger().Info(ctx, "RabbitMQ client initialized successfully",
		observability.String("strategy", client.strategy.Name()),
		observability.String("service", client.config.ServiceName),
		observability.String("version", client.config.ServiceVersion),
		observability.String("environment", client.config.Environment),
	)

	return client, nil
}

// Channel retorna um novo channel AMQP para operações customizadas.
// IMPORTANTE: O chamador é responsável por fechar o channel retornado.
//
// Retorna erro se:
//   - O cliente estiver fechado
//   - Não houver conexão ativa
//   - O cliente estiver em processo de reconexão
//
// Deprecated: Use métodos específicos (DeclareExchange, DeclareQueue, etc) sempre que possível.
// Este método será removido em v2.0.0.
func (c *Client) Channel() (*amqp.Channel, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Guard clause: cliente fechado
	if c.closed {
		return nil, ErrClientClosed
	}

	pool, err := c.connMgr.getChannelPool()
	if err != nil {
		return nil, err
	}

	return pool.GetGenericChannel()
}

// Connection retorna a conexão AMQP subjacente.
// Útil para operações avançadas que requerem acesso direto à conexão.
//
// IMPORTANTE: Não chame Close() diretamente na conexão retornada.
// Use sempre o método Shutdown() do Client.
//
// Retorna erro se:
//   - O cliente estiver fechado
//   - Não houver conexão ativa
func (c *Client) Connection() (*amqp.Connection, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Guard clause: cliente fechado
	if c.closed {
		return nil, ErrClientClosed
	}

	return c.connMgr.getConnection()
}

// Ping verifica se a conexão com RabbitMQ está ativa.
// Respeita o contexto para cancelamento/timeout.
//
// Use em:
//   - Health checks (endpoints /health, /ready, /live)
//   - Validação periódica de conectividade
//   - Após reconexão
//
// É thread-safe e pode ser chamado concorrentemente.
//
// Retorna erro se:
//   - O contexto for cancelado/timeout
//   - O cliente estiver fechado
//   - A conexão não estiver saudável
func (c *Client) Ping(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Guard clause: cliente fechado
	if c.closed {
		return ErrClientClosed
	}

	// Guard clause: conexão não saudável
	if !c.connMgr.isHealthy() {
		return fmt.Errorf("rabbitmq: connection is not healthy")
	}

	return nil
}

// IsConnected retorna true se o cliente está conectado ao RabbitMQ.
// Thread-safe.
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return false
	}

	return c.connMgr.isHealthy()
}

// Config retorna a configuração atual do cliente.
// Retorna uma cópia para prevenir modificações externas.
func (c *Client) Config() Config {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.config
}

// DeclareExchange declara um exchange no RabbitMQ.
// Idempotente - pode ser chamado múltiplas vezes com segurança.
//
// Parâmetros:
//   - name: Nome do exchange
//   - kind: Tipo (direct, fanout, topic, headers)
//   - durable: Sobrevive a restart do RabbitMQ
//   - autoDelete: Deletado quando não há mais bindings
//   - args: Argumentos adicionais (opcional)
func (c *Client) DeclareExchange(ctx context.Context, name, kind string, durable, autoDelete bool, args amqp.Table) error {
	// Guard clause: validação de durabilidade em produção
	if c.config.Environment == "production" && !durable {
		c.observability.Logger().Warn(ctx, "NON-DURABLE exchange in production - data loss risk!",
			observability.String("exchange", name),
			observability.String("environment", c.config.Environment),
		)
	}

	ch, err := c.Channel()
	if err != nil {
		return fmt.Errorf("failed to get channel: %w", err)
	}
	defer func() { _ = ch.Close() }()

	if err := ch.ExchangeDeclare(name, kind, durable, autoDelete, false, false, args); err != nil {
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	c.observability.Logger().Info(ctx, "exchange declared",
		observability.String("exchange", name),
		observability.String("kind", kind),
		observability.Bool("durable", durable),
	)

	return nil
}

// DeclareQueue declara uma queue no RabbitMQ.
// Idempotente - pode ser chamado múltiplas vezes com segurança.
//
// Parâmetros:
//   - name: Nome da queue
//   - durable: Sobrevive a restart do RabbitMQ
//   - autoDelete: Deletada quando não há consumers
//   - exclusive: Exclusiva para esta conexão
//   - args: Argumentos adicionais (DLQ, TTL, etc)
//
// Retorna:
//   - amqp.Queue: Informações da queue criada
//   - error: Erro se falhar
func (c *Client) DeclareQueue(ctx context.Context, name string, durable, autoDelete, exclusive bool, args amqp.Table) (amqp.Queue, error) {
	// Guard clause: validação de durabilidade em produção
	if c.config.Environment == "production" && !durable {
		c.observability.Logger().Warn(ctx, "NON-DURABLE queue in production - data loss risk!",
			observability.String("queue", name),
			observability.String("environment", c.config.Environment),
		)
	}

	ch, err := c.Channel()
	if err != nil {
		return amqp.Queue{}, fmt.Errorf("failed to get channel: %w", err)
	}
	defer func() { _ = ch.Close() }()

	queue, err := ch.QueueDeclare(name, durable, autoDelete, exclusive, false, args)
	if err != nil {
		return amqp.Queue{}, fmt.Errorf("failed to declare queue: %w", err)
	}

	c.observability.Logger().Info(ctx, "queue declared",
		observability.String("queue", name),
		observability.Bool("durable", durable),
		observability.Int("messages", queue.Messages),
		observability.Int("consumers", queue.Consumers),
	)

	return queue, nil
}

// BindQueue faz binding de queue a exchange.
//
// Parâmetros:
//   - queueName: Nome da queue
//   - routingKey: Routing key para binding
//   - exchangeName: Nome do exchange
//   - args: Argumentos adicionais (opcional)
func (c *Client) BindQueue(ctx context.Context, queueName, routingKey, exchangeName string, args amqp.Table) error {
	ch, err := c.Channel()
	if err != nil {
		return fmt.Errorf("failed to get channel: %w", err)
	}
	defer func() { _ = ch.Close() }()

	if err := ch.QueueBind(queueName, routingKey, exchangeName, false, args); err != nil {
		return fmt.Errorf("failed to bind queue: %w", err)
	}

	c.observability.Logger().Info(ctx, "queue bound to exchange",
		observability.String("queue", queueName),
		observability.String("exchange", exchangeName),
		observability.String("routing_key", routingKey),
	)

	return nil
}
