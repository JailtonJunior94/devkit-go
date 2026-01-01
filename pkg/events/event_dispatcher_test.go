package events

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Mock event implementation
type testEvent struct {
	eventType string
	payload   any
}

func (e *testEvent) GetEventType() string {
	return e.eventType
}

func (e *testEvent) GetPayload() any {
	return e.payload
}

// Mock handler that counts calls
type testHandler struct {
	callCount atomic.Int32
	mu        sync.Mutex
	events    []Event
	err       error
	delay     time.Duration
}

func (h *testHandler) Handle(ctx context.Context, event Event) error {
	if h.delay > 0 {
		select {
		case <-time.After(h.delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	h.callCount.Add(1)
	h.mu.Lock()
	h.events = append(h.events, event)
	h.mu.Unlock()

	return h.err
}

func (h *testHandler) GetCallCount() int {
	return int(h.callCount.Load())
}

func (h *testHandler) GetEvents() []Event {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]Event(nil), h.events...)
}

// ============================================================================
// BASIC FUNCTIONALITY TESTS
// ============================================================================

func TestNewEventDispatcher(t *testing.T) {
	dispatcher := NewEventDispatcher()
	if dispatcher == nil {
		t.Fatal("NewEventDispatcher returned nil")
	}
}

func TestRegister_Success(t *testing.T) {
	dispatcher := NewEventDispatcher()
	handler := &testHandler{}

	err := dispatcher.Register("test.event", handler)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if !dispatcher.Has("test.event", handler) {
		t.Error("Handler was not registered")
	}
}

func TestRegister_MultipleHandlers(t *testing.T) {
	dispatcher := NewEventDispatcher()
	handler1 := &testHandler{}
	handler2 := &testHandler{}
	handler3 := &testHandler{}

	err := dispatcher.Register("test.event", handler1)
	if err != nil {
		t.Fatalf("Register handler1 failed: %v", err)
	}

	err = dispatcher.Register("test.event", handler2)
	if err != nil {
		t.Fatalf("Register handler2 failed: %v", err)
	}

	err = dispatcher.Register("test.event", handler3)
	if err != nil {
		t.Fatalf("Register handler3 failed: %v", err)
	}

	if !dispatcher.Has("test.event", handler1) {
		t.Error("Handler1 not found")
	}
	if !dispatcher.Has("test.event", handler2) {
		t.Error("Handler2 not found")
	}
	if !dispatcher.Has("test.event", handler3) {
		t.Error("Handler3 not found")
	}
}

func TestDispatch_Success(t *testing.T) {
	dispatcher := NewEventDispatcher()
	handler := &testHandler{}

	err := dispatcher.Register("test.event", handler)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	event := &testEvent{
		eventType: "test.event",
		payload:   "test data",
	}

	err = dispatcher.Dispatch(context.Background(), event)
	if err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}

	if handler.GetCallCount() != 1 {
		t.Errorf("Expected handler to be called once, got %d", handler.GetCallCount())
	}
}

func TestDispatch_MultipleHandlers(t *testing.T) {
	dispatcher := NewEventDispatcher()
	handler1 := &testHandler{}
	handler2 := &testHandler{}
	handler3 := &testHandler{}

	dispatcher.Register("test.event", handler1)
	dispatcher.Register("test.event", handler2)
	dispatcher.Register("test.event", handler3)

	event := &testEvent{eventType: "test.event", payload: "data"}

	err := dispatcher.Dispatch(context.Background(), event)
	if err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}

	if handler1.GetCallCount() != 1 {
		t.Errorf("Handler1: expected 1 call, got %d", handler1.GetCallCount())
	}
	if handler2.GetCallCount() != 1 {
		t.Errorf("Handler2: expected 1 call, got %d", handler2.GetCallCount())
	}
	if handler3.GetCallCount() != 1 {
		t.Errorf("Handler3: expected 1 call, got %d", handler3.GetCallCount())
	}
}

