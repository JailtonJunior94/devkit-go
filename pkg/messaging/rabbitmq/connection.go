package rabbitmq

import (
	"context"
	"fmt"
	"sync"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/cenkalti/backoff/v4"
	amqp "github.com/rabbitmq/amqp091-go"
)

// connectionManager gerencia conexão AMQP com reconexão automática.
// Thread-safe e resiliente a falhas de rede.
type connectionManager struct {
	config        Config
	strategy      ConnectionStrategy
	observability observability.Observability

	mu             sync.RWMutex
	conn           *amqp.Connection
	channel        *amqp.Channel
	isConnected    bool
	isReconnecting bool
	closed         bool

	reconnectChan chan struct{}
	closeChan     chan struct{}
	closeOnce     sync.Once

	// Controle do watcher para prevenir goroutine leak
	watcherCancel context.CancelFunc
	watcherCtx    context.Context
}

// newConnectionManager cria um novo gerenciador de conexão.
func newConnectionManager(
	config Config,
	strategy ConnectionStrategy,
	o11y observability.Observability,
) *connectionManager {
	return &connectionManager{
		config:        config,
		strategy:      strategy,
		observability: o11y,
		isConnected:   false,
		reconnectChan: make(chan struct{}, 1),
		closeChan:     make(chan struct{}),
	}
}

// connect estabelece conexão inicial com RabbitMQ.
// Retorna erro se falhar após todas as tentativas.
func (cm *connectionManager) connect(ctx context.Context) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.closed {
		return ErrClientClosed
	}

	if cm.isConnected {
		return nil
	}

	cm.observability.Logger().Info(ctx, "connecting to RabbitMQ",
		observability.String("strategy", cm.strategy.Name()),
	)

	conn, err := cm.strategy.Dial(cm.config)
	if err != nil {
		return fmt.Errorf("failed to dial: %w", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("failed to create channel: %w", err)
	}

	if cm.config.EnablePublisherConfirms {
		if err := channel.Confirm(false); err != nil {
			_ = channel.Close()
			_ = conn.Close()
			return fmt.Errorf("failed to enable publisher confirms: %w", err)
		}
	}

	cm.conn = conn
	cm.channel = channel
	cm.isConnected = true

	cm.observability.Logger().Info(ctx, "connected to RabbitMQ successfully",
		observability.String("strategy", cm.strategy.Name()),
	)

	if cm.config.EnableAutoReconnect {
		// Cancelar watcher anterior se existir (previne goroutine leak)
		if cm.watcherCancel != nil {
			cm.watcherCancel()
			cm.watcherCancel = nil
		}

		// Criar novo contexto para o watcher
		cm.watcherCtx, cm.watcherCancel = context.WithCancel(ctx)
		go cm.watchConnection(cm.watcherCtx)
	}

	return nil
}

// watchConnection monitora a conexão e reconecta automaticamente se cair.
func (cm *connectionManager) watchConnection(ctx context.Context) {
	cm.mu.RLock()
	if cm.closed || cm.conn == nil {
		cm.mu.RUnlock()
		return
	}

	connCloseChan := cm.conn.NotifyClose(make(chan *amqp.Error, 1))
	chanCloseChan := cm.channel.NotifyClose(make(chan *amqp.Error, 1))
	cm.mu.RUnlock()

	select {
	case err := <-connCloseChan:
		if err != nil {
			cm.observability.Logger().Warn(ctx, "connection closed unexpectedly",
				observability.Error(err),
			)
			cm.triggerReconnect(ctx)
		}
	case err := <-chanCloseChan:
		if err != nil {
			cm.observability.Logger().Warn(ctx, "channel closed unexpectedly",
				observability.Error(err),
			)
			cm.triggerReconnect(ctx)
		}
	case <-cm.closeChan:
		return
	case <-ctx.Done():
		return
	}
}

// triggerReconnect inicia processo de reconexão.
func (cm *connectionManager) triggerReconnect(ctx context.Context) {
	cm.mu.Lock()
	if cm.closed || cm.isReconnecting {
		cm.mu.Unlock()
		return
	}

	cm.isConnected = false
	cm.isReconnecting = true
	cm.mu.Unlock()

	go cm.reconnect(ctx)
}

