package rabbitmq

import (
	"errors"
	"time"
)

// Config contém todas as configurações do cliente RabbitMQ.
// Segue o mesmo padrão de pkg/http_server/server_fiber/config.go.
type Config struct {
	// URL de conexão (amqp:// ou amqps://)
	// Sobrescrita pela ConnectionStrategy se fornecida
	URL string

	// Heartbeat interval para detectar conexões mortas
	// Valor padrão: 10s
	Heartbeat time.Duration

	// Timeout para operações de conexão
	// Valor padrão: 30s
	ConnectionTimeout time.Duration

	// Timeout para operações de publish
	// Valor padrão: 5s
	PublishTimeout time.Duration

	// Timeout para reconexão automática
	// Valor padrão: 5min
	ReconnectTimeout time.Duration

	// Intervalo inicial do backoff exponencial
	// Valor padrão: 1s
	ReconnectInitialInterval time.Duration

	// Intervalo máximo do backoff exponencial
	// Valor padrão: 30s
	ReconnectMaxInterval time.Duration

	// Prefetch count padrão para consumers
	// Valor padrão: 10
	DefaultPrefetchCount int

	// Habilita reconexão automática
	// Valor padrão: true
	EnableAutoReconnect bool

	// Habilita publisher confirms
	// Valor padrão: true (recomendado para produção)
	EnablePublisherConfirms bool

	// Número máximo de retries antes de enviar para DLQ
	// Valor padrão: 3
	MaxRetries int

	// Usar delayed retry (requer plugin rabbitmq-delayed-message-exchange)
	// Se false, usa requeue imediato
	// Valor padrão: false
	UseDelayedRetry bool

	// Nome do serviço (para logs e métricas)
	ServiceName string

	// Versão do serviço
	ServiceVersion string

	// Ambiente (development, staging, production)
	Environment string
}

// DefaultConfig retorna configurações seguras para produção.
// Valores escolhidos baseados em best practices do RabbitMQ.
func DefaultConfig() Config {
	return Config{
		Heartbeat:                10 * time.Second,
		ConnectionTimeout:        30 * time.Second,
		PublishTimeout:           5 * time.Second,
		ReconnectTimeout:         5 * time.Minute,
		ReconnectInitialInterval: 1 * time.Second,
		ReconnectMaxInterval:     30 * time.Second,
		DefaultPrefetchCount:     10,
		EnableAutoReconnect:      true,
		EnablePublisherConfirms:  true,
		MaxRetries:               3,
		UseDelayedRetry:          false,
		ServiceName:              "unknown-service",
		ServiceVersion:           "unknown",
		Environment:              "development",
	}
}

// Validate verifica se a configuração é válida.
// Retorna erro descritivo caso alguma configuração seja inválida.
func (c Config) Validate() error {
	if c.ServiceName == "" {
		return errors.New("service name is required")
	}

	if c.ServiceVersion == "" {
		return errors.New("service version is required")
	}

	if c.Environment == "" {
		return errors.New("environment is required")
	}

	if c.Heartbeat <= 0 {
		return errors.New("heartbeat must be positive")
	}

	if c.ConnectionTimeout <= 0 {
		return errors.New("connection timeout must be positive")
	}

	if c.PublishTimeout <= 0 {
		return errors.New("publish timeout must be positive")
	}

	if c.ReconnectTimeout <= 0 {
		return errors.New("reconnect timeout must be positive")
	}

	if c.ReconnectInitialInterval <= 0 {
		return errors.New("reconnect initial interval must be positive")
	}

	if c.ReconnectMaxInterval <= 0 {
		return errors.New("reconnect max interval must be positive")
	}

	if c.ReconnectMaxInterval < c.ReconnectInitialInterval {
		return errors.New("reconnect max interval must be >= initial interval")
	}

	if c.DefaultPrefetchCount < 1 {
		return errors.New("default prefetch count must be >= 1")
	}

	if c.MaxRetries < 0 {
		return errors.New("max retries must be >= 0")
	}

	return nil
}
