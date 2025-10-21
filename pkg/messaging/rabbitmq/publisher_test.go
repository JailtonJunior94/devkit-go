package rabbitmq_test

import (
	"context"
	"encoding/json"
	"log"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging"
	"github.com/JailtonJunior94/devkit-go/pkg/messaging/rabbitmq"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/suite"
)

type PublisherSuite struct {
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

func TestPublisherSuite(t *testing.T) {
	suite.Run(t, new(PublisherSuite))
}

func (s *PublisherSuite) SetupTest() {
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
}

func (s *PublisherSuite) TearDownTest() {
	s.connection.Close()
	s.channel.Close()
	s.rabbitMQContainer.Teardown(s.T())
}

func (s *PublisherSuite) TestPublish() {
	type args struct {
		message any
	}

	type order struct {
		ID     int     `json:"id"`
		Status string  `json:"status"`
		Value  float64 `json:"value"`
	}

	scenarios := []struct {
		name     string
		args     args
		expected func(err error)
	}{
		{
			name: "should publish message",
			args: args{
				message: &order{ID: 1, Status: "created", Value: 100.0},
			},
			expected: func(err error) {
				s.NoError(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.T().Run(scenario.name, func(t *testing.T) {
			json, err := json.Marshal(scenario.args.message)
			s.Require().NoError(err)

			err = s.publisher.Publish(s.ctx, s.orderQueue, s.orderCreated, map[string]string{
				"content_type": "application/json",
				"event_type":   s.orderCreated,
			}, &messaging.Message{
				Body: json,
			})

			scenario.expected(err)
		})
	}
}
