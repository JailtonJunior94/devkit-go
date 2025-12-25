package rabbitmq

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	amqp "github.com/rabbitmq/amqp091-go"
)

type ConnectionPool struct {
	config      *Config
	connections chan *amqp.Connection
	channels    chan *amqp.Channel
	mu          sync.RWMutex
	closed      bool
	ctx         context.Context
	cancel      context.CancelFunc
}

func NewConnectionPool(config *Config) (*ConnectionPool, error) {
	ctx, cancel := context.WithCancel(context.Background())

	pool := &ConnectionPool{
		config:      config,
		connections: make(chan *amqp.Connection, config.MaxConnections),
		channels:    make(chan *amqp.Channel, config.MaxChannels),
		ctx:         ctx,
		cancel:      cancel,
	}

	if err := pool.initConnections(); err != nil {
		return nil, fmt.Errorf("failed to initialize connections: %w", err)
	}

	go pool.healthMonitor()

	return pool, nil
}

func (p *ConnectionPool) initConnections() error {
	for i := 0; i < p.config.MaxConnections; i++ {
		conn, err := p.createConnection()
		if err != nil {
			return fmt.Errorf("failed to create connection %d: %w", i, err)
		}
		p.connections <- conn
	}

	for i := 0; i < p.config.MaxChannels; i++ {
		ch, err := p.createChannel()
		if err != nil {
			return fmt.Errorf("failed to create channel %d: %w", i, err)
		}
		p.channels <- ch
	}

	return nil
}

func (p *ConnectionPool) createConnection() (*amqp.Connection, error) {
	amqpConfig := amqp.Config{
		Heartbeat: p.config.HeartbeatInterval,
		Locale:    "en_US",
	}

	var conn *amqp.Connection
	operation := func() error {
		var err error
		conn, err = amqp.DialConfig(p.config.URL, amqpConfig)
		if err != nil {
			return fmt.Errorf("failed to dial RabbitMQ: %v", err)
		}
		return nil
	}

	backoffConfig := backoff.NewExponentialBackOff()
	backoffConfig.InitialInterval = p.config.ReconnectDelay
	backoffConfig.MaxInterval = 30 * time.Second
	backoffConfig.MaxElapsedTime = time.Duration(p.config.MaxReconnectAttempts) * p.config.ReconnectDelay

	if err := backoff.Retry(operation, backoffConfig); err != nil {
		return nil, err
	}

	go p.monitorConnection(conn)

	return conn, nil
}

func (p *ConnectionPool) createChannel() (*amqp.Channel, error) {
	conn := <-p.connections
	defer func() { p.connections <- conn }()

	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}

	if err := ch.Qos(p.config.PrefetchCount, p.config.PrefetchSize, false); err != nil {
		ch.Close()
		return nil, err
	}

	return ch, nil
}

func (p *ConnectionPool) GetChannel() (*amqp.Channel, error) {
	select {
	case ch := <-p.channels:
		if ch.IsClosed() {
			newCh, err := p.createChannel()
			if err != nil {
				return nil, err
			}
			return newCh, nil
		}
		return ch, nil
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("timeout getting channel from pool")
	}
}

func (p *ConnectionPool) ReturnChannel(ch *amqp.Channel) {
	if !ch.IsClosed() {
		select {
		case p.channels <- ch:
		default:
			ch.Close()
		}
	}
}

func (p *ConnectionPool) monitorConnection(conn *amqp.Connection) {
	notifyClose := make(chan *amqp.Error)
	conn.NotifyClose(notifyClose)

	select {
	case err := <-notifyClose:
		if err != nil {
			log.Printf("connection closed: %v", err)
			go p.reconnect()
		}
	case <-p.ctx.Done():
		return
	}
}

func (p *ConnectionPool) reconnect() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	log.Println("attempting to reconnect to RabbitMQ")

	// Drain and close old connections and channels to prevent memory leak
	p.drainAndCloseConnections()

	if err := p.initConnections(); err != nil {
		log.Printf("failed to reconnect: %v", err)
		time.AfterFunc(p.config.ReconnectDelay, func() {
			p.reconnect()
		})
	} else {
		log.Println("successfully reconnected to RabbitMQ")
	}
}

func (p *ConnectionPool) drainAndCloseConnections() {
	// Drain and close all existing connections
	for {
		select {
		case conn := <-p.connections:
			if conn != nil && !conn.IsClosed() {
				conn.Close()
			}
		default:
			goto drainChannels
		}
	}

drainChannels:
	// Drain and close all existing channels
	for {
		select {
		case ch := <-p.channels:
			if ch != nil && !ch.IsClosed() {
				ch.Close()
			}
		default:
			return
		}
	}
}

func (p *ConnectionPool) healthMonitor() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.checkHealth()
		case <-p.ctx.Done():
			return
		}
	}
}

func (p *ConnectionPool) checkHealth() {
	ch, err := p.GetChannel()
	if err != nil {
		log.Printf("health check failed: %v", err)
		return
	}
	defer p.ReturnChannel(ch)

	if err := ch.ExchangeDeclarePassive("amq.direct", "direct", true, false, false, false, nil); err != nil {
		log.Printf("health check failed: %v", err)
	}
}

func (p *ConnectionPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true
	p.cancel()

	close(p.connections)
	for conn := range p.connections {
		conn.Close()
	}

	close(p.channels)
	for ch := range p.channels {
		ch.Close()
	}

	return nil
}

func ProvideConnectionPool(config *Config) (*ConnectionPool, error) {
	return NewConnectionPool(config)
}
