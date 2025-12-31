package rabbitmq

import (
	"context"
	"fmt"
	"sync"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	amqp "github.com/rabbitmq/amqp091-go"
)

// MessageHandler processa uma mensagem recebida.
// Retorna nil para ACK, erro para NACK.
type MessageHandler func(ctx context.Context, msg Message) error

// Message representa uma mensagem recebida do RabbitMQ.
type Message struct {
	Body        []byte
	Headers     map[string]interface{}
	RoutingKey  string
	Exchange    string
	ContentType string
	MessageID   string
	Timestamp   int64
	Delivery    amqp.Delivery
}

// Consumer representa um consumidor de mensagens RabbitMQ.
// Thread-safe e integrado com o sistema de observability do projeto.
type Consumer struct {
	client        *Client
	observability observability.Observability
	queue         string
	prefetchCount int
	autoAck       bool
	exclusive     bool

	mu       sync.RWMutex
	handlers map[string]MessageHandler
	workers  int
	closed   bool
}

// ConsumerOption configura opções do consumer.
type ConsumerOption func(*Consumer)

// WithQueue define a queue a ser consumida.
func WithQueue(name string) ConsumerOption {
	return func(c *Consumer) {
		c.queue = name
	}
}

// WithPrefetchCount define quantas mensagens buscar antecipadamente.
func WithPrefetchCount(count int) ConsumerOption {
	return func(c *Consumer) {
		c.prefetchCount = count
	}
}

// WithAutoAck habilita/desabilita auto-ack.
// Recomendado: false para produção (ACK manual após processamento).
func WithAutoAck(autoAck bool) ConsumerOption {
	return func(c *Consumer) {
		c.autoAck = autoAck
	}
}

// WithExclusive define se a queue é exclusiva desta conexão.
func WithExclusive(exclusive bool) ConsumerOption {
	return func(c *Consumer) {
		c.exclusive = exclusive
	}
}

// WithWorkerPool define número de workers concorrentes.
// Padrão: 1 (sem concorrência).
func WithWorkerPool(workers int) ConsumerOption {
	return func(c *Consumer) {
		c.workers = workers
	}
}

// NewConsumer cria um novo Consumer a partir do Client.
//
// Exemplo:
//
//	consumer := rabbitmq.NewConsumer(
//	    client,
//	    rabbitmq.WithQueue("my-queue"),
//	    rabbitmq.WithPrefetchCount(10),
//	    rabbitmq.WithWorkerPool(5),
//	)
func NewConsumer(client *Client, opts ...ConsumerOption) *Consumer {
	c := &Consumer{
		client:        client,
		observability: client.observability,
		prefetchCount: client.config.DefaultPrefetchCount,
		autoAck:       false,
		exclusive:     false,
		handlers:      make(map[string]MessageHandler),
		workers:       1,
		closed:        false,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// RegisterHandler registra handler para routing key específica.
// Routing key pode ser:
//   - Exata: "user.created"
//   - Pattern: "user.*"
//   - Catch-all: "*" (processa todas as mensagens)
func (c *Consumer) RegisterHandler(routingKey string, handler MessageHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.handlers[routingKey] = handler

	c.observability.Logger().Info(context.Background(), "handler registered",
		observability.String("queue", c.queue),
		observability.String("routing_key", routingKey),
	)
}

// Consume inicia consumo de mensagens da queue.
// Bloqueia até contexto ser cancelado ou ocorrer erro fatal.
//
// Comportamento:
//   - Configura QoS (prefetch)
//   - Inicia workers (se configurado)
//   - Processa mensagens continuamente
//   - ACK/NACK automático baseado no retorno do handler
//   - Trata erros e reconexões automaticamente
//
// Retorna erro se:
//   - Falhar ao configurar QoS
//   - Falhar ao iniciar consumo
//   - Consumer já estiver fechado
func (c *Consumer) Consume(ctx context.Context) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return ErrClientClosed
	}
	c.mu.RUnlock()

	ch, err := c.client.Channel()
	if err != nil {
		return fmt.Errorf("failed to get channel: %w", err)
	}

	if err := ch.Qos(c.prefetchCount, 0, false); err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	deliveries, err := ch.Consume(
		c.queue,
		"",
		c.autoAck,
		c.exclusive,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to start consuming: %w", err)
	}

	c.observability.Logger().Info(ctx, "consumer started",
		observability.String("queue", c.queue),
		observability.Int("prefetch", c.prefetchCount),
		observability.Int("workers", c.workers),
		observability.Bool("auto_ack", c.autoAck),
	)

	if c.workers > 1 {
		return c.consumeWithWorkerPool(ctx, deliveries)
	}

	return c.consumeSingleWorker(ctx, deliveries)
}

