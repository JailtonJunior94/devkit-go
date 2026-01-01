package rabbitmq

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	amqp "github.com/rabbitmq/amqp091-go"
)

// DLQConfig configura Dead Letter Queue.
type DLQConfig struct {
	// Nome da queue DLQ
	QueueName string

	// Nome do exchange DLQ
	ExchangeName string

	// Routing key para DLQ
	RoutingKey string

	// TTL das mensagens na DLQ em milliseconds (0 = sem TTL)
	MessageTTL int32

	// Tamanho máximo da DLQ (0 = sem limite)
	MaxLength int32

	// Tipo do exchange DLQ (default: "direct")
	ExchangeType string

	// Tornar exchange e queue duráveis
	Durable bool
}

// DefaultDLQConfig retorna configuração padrão para DLQ.
func DefaultDLQConfig(queueName string) DLQConfig {
	return DLQConfig{
		QueueName:    queueName + ".dlq",
		ExchangeName: queueName + ".dlq.exchange",
		RoutingKey:   queueName + ".failed",
		MessageTTL:   0, // Sem TTL por padrão
		MaxLength:    0, // Sem limite por padrão
		ExchangeType: "direct",
		Durable:      true,
	}
}

// SetupDLQ configura Dead Letter Queue completa.
// Cria exchange DLQ, queue DLQ e faz binding.
//
// Uso:
//
//	dlqCfg := rabbitmq.DefaultDLQConfig("my-queue")
//	if err := client.SetupDLQ(ctx, dlqCfg); err != nil {
//	    log.Fatal(err)
//	}
func (c *Client) SetupDLQ(ctx context.Context, cfg DLQConfig) error {
	// Guard clause: validação básica
	if cfg.QueueName == "" {
		return fmt.Errorf("DLQ queue name is required")
	}

	if cfg.ExchangeName == "" {
		return fmt.Errorf("DLQ exchange name is required")
	}

	if cfg.ExchangeType == "" {
		cfg.ExchangeType = "direct"
	}

	// Criar exchange DLQ
	if err := c.DeclareExchange(ctx, cfg.ExchangeName, cfg.ExchangeType, cfg.Durable, false, nil); err != nil {
		return fmt.Errorf("failed to create DLQ exchange: %w", err)
	}

	// Criar queue DLQ com argumentos
	args := amqp.Table{}
	if cfg.MessageTTL > 0 {
		args["x-message-ttl"] = cfg.MessageTTL
	}
	if cfg.MaxLength > 0 {
		args["x-max-length"] = cfg.MaxLength
	}

	if _, err := c.DeclareQueue(ctx, cfg.QueueName, cfg.Durable, false, false, args); err != nil {
		return fmt.Errorf("failed to create DLQ queue: %w", err)
	}

	// Binding
	routingKey := cfg.RoutingKey
	if routingKey == "" {
		routingKey = "#"
	}

	if err := c.BindQueue(ctx, cfg.QueueName, routingKey, cfg.ExchangeName, nil); err != nil {
		return fmt.Errorf("failed to bind DLQ: %w", err)
	}

	c.observability.Logger().Info(ctx, "DLQ setup completed",
		observability.String("dlq_queue", cfg.QueueName),
		observability.String("dlq_exchange", cfg.ExchangeName),
		observability.String("routing_key", routingKey),
	)

	return nil
}

// DeclareQueueWithDLQ declara uma queue com Dead Letter Exchange configurado.
// Mensagens rejeitadas (nack sem requeue) vão automaticamente para o DLX.
//
// Uso:
//
//	queue, err := client.DeclareQueueWithDLQ(
//	    ctx,
//	    "my-queue",
//	    "my-queue.dlq.exchange",
//	    true, // durable
//	)
func (c *Client) DeclareQueueWithDLQ(
	ctx context.Context,
	queueName string,
	dlxExchange string,
	durable bool,
) (amqp.Queue, error) {
	// Guard clause: validação
	if queueName == "" {
		return amqp.Queue{}, fmt.Errorf("queue name is required")
	}

	if dlxExchange == "" {
		return amqp.Queue{}, fmt.Errorf("DLX exchange is required")
	}

	args := amqp.Table{
		"x-dead-letter-exchange": dlxExchange,
	}

	queue, err := c.DeclareQueue(ctx, queueName, durable, false, false, args)
	if err != nil {
		return amqp.Queue{}, err
	}

	c.observability.Logger().Info(ctx, "queue with DLX declared",
		observability.String("queue", queueName),
		observability.String("dlx", dlxExchange),
	)

	return queue, nil
}

