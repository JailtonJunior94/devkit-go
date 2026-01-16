package kafka

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging"
	"github.com/segmentio/kafka-go"
)

// TestErrorChannelNoLeak validates that sendError() doesn't block when channel is full
// Run with: go test -run TestErrorChannelNoLeak -timeout 30s
func TestErrorChannelNoLeak(t *testing.T) {
	cfg := defaultConfig()
	cfg.logger = NewNoopLogger()

	c := &consumer{
		config:             cfg,
		consumerCfg:        &consumerConfig{},
		handlers:           make(map[string][]messaging.ConsumeHandler),
		errorCh:            make(chan error, 10), // Small buffer to force overflow
		monitoringShutdown: make(chan struct{}),
	}

	// Start monitoring (optional, just to test it doesn't interfere)
	c.startErrorChannelMonitoring()
	defer c.stopErrorChannelMonitoring()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Send more errors than the buffer can hold
	numErrors := 1000
	done := make(chan struct{})

	go func() {
		for i := 0; i < numErrors; i++ {
			// This should NEVER block even when channel is full
			c.sendError(ErrMaxRetriesExceeded)
		}
		close(done)
	}()

	// Wait for completion or timeout
	select {
	case <-done:
		t.Logf("✅ Successfully sent %d errors without blocking", numErrors)
	case <-ctx.Done():
		t.Fatal("❌ MEMORY LEAK: sendError() blocked when channel was full")
	}

	// Verify dropped errors were counted
	dropped := c.droppedErrors.Load()
	if dropped == 0 {
		t.Errorf("Expected some errors to be dropped, got 0")
	}

	t.Logf("Dropped errors: %d (expected since channel was full)", dropped)

	// Verify channel didn't overflow
	if len(c.errorCh) > cap(c.errorCh) {
		t.Errorf("Channel overflow: len=%d, cap=%d", len(c.errorCh), cap(c.errorCh))
	}
}

// TestErrorChannelMonitoring validates the monitoring goroutine
func TestErrorChannelMonitoring(t *testing.T) {
	cfg := defaultConfig()
	cfg.logger = NewNoopLogger()

	c := &consumer{
		config:             cfg,
		consumerCfg:        &consumerConfig{},
		handlers:           make(map[string][]messaging.ConsumeHandler),
		errorCh:            make(chan error, 100),
		monitoringShutdown: make(chan struct{}),
	}

	// Start monitoring
	c.startErrorChannelMonitoring()

	// Let it run for a bit
	time.Sleep(100 * time.Millisecond)

	// Stop monitoring
	stopDone := make(chan struct{})
	go func() {
		c.stopErrorChannelMonitoring()
		close(stopDone)
	}()

	// Verify it stops within timeout
	select {
	case <-stopDone:
		t.Log("✅ Monitoring stopped gracefully")
	case <-time.After(6 * time.Second):
		t.Fatal("❌ Monitoring goroutine didn't stop within timeout")
	}

	// Verify shutdown channel was closed
	select {
	case <-c.monitoringShutdown:
		t.Log("✅ Shutdown channel closed")
	default:
		t.Error("❌ Shutdown channel not closed")
	}
}

// TestDroppedErrorsCounter validates the dropped errors counter
func TestDroppedErrorsCounter(t *testing.T) {
	cfg := defaultConfig()
	cfg.logger = NewNoopLogger()

	c := &consumer{
		config:             cfg,
		consumerCfg:        &consumerConfig{},
		handlers:           make(map[string][]messaging.ConsumeHandler),
		errorCh:            make(chan error, 5),
		monitoringShutdown: make(chan struct{}),
	}

	// Fill the channel completely
	for i := 0; i < cap(c.errorCh); i++ {
		c.errorCh <- ErrMaxRetriesExceeded
	}

	// These should be dropped
	for i := 0; i < 10; i++ {
		c.sendError(ErrMaxRetriesExceeded)
	}

	// Verify count
	dropped := c.DroppedErrors()
	if dropped != 10 {
		t.Errorf("Expected 10 dropped errors, got %d", dropped)
	}

	t.Logf("✅ Correctly counted %d dropped errors", dropped)
}

// TestErrorChannelRateLimitedLogging validates that dropped error logging is rate-limited
func TestErrorChannelRateLimitedLogging(t *testing.T) {
	cfg := defaultConfig()
	logCalls := 0
	cfg.logger = &testLogger{
		warnFunc: func(ctx context.Context, msg string, fields ...Field) {
			if msg == "error channel full, dropping errors - consume from Errors() channel to prevent loss" {
				logCalls++
			}
		},
	}

	c := &consumer{
		config:             cfg,
		consumerCfg:        &consumerConfig{},
		handlers:           make(map[string][]messaging.ConsumeHandler),
		errorCh:            make(chan error, 5),
		monitoringShutdown: make(chan struct{}),
	}

	// Fill the channel
	for i := 0; i < cap(c.errorCh); i++ {
		c.errorCh <- ErrMaxRetriesExceeded
	}

	// Send 100 errors in quick succession
	for i := 0; i < 100; i++ {
		c.sendError(ErrMaxRetriesExceeded)
	}

	// Should only log once due to rate limiting (10 second window)
	if logCalls > 1 {
		t.Errorf("Expected at most 1 warning log, got %d (rate limiting not working)", logCalls)
	}

	t.Logf("✅ Rate limiting working: %d log calls for 100 dropped errors", logCalls)
}