// consumeSingleWorker processa mensagens sequencialmente.
func (c *Consumer) consumeSingleWorker(ctx context.Context, deliveries <-chan amqp.Delivery) error {
	for {
		select {
		case <-ctx.Done():
			c.observability.Logger().Info(ctx, "consumer stopped by context",
				observability.String("queue", c.queue),
			)
			return ctx.Err()

		case delivery, ok := <-deliveries:
			if !ok {
				c.observability.Logger().Warn(ctx, "deliveries channel closed",
					observability.String("queue", c.queue),
				)
				return fmt.Errorf("deliveries channel closed")
			}

			c.processMessage(ctx, delivery)
		}
	}
}

// consumeWithWorkerPool processa mensagens com pool de workers.
func (c *Consumer) consumeWithWorkerPool(ctx context.Context, deliveries <-chan amqp.Delivery) error {
	messageChan := make(chan amqp.Delivery, c.workers*2)
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < c.workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			c.worker(ctx, workerID, messageChan)
		}(i)
	}

	// Goroutine dedicada para distribuir mensagens
	// Isso previne race condition ao fechar o canal
	distributorDone := make(chan struct{})
	go func() {
		defer close(messageChan)
		defer close(distributorDone)

		for {
			select {
			case <-ctx.Done():
				return
			case delivery, ok := <-deliveries:
				if !ok {
					// Canal de deliveries fechado
					return
				}

				// Tentar enviar mensagem ou cancelar se contexto cancelado
				select {
				case <-ctx.Done():
					return
				case messageChan <- delivery:
				}
			}
		}
	}()

	// Aguardar distribuidor terminar
	<-distributorDone

	// Aguardar todos os workers finalizarem
	wg.Wait()

	return ctx.Err()
}

// worker processa mensagens do canal de trabalho.
func (c *Consumer) worker(ctx context.Context, workerID int, messageChan <-chan amqp.Delivery) {
	c.observability.Logger().Debug(ctx, "worker started",
		observability.String("queue", c.queue),
		observability.Int("worker_id", workerID),
	)

	for {
		select {
		case <-ctx.Done():
			return
		case delivery, ok := <-messageChan:
			if !ok {
				return
			}
			c.processMessage(ctx, delivery)
		}
	}
}

// processMessage processa uma mensagem recebida.
func (c *Consumer) processMessage(ctx context.Context, delivery amqp.Delivery) {
	msg := Message{
		Body:        delivery.Body,
		Headers:     make(map[string]interface{}),
		RoutingKey:  delivery.RoutingKey,
		Exchange:    delivery.Exchange,
		ContentType: delivery.ContentType,
		MessageID:   delivery.MessageId,
		Timestamp:   delivery.Timestamp.Unix(),
		Delivery:    delivery,
	}

	for k, v := range delivery.Headers {
		msg.Headers[k] = v
	}

	c.mu.RLock()
	handler, exists := c.handlers[delivery.RoutingKey]
	if !exists {
		handler, exists = c.handlers["*"]
	}
	c.mu.RUnlock()

	if !exists {
		c.observability.Logger().Warn(ctx, "no handler for routing key",
			observability.String("queue", c.queue),
			observability.String("routing_key", delivery.RoutingKey),
		)

		if !c.autoAck {
			if err := delivery.Nack(false, false); err != nil {
				c.observability.Logger().Error(ctx, "failed to nack unhandled message",
					observability.Error(err),
				)
			}
		}
		return
	}

	if err := handler(ctx, msg); err != nil {
		c.observability.Logger().Error(ctx, "handler error",
			observability.String("queue", c.queue),
			observability.String("routing_key", delivery.RoutingKey),
			observability.Error(err),
		)

		if !c.autoAck {
			if err := delivery.Nack(false, true); err != nil {
				c.observability.Logger().Error(ctx, "failed to nack message",
					observability.Error(err),
				)
			}
		}
		return
	}

	if !c.autoAck {
		if err := delivery.Ack(false); err != nil {
			c.observability.Logger().Error(ctx, "failed to ack message",
				observability.Error(err),
			)
		}
	}
}

// Close encerra o consumer.
func (c *Consumer) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.closed = true

	c.observability.Logger().Info(context.Background(), "consumer closed",
		observability.String("queue", c.queue),
	)

	return nil
}
