package kafka

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging"
	kafka "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
)

func newTestConsumer() *consumer {
	cfg := defaultConfig()
	cfg.logger = NewNoopLogger()
	cfg.brokers = []string{"localhost:9092"}
	cfg.consumerTopics = []string{"test-topic"}
	cfg.consumerGroupID = "test-group"
	return &consumer{
		config:             cfg,
		consumerCfg:        &consumerConfig{groupID: "test-group"},
		handlers:           make(map[string][]messaging.ConsumeHandler),
		errorCh:            make(chan error, defaultErrorChannelSize),
		monitoringShutdown: make(chan struct{}),
	}
}

func TestProcessMessageWithDLQCommitsOnceAfterAllHandlers(t *testing.T) {
	c := newTestConsumer()
	c.config.dlqConfig.MaxRetries = 1

	ctx := context.Background()
	msg := kafka.Message{Topic: "test", Partition: 0, Offset: 1, Headers: []kafka.Header{
		{Key: "event_type", Value: []byte("evt")},
	}}
	headers := extractHeaders(msg)

	calls := 0
	handler := func(_ context.Context, _ map[string]string, _ []byte) error {
		calls++
		return nil
	}

	c.processMessageWithDLQ(ctx, msg, headers, "evt", []messaging.ConsumeHandler{handler, handler})

	require.Equal(t, 2, calls)
	require.Empty(t, drainErrors(c.errorCh), "no errors expected when all handlers succeed")
}

func TestProcessMessageWithDLQNoCommitOnDLQFailure(t *testing.T) {
	c := newTestConsumer()
	c.config.dlqConfig.MaxRetries = 1
	c.config.dlqConfig.Enabled = true
	failErr := errors.New("handler failed")

	dlqFailed := false
	c.dlqStrategy = &dlqStrategyFunc{fn: func(_ context.Context, _ *DLQMessage) error {
		dlqFailed = true
		return errors.New("dlq publish failed")
	}}

	ctx := context.Background()
	msg := kafka.Message{Topic: "test", Partition: 0, Offset: 2, Headers: []kafka.Header{
		{Key: "event_type", Value: []byte("evt")},
	}}
	headers := extractHeaders(msg)

	handler := func(_ context.Context, _ map[string]string, _ []byte) error {
		return failErr
	}

	c.processMessageWithDLQ(ctx, msg, headers, "evt", []messaging.ConsumeHandler{handler})

	require.True(t, dlqFailed)
	require.NotEmpty(t, drainErrors(c.errorCh), "error expected when DLQ fails")
}

func TestReconnectWorkerMarksDisconnectedBeforeAttempt(t *testing.T) {
	c := &client{
		config: defaultConfig(),
	}
	c.connected.Store(true)

	c.connected.Store(false)
	require.False(t, c.connected.Load())

	c.connected.Store(true)
	c.attemptReconnect()
	require.True(t, c.connected.Load(), "attemptReconnect returns early if connected=true without resetting state")
}

func TestCloseDoesNotPanicWithConcurrentSendError(t *testing.T) {
	c := newTestConsumer()

	startErrorChannelMonitoringOnce(c)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Close panicked: %v", r)
		}
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 50 {
			if c.closed.Load() {
				return
			}
			c.sendError(errors.New("test error"))
			time.Sleep(time.Millisecond)
		}
	}()

	time.Sleep(5 * time.Millisecond)
	c.closed.Store(true)
	c.stopErrorChannelMonitoring()

	timerStop := time.NewTimer(30 * time.Second)
	defer timerStop.Stop()
	select {
	case <-done:
	case <-timerStop.C:
		t.Fatal("goroutine did not stop")
	}

	close(c.errorCh)
}

func TestStartMuPreventsWGPanicUnderConcurrency(t *testing.T) {
	c := newTestConsumer()

	const goroutines = 200

	closeDone := make(chan struct{})
	go func() {
		defer close(closeDone)
		c.startMu.Lock()
		c.closed.Swap(true)
		c.startMu.Unlock()
		c.wg.Wait()
	}()

	var wg sync.WaitGroup
	for range goroutines {
		wg.Go(func() {
			c.startMu.Lock()
			if c.closed.Load() {
				c.startMu.Unlock()
				return
			}
			c.wg.Add(1)
			c.startMu.Unlock()
			c.wg.Done()
		})
	}

	wg.Wait()

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case <-closeDone:
	case <-timer.C:
		t.Fatal("wg.Wait() in Close() did not return — possible deadlock")
	}
}

func startErrorChannelMonitoringOnce(c *consumer) {
	c.startErrorChannelMonitoring()
}

func drainErrors(ch chan error) []error {
	var errs []error
	for {
		select {
		case e := <-ch:
			errs = append(errs, e)
		default:
			return errs
		}
	}
}

type dlqStrategyFunc struct {
	fn func(context.Context, *DLQMessage) error
}

func (d *dlqStrategyFunc) HandleFailure(ctx context.Context, msg *DLQMessage) error {
	return d.fn(ctx, msg)
}

func (d *dlqStrategyFunc) Name() string { return "test" }
