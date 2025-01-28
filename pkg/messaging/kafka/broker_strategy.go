package kafka

import (
	"crypto/tls"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
)

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

	dialer := &kafka.Dialer{
		SASLMechanism: mechanism,
		Timeout:       10 * time.Second,
		DualStack:     true,
		TLS: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	return dialer, nil
}

type SCRAM struct{}

func (s *SCRAM) Configure(authConfig *AuthConfig) (*kafka.Dialer, error) {
	mechanism, err := scram.Mechanism(scram.SHA512, authConfig.Username, authConfig.Password)
	if err != nil {
		return nil, err
	}

	return &kafka.Dialer{
		SASLMechanism: mechanism,
		Timeout:       10 * time.Second,
		DualStack:     true,
		TLS: &tls.Config{
			InsecureSkipVerify: true,
		},
	}, nil
}
