package kafkafx

import (
	"os"
	"strconv"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging/kafka"
	"github.com/JailtonJunior94/devkit-go/pkg/vos"
	"go.uber.org/fx"
)

// BrokerConfig holds configuration for creating a Kafka broker.
type BrokerConfig struct {
	// Brokers is the list of Kafka broker addresses.
	Brokers []string

	// Mechanism is the authentication mechanism.
	// Options: vos.PlainText, vos.Plain, vos.Scram
	Mechanism vos.Mechanism

	// Auth holds authentication credentials (required for Plain and Scram).
	Auth *kafka.AuthConfig
}

// DefaultBrokerConfig returns a default broker configuration for local development.
func DefaultBrokerConfig() BrokerConfig {
	return BrokerConfig{
		Brokers:   []string{"localhost:9092"},
		Mechanism: vos.PlainText,
		Auth:      nil,
	}
}

// ConfigFromEnvModule provides Kafka config from environment variables.
// Environment variables:
//   - KAFKA_BROKERS: Comma-separated list of broker addresses (default: "localhost:9092")
//   - KAFKA_MECHANISM: Authentication mechanism - "plaintext", "plain", or "scram" (default: "plaintext")
//   - KAFKA_USERNAME: Username for SASL authentication
//   - KAFKA_PASSWORD: Password for SASL authentication
//   - KAFKA_TLS_SKIP_VERIFY: Skip TLS verification (default: false)
var ConfigFromEnvModule = fx.Provide(ConfigFromEnv)

// ConfigFromEnv creates Kafka broker config from environment variables.
func ConfigFromEnv() BrokerConfig {
	brokers := getEnv("KAFKA_BROKERS", "localhost:9092")
	brokerList := strings.Split(brokers, ",")
	for i := range brokerList {
		brokerList[i] = strings.TrimSpace(brokerList[i])
	}

	mechanism := parseMechanism(getEnv("KAFKA_MECHANISM", "plaintext"))

	var auth *kafka.AuthConfig
	if mechanism != vos.PlainText {
		auth = &kafka.AuthConfig{
			Username:           getEnv("KAFKA_USERNAME", ""),
			Password:           getEnv("KAFKA_PASSWORD", ""),
			InsecureSkipVerify: getEnvBool("KAFKA_TLS_SKIP_VERIFY", false),
		}
	}

	return BrokerConfig{
		Brokers:   brokerList,
		Mechanism: mechanism,
		Auth:      auth,
	}
}

// ConsumerConfigFromEnvModule provides Kafka consumer config from environment variables.
// Environment variables:
//   - KAFKA_TOPIC: Topic name to consume from
//   - KAFKA_CONSUMER_GROUP: Consumer group ID
//   - KAFKA_OFFSET: Starting offset - "latest" or "earliest" (default: "latest")
//   - KAFKA_DLT_TOPIC: Dead letter topic name
//   - KAFKA_MAX_RETRIES: Maximum retry attempts (default: 3)
//   - KAFKA_ENABLE_RETRY: Enable retry functionality (default: false)
//   - KAFKA_RETRY_CHAN_SIZE: Retry channel size (default: 100)
//   - KAFKA_WORKER_COUNT: Number of worker goroutines (default: 1)
var ConsumerConfigFromEnvModule = fx.Provide(ConsumerConfigFromEnv)

// ConsumerConfigFromEnv creates Kafka consumer config from environment variables.
func ConsumerConfigFromEnv() ConsumerConfig {
	offset := kafka.LastOffset
	if getEnv("KAFKA_OFFSET", "latest") == "earliest" {
		offset = kafka.FirstOffset
	}

	return ConsumerConfig{
		TopicName:       getEnv("KAFKA_TOPIC", ""),
		ConsumerGroupID: getEnv("KAFKA_CONSUMER_GROUP", ""),
		Offset:          offset,
		TopicNameDLT:    getEnv("KAFKA_DLT_TOPIC", ""),
		MaxRetries:      getEnvInt("KAFKA_MAX_RETRIES", 3),
		EnableRetry:     getEnvBool("KAFKA_ENABLE_RETRY", false),
		RetryChanSize:   getEnvInt("KAFKA_RETRY_CHAN_SIZE", 100),
		WorkerCount:     getEnvInt("KAFKA_WORKER_COUNT", 1),
	}
}

func parseMechanism(s string) vos.Mechanism {
	switch strings.ToLower(s) {
	case "plain":
		return vos.Plain
	case "scram":
		return vos.Scram
	default:
		return vos.PlainText
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

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}
