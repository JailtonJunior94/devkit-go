package rabbitmq

import (
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
)

func newTestObservability() observability.Observability {
	return noop.NewProvider()
}

func TestGetRetryCountFromXRetryCountHeader(t *testing.T) {
	tests := []struct {
		name     string
		headers  amqp.Table
		expected int
	}{
		{
			name:     "no header returns zero",
			headers:  amqp.Table{},
			expected: 0,
		},
		{
			name:     "int32 header",
			headers:  amqp.Table{"x-retry-count": int32(3)},
			expected: 3,
		},
		{
			name:     "int64 header",
			headers:  amqp.Table{"x-retry-count": int64(5)},
			expected: 5,
		},
		{
			name:     "int header",
			headers:  amqp.Table{"x-retry-count": int(2)},
			expected: 2,
		},
		{
			name:     "wrong type returns zero",
			headers:  amqp.Table{"x-retry-count": "bad"},
			expected: 0,
		},
		{
			name:     "x-death header ignored",
			headers:  amqp.Table{"x-death": []any{"entry1", "entry2", "entry3"}},
			expected: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			delivery := amqp.Delivery{Headers: tc.headers}
			got := getRetryCount(delivery)
			require.Equal(t, tc.expected, got)
		})
	}
}

func TestNewConsumerCheckedValidatesQueue(t *testing.T) {
	client := &Client{
		config:        DefaultConfig(),
		observability: newTestObservability(),
	}

	_, err := NewConsumerChecked(client)
	require.Error(t, err)
	require.Contains(t, err.Error(), "queue name is required")
}

func TestNewConsumerCheckedValidatesPrefetchCount(t *testing.T) {
	client := &Client{
		config:        DefaultConfig(),
		observability: newTestObservability(),
	}

	_, err := NewConsumerChecked(client,
		WithQueue("my-queue"),
		WithPrefetchCount(-1),
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "prefetch count must be non-negative")
}

func TestNewConsumerCheckedNormalizesWorkers(t *testing.T) {
	client := &Client{
		config:        DefaultConfig(),
		observability: newTestObservability(),
	}

	c, err := NewConsumerChecked(client,
		WithQueue("my-queue"),
		WithWorkerPool(0),
	)
	require.NoError(t, err)
	require.Equal(t, 1, c.workers)
}

func TestNewConsumerCheckedSucceeds(t *testing.T) {
	client := &Client{
		config:        DefaultConfig(),
		observability: newTestObservability(),
	}

	c, err := NewConsumerChecked(client,
		WithQueue("my-queue"),
		WithPrefetchCount(10),
		WithWorkerPool(3),
	)
	require.NoError(t, err)
	require.Equal(t, "my-queue", c.queue)
	require.Equal(t, 3, c.workers)
	require.Equal(t, 10, c.prefetchCount)
}
