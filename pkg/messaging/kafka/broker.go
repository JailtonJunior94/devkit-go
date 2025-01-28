package kafka

import (
	"context"
	"errors"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging"
	"github.com/JailtonJunior94/devkit-go/pkg/vos"

	"github.com/segmentio/kafka-go"
)

type Broker interface {
	Close() error
	NewProducerFromBroker() (messaging.Publisher, error)
	NewConsumerFromBroker() (messaging.Consumer, error)
}

type broker struct {
	brokers []string
	conn    *kafka.Conn
	dialer  *kafka.Dialer
}

func NewBroker(ctx context.Context, brokers []string, mechanism vos.Mechanism, authConfig *AuthConfig) (Broker, error) {
	auth := map[vos.Mechanism]ClientStrategy{
		vos.Plain:     &Plain{},
		vos.Scram:     &SCRAM{},
		vos.PlainText: &PlainText{},
	}

	config, exists := auth[mechanism]
	if !exists {
		return nil, errors.New("mechanism not supported")
	}

	dialer := config.Configure(authConfig)
	conn, err := dialer.DialContext(ctx, "tcp", brokers[0])
	if err != nil {
		return nil, err
	}

	return &broker{
		conn:    conn,
		dialer:  dialer,
		brokers: brokers,
	}, nil
}

func (b *broker) Close() error {
	return b.conn.Close()
}
