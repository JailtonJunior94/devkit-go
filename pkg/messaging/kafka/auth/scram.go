package auth

import (
	"crypto/tls"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/scram"
)

// Scram implements Strategy for SCRAM SASL authentication.
// Supports both SCRAM-SHA-256 and SCRAM-SHA-512.
type Scram struct{}

// Configure creates a dialer with SCRAM SASL authentication.
func (s *Scram) Configure(config *Config) (*kafka.Dialer, error) {
	var mechanism sasl.Mechanism
	var err error

	switch config.Algorithm {
	case ScramSHA256:
		mechanism, err = scram.Mechanism(scram.SHA256, config.Username, config.Password)
	case ScramSHA512, "":
		mechanism, err = scram.Mechanism(scram.SHA512, config.Username, config.Password)
	default:
		return nil, fmt.Errorf("unsupported SCRAM algorithm: %s", config.Algorithm)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create SCRAM mechanism: %w", err)
	}

	tlsConfig := config.TLSConfig
	if tlsConfig == nil {
		tlsConfig = &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: config.InsecureSkipVerify,
		}
	}

	return &kafka.Dialer{
		SASLMechanism: mechanism,
		Timeout:       10 * time.Second,
		DualStack:     true,
		TLS:           tlsConfig,
	}, nil
}

// Name returns the strategy name.
func (s *Scram) Name() string {
	return string(StrategyScram)
}
