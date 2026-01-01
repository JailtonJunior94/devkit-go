package rabbitmq

import (
	"context"
	"fmt"
	"sync"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	amqp "github.com/rabbitmq/amqp091-go"
)

// ChannelPool gerencia channels dedicados para diferentes operações.
// Resolve o problema crítico de channel sharing implementando:
//   - Channel dedicado para publisher com confirms
//   - Channels dedicados por consumer
//   - Thread-safety completo
//   - Resource cleanup adequado
type ChannelPool struct {
	conn          *amqp.Connection
	publisherCh   *publisherChannel
	consumerPools map[string]*consumerChannel
	mu            sync.RWMutex
	closed        bool
	o11y          observability.Observability
}

// publisherChannel representa um channel dedicado para publicação.
// Configurado uma única vez com publisher confirms.
type publisherChannel struct {
	ch       *amqp.Channel
	confirms chan amqp.Confirmation
	mu       sync.Mutex
	closed   bool
}

// consumerChannel representa um channel dedicado para um consumer.
type consumerChannel struct {
	ch     *amqp.Channel
	tag    string
	mu     sync.Mutex
	closed bool
}

// newChannelPool cria um novo pool de channels.
func newChannelPool(conn *amqp.Connection, o11y observability.Observability, enableConfirms bool) (*ChannelPool, error) {
	// Guard clause: conexão inválida
	if conn == nil {
		return nil, fmt.Errorf("connection cannot be nil")
	}

	pool := &ChannelPool{
		conn:          conn,
		consumerPools: make(map[string]*consumerChannel),
		closed:        false,
		o11y:          o11y,
	}

	// Guard clause: publisher confirms desabilitado
	if !enableConfirms {
		return pool, nil
	}

	// Criar channel de publisher com confirms
	pubCh, err := pool.createPublisherChannel()
	if err != nil {
		return nil, fmt.Errorf("failed to create publisher channel: %w", err)
	}

	pool.publisherCh = pubCh

	return pool, nil
}

// createPublisherChannel cria e configura channel de publisher.
func (cp *ChannelPool) createPublisherChannel() (*publisherChannel, error) {
	ch, err := cp.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to create channel: %w", err)
	}

	// Habilitar confirms
	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		return nil, fmt.Errorf("failed to enable confirms: %w", err)
	}

	// Registrar canal de confirmações (uma única vez)
	confirms := ch.NotifyPublish(make(chan amqp.Confirmation, 100))

	return &publisherChannel{
		ch:       ch,
		confirms: confirms,
		closed:   false,
	}, nil
}

// GetPublisherChannel retorna o channel de publisher.
// Thread-safe e pronto para uso com confirms.
func (cp *ChannelPool) GetPublisherChannel() (*publisherChannel, error) {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	// Guard clause: pool fechado
	if cp.closed {
		return nil, ErrClientClosed
	}

	// Guard clause: publisher channel não configurado
	if cp.publisherCh == nil {
		return nil, fmt.Errorf("publisher channel not configured")
	}

	// Guard clause: channel fechado
	if cp.publisherCh.closed || cp.publisherCh.ch.IsClosed() {
		return nil, ErrChannelClosed
	}

	return cp.publisherCh, nil
}

// GetConsumerChannel retorna ou cria um channel dedicado para consumer.
// Cada consumer identificado por tag recebe seu próprio channel.
func (cp *ChannelPool) GetConsumerChannel(consumerTag string) (*consumerChannel, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	// Guard clause: pool fechado
	if cp.closed {
		return nil, ErrClientClosed
	}

	// Guard clause: tag vazia
	if consumerTag == "" {
		return nil, fmt.Errorf("consumer tag cannot be empty")
	}

	// Verificar se já existe channel para este consumer
	if existing, ok := cp.consumerPools[consumerTag]; ok {
		if !existing.closed && !existing.ch.IsClosed() {
			return existing, nil
		}
		// Channel existente está fechado, remover
		delete(cp.consumerPools, consumerTag)
	}

	// Criar novo channel
	ch, err := cp.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer channel: %w", err)
	}

	consumerCh := &consumerChannel{
		ch:     ch,
		tag:    consumerTag,
		closed: false,
	}

	cp.consumerPools[consumerTag] = consumerCh

	cp.o11y.Logger().Debug(context.Background(), "consumer channel created",
		observability.String("consumer_tag", consumerTag),
	)

	return consumerCh, nil
}

