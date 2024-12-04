package kafka

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging"

	"github.com/stretchr/testify/suite"
)

type PublisherSuite struct {
	suite.Suite

	brokers        []string
	ctx            context.Context
	kafkaContainer *KafkaContainer
	publisher      messaging.Publisher
}

func TestPublisherSuite(t *testing.T) {
	suite.Run(t, new(PublisherSuite))
}

func (s *PublisherSuite) SetupTest() {
	s.ctx = context.Background()
	s.kafkaContainer = SetupKafka(s.T())
	s.brokers = s.kafkaContainer.Brokers
	client, err := NewClient(s.brokers)
	s.Require().NoError(err)

	buider, err := NewKafkaBuilder(client)
	s.Require().NoError(err)

	err = buider.DeclareTopics(NewTopicConfig("orders", 1, 1)).Build()
	s.Require().NoError(err)

	s.publisher, err = NewPublisher(client)
	s.Require().NoError(err)
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

			err = s.publisher.Publish(s.ctx, "orders", "", map[string]string{
				"content_type": "application/json",
				"event_type":   "order.created",
			}, &messaging.Message{
				Body: json,
			})

			scenario.expected(err)
		})
	}
}
