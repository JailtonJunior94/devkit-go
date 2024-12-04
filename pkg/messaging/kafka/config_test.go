package kafka

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ConfigSuite struct {
	suite.Suite

	ctx            context.Context
	brokers        []string
	kafkaContainer *KafkaContainer
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigSuite))
}

func (s *ConfigSuite) SetupTest() {
	s.ctx = context.Background()
	s.kafkaContainer = SetupKafka(s.T())
	s.brokers = s.kafkaContainer.Brokers
}

func (s *ConfigSuite) TearDownTest() {
	s.kafkaContainer.Teardown(s.T())
}

func (s *ConfigSuite) TestCreateTopics() {
	client, err := NewClient(s.brokers)
	s.Require().NoError(err)

	buider, err := NewKafkaBuilder(client)
	s.Require().NoError(err)

	err = buider.DeclareTopics(
		NewTopicConfig("test", 1, 1),
		NewTopicConfig("test2", 1, 1),
	).Build()
	s.Require().NoError(err)

	topicOne, _ := buider.Topic("test")
	topicTwo, _ := buider.Topic("test2")

	s.Assert().Equal("test", topicOne)
	s.Assert().Equal("test2", topicTwo)
}
