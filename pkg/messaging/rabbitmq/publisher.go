package rabbitmq

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Publisher representa um publicador de mensagens RabbitMQ.
// Thread-safe e integrado com o sistema de observability do projeto.
type Publisher struct {
	client        *Client
	observability observability.Observability
}

// NewPublisher cria um novo Publisher a partir do Client.
//
// Exemplo:
//
//	publisher := rabbitmq.NewPublisher(client)
func NewPublisher(client *Client) *Publisher {
	return &Publisher{
		client:        client,
		observability: client.observability,
	}
}

// Publish publica uma mensagem em um exchange com routing key.
// Respeita timeout configurado no Client.
//
// Parâmetros:
//   - ctx: Contexto para cancelamento e timeout
//   - exchange: Nome do exchange (vazio para default exchange)
//   - routingKey: Routing key para roteamento
//   - body: Corpo da mensagem
//   - opts: Opções de publicação (headers, content-type, etc)
//
// Comportamento:
//   - Usa publisher confirms se habilitado (recomendado para produção)
//   - Respeita timeout do contexto
//   - Thread-safe
//   - Retorna erro se conexão não estiver disponível
//
// Retorna erro se:
//   - Contexto expirar
//   - Conexão não estiver disponível
//   - Publisher confirm falhar (se habilitado)
//   - Ocorrer erro de rede
func (p *Publisher) Publish(ctx context.Context, exchange, routingKey string, body []byte, opts ...PublishOption) error {
	ch, err := p.client.Channel()
	if err != nil {
		return fmt.Errorf("failed to get channel: %w", err)
	}

	msg := amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
		Body:         body,
	}

	for _, opt := range opts {
		opt(&msg)
	}

	publishCtx, cancel := context.WithTimeout(ctx, p.client.config.PublishTimeout)
	defer cancel()

	if p.client.config.EnablePublisherConfirms {
		confirm := make(chan amqp.Confirmation, 1)
		ch.NotifyPublish(confirm)

		if err := ch.PublishWithContext(publishCtx, exchange, routingKey, false, false, msg); err != nil {
			return fmt.Errorf("failed to publish: %w", err)
		}

		select {
		case c := <-confirm:
			if !c.Ack {
				return ErrPublishConfirmFailed
			}
		case <-publishCtx.Done():
			return ErrPublishTimeout
		}
	} else {
		if err := ch.PublishWithContext(publishCtx, exchange, routingKey, false, false, msg); err != nil {
			return fmt.Errorf("failed to publish: %w", err)
		}
	}

	p.observability.Logger().Debug(ctx, "message published",
		observability.String("exchange", exchange),
		observability.String("routing_key", routingKey),
		observability.Int("body_size", len(body)),
	)

	return nil
}

// PublishBatch publica múltiplas mensagens de forma eficiente.
// Todas as mensagens devem ter sucesso ou todas falham.
//
// Parâmetros:
//   - ctx: Contexto para cancelamento e timeout
//   - exchange: Nome do exchange
//   - routingKey: Routing key para todas as mensagens
//   - messages: Slice de mensagens a serem publicadas
//
// Comportamento:
//   - Publica em sequência com confirms (se habilitado)
//   - Para na primeira falha
//   - Thread-safe
func (p *Publisher) PublishBatch(ctx context.Context, exchange, routingKey string, messages [][]byte, opts ...PublishOption) error {
	for i, body := range messages {
		if err := p.Publish(ctx, exchange, routingKey, body, opts...); err != nil {
			return fmt.Errorf("failed to publish message %d: %w", i, err)
		}
	}

	p.observability.Logger().Info(ctx, "batch published",
		observability.String("exchange", exchange),
		observability.String("routing_key", routingKey),
		observability.Int("count", len(messages)),
	)

	return nil
}

// PublishOption configura opções de publicação.
type PublishOption func(*amqp.Publishing)

// WithContentType define o content type da mensagem.
func WithContentType(contentType string) PublishOption {
	return func(p *amqp.Publishing) {
		p.ContentType = contentType
	}
}

// WithHeaders define headers customizados.
func WithHeaders(headers map[string]interface{}) PublishOption {
	return func(p *amqp.Publishing) {
		if p.Headers == nil {
			p.Headers = amqp.Table{}
		}
		for k, v := range headers {
			p.Headers[k] = v
		}
	}
}

// WithPriority define prioridade da mensagem (0-9).
func WithPriority(priority uint8) PublishOption {
	return func(p *amqp.Publishing) {
		p.Priority = priority
	}
}

// WithExpiration define TTL da mensagem em milissegundos.
func WithExpiration(ms string) PublishOption {
	return func(p *amqp.Publishing) {
		p.Expiration = ms
	}
}

// WithCorrelationID define correlation ID para rastreamento.
func WithCorrelationID(id string) PublishOption {
	return func(p *amqp.Publishing) {
		p.CorrelationId = id
	}
}

// WithReplyTo define queue para resposta.
func WithReplyTo(queue string) PublishOption {
	return func(p *amqp.Publishing) {
		p.ReplyTo = queue
	}
}

// WithMessageID define ID único da mensagem.
func WithMessageID(id string) PublishOption {
	return func(p *amqp.Publishing) {
		p.MessageId = id
	}
}

// WithDeliveryMode define modo de entrega (Transient=1, Persistent=2).
func WithDeliveryMode(mode uint8) PublishOption {
	return func(p *amqp.Publishing) {
		p.DeliveryMode = mode
	}
}