func TestDispatch_NoHandlers(t *testing.T) {
	dispatcher := NewEventDispatcher()
	event := &testEvent{eventType: "unknown.event", payload: nil}

	err := dispatcher.Dispatch(context.Background(), event)
	if err != nil {
		t.Errorf("Dispatch with no handlers should not error, got: %v", err)
	}
}

func TestRemove_Success(t *testing.T) {
	dispatcher := NewEventDispatcher()
	handler := &testHandler{}

	dispatcher.Register("test.event", handler)

	if !dispatcher.Has("test.event", handler) {
		t.Fatal("Handler not registered")
	}

	err := dispatcher.Remove("test.event", handler)
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	if dispatcher.Has("test.event", handler) {
		t.Error("Handler still registered after removal")
	}
}

func TestRemove_NotFound(t *testing.T) {
	dispatcher := NewEventDispatcher()
	handler := &testHandler{}

	err := dispatcher.Remove("test.event", handler)
	if err != nil {
		t.Errorf("Remove non-existent handler should not error, got: %v", err)
	}
}

func TestRemove_OnlyFirstOccurrence(t *testing.T) {
	dispatcher := NewEventDispatcher().(*eventDispatcher)
	handler := &testHandler{}

	// Manually add handler twice (bypassing duplicate check for testing)
	dispatcher.mu.Lock()
	dispatcher.handlers["test.event"] = append(dispatcher.handlers["test.event"], handler, handler)
	dispatcher.mu.Unlock()

	err := dispatcher.Remove("test.event", handler)
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Should still have one instance
	if !dispatcher.Has("test.event", handler) {
		t.Error("Second instance should still be present")
	}
}

func TestClear(t *testing.T) {
	dispatcher := NewEventDispatcher()
	handler1 := &testHandler{}
	handler2 := &testHandler{}

	dispatcher.Register("event1", handler1)
	dispatcher.Register("event2", handler2)

	dispatcher.Clear()

	if dispatcher.Has("event1", handler1) {
		t.Error("Handler1 still present after Clear")
	}
	if dispatcher.Has("event2", handler2) {
		t.Error("Handler2 still present after Clear")
	}
}

func TestHas_False(t *testing.T) {
	dispatcher := NewEventDispatcher()
	handler := &testHandler{}

	if dispatcher.Has("test.event", handler) {
		t.Error("Has returned true for non-existent handler")
	}
}

// ============================================================================
// ERROR VALIDATION TESTS
// ============================================================================

func TestRegister_NilHandler(t *testing.T) {
	dispatcher := NewEventDispatcher()

	err := dispatcher.Register("test.event", nil)
	if !errors.Is(err, ErrHandlerNil) {
		t.Errorf("Expected ErrHandlerNil, got %v", err)
	}
}

func TestRegister_EmptyEventType(t *testing.T) {
	dispatcher := NewEventDispatcher()
	handler := &testHandler{}

	err := dispatcher.Register("", handler)
	if !errors.Is(err, ErrEventTypeEmpty) {
		t.Errorf("Expected ErrEventTypeEmpty, got %v", err)
	}
}

func TestRegister_DuplicateHandler(t *testing.T) {
	dispatcher := NewEventDispatcher()
	handler := &testHandler{}

	err := dispatcher.Register("test.event", handler)
	if err != nil {
		t.Fatalf("First registration failed: %v", err)
	}

	err = dispatcher.Register("test.event", handler)
	if !errors.Is(err, ErrHandlerAlreadyRegistered) {
		t.Errorf("Expected ErrHandlerAlreadyRegistered, got %v", err)
	}
}

func TestDispatch_NilEvent(t *testing.T) {
	dispatcher := NewEventDispatcher()

	err := dispatcher.Dispatch(context.Background(), nil)
	if !errors.Is(err, ErrEventNil) {
		t.Errorf("Expected ErrEventNil, got %v", err)
	}
}

