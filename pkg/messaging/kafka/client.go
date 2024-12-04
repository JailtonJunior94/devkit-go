package kafka

import "github.com/IBM/sarama"

type Client struct {
	client sarama.Client
}

func NewClient(brokers []string) (*Client, error) {
	config := sarama.NewConfig()
	config.Version = sarama.V3_6_0_0

	/* Config Producer */
	config.Producer.Return.Errors = true
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForLocal

	/* Config Consumer */
	config.Consumer.Offsets.Initial = sarama.OffsetNewest
	config.Consumer.Offsets.AutoCommit.Enable = false

	client, err := sarama.NewClient(brokers, config)
	if err != nil {
		return nil, err
	}
	return &Client{client: client}, nil
}