// DeclareQueueWithDLQAndTTL declara queue com DLX e TTL.
// Mensagens que não são processadas dentro do TTL vão para DLX.
//
// Uso:
//
//	queue, err := client.DeclareQueueWithDLQAndTTL(
//	    ctx,
//	    "my-queue",
//	    "my-queue.dlq.exchange",
//	    true,        // durable
//	    300000,      // 5 minutos em ms
//	)
func (c *Client) DeclareQueueWithDLQAndTTL(
	ctx context.Context,
	queueName string,
	dlxExchange string,
	durable bool,
	ttlMs int32,
) (amqp.Queue, error) {
	// Guard clause: validação
	if queueName == "" {
		return amqp.Queue{}, fmt.Errorf("queue name is required")
	}

	if dlxExchange == "" {
		return amqp.Queue{}, fmt.Errorf("DLX exchange is required")
	}

	if ttlMs <= 0 {
		return amqp.Queue{}, fmt.Errorf("TTL must be positive")
	}

	args := amqp.Table{
		"x-dead-letter-exchange": dlxExchange,
		"x-message-ttl":          ttlMs,
	}

	queue, err := c.DeclareQueue(ctx, queueName, durable, false, false, args)
	if err != nil {
		return amqp.Queue{}, err
	}

	c.observability.Logger().Info(ctx, "queue with DLX and TTL declared",
		observability.String("queue", queueName),
		observability.String("dlx", dlxExchange),
		observability.Int("ttl_ms", int(ttlMs)),
	)

	return queue, nil
}

// SetupDelayedRetryQueue configura queue com delayed retry pattern.
// Mensagens que falham vão para uma queue de delay, depois retornam para a queue original.
//
// Arquitetura:
//   - Queue principal: my-queue
//   - Queue de retry/delay: my-queue.retry
//   - DLQ final: my-queue.dlq
//
// Uso:
//
//	if err := client.SetupDelayedRetryQueue(
//	    ctx,
//	    "my-queue",
//	    30000, // 30 segundos de delay
//	); err != nil {
//	    log.Fatal(err)
//	}
func (c *Client) SetupDelayedRetryQueue(
	ctx context.Context,
	queueName string,
	retryDelayMs int32,
) error {
	// Guard clause: validação
	if queueName == "" {
		return fmt.Errorf("queue name is required")
	}

	if retryDelayMs <= 0 {
		return fmt.Errorf("retry delay must be positive")
	}

	mainExchange := queueName + ".exchange"
	retryExchange := queueName + ".retry.exchange"
	retryQueue := queueName + ".retry"
	dlqExchange := queueName + ".dlq.exchange"

	// 1. Criar exchange principal
	if err := c.DeclareExchange(ctx, mainExchange, "direct", true, false, nil); err != nil {
		return err
	}

	// 2. Criar queue principal com DLX para retry
	mainArgs := amqp.Table{
		"x-dead-letter-exchange": retryExchange,
	}
	if _, err := c.DeclareQueue(ctx, queueName, true, false, false, mainArgs); err != nil {
		return err
	}

	// 3. Binding queue principal
	if err := c.BindQueue(ctx, queueName, queueName, mainExchange, nil); err != nil {
		return err
	}

	// 4. Criar exchange de retry
	if err := c.DeclareExchange(ctx, retryExchange, "direct", true, false, nil); err != nil {
		return err
	}

	// 5. Criar queue de retry com TTL e DLX de volta para main
	retryArgs := amqp.Table{
		"x-message-ttl":             retryDelayMs,
		"x-dead-letter-exchange":    mainExchange,
		"x-dead-letter-routing-key": queueName,
	}
	if _, err := c.DeclareQueue(ctx, retryQueue, true, false, false, retryArgs); err != nil {
		return err
	}

	// 6. Binding queue de retry
	if err := c.BindQueue(ctx, retryQueue, retryQueue, retryExchange, nil); err != nil {
		return err
	}

	// 7. Setup DLQ final
	dlqCfg := DLQConfig{
		QueueName:    queueName + ".dlq",
		ExchangeName: dlqExchange,
		RoutingKey:   queueName + ".failed",
		Durable:      true,
	}
	if err := c.SetupDLQ(ctx, dlqCfg); err != nil {
		return err
	}

	c.observability.Logger().Info(ctx, "delayed retry queue setup completed",
		observability.String("queue", queueName),
		observability.String("retry_queue", retryQueue),
		observability.Int("retry_delay_ms", int(retryDelayMs)),
	)

	return nil
}
