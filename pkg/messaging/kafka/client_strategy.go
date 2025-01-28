package kafka

import (
	"crypto/tls"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
)

type ClientStrategy interface {
	Configure(authConfig *AuthConfig) *kafka.Dialer
}

type PlainText struct{}

func (p *PlainText) Configure(authConfig *AuthConfig) *kafka.Dialer {
	return &kafka.Dialer{
		DualStack: true,
		Timeout:   10 * time.Second,
		TLS: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
}

type Plain struct{}

func (p *Plain) Configure(authConfig *AuthConfig) *kafka.Dialer {
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

	return dialer
}

type SCRAM struct{}

func (s *SCRAM) Configure(authConfig *AuthConfig) *kafka.Dialer {
	mechanism, err := scram.Mechanism(scram.SHA512, authConfig.Username, authConfig.Password)
	if err != nil {
		panic(err)
	}

	return &kafka.Dialer{
		SASLMechanism: mechanism,
		Timeout:       10 * time.Second,
		DualStack:     true,
		TLS: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
}
