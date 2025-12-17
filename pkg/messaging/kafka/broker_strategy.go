package kafka

import (
	"crypto/tls"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
)

// AuthConfig holds the authentication configuration for Kafka connections.
type AuthConfig struct {
	Username string
	Password string
	// TLSConfig allows custom TLS configuration. If nil, a secure default is used.
	TLSConfig *tls.Config
	// InsecureSkipVerify disables TLS certificate verification.
	// WARNING: Only use in development environments. Never use in production.
	InsecureSkipVerify bool
}

type BrokerStrategy interface {
	Configure(authConfig *AuthConfig) (*kafka.Dialer, error)
}

type PlainText struct{}

func (p *PlainText) Configure(authConfig *AuthConfig) (*kafka.Dialer, error) {
	return &kafka.Dialer{
		DualStack: true,
		Timeout:   10 * time.Second,
	}, nil
}

type Plain struct{}

func (p *Plain) Configure(authConfig *AuthConfig) (*kafka.Dialer, error) {
	mechanism := plain.Mechanism{
		Username: authConfig.Username,
		Password: authConfig.Password,
	}

	tlsConfig := authConfig.TLSConfig
	if tlsConfig == nil {
		tlsConfig = &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: authConfig.InsecureSkipVerify,
		}
	}

	dialer := &kafka.Dialer{
		SASLMechanism: mechanism,
		Timeout:       10 * time.Second,
		DualStack:     true,
		TLS:           tlsConfig,
	}

	return dialer, nil
}

type SCRAM struct{}

func (s *SCRAM) Configure(authConfig *AuthConfig) (*kafka.Dialer, error) {
	mechanism, err := scram.Mechanism(scram.SHA512, authConfig.Username, authConfig.Password)
	if err != nil {
		return nil, err
	}

	tlsConfig := authConfig.TLSConfig
	if tlsConfig == nil {
		tlsConfig = &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: authConfig.InsecureSkipVerify,
		}
	}

	return &kafka.Dialer{
		SASLMechanism: mechanism,
		Timeout:       10 * time.Second,
		DualStack:     true,
		TLS:           tlsConfig,
	}, nil
}
