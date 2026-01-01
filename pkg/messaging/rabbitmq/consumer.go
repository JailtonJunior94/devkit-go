package rabbitmq

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

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

// Start inicia o consumer com auto-recovery.
// Loop infinito que tenta consumir mensagens e se recupera automaticamente de falhas.
//
// Comportamento:
//   - Tenta iniciar consumo
//   - Se falhar, aguarda backoff e tenta novamente
//   - Se conexão cair, reconecta automaticamente
//   - Respeita contexto para shutdown gracioso
//
// Retorna erro se:
//   - Contexto for cancelado
//   - Consumer estiver fechado
func (c *Consumer) Start(ctx context.Context) error {
	c.observability.Logger().Info(ctx, "starting consumer with auto-recovery",
		observability.String("queue", c.queue),
	)

	backoffInterval := 1 * time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Guard clause: consumer fechado
		if c.isClosed() {
			return ErrClientClosed
		}

		err := c.consume(ctx)

		// Guard clause: contexto cancelado
		if err == ctx.Err() {
			return err
		}

		// Guard clause: sucesso (não deveria chegar aqui normalmente)
		if err == nil {
			return nil
		}

		// Log e backoff
		c.observability.Logger().Warn(ctx, "consumer failed, retrying",
			observability.String("queue", c.queue),
			observability.Error(err),
			observability.String("backoff", backoffInterval.String()),
		)

		c.waitBeforeRetry(ctx, backoffInterval)

		// Aumentar backoff exponencialmente
		backoffInterval *= 2
		if backoffInterval > maxBackoff {
			backoffInterval = maxBackoff
		}
	}
}

// Consume inicia consumo de mensagens da queue.
//
// Deprecated: Use Start() para auto-recovery. Consume será removido em v2.0.0.
func (c *Consumer) Consume(ctx context.Context) error {
	return c.Start(ctx)
}

// consume é a implementação interna de consumo (sem auto-recovery).
func (c *Consumer) consume(ctx context.Context) error {
	c.mu.RLock()

	// Guard clause: consumer fechado
	if c.closed {
		c.mu.RUnlock()
		return ErrClientClosed
	}
	c.mu.RUnlock()

	// Obter channel dedicado do pool
	pool, err := c.client.connMgr.getChannelPool()
	if err != nil {
		return fmt.Errorf("failed to get channel pool: %w", err)
	}

	consumerCh, err := pool.GetConsumerChannel(c.queue)
	if err != nil {
		return fmt.Errorf("failed to get consumer channel: %w", err)
	}
	defer func() { _ = pool.ReleaseConsumerChannel(c.queue) }()

	ch, err := consumerCh.Channel()
	if err != nil {
		return fmt.Errorf("failed to get AMQP channel: %w", err)
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

	// Guard clause: usar worker pool
	if c.workers > 1 {
		return c.consumeWithWorkerPool(ctx, deliveries)
	}

	return c.consumeSingleWorker(ctx, deliveries)
}

// waitBeforeRetry aguarda antes de tentar novamente, respeitando o contexto.
func (c *Consumer) waitBeforeRetry(ctx context.Context, interval time.Duration) {
	timer := time.NewTimer(interval)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return
	case <-timer.C:
		return
	}
}

// isClosed verifica se o consumer está fechado.
func (c *Consumer) isClosed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.closed
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

// processMessage processa uma mensagem recebida com panic recovery.
func (c *Consumer) processMessage(ctx context.Context, delivery amqp.Delivery) {
	defer func() {
		if r := recover(); r != nil {
			c.handlePanic(ctx, delivery, r)
		}
	}()

	c.processMessageLogic(ctx, delivery)
}

// handlePanic trata panics ocorridos durante processamento de mensagem.
func (c *Consumer) handlePanic(ctx context.Context, delivery amqp.Delivery, panicValue interface{}) {
	c.observability.Logger().Error(ctx, "PANIC in message handler",
		observability.String("queue", c.queue),
		observability.String("routing_key", delivery.RoutingKey),
		observability.Any("panic", panicValue),
		observability.String("stack", string(debug.Stack())),
	)

	// Guard clause: auto-ack habilitado
	if c.autoAck {
		return
	}

	// NACK sem requeue em caso de panic (vai para DLQ se configurado)
	if err := delivery.Nack(false, false); err != nil {
		c.observability.Logger().Error(ctx, "failed to nack after panic",
			observability.Error(err),
		)
	}
}

