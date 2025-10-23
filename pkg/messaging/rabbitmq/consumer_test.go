package rabbitmq_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging"
	"github.com/JailtonJunior94/devkit-go/pkg/messaging/rabbitmq"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/suite"
)

type ConsumerSuite struct {
	suite.Suite

	orderExchange     string
	orderQueue        string
	orderCreated      string
	orderUpdated      string
	channel           *amqp.Channel
	ctx               context.Context
	connection        *amqp.Connection
	publisher         messaging.Publisher
	rabbitMQContainer *rabbitmq.RabbitMQContainer
}

func TestConsumerSuite(t *testing.T) {
	suite.Run(t, new(ConsumerSuite))
}

func (s *ConsumerSuite) SetupTest() {
	s.ctx = context.Background()
	s.rabbitMQContainer = rabbitmq.SetupRabbitMQ(s.T())
	s.orderExchange = "order"
	s.orderQueue = "order"
	s.orderCreated = "order_created"
	s.orderUpdated = "order_updated"

	var (
		Exchanges = []*rabbitmq.Exchange{
			rabbitmq.NewExchange(s.orderExchange, "direct"),
		}

		Bindings = []*rabbitmq.Binding{
			rabbitmq.NewBindingRouting(s.orderQueue, s.orderExchange, s.orderCreated),
			rabbitmq.NewBindingRouting(s.orderQueue, s.orderExchange, s.orderUpdated),
		}
	)

	s.connection, _ = amqp.Dial(s.rabbitMQContainer.URL)
	s.channel, _ = s.connection.Channel()

	_, err := rabbitmq.NewAmqpBuilder(s.channel).
		DeclareExchanges(Exchanges...).
		DeclareBindings(Bindings...).
		DeclarePrefetchCount(5).
		WithDLQ().
		WithRetry().
		DeclareTTL(3 * time.Second).
		Apply()

	if err != nil {
		log.Fatal(err)
	}

	s.publisher = rabbitmq.NewRabbitMQPublisher(s.channel)

	rawJSON := []byte(``)
	if err := s.publisher.Publish(s.ctx, s.orderQueue, s.orderCreated, map[string]string{"content-type": "application/json"}, &messaging.Message{Body: rawJSON}); err != nil {
		log.Fatal(err)
	}
}

func (s *ConsumerSuite) TearDownTest() {
	s.connection.Close()
	s.channel.Close()
	s.rabbitMQContainer.Teardown(s.T())
}

func (s *ConsumerSuite) TestConsumer() {
	scenarios := []struct {
		name     string
		expected func(msg *Message, err error)
	}{
		{
			name: "should consume message successfully",
			expected: func(msg *Message, err error) {
				s.Require().NoError(err)
				s.Require().Equal("mb.customer.features.updated", msg.Key)
			},
		},
	}

	for _, scenario := range scenarios {
		s.T().Run(scenario.name, func(t *testing.T) {
			consumer, err := rabbitmq.NewConsumer(
				rabbitmq.WithName("order"),
				rabbitmq.WithChannel(s.channel),
				rabbitmq.WithQueue(s.orderQueue),
			)
			s.Require().NoError(err)

			consumer.RegisterHandler(s.orderCreated, func(ctx context.Context, params map[string]string, body []byte) error {
				msg, err := ParseRabbitPayload(body)
				scenario.expected(msg, err)
				return nil
			})

			err = consumer.Consume(s.ctx)
			defer consumer.Close()
			s.Require().NoError(err)
		})
	}
}

type Message struct {
	Key    string `json:"key"`
	Source string `json:"source"`
	Data   struct {
		Feature    string `json:"feature"`
		Status     string `json:"status"`
		AccountID  string `json:"account_id"`
		IdentityID string `json:"identity_id"`
		UserID     int    `json:"user_id"`
		UpdatedAt  string `json:"updated_at"`
	} `json:"data"`
}

func ParseRabbitPayload(body []byte) (*Message, error) {
	var outer []json.RawMessage
	if err := json.Unmarshal(body, &outer); err != nil {
		return nil, fmt.Errorf("error decode envelop: %v", err)
	}

	if len(outer) == 0 {
		return nil, fmt.Errorf("payload empty")
	}

	var inner []Message
	if err := json.Unmarshal(outer[0], &inner); err != nil {
		return nil, fmt.Errorf("error decode message: %w", err)
	}

	if len(inner) == 0 {
		return nil, fmt.Errorf("no message found")
	}

	return &inner[0], nil
}
