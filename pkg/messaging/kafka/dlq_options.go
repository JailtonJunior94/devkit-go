package kafka

import "time"

// WithDLQEnabled enables or disables the Dead Letter Queue.
func WithDLQEnabled(enabled bool) Option {
	return func(c *config) {
		c.dlqConfig.Enabled = enabled
	}
}

// WithDLQTopic sets the topic name for the Dead Letter Queue.
func WithDLQTopic(topic string) Option {
	return func(c *config) {
		if topic != "" {
			c.dlqConfig.Topic = topic
			c.dlqConfig.Enabled = true
		}
	}
}

// WithDLQMaxRetries sets the maximum number of retries before sending to DLQ.
func WithDLQMaxRetries(maxRetries int) Option {
	return func(c *config) {
		if maxRetries >= 0 {
			c.dlqConfig.MaxRetries = maxRetries
		}
	}
}

// WithDLQRetryBackoff sets the base backoff duration for DLQ retries.
func WithDLQRetryBackoff(backoff time.Duration) Option {
	return func(c *config) {
		if backoff > 0 {
			c.dlqConfig.RetryBackoff = backoff
		}
	}
}

// WithDLQMaxRetryBackoff sets the maximum backoff duration for DLQ retries.
func WithDLQMaxRetryBackoff(maxBackoff time.Duration) Option {
	return func(c *config) {
		if maxBackoff > 0 {
			c.dlqConfig.MaxRetryBackoff = maxBackoff
		}
	}
}

// WithDLQServiceName sets the service name for DLQ messages.
func WithDLQServiceName(serviceName string) Option {
	return func(c *config) {
		if serviceName != "" {
			c.dlqConfig.ServiceName = serviceName
		}
	}
}

// WithDLQEnvironment sets the environment identifier for DLQ messages.
func WithDLQEnvironment(environment string) Option {
	return func(c *config) {
		if environment != "" {
			c.dlqConfig.Environment = environment
		}
	}
}

// WithDLQIncludeStackTrace enables or disables stack trace inclusion in DLQ messages.
func WithDLQIncludeStackTrace(include bool) Option {
	return func(c *config) {
		c.dlqConfig.IncludeStackTrace = include
	}
}

// WithDLQStrategy sets a custom DLQ strategy.
func WithDLQStrategy(strategy DLQStrategy) Option {
	return func(c *config) {
		if strategy != nil {
			c.dlqConfig.Strategy = strategy
		}
	}
}

// ConsumerWithDLQEnabled enables DLQ for a specific consumer.
func ConsumerWithDLQEnabled(enabled bool) ConsumerOption {
	return func(c *consumerConfig) {
		c.dlqEnabled = enabled
	}
}

// ConsumerWithDLQTopic sets the DLQ topic for a specific consumer.
func ConsumerWithDLQTopic(topic string) ConsumerOption {
	return func(c *consumerConfig) {
		if topic != "" {
			c.dlqTopic = topic
			c.dlqEnabled = true
		}
	}
}
