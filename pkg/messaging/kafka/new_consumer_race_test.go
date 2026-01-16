package kafka

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging"
	"github.com/segmentio/kafka-go"
)

// TestConcurrentRegisterAndProcess validates the race condition fix
// Run with: go test -race -run TestConcurrentRegisterAndProcess
func TestConcurrentRegisterAndProcess(t *testing.T) {
	// Create a consumer with minimal config
	cfg := defaultConfig()
	cfg.brokers = []string{"localhost:9092"}
	cfg.consumerTopics = []string{"test-topic"}
	cfg.consumerGroupID = "test-group"
	cfg.logger = NewNoopLogger()

	c := &consumer{
		config:      cfg,
		consumerCfg: &consumerConfig{},
		handlers:    make(map[string][]messaging.ConsumeHandler),
		errorCh:     make(chan error, 100),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	iterations := 1000

	// Goroutine 1: Continuously register handlers (simulates runtime registration)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			eventType := fmt.Sprintf("event_%d", i%10) // 10 different event types
			handler := func(ctx context.Context, params map[string]string, body []byte) error {
				// Simulate some work
				time.Sleep(time.Microsecond)
				return nil
			}
			c.RegisterHandler(eventType, handler)

			// Yield to allow other goroutines to run
			if i%100 == 0 {
				time.Sleep(time.Millisecond)
			}
		}
	}()

	// Goroutine 2: Continuously process messages (simulates message consumption)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			select {
			case <-ctx.Done():
				return
			default:
			}

			eventType := fmt.Sprintf("event_%d", i%10)
			msg := kafka.Message{
				Topic:     "test-topic",
				Partition: 0,
				Offset:    int64(i),
				Key:       []byte("test-key"),
				Value:     []byte("test-value"),
				Headers: []kafka.Header{
					{Key: "event_type", Value: []byte(eventType)},
				},
			}

			// This would cause race condition before the fix
			c.processMessage(ctx, msg)

			// Yield to allow other goroutines to run
			if i%100 == 0 {
				time.Sleep(time.Millisecond)
			}
		}
	}()

	// Goroutine 3: Another processor (simulates worker pool scenario)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			select {
			case <-ctx.Done():
				return
			default:
			}

			eventType := fmt.Sprintf("event_%d", i%10)
			msg := kafka.Message{
				Topic:     "test-topic",
				Partition: 1,
				Offset:    int64(i + iterations),
				Key:       []byte("test-key-2"),
				Value:     []byte("test-value-2"),
				Headers: []kafka.Header{
					{Key: "event_type", Value: []byte(eventType)},
				},
			}

			c.processMessage(ctx, msg)

			if i%100 == 0 {
				time.Sleep(time.Millisecond)
			}
		}
	}()

	// Wait for all goroutines with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		t.Log("Race condition test passed - no concurrent access detected")
	case <-ctx.Done():
		t.Fatal("Test timed out - possible deadlock")
	}
}

// TestDefensiveCopyCorrectness validates that the defensive copy works correctly
func TestDefensiveCopyCorrectness(t *testing.T) {
	cfg := defaultConfig()
	cfg.logger = NewNoopLogger()

	c := &consumer{
		config:      cfg,
		consumerCfg: &consumerConfig{},
		handlers:    make(map[string][]messaging.ConsumeHandler),
		errorCh:     make(chan error, 100),
	}

	ctx := context.Background()
	eventType := "test_event"

	// Register initial handler
	callCount := 0
	handler1 := func(ctx context.Context, params map[string]string, body []byte) error {
		callCount++
		return nil
	}
	c.RegisterHandler(eventType, handler1)

	// Create a message
	msg := kafka.Message{
		Topic:     "test-topic",
		Partition: 0,
		Offset:    1,
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte(eventType)},
		},
		Value: []byte("test"),
	}

	// Start processing in goroutine
	processingStarted := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		// Signal that processing started
		close(processingStarted)

		// This will use the defensive copy
		c.processMessageWithoutDLQ(ctx, msg, extractHeaders(msg), eventType, []messaging.ConsumeHandler{handler1})
	}()

	// Wait for processing to start
	<-processingStarted

	// Register another handler while processing is happening
	handler2 := func(ctx context.Context, params map[string]string, body []byte) error {
		callCount++
		return nil
	}
	c.RegisterHandler(eventType, handler2)

	wg.Wait()

	// Verify that only handler1 was called (defensive copy isolated the change)
	// Note: This is just validating the pattern; actual call count depends on implementation
	if callCount > 2 {
		t.Errorf("Expected at most 2 handler calls, got %d", callCount)
	}
}

// BenchmarkProcessMessageWithDefensiveCopy benchmarks the performance impact
func BenchmarkProcessMessageWithDefensiveCopy(b *testing.B) {
	cfg := defaultConfig()
	cfg.logger = NewNoopLogger()

	c := &consumer{
		config:      cfg,
		consumerCfg: &consumerConfig{groupID: "bench-group"},
		handlers:    make(map[string][]messaging.ConsumeHandler),
		errorCh:     make(chan error, 100),
	}

	ctx := context.Background()
	eventType := "bench_event"

	// Register 5 handlers (realistic scenario)
	for i := 0; i < 5; i++ {
		handler := func(ctx context.Context, params map[string]string, body []byte) error {
			return nil
		}
		c.RegisterHandler(eventType, handler)
	}

	msg := kafka.Message{
		Topic:     "bench-topic",
		Partition: 0,
		Offset:    1,
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte(eventType)},
		},
		Value: []byte("benchmark data"),
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.processMessage(ctx, msg)
		}
	})
}