func TestDispatch_HandlerError(t *testing.T) {
	dispatcher := NewEventDispatcher()
	expectedErr := errors.New("handler error")
	handler := &testHandler{err: expectedErr}

	dispatcher.Register("test.event", handler)

	event := &testEvent{eventType: "test.event", payload: nil}
	err := dispatcher.Dispatch(context.Background(), event)

	if !errors.Is(err, expectedErr) {
		t.Errorf("Expected handler error to be propagated, got %v", err)
	}
}

func TestDispatch_StopsOnFirstError(t *testing.T) {
	dispatcher := NewEventDispatcher()
	handler1 := &testHandler{}
	handler2 := &testHandler{err: errors.New("error")}
	handler3 := &testHandler{}

	dispatcher.Register("test.event", handler1)
	dispatcher.Register("test.event", handler2)
	dispatcher.Register("test.event", handler3)

	event := &testEvent{eventType: "test.event", payload: nil}
	dispatcher.Dispatch(context.Background(), event)

	if handler1.GetCallCount() != 1 {
		t.Error("Handler1 should be called")
	}
	if handler2.GetCallCount() != 1 {
		t.Error("Handler2 should be called")
	}
	if handler3.GetCallCount() != 0 {
		t.Error("Handler3 should not be called after error")
	}
}

// ============================================================================
// CONTEXT CANCELLATION TESTS
// ============================================================================

func TestDispatch_ContextCancellation(t *testing.T) {
	dispatcher := NewEventDispatcher()
	handler := &testHandler{delay: 100 * time.Millisecond}

	dispatcher.Register("test.event", handler)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	event := &testEvent{eventType: "test.event", payload: nil}
	err := dispatcher.Dispatch(ctx, event)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}
}

func TestDispatch_ContextCancelledBeforeDispatch(t *testing.T) {
	dispatcher := NewEventDispatcher()
	handler := &testHandler{}

	dispatcher.Register("test.event", handler)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	event := &testEvent{eventType: "test.event", payload: nil}
	err := dispatcher.Dispatch(ctx, event)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled, got %v", err)
	}

	if handler.GetCallCount() != 0 {
		t.Error("Handler should not be called with cancelled context")
	}
}

func TestDispatch_ContextCancelledBetweenHandlers(t *testing.T) {
	dispatcher := NewEventDispatcher()
	handler1 := &testHandler{}
	handler2 := &testHandler{delay: 50 * time.Millisecond}
	handler3 := &testHandler{}

	dispatcher.Register("test.event", handler1)
	dispatcher.Register("test.event", handler2)
	dispatcher.Register("test.event", handler3)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	event := &testEvent{eventType: "test.event", payload: nil}
	err := dispatcher.Dispatch(ctx, event)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}

	// Handler1 should complete, handler2 should timeout, handler3 should not run
	if handler1.GetCallCount() != 1 {
		t.Error("Handler1 should complete")
	}
	if handler3.GetCallCount() != 0 {
		t.Error("Handler3 should not be called after timeout")
	}
}

// ============================================================================
// CONCURRENCY AND RACE CONDITION TESTS
// ============================================================================

func TestConcurrentRegister(t *testing.T) {
	dispatcher := NewEventDispatcher()
	var wg sync.WaitGroup

	numGoroutines := 100
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			handler := &testHandler{}
			err := dispatcher.Register("test.event", handler)
			if err != nil {
				t.Errorf("Goroutine %d: Register failed: %v", id, err)
			}
		}(i)
	}

	wg.Wait()
}