// reconnect tenta reconectar com backoff exponencial.
func (cm *connectionManager) reconnect(ctx context.Context) {
	defer func() {
		cm.mu.Lock()
		cm.isReconnecting = false
		cm.mu.Unlock()
	}()

	cm.observability.Logger().Info(ctx, "starting reconnection process",
		observability.String("strategy", cm.strategy.Name()),
	)

	backoffConfig := backoff.NewExponentialBackOff()
	backoffConfig.InitialInterval = cm.config.ReconnectInitialInterval
	backoffConfig.MaxInterval = cm.config.ReconnectMaxInterval
	backoffConfig.MaxElapsedTime = cm.config.ReconnectTimeout

	operation := func() error {
		select {
		case <-cm.closeChan:
			return backoff.Permanent(ErrClientClosed)
		case <-ctx.Done():
			return backoff.Permanent(ctx.Err())
		default:
		}

		cm.observability.Logger().Info(ctx, "attempting to reconnect",
			observability.String("strategy", cm.strategy.Name()),
		)

		conn, err := cm.strategy.Dial(cm.config)
		if err != nil {
			cm.observability.Logger().Warn(ctx, "reconnection attempt failed",
				observability.Error(err),
			)
			return err
		}

		channel, err := conn.Channel()
		if err != nil {
			_ = conn.Close()
			cm.observability.Logger().Warn(ctx, "failed to create channel during reconnect",
				observability.Error(err),
			)
			return err
		}

		if cm.config.EnablePublisherConfirms {
			if err := channel.Confirm(false); err != nil {
				_ = channel.Close()
				_ = conn.Close()
				return err
			}
		}

		cm.mu.Lock()
		cm.conn = conn
		cm.channel = channel
		cm.isConnected = true

		// Cancelar watcher anterior se existir (previne goroutine leak)
		if cm.watcherCancel != nil {
			cm.watcherCancel()
			cm.watcherCancel = nil
		}

		// Criar novo contexto para o watcher
		cm.watcherCtx, cm.watcherCancel = context.WithCancel(ctx)
		cm.mu.Unlock()

		cm.observability.Logger().Info(ctx, "reconnected successfully",
			observability.String("strategy", cm.strategy.Name()),
		)

		go cm.watchConnection(cm.watcherCtx)

		return nil
	}

	if err := backoff.Retry(operation, backoffConfig); err != nil {
		cm.observability.Logger().Error(ctx, "failed to reconnect after all retries",
			observability.Error(err),
		)
	}
}

// getChannel retorna o channel atual.
// Thread-safe.
func (cm *connectionManager) getChannel() (*amqp.Channel, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cm.closed {
		return nil, ErrClientClosed
	}

	if !cm.isConnected {
		return nil, ErrNoConnection
	}

	if cm.isReconnecting {
		return nil, ErrReconnecting
	}

	return cm.channel, nil
}

// getConnection retorna a conexão atual.
// Thread-safe.
func (cm *connectionManager) getConnection() (*amqp.Connection, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cm.closed {
		return nil, ErrClientClosed
	}

	if !cm.isConnected {
		return nil, ErrNoConnection
	}

	return cm.conn, nil
}

// isHealthy verifica se a conexão está saudável.
func (cm *connectionManager) isHealthy() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return cm.isConnected && !cm.closed && !cm.isReconnecting && cm.conn != nil && !cm.conn.IsClosed()
}

// close encerra a conexão graciosamente.
func (cm *connectionManager) close(ctx context.Context) error {
	var closeErr error

	cm.closeOnce.Do(func() {
		cm.mu.Lock()

		// Cancelar watcher antes de fechar (previne goroutine leak)
		if cm.watcherCancel != nil {
			cm.watcherCancel()
			cm.watcherCancel = nil
		}

		cm.closed = true
		close(cm.closeChan)

		if cm.channel != nil {
			if err := cm.channel.Close(); err != nil {
				cm.observability.Logger().Warn(ctx, "error closing channel",
					observability.Error(err),
				)
				closeErr = err
			}
		}

		if cm.conn != nil {
			if err := cm.conn.Close(); err != nil {
				cm.observability.Logger().Warn(ctx, "error closing connection",
					observability.Error(err),
				)
				if closeErr == nil {
					closeErr = err
				}
			}
		}

		cm.isConnected = false
		cm.mu.Unlock()

		cm.observability.Logger().Info(ctx, "connection closed")
	})

	return closeErr
}
