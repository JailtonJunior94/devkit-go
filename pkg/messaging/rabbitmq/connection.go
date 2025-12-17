package rabbitmq

import (
	"fmt"
	"log"
	"time"

	"github.com/cenkalti/backoff/v4"
	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQConnection struct {
	Connection *amqp.Connection
	Channel    *amqp.Channel
}

func NewConnection(url string) (*RabbitMQConnection, func() error, func() error, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("connection: error to create connection: %v", err)
	}

	shutdownConn := func() error {
		if err := conn.Close(); err != nil {
			return err
		}
		return nil
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("channel: error to create channel: %v", err)
	}

	shutdownCh := func() error {
		if err := ch.Close(); err != nil {
			return err
		}
		return nil
	}

	return &RabbitMQConnection{Connection: conn, Channel: ch}, shutdownConn, shutdownCh, nil
}

func NewConnectWithRetry(url string) (*RabbitMQConnection, func() error, func() error, error) {
	backoffConfig := backoff.NewExponentialBackOff()
	backoffConfig.InitialInterval = 1 * time.Second
	backoffConfig.MaxInterval = 30 * time.Second
	backoffConfig.MaxElapsedTime = 5 * time.Minute

	var connection *RabbitMQConnection
	var shutdown, shutdownCh func() error

	operation := func() error {
		conn, shutdownFunc, shutdownChFunc, err := NewConnection(url)
		if err != nil {
			log.Printf("failed to connect to RabbitMQ: %v", err)
			return err
		}
		connection = conn
		shutdown = shutdownFunc
		shutdownCh = shutdownChFunc
		return nil
	}

	err := backoff.Retry(operation, backoffConfig)
	return connection, shutdown, shutdownCh, err
}

func Cleanup(shutdown, shutdownCh func() error) {
	if shutdownCh != nil {
		if err := shutdownCh(); err != nil {
			log.Printf("Error closing channel: %v", err)
		}
	}
	if shutdown != nil {
		if err := shutdown(); err != nil {
			log.Printf("Error closing connection: %v", err)
		}
	}
}
