package kafka

import (
	"time"

	"github.com/IBM/sarama"
)

type Client struct {
	client sarama.Client
}

func NewClient(brokers []string) (*Client, error) {
	config := sarama.NewConfig()
	config.Version = sarama.V3_6_0_0

	/* Config Producer */
	config.Producer.Return.Errors = true
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 10
	config.Producer.Retry.Backoff = 100 * time.Millisecond
	config.Producer.Flush.Bytes = 1048576 // 1MB
	config.Producer.Flush.Messages = 1000
	config.Producer.Flush.Frequency = 500 * time.Millisecond
	config.Producer.Compression = sarama.CompressionSnappy

	/* Config Consumer */
	config.Consumer.Offsets.Initial = sarama.OffsetNewest
	config.Consumer.Offsets.AutoCommit.Enable = false
	config.Consumer.Offsets.AutoCommit.Interval = 1 * time.Second
	config.Consumer.Group.Session.Timeout = 10 * time.Second
	config.Consumer.Group.Heartbeat.Interval = 3 * time.Second
	config.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()
	config.Consumer.Fetch.Min = 1
	config.Consumer.Fetch.Default = 10 * 1024 * 1024 // 10MB
	config.Consumer.MaxWaitTime = 250 * time.Millisecond
	config.Consumer.Retry.Backoff = 2 * time.Second

	client, err := sarama.NewClient(brokers, config)
	if err != nil {
		return nil, err
	}
	return &Client{client: client}, nil
}