// processMessageLogic contém a lógica de processamento de mensagem.
func (c *Consumer) processMessageLogic(ctx context.Context, delivery amqp.Delivery) {
	msg := c.buildMessage(delivery)
	retryCount := getRetryCount(delivery)
	handler := c.findHandler(delivery.RoutingKey)

	// Guard clause: sem handler registrado
	if handler == nil {
		c.handleNoHandler(ctx, delivery)
		return
	}

	// Timeout no handler
	handlerCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	err := handler(handlerCtx, msg)

	// Guard clause: sucesso
	if err == nil {
		c.handleSuccess(ctx, delivery)
		return
	}

	// Tratamento de erro com retry logic
	c.handleError(ctx, delivery, err, retryCount)
}

// buildMessage constrói Message a partir de Delivery.
func (c *Consumer) buildMessage(delivery amqp.Delivery) Message {
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

	return msg
}

// findHandler encontra handler para routing key.
func (c *Consumer) findHandler(routingKey string) MessageHandler {
	c.mu.RLock()
	defer c.mu.RUnlock()

	handler, exists := c.handlers[routingKey]
	if exists {
		return handler
	}

	// Fallback para catch-all
	handler, exists = c.handlers["*"]
	if exists {
		return handler
	}

	return nil
}

// handleNoHandler trata mensagens sem handler registrado.
func (c *Consumer) handleNoHandler(ctx context.Context, delivery amqp.Delivery) {
	c.observability.Logger().Warn(ctx, "no handler for routing key",
		observability.String("queue", c.queue),
		observability.String("routing_key", delivery.RoutingKey),
	)

	// Guard clause: auto-ack habilitado
	if c.autoAck {
		return
	}

	// NACK sem requeue (vai para DLQ se configurado)
	if err := delivery.Nack(false, false); err != nil {
		c.observability.Logger().Error(ctx, "failed to nack unhandled message",
			observability.Error(err),
		)
	}
}

// handleSuccess trata processamento bem-sucedido.
func (c *Consumer) handleSuccess(ctx context.Context, delivery amqp.Delivery) {
	// Guard clause: auto-ack habilitado
	if c.autoAck {
		return
	}

	if err := delivery.Ack(false); err != nil {
		c.observability.Logger().Error(ctx, "failed to ack message",
			observability.Error(err),
		)
	}
}

// handleError trata erros de processamento com retry logic.
func (c *Consumer) handleError(ctx context.Context, delivery amqp.Delivery, err error, retryCount int) {
	c.observability.Logger().Error(ctx, "handler error",
		observability.String("queue", c.queue),
		observability.String("routing_key", delivery.RoutingKey),
		observability.Int("retry_count", retryCount),
		observability.Error(err),
	)

	// Guard clause: auto-ack habilitado
	if c.autoAck {
		return
	}

	maxRetries := c.getMaxRetries()

	// Guard clause: excedeu max retries
	if retryCount >= maxRetries {
		c.sendToDLQ(ctx, delivery, retryCount)
		return
	}

	// Requeue para retry
	c.requeueMessage(ctx, delivery, retryCount)
}

// sendToDLQ envia mensagem para Dead Letter Queue.
func (c *Consumer) sendToDLQ(ctx context.Context, delivery amqp.Delivery, retryCount int) {
	c.observability.Logger().Warn(ctx, "max retries exceeded, sending to DLQ",
		observability.String("queue", c.queue),
		observability.String("routing_key", delivery.RoutingKey),
		observability.Int("retry_count", retryCount),
		observability.Int("max_retries", c.getMaxRetries()),
	)

	// NACK sem requeue (vai para DLQ se configurado)
	if err := delivery.Nack(false, false); err != nil {
		c.observability.Logger().Error(ctx, "failed to nack to DLQ",
			observability.Error(err),
		)
	}
}

// requeueMessage recoloca mensagem na fila para retry.
func (c *Consumer) requeueMessage(ctx context.Context, delivery amqp.Delivery, retryCount int) {
	c.observability.Logger().Debug(ctx, "requeuing message for retry",
		observability.String("queue", c.queue),
		observability.String("routing_key", delivery.RoutingKey),
		observability.Int("retry_count", retryCount),
	)

	// NACK com requeue
	if err := delivery.Nack(false, true); err != nil {
		c.observability.Logger().Error(ctx, "failed to nack message",
			observability.Error(err),
		)
	}
}

// getMaxRetries retorna o número máximo de retries configurado.
func (c *Consumer) getMaxRetries() int {
	return c.client.config.MaxRetries
}

// getRetryCount extrai a contagem de retries dos headers da mensagem.
// RabbitMQ incrementa x-death quando mensagem é reentregue via DLQ.
func getRetryCount(delivery amqp.Delivery) int {
	xDeath, ok := delivery.Headers["x-death"].([]interface{})
	if !ok {
		return 0
	}
	return len(xDeath)
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