func TestConcurrentDispatch(t *testing.T) {
	dispatcher := NewEventDispatcher()
	handler := &testHandler{}
	dispatcher.Register("test.event", handler)

	var wg sync.WaitGroup
	numGoroutines := 100
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			event := &testEvent{eventType: "test.event", payload: "data"}
			err := dispatcher.Dispatch(context.Background(), event)
			if err != nil {
				t.Errorf("Dispatch failed: %v", err)
			}
		}()
	}

	wg.Wait()

	if handler.GetCallCount() != numGoroutines {
		t.Errorf("Expected %d calls, got %d", numGoroutines, handler.GetCallCount())
	}
}

func TestConcurrentRegisterAndDispatch(t *testing.T) {
	dispatcher := NewEventDispatcher()
	var wg sync.WaitGroup

	numOperations := 50
	wg.Add(numOperations * 2)

	// Concurrent registrations
	for i := 0; i < numOperations; i++ {
		go func() {
			defer wg.Done()
			handler := &testHandler{}
			dispatcher.Register("test.event", handler)
		}()
	}

	// Concurrent dispatches
	for i := 0; i < numOperations; i++ {
		go func() {
			defer wg.Done()
			event := &testEvent{eventType: "test.event", payload: "data"}
			dispatcher.Dispatch(context.Background(), event)
		}()
	}

	wg.Wait()
}

func TestConcurrentRegisterRemoveDispatch(t *testing.T) {
	dispatcher := NewEventDispatcher()
	var wg sync.WaitGroup

	handlers := make([]*testHandler, 20)
	for i := range handlers {
		handlers[i] = &testHandler{}
	}

	numOperations := 100
	wg.Add(numOperations * 3)

	// Concurrent registrations
	for i := 0; i < numOperations; i++ {
		go func(idx int) {
			defer wg.Done()
			handler := handlers[idx%len(handlers)]
			dispatcher.Register("test.event", handler)
		}(i)
	}

	// Concurrent removals
	for i := 0; i < numOperations; i++ {
		go func(idx int) {
			defer wg.Done()
			handler := handlers[idx%len(handlers)]
			dispatcher.Remove("test.event", handler)
		}(i)
	}

	// Concurrent dispatches
	for i := 0; i < numOperations; i++ {
		go func() {
			defer wg.Done()
			event := &testEvent{eventType: "test.event", payload: "data"}
			dispatcher.Dispatch(context.Background(), event)
		}()
	}

	wg.Wait()
}

func TestConcurrentHasAndRegister(t *testing.T) {
	dispatcher := NewEventDispatcher()
	handler := &testHandler{}

	var wg sync.WaitGroup
	numOperations := 100
	wg.Add(numOperations * 2)

	// Concurrent Has checks
	for i := 0; i < numOperations; i++ {
		go func() {
			defer wg.Done()
			dispatcher.Has("test.event", handler)
		}()
	}

	// Concurrent Register
	for i := 0; i < numOperations; i++ {
		go func() {
			defer wg.Done()
			dispatcher.Register("test.event", handler)
		}()
	}

	wg.Wait()
}

func TestConcurrentClearAndDispatch(t *testing.T) {
	dispatcher := NewEventDispatcher()
	handler := &testHandler{}
	dispatcher.Register("test.event", handler)

	var wg sync.WaitGroup
	numOperations := 50
	wg.Add(numOperations * 2)

	// Concurrent Clear
	for i := 0; i < numOperations; i++ {
		go func() {
			defer wg.Done()
			dispatcher.Clear()
		}()
	}

	// Concurrent Dispatch
	for i := 0; i < numOperations; i++ {
		go func() {
			defer wg.Done()
			event := &testEvent{eventType: "test.event", payload: "data"}
			dispatcher.Dispatch(context.Background(), event)
		}()
	}

	wg.Wait()
}

