package rabbitmq

import (
	"context"
	"fmt"
	"sync"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	amqp "github.com/rabbitmq/amqp091-go"
)

type ChannelPool struct {
	conn          *amqp.Connection
	publisherCh   *publisherChannel
	consumerPools map[string]*consumerChannel
	mu            sync.RWMutex
	closed        bool
	o11y          observability.Observability
}

type publisherChannel struct {
	ch       *amqp.Channel
	confirms chan amqp.Confirmation
	mu       sync.Mutex
	closed   bool
}

type consumerChannel struct {
	ch     *amqp.Channel
	tag    string
	mu     sync.Mutex
	closed bool
}

func newChannelPool(conn *amqp.Connection, o11y observability.Observability, enableConfirms bool) (*ChannelPool, error) {
	if conn == nil {
		return nil, fmt.Errorf("connection cannot be nil")
	}

	pool := &ChannelPool{
		conn:          conn,
		consumerPools: make(map[string]*consumerChannel),
		o11y:          o11y,
	}

	var (
		pubCh *publisherChannel
		err   error
	)
	if enableConfirms {
		pubCh, err = pool.createPublisherChannel()
	} else {
		pubCh, err = pool.createPublisherChannelNoConfirms()
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create publisher channel: %w", err)
	}
	pool.publisherCh = pubCh

	return pool, nil
}

func (cp *ChannelPool) createPublisherChannel() (*publisherChannel, error) {
	ch, err := cp.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to create channel: %w", err)
	}

	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		return nil, fmt.Errorf("failed to enable confirms: %w", err)
	}

	confirms := ch.NotifyPublish(make(chan amqp.Confirmation, 100))

	return &publisherChannel{
		ch:       ch,
		confirms: confirms,
	}, nil
}

func (cp *ChannelPool) createPublisherChannelNoConfirms() (*publisherChannel, error) {
	ch, err := cp.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to create channel: %w", err)
	}
	return &publisherChannel{ch: ch}, nil
}

func (cp *ChannelPool) GetPublisherChannel() (*publisherChannel, error) {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	if cp.closed {
		return nil, ErrClientClosed
	}

	if cp.publisherCh == nil {
		return nil, fmt.Errorf("publisher channel not configured")
	}

	if cp.publisherCh.closed || cp.publisherCh.ch.IsClosed() {
		return nil, ErrChannelClosed
	}

	return cp.publisherCh, nil
}

func (cp *ChannelPool) GetConsumerChannel(consumerTag string) (*consumerChannel, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if cp.closed {
		return nil, ErrClientClosed
	}

	if consumerTag == "" {
		return nil, fmt.Errorf("consumer tag cannot be empty")
	}

	if existing, ok := cp.consumerPools[consumerTag]; ok {
		if !existing.closed && !existing.ch.IsClosed() {
			return existing, nil
		}
		delete(cp.consumerPools, consumerTag)
	}

	ch, err := cp.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer channel: %w", err)
	}

	consumerCh := &consumerChannel{
		ch:  ch,
		tag: consumerTag,
	}

	cp.consumerPools[consumerTag] = consumerCh

	cp.o11y.Logger().Debug(context.Background(), "consumer channel created",
		observability.String("consumer_tag", consumerTag),
	)

	return consumerCh, nil
}

func (cp *ChannelPool) ReleaseConsumerChannel(consumerTag string) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	consumerCh, ok := cp.consumerPools[consumerTag]
	if !ok {
		return nil
	}

	consumerCh.mu.Lock()
	if !consumerCh.closed {
		consumerCh.closed = true
		if err := consumerCh.ch.Close(); err != nil {
			consumerCh.mu.Unlock()
			return fmt.Errorf("failed to close consumer channel: %w", err)
		}
	}
	consumerCh.mu.Unlock()

	delete(cp.consumerPools, consumerTag)

	cp.o11y.Logger().Debug(context.Background(), "consumer channel released",
		observability.String("consumer_tag", consumerTag),
	)

	return nil
}

func (cp *ChannelPool) GetGenericChannel() (*amqp.Channel, error) {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	if cp.closed {
		return nil, ErrClientClosed
	}

	ch, err := cp.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to create generic channel: %w", err)
	}

	return ch, nil
}

func (cp *ChannelPool) Close(ctx context.Context) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if cp.closed {
		return nil
	}

	cp.closed = true

	var firstErr error

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

func (pc *publisherChannel) PublishWithConfirm(
	ctx context.Context,
	exchange, routingKey string,
	msg amqp.Publishing,
) error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.closed {
		return ErrChannelClosed
	}

	if pc.ch == nil || pc.ch.IsClosed() {
		return ErrChannelClosed
	}

	if err := pc.ch.PublishWithContext(ctx, exchange, routingKey, false, false, msg); err != nil {
		return fmt.Errorf("publish failed: %w", err)
	}

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

func (pc *publisherChannel) PublishWithoutConfirm(
	ctx context.Context,
	exchange, routingKey string,
	msg amqp.Publishing,
) error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.closed {
		return ErrChannelClosed
	}

	if pc.ch == nil || pc.ch.IsClosed() {
		return ErrChannelClosed
	}

	if err := pc.ch.PublishWithContext(ctx, exchange, routingKey, false, false, msg); err != nil {
		return fmt.Errorf("publish failed: %w", err)
	}

	return nil
}

func (cc *consumerChannel) Channel() (*amqp.Channel, error) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if cc.closed {
		return nil, ErrChannelClosed
	}

	if cc.ch == nil || cc.ch.IsClosed() {
		return nil, ErrChannelClosed
	}

	return cc.ch, nil
}
