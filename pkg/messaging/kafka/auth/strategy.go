package auth

import (
	"crypto/tls"

	"github.com/segmentio/kafka-go"
)

// Strategy defines the interface for Kafka authentication strategies.
// Each strategy implements a different authentication mechanism (Plain, SCRAM, etc.).
type Strategy interface {
	// Configure configures and returns a Kafka dialer with the appropriate authentication.
	Configure(config *Config) (*kafka.Dialer, error)
	// Name returns the name of the authentication strategy.
	Name() string
}

// Config holds the authentication configuration for Kafka connections.
type Config struct {
	// Username for SASL authentication.
	Username string
	// Password for SASL authentication.
	Password string
	// TLSConfig allows custom TLS configuration. If nil, a secure default is used.
	TLSConfig *tls.Config
	// InsecureSkipVerify disables TLS certificate verification.
	// WARNING: Only use in development environments. Never use in production.
	InsecureSkipVerify bool
	// Algorithm specifies the SCRAM algorithm (SHA256 or SHA512).
	// Only used for SCRAM authentication.
	Algorithm ScramAlgorithm
}

// ScramAlgorithm represents the SCRAM hashing algorithm.
type ScramAlgorithm string

const (
	// ScramSHA256 uses SCRAM-SHA-256 authentication.
	ScramSHA256 ScramAlgorithm = "SCRAM-SHA-256"
	// ScramSHA512 uses SCRAM-SHA-512 authentication.
	ScramSHA512 ScramAlgorithm = "SCRAM-SHA-512"
)

// NewStrategy returns the appropriate authentication strategy based on the type.
// If no type is provided, returns Confluent as the default (recommended for production).
//
// Por quê: Confluent é o padrão mais seguro e amplamente usado em produção,
// com suporte a SASL_SSL e SCRAM-SHA-512 out-of-the-box.
func NewStrategy(strategyType StrategyType) Strategy {
	switch strategyType {
	case StrategyPlaintext:
		return &Plaintext{}
	case StrategyPlain:
		return &Plain{}
	case StrategyScram:
		return &Scram{}
	case StrategyConfluent:
		return &Confluent{}
	default:
		// Default para Confluent (mais seguro para produção)
		return &Confluent{}
	}
}

// StrategyType represents the type of authentication strategy.
type StrategyType string

const (
	// StrategyPlaintext uses no authentication (insecure).
	// WARNING: Only for local development. Never use in production.
	StrategyPlaintext StrategyType = "PLAINTEXT"

	// StrategyPlain uses PLAIN SASL authentication with TLS.
	// Suitable for Kafka clusters with SASL/PLAIN enabled.
	StrategyPlain StrategyType = "PLAIN"

	// StrategyScram uses SCRAM SASL authentication (SHA-256 or SHA-512).
	// More secure than PLAIN, suitable for self-managed Kafka clusters.
	StrategyScram StrategyType = "SCRAM"

	// StrategyConfluent uses Confluent Cloud authentication (SASL_SSL + SCRAM).
	// RECOMMENDED: Default strategy for production environments.
	// Compatible with Confluent Cloud and Confluent Platform.
	StrategyConfluent StrategyType = "CONFLUENT"
)
