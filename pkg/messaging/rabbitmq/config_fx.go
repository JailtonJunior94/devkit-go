package rabbitmq

import (
	"time"

	"go.uber.org/fx"
)

type Config struct {
	URL                  string        `env:"RABBITMQ_URL"`
	MaxConnections       int           `env:"RABBITMQ_MAX_CONNECTIONS"`
	MaxChannels          int           `env:"RABBITMQ_MAX_CHANNELS"`
	HeartbeatInterval    time.Duration `env:"RABBITMQ_HEARTBEAT_INTERVAL"`
	ReconnectDelay       time.Duration `env:"RABBITMQ_RECONNECT_DELAY"`
	MaxReconnectAttempts int           `env:"RABBITMQ_MAX_RECONNECT_ATTEMPTS"`
	PrefetchCount        int           `env:"RABBITMQ_PREFETCH_COUNT"`
	PrefetchSize         int           `env:"RABBITMQ_PREFETCH_SIZE"`
}

func DefaultConfig() *Config {
	return &Config{
		URL:                  "amqp://guest:pass@rabbitmq@localhost:5672/",
		MaxConnections:       10,
		MaxChannels:          100,
		HeartbeatInterval:    60 * time.Second,
		ReconnectDelay:       5 * time.Second,
		MaxReconnectAttempts: 10,
		PrefetchCount:        10,
		PrefetchSize:         0,
	}
}

var ConfigModule = fx.Options(
	fx.Provide(DefaultConfig),
)