func TestConcurrentMultipleEventTypes(t *testing.T) {
	dispatcher := NewEventDispatcher()
	var wg sync.WaitGroup

	eventTypes := []string{"event1", "event2", "event3", "event4", "event5"}
	numOperations := 50
	wg.Add(len(eventTypes) * numOperations * 3)

	for _, eventType := range eventTypes {
		for i := 0; i < numOperations; i++ {
			// Register
			go func(et string) {
				defer wg.Done()
				handler := &testHandler{}
				dispatcher.Register(et, handler)
			}(eventType)

			// Dispatch
			go func(et string) {
				defer wg.Done()
				event := &testEvent{eventType: et, payload: "data"}
				dispatcher.Dispatch(context.Background(), event)
			}(eventType)

			// Has
			go func(et string) {
				defer wg.Done()
				handler := &testHandler{}
				dispatcher.Has(et, handler)
			}(eventType)
		}
	}

	wg.Wait()
}

// ============================================================================
// STRESS TESTS
// ============================================================================

func TestStress_MassiveParallelOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	dispatcher := NewEventDispatcher()
	var wg sync.WaitGroup

	numGoroutines := 1000
	handlers := make([]*testHandler, 100)
	for i := range handlers {
		handlers[i] = &testHandler{}
	}

	wg.Add(numGoroutines * 4)

	for i := 0; i < numGoroutines; i++ {
		// Register
		go func(idx int) {
			defer wg.Done()
			handler := handlers[idx%len(handlers)]
			dispatcher.Register("stress.event", handler)
		}(i)

		// Dispatch
		go func() {
			defer wg.Done()
			event := &testEvent{eventType: "stress.event", payload: "data"}
			dispatcher.Dispatch(context.Background(), event)
		}()

		// Has
		go func(idx int) {
			defer wg.Done()
			handler := handlers[idx%len(handlers)]
			dispatcher.Has("stress.event", handler)
		}(i)

		// Remove
		go func(idx int) {
			defer wg.Done()
			handler := handlers[idx%len(handlers)]
			dispatcher.Remove("stress.event", handler)
		}(i)
	}

	wg.Wait()
}

// ============================================================================
// BENCHMARKS
// ============================================================================

func BenchmarkRegister(b *testing.B) {
	dispatcher := NewEventDispatcher()
	handlers := make([]*testHandler, b.N)
	for i := range handlers {
		handlers[i] = &testHandler{}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dispatcher.Register("bench.event", handlers[i])
	}
}

func BenchmarkDispatch_SingleHandler(b *testing.B) {
	dispatcher := NewEventDispatcher()
	handler := &testHandler{}
	dispatcher.Register("bench.event", handler)

	event := &testEvent{eventType: "bench.event", payload: "data"}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dispatcher.Dispatch(ctx, event)
	}
}

func BenchmarkDispatch_MultipleHandlers(b *testing.B) {
	dispatcher := NewEventDispatcher()

	// Register 10 handlers
	for i := 0; i < 10; i++ {
		dispatcher.Register("bench.event", &testHandler{})
	}

	event := &testEvent{eventType: "bench.event", payload: "data"}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dispatcher.Dispatch(ctx, event)
	}
}

func BenchmarkHas(b *testing.B) {
	dispatcher := NewEventDispatcher()
	handler := &testHandler{}
	dispatcher.Register("bench.event", handler)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dispatcher.Has("bench.event", handler)
	}
}

func BenchmarkRemove(b *testing.B) {
	dispatcher := NewEventDispatcher()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		handler := &testHandler{}
		dispatcher.Register("bench.event", handler)
		b.StartTimer()

		dispatcher.Remove("bench.event", handler)
	}
}

func BenchmarkConcurrentDispatch(b *testing.B) {
	dispatcher := NewEventDispatcher()
	handler := &testHandler{}
	dispatcher.Register("bench.event", handler)

	event := &testEvent{eventType: "bench.event", payload: "data"}
	ctx := context.Background()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			dispatcher.Dispatch(ctx, event)
		}
	})
}

func BenchmarkConcurrentRegister(b *testing.B) {
	dispatcher := NewEventDispatcher()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			handler := &testHandler{}
			dispatcher.Register("bench.event", handler)
		}
	})
}
