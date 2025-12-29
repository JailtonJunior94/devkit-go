package auth

import (
	"time"

	"github.com/segmentio/kafka-go"
)

// Plaintext implements Strategy for connections without authentication.
// WARNING: This should only be used in development environments.
type Plaintext struct{}

// Configure creates a dialer without authentication.
func (p *Plaintext) Configure(config *Config) (*kafka.Dialer, error) {
	return &kafka.Dialer{
		Timeout:   10 * time.Second,
		DualStack: true,
	}, nil
}

// Name returns the strategy name.
func (p *Plaintext) Name() string {
	return string(StrategyPlaintext)
}
