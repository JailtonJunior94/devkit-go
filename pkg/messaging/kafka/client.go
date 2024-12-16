package kafka

import (
	"crypto/sha256"
	"crypto/sha512"
	"time"

	"github.com/IBM/sarama"
	"github.com/xdg-go/scram"
)

type (
	Client struct {
		client sarama.Client
	}
	AuthConfig struct {
		Username string
		Password string
	}
)

func NewClient(brokers []string, authConfig *AuthConfig) (*Client, error) {
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

	// Configure authentication if authConfig is provided
	if authConfig != nil {
		config.Net.SASL.Enable = true
		config.Net.SASL.Handshake = true
		config.Net.SASL.User = authConfig.Username
		config.Net.SASL.Password = authConfig.Password
		config.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
		config.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: SHA512} }
	}

	client, err := sarama.NewClient(brokers, config)
	if err != nil {
		return nil, err
	}
	return &Client{client: client}, nil
}

var (
	SHA256 scram.HashGeneratorFcn = sha256.New
	SHA512 scram.HashGeneratorFcn = sha512.New
)

type XDGSCRAMClient struct {
	*scram.Client
	*scram.ClientConversation
	scram.HashGeneratorFcn
}

func (x *XDGSCRAMClient) Begin(userName, password, authzID string) (err error) {
	x.Client, err = x.HashGeneratorFcn.NewClient(userName, password, authzID)
	if err != nil {
		return err
	}
	x.ClientConversation = x.Client.NewConversation()
	return nil
}

func (x *XDGSCRAMClient) Step(challenge string) (response string, err error) {
	response, err = x.ClientConversation.Step(challenge)
	return
}

func (x *XDGSCRAMClient) Done() bool {
	return x.ClientConversation.Done()
}