// ReleaseConsumerChannel libera um channel de consumer.
func (cp *ChannelPool) ReleaseConsumerChannel(consumerTag string) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	// Guard clause: consumer não existe
	consumerCh, ok := cp.consumerPools[consumerTag]
	if !ok {
		return nil // Já foi liberado
	}

	// Fechar channel
	consumerCh.mu.Lock()
	if !consumerCh.closed {
		consumerCh.closed = true
		if err := consumerCh.ch.Close(); err != nil {
			consumerCh.mu.Unlock()
			return fmt.Errorf("failed to close consumer channel: %w", err)
		}
	}
	consumerCh.mu.Unlock()

	// Remover do pool
	delete(cp.consumerPools, consumerTag)

	cp.o11y.Logger().Debug(context.Background(), "consumer channel released",
		observability.String("consumer_tag", consumerTag),
	)

	return nil
}

// GetGenericChannel retorna um novo channel para operações gerais.
// Usado para operações que não são publish/consume (declarações, etc).
// IMPORTANTE: O chamador é responsável por fechar o channel.
func (cp *ChannelPool) GetGenericChannel() (*amqp.Channel, error) {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	// Guard clause: pool fechado
	if cp.closed {
		return nil, ErrClientClosed
	}

	ch, err := cp.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to create generic channel: %w", err)
	}

	return ch, nil
}

// Close fecha todos os channels do pool.
func (cp *ChannelPool) Close(ctx context.Context) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	// Guard clause: já fechado
	if cp.closed {
		return nil
	}

	cp.closed = true

	var firstErr error

	// Fechar publisher channel
	if cp.publisherCh != nil {
		cp.publisherCh.mu.Lock()
		if !cp.publisherCh.closed {
			cp.publisherCh.closed = true
			if err := cp.publisherCh.ch.Close(); err != nil {
				cp.o11y.Logger().Warn(ctx, "error closing publisher channel",
					observability.Error(err),
				)
				if firstErr == nil {
					firstErr = err
				}
			}
		}
		cp.publisherCh.mu.Unlock()
	}

	// Fechar consumer channels
	for tag, consumerCh := range cp.consumerPools {
		consumerCh.mu.Lock()
		if !consumerCh.closed {
			consumerCh.closed = true
			if err := consumerCh.ch.Close(); err != nil {
				cp.o11y.Logger().Warn(ctx, "error closing consumer channel",
					observability.String("consumer_tag", tag),
					observability.Error(err),
				)
				if firstErr == nil {
					firstErr = err
				}
			}
		}
		consumerCh.mu.Unlock()
	}

	cp.consumerPools = make(map[string]*consumerChannel)

	cp.o11y.Logger().Debug(ctx, "channel pool closed")

	return firstErr
}

// PublishWithConfirm publica uma mensagem e aguarda confirmação.
// Thread-safe e com timeout configurável via contexto.
func (pc *publisherChannel) PublishWithConfirm(
	ctx context.Context,
	exchange, routingKey string,
	msg amqp.Publishing,
) error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	// Guard clause: channel fechado
	if pc.closed {
		return ErrChannelClosed
	}

	// Guard clause: channel AMQP fechado
	if pc.ch == nil || pc.ch.IsClosed() {
		return ErrChannelClosed
	}

	// Publicar mensagem
	if err := pc.ch.PublishWithContext(ctx, exchange, routingKey, false, false, msg); err != nil {
		return fmt.Errorf("publish failed: %w", err)
	}

	// Aguardar confirmação
	select {
	case confirm := <-pc.confirms:
		if !confirm.Ack {
			return ErrPublishConfirmFailed
		}
		return nil
	case <-ctx.Done():
		return ErrPublishTimeout
	}
}

// PublishWithoutConfirm publica uma mensagem sem aguardar confirmação.
// Mais rápido mas sem garantia de entrega.
func (pc *publisherChannel) PublishWithoutConfirm(
	ctx context.Context,
	exchange, routingKey string,
	msg amqp.Publishing,
) error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	// Guard clause: channel fechado
	if pc.closed {
		return ErrChannelClosed
	}

	// Guard clause: channel AMQP fechado
	if pc.ch == nil || pc.ch.IsClosed() {
		return ErrChannelClosed
	}

	if err := pc.ch.PublishWithContext(ctx, exchange, routingKey, false, false, msg); err != nil {
		return fmt.Errorf("publish failed: %w", err)
	}

	return nil
}

// Channel retorna o channel AMQP subjacente para consumer.
// Usado para configurações e consumo.
func (cc *consumerChannel) Channel() (*amqp.Channel, error) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	// Guard clause: channel fechado
	if cc.closed {
		return nil, ErrChannelClosed
	}

	// Guard clause: channel AMQP fechado
	if cc.ch == nil || cc.ch.IsClosed() {
		return nil, ErrChannelClosed
	}

	return cc.ch, nil
}
