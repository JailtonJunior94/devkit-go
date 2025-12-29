package kafka

import "errors"

var (
	// ErrClientNotConnected indicates the client is not connected to Kafka.
	ErrClientNotConnected = errors.New("kafka client is not connected")

	// ErrClientAlreadyConnected indicates the client is already connected.
	ErrClientAlreadyConnected = errors.New("kafka client is already connected")

	// ErrClientClosed indicates the client has been closed.
	ErrClientClosed = errors.New("kafka client is closed")

	// ErrInvalidBrokers indicates no brokers were provided.
	ErrInvalidBrokers = errors.New("at least one broker address is required")

	// ErrInvalidAuthStrategy indicates an unsupported authentication strategy.
	ErrInvalidAuthStrategy = errors.New("invalid authentication strategy")

	// ErrConnectionFailed indicates connection to Kafka failed.
	ErrConnectionFailed = errors.New("failed to connect to kafka")

	// ErrHealthCheckFailed indicates health check failed.
	ErrHealthCheckFailed = errors.New("kafka health check failed")

	// ErrProducerClosed indicates the producer has been closed.
	ErrProducerClosed = errors.New("kafka producer is closed")

	// ErrConsumerClosed indicates the consumer has been closed.
	ErrConsumerClosed = errors.New("kafka consumer is closed")

	// ErrNoHandler indicates no handler was registered for an event type.
	ErrNoHandler = errors.New("no handler found for event type")

	// ErrMaxRetriesExceeded indicates maximum retry attempts were exceeded.
	ErrMaxRetriesExceeded = errors.New("maximum retry attempts exceeded")

	// ErrPublishFailed indicates message publication failed.
	ErrPublishFailed = errors.New("failed to publish message to kafka")

	// ErrConsumeFailed indicates message consumption failed.
	ErrConsumeFailed = errors.New("failed to consume message from kafka")
)
