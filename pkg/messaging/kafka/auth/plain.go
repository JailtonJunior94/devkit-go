package auth

import (
	"crypto/tls"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
)

// Plain implements Strategy for PLAIN SASL authentication.
type Plain struct{}

// Configure creates a dialer with PLAIN SASL authentication.
func (p *Plain) Configure(config *Config) (*kafka.Dialer, error) {
	mechanism := plain.Mechanism{
		Username: config.Username,
		Password: config.Password,
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
func (p *Plain) Name() string {
	return string(StrategyPlain)
}
