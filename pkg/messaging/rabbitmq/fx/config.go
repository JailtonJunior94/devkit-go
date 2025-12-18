package rabbitmqfx

import (
	"os"
	"strconv"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging/rabbitmq"
	"go.uber.org/fx"
)

// ConfigFromEnvModule provides RabbitMQ config from environment variables.
// Environment variables:
//   - RABBITMQ_URL: Connection URL (default: "amqp://guest:guest@localhost:5672/")
//   - RABBITMQ_MAX_CONNECTIONS: Max connections in pool (default: 10)
//   - RABBITMQ_MAX_CHANNELS: Max channels in pool (default: 100)
//   - RABBITMQ_HEARTBEAT_INTERVAL: Heartbeat interval in seconds (default: 60)
//   - RABBITMQ_RECONNECT_DELAY: Delay between reconnect attempts in seconds (default: 5)
//   - RABBITMQ_MAX_RECONNECT_ATTEMPTS: Max reconnect attempts (default: 10)
//   - RABBITMQ_PREFETCH_COUNT: Prefetch count for QoS (default: 10)
//   - RABBITMQ_PREFETCH_SIZE: Prefetch size for QoS (default: 0)
var ConfigFromEnvModule = fx.Provide(ConfigFromEnv)

// ConfigFromEnv creates RabbitMQ config from environment variables.
func ConfigFromEnv() *rabbitmq.Config {
	return &rabbitmq.Config{
		URL:                  getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		MaxConnections:       getEnvInt("RABBITMQ_MAX_CONNECTIONS", 10),
		MaxChannels:          getEnvInt("RABBITMQ_MAX_CHANNELS", 100),
		HeartbeatInterval:    getEnvDuration("RABBITMQ_HEARTBEAT_INTERVAL", 60*time.Second),
		ReconnectDelay:       getEnvDuration("RABBITMQ_RECONNECT_DELAY", 5*time.Second),
		MaxReconnectAttempts: getEnvInt("RABBITMQ_MAX_RECONNECT_ATTEMPTS", 10),
		PrefetchCount:        getEnvInt("RABBITMQ_PREFETCH_COUNT", 10),
		PrefetchSize:         getEnvInt("RABBITMQ_PREFETCH_SIZE", 0),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if seconds, err := strconv.Atoi(value); err == nil {
			return time.Duration(seconds) * time.Second
		}
	}
	return defaultValue
}