// TestConsumerCloseLogsDroppedErrors validates that Close() logs dropped errors
func TestConsumerCloseLogsDroppedErrors(t *testing.T) {
	cfg := defaultConfig()
	loggedDroppedCount := false
	cfg.logger = &testLogger{
		warnFunc: func(ctx context.Context, msg string, fields ...Field) {
			if msg == "consumer closed with dropped errors - configure error consumption to prevent loss" {
				loggedDroppedCount = true
			}
		},
	}

	c := &consumer{
		config:             cfg,
		consumerCfg:        &consumerConfig{groupID: "test-group"},
		handlers:           make(map[string][]messaging.ConsumeHandler),
		errorCh:            make(chan error, 5),
		monitoringShutdown: make(chan struct{}),
	}

	// Create a fake reader to avoid nil pointer
	c.reader = kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{"localhost:9092"},
		GroupID: "test",
		Topic:   "test",
	})

	// Start monitoring
	c.startErrorChannelMonitoring()

	// Fill channel and drop some errors
	for i := 0; i < cap(c.errorCh); i++ {
		c.errorCh <- ErrMaxRetriesExceeded
	}
	for i := 0; i < 10; i++ {
		c.sendError(ErrMaxRetriesExceeded)
	}

	// Close consumer
	_ = c.Close()

	if !loggedDroppedCount {
		t.Error("❌ Close() did not log dropped error count")
	} else {
		t.Log("✅ Close() correctly logged dropped errors")
	}
}

// TestErrorChannelHealthCheck validates the health check logic
func TestErrorChannelHealthCheck(t *testing.T) {
	cfg := defaultConfig()
	warnLogged := false
	debugLogged := false

	cfg.logger = &testLogger{
		warnFunc: func(ctx context.Context, msg string, fields ...Field) {
			if msg == "error channel approaching capacity - consume from Errors() to prevent data loss" {
				warnLogged = true
			}
		},
		debugFunc: func(ctx context.Context, msg string, fields ...Field) {
			if msg == "error channel has buffered errors" {
				debugLogged = true
			}
		},
	}

	c := &consumer{
		config:             cfg,
		consumerCfg:        &consumerConfig{},
		handlers:           make(map[string][]messaging.ConsumeHandler),
		errorCh:            make(chan error, 1000),
		monitoringShutdown: make(chan struct{}),
	}

	// Test 1: Channel below threshold - should log debug
	for i := 0; i < 100; i++ {
		c.errorCh <- ErrMaxRetriesExceeded
	}
	c.checkErrorChannelHealth()

	if !debugLogged {
		t.Error("❌ Expected debug log for low channel usage")
	} else {
		t.Log("✅ Debug logged for buffered errors")
	}

	// Test 2: Channel above threshold - should log warning
	for i := 0; i < 750; i++ { // Total now 850 (>800 threshold)
		c.errorCh <- ErrMaxRetriesExceeded
	}
	c.checkErrorChannelHealth()

	if !warnLogged {
		t.Error("❌ Expected warning log when channel >80% full")
	} else {
		t.Log("✅ Warning logged when channel approaching capacity")
	}
}

// BenchmarkSendErrorNoBlock benchmarks sendError to verify it never blocks
func BenchmarkSendErrorNoBlock(b *testing.B) {
	cfg := defaultConfig()
	cfg.logger = NewNoopLogger()

	c := &consumer{
		config:             cfg,
		consumerCfg:        &consumerConfig{},
		handlers:           make(map[string][]messaging.ConsumeHandler),
		errorCh:            make(chan error, 10), // Small buffer
		monitoringShutdown: make(chan struct{}),
	}

	// Fill the channel
	for i := 0; i < cap(c.errorCh); i++ {
		c.errorCh <- ErrMaxRetriesExceeded
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// This should complete in nanoseconds even with full channel
			c.sendError(ErrMaxRetriesExceeded)
		}
	})

	b.Logf("Dropped errors: %d", c.DroppedErrors())
}

// testLogger is a simple logger for testing
type testLogger struct {
	infoFunc  func(ctx context.Context, msg string, fields ...Field)
	warnFunc  func(ctx context.Context, msg string, fields ...Field)
	errorFunc func(ctx context.Context, msg string, fields ...Field)
	debugFunc func(ctx context.Context, msg string, fields ...Field)
	mu        sync.Mutex
}

func (l *testLogger) Info(ctx context.Context, msg string, fields ...Field) {
	if l.infoFunc != nil {
		l.mu.Lock()
		defer l.mu.Unlock()
		l.infoFunc(ctx, msg, fields...)
	}
}

func (l *testLogger) Warn(ctx context.Context, msg string, fields ...Field) {
	if l.warnFunc != nil {
		l.mu.Lock()
		defer l.mu.Unlock()
		l.warnFunc(ctx, msg, fields...)
	}
}

func (l *testLogger) Error(ctx context.Context, msg string, fields ...Field) {
	if l.errorFunc != nil {
		l.mu.Lock()
		defer l.mu.Unlock()
		l.errorFunc(ctx, msg, fields...)
	}
}

func (l *testLogger) Debug(ctx context.Context, msg string, fields ...Field) {
	if l.debugFunc != nil {
		l.mu.Lock()
		defer l.mu.Unlock()
		l.debugFunc(ctx, msg, fields...)
	}
}
