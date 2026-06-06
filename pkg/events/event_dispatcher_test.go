package events

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type testEvent struct {
	eventType string
	payload   any
}

func (e *testEvent) GetEventType() string { return e.eventType }
func (e *testEvent) GetPayload() any      { return e.payload }

type testHandler struct {
	callCount atomic.Int32
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
	return h.err
}

func (h *testHandler) count() int {
	return int(h.callCount.Load())
}

type EventDispatcherSuite struct {
	suite.Suite
}

func TestEventDispatcherSuite(t *testing.T) {
	suite.Run(t, new(EventDispatcherSuite))
}

func (s *EventDispatcherSuite) TestNewEventDispatcher() {
	s.Require().NotNil(NewEventDispatcher())
	s.Require().NotNil(NewEventDispatcher(WithCapacity(10)))
}

func (s *EventDispatcherSuite) TestWithCapacity() {
	scenarios := []struct {
		name     string
		capacity int
	}{
		{name: "deve aceitar capacidade positiva", capacity: 50},
		{name: "deve aceitar capacidade zero", capacity: 0},
		{name: "deve tratar capacidade negativa como zero", capacity: -1},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			dispatcher := NewEventDispatcher(WithCapacity(sc.capacity))
			handler := &testHandler{}
			s.Require().NoError(dispatcher.Register("test.event", handler))
			s.True(dispatcher.Has("test.event", handler))
		})
	}
}

func (s *EventDispatcherSuite) TestRegister() {
	scenarios := []struct {
		name      string
		eventType string
		handler   EventHandler
		wantErr   error
	}{
		{
			name:      "deve registrar handler com sucesso",
			eventType: "test.event",
			handler:   &testHandler{},
		},
		{
			name:      "deve retornar erro para event type vazio",
			eventType: "",
			handler:   &testHandler{},
			wantErr:   ErrEventTypeEmpty,
		},
		{
			name:      "deve retornar erro para handler nil",
			eventType: "test.event",
			handler:   nil,
			wantErr:   ErrHandlerNil,
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			dispatcher := NewEventDispatcher()
			err := dispatcher.Register(sc.eventType, sc.handler)
			if sc.wantErr != nil {
				s.Require().ErrorIs(err, sc.wantErr)
				return
			}
			s.Require().NoError(err)
			s.True(dispatcher.Has(sc.eventType, sc.handler))
		})
	}
}

func (s *EventDispatcherSuite) TestRegisterDuplicate() {
	dispatcher := NewEventDispatcher()
	handler := &testHandler{}
	s.Require().NoError(dispatcher.Register("test.event", handler))
	s.Require().ErrorIs(dispatcher.Register("test.event", handler), ErrHandlerAlreadyRegistered)
}

func (s *EventDispatcherSuite) TestRegisterMultipleHandlers() {
	dispatcher := NewEventDispatcher()
	h1, h2, h3 := &testHandler{}, &testHandler{}, &testHandler{}
	s.Require().NoError(dispatcher.Register("test.event", h1))
	s.Require().NoError(dispatcher.Register("test.event", h2))
	s.Require().NoError(dispatcher.Register("test.event", h3))
	s.True(dispatcher.Has("test.event", h1))
	s.True(dispatcher.Has("test.event", h2))
	s.True(dispatcher.Has("test.event", h3))
}

func (s *EventDispatcherSuite) TestHas() {
	scenarios := []struct {
		name      string
		eventType string
		handler   EventHandler
		register  bool
		want      bool
	}{
		{
			name:      "deve retornar true para handler registrado",
			eventType: "test.event",
			handler:   &testHandler{},
			register:  true,
			want:      true,
		},
		{
			name:      "deve retornar false para handler nao registrado",
			eventType: "test.event",
			handler:   &testHandler{},
			want:      false,
		},
		{
			name:      "deve retornar false para event type vazio",
			eventType: "",
			handler:   &testHandler{},
			want:      false,
		},
		{
			name:      "deve retornar false para handler nil",
			eventType: "test.event",
			handler:   nil,
			want:      false,
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			dispatcher := NewEventDispatcher()
			if sc.register && sc.eventType != "" && sc.handler != nil {
				s.Require().NoError(dispatcher.Register(sc.eventType, sc.handler))
			}
			s.Equal(sc.want, dispatcher.Has(sc.eventType, sc.handler))
		})
	}
}

func (s *EventDispatcherSuite) TestDispatch() {
	scenarios := []struct {
		name    string
		event   Event
		wantErr error
	}{
		{
			name:    "deve retornar ErrEventNil para evento nil",
			event:   nil,
			wantErr: ErrEventNil,
		},
		{
			name:    "deve retornar ErrEventTypeEmpty para event type vazio",
			event:   &testEvent{eventType: ""},
			wantErr: ErrEventTypeEmpty,
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			s.Require().ErrorIs(NewEventDispatcher().Dispatch(context.Background(), sc.event), sc.wantErr)
		})
	}
}

func (s *EventDispatcherSuite) TestDispatchNoHandlers() {
	s.Require().NoError(NewEventDispatcher().Dispatch(context.Background(), &testEvent{eventType: "unknown.event"}))
}

func (s *EventDispatcherSuite) TestDispatchSingleHandler() {
	dispatcher := NewEventDispatcher()
	handler := &testHandler{}
	s.Require().NoError(dispatcher.Register("test.event", handler))
	s.Require().NoError(dispatcher.Dispatch(context.Background(), &testEvent{eventType: "test.event", payload: "data"}))
	s.Equal(1, handler.count())
}

func (s *EventDispatcherSuite) TestDispatchMultipleHandlers() {
	dispatcher := NewEventDispatcher()
	h1, h2, h3 := &testHandler{}, &testHandler{}, &testHandler{}
	s.Require().NoError(dispatcher.Register("test.event", h1))
	s.Require().NoError(dispatcher.Register("test.event", h2))
	s.Require().NoError(dispatcher.Register("test.event", h3))
	s.Require().NoError(dispatcher.Dispatch(context.Background(), &testEvent{eventType: "test.event"}))
	s.Equal(1, h1.count())
	s.Equal(1, h2.count())
	s.Equal(1, h3.count())
}

func (s *EventDispatcherSuite) TestDispatchHandlerError() {
	dispatcher := NewEventDispatcher()
	expectedErr := errors.New("handler error")
	handler := &testHandler{err: expectedErr}
	s.Require().NoError(dispatcher.Register("test.event", handler))
	s.Require().ErrorIs(dispatcher.Dispatch(context.Background(), &testEvent{eventType: "test.event"}), expectedErr)
}

func (s *EventDispatcherSuite) TestDispatchStopsOnFirstError() {
	dispatcher := NewEventDispatcher()
	h1 := &testHandler{}
	h2 := &testHandler{err: errors.New("stop")}
	h3 := &testHandler{}
	s.Require().NoError(dispatcher.Register("test.event", h1))
	s.Require().NoError(dispatcher.Register("test.event", h2))
	s.Require().NoError(dispatcher.Register("test.event", h3))
	_ = dispatcher.Dispatch(context.Background(), &testEvent{eventType: "test.event"})
	s.Equal(1, h1.count())
	s.Equal(1, h2.count())
	s.Equal(0, h3.count())
}

func (s *EventDispatcherSuite) TestDispatchContextTimeout() {
	dispatcher := NewEventDispatcher()
	handler := &testHandler{delay: 100 * time.Millisecond}
	s.Require().NoError(dispatcher.Register("test.event", handler))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	s.Require().ErrorIs(dispatcher.Dispatch(ctx, &testEvent{eventType: "test.event"}), context.DeadlineExceeded)
}

func (s *EventDispatcherSuite) TestDispatchContextCancelledBeforeDispatch() {
	dispatcher := NewEventDispatcher()
	handler := &testHandler{}
	s.Require().NoError(dispatcher.Register("test.event", handler))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	s.Require().ErrorIs(dispatcher.Dispatch(ctx, &testEvent{eventType: "test.event"}), context.Canceled)
	s.Equal(0, handler.count())
}

func (s *EventDispatcherSuite) TestDispatchContextCancelledBetweenHandlers() {
	dispatcher := NewEventDispatcher()
	h1 := &testHandler{}
	h2 := &testHandler{delay: 50 * time.Millisecond}
	h3 := &testHandler{}
	s.Require().NoError(dispatcher.Register("test.event", h1))
	s.Require().NoError(dispatcher.Register("test.event", h2))
	s.Require().NoError(dispatcher.Register("test.event", h3))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	s.Require().ErrorIs(dispatcher.Dispatch(ctx, &testEvent{eventType: "test.event"}), context.DeadlineExceeded)
	s.Equal(1, h1.count())
	s.Equal(0, h3.count())
}

func (s *EventDispatcherSuite) TestRemove() {
	scenarios := []struct {
		name      string
		eventType string
		handler   EventHandler
		register  bool
		wantHas   bool
	}{
		{
			name:      "deve remover handler registrado com sucesso",
			eventType: "test.event",
			handler:   &testHandler{},
			register:  true,
			wantHas:   false,
		},
		{
			name:      "deve ser idempotente para handler nao registrado",
			eventType: "test.event",
			handler:   &testHandler{},
			register:  false,
		},
		{
			name:      "deve ser idempotente para event type vazio",
			eventType: "",
			handler:   &testHandler{},
		},
		{
			name:      "deve ser idempotente para handler nil",
			eventType: "test.event",
			handler:   nil,
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			dispatcher := NewEventDispatcher()
			if sc.register {
				s.Require().NoError(dispatcher.Register(sc.eventType, sc.handler))
			}
			s.Require().NoError(dispatcher.Remove(sc.eventType, sc.handler))
			if sc.register && sc.eventType != "" && sc.handler != nil {
				s.Equal(sc.wantHas, dispatcher.Has(sc.eventType, sc.handler))
			}
		})
	}
}

func (s *EventDispatcherSuite) TestRemoveOnlyFirstOccurrence() {
	dispatcher := NewEventDispatcher().(*eventDispatcher)
	handler := &testHandler{}

	dispatcher.mu.Lock()
	dispatcher.handlers["test.event"] = append(dispatcher.handlers["test.event"], handler, handler)
	dispatcher.mu.Unlock()

	s.Require().NoError(dispatcher.Remove("test.event", handler))
	s.True(dispatcher.Has("test.event", handler))
}

func (s *EventDispatcherSuite) TestClear() {
	dispatcher := NewEventDispatcher()
	h1, h2 := &testHandler{}, &testHandler{}
	s.Require().NoError(dispatcher.Register("event1", h1))
	s.Require().NoError(dispatcher.Register("event2", h2))

	dispatcher.Clear()

	s.False(dispatcher.Has("event1", h1))
	s.False(dispatcher.Has("event2", h2))
}

func (s *EventDispatcherSuite) TestConcurrentRegister() {
	dispatcher := NewEventDispatcher()
	var wg sync.WaitGroup
	const n = 100
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			_ = dispatcher.Register("test.event", &testHandler{})
		}()
	}
	wg.Wait()
}

func (s *EventDispatcherSuite) TestConcurrentDispatch() {
	dispatcher := NewEventDispatcher()
	handler := &testHandler{}
	s.Require().NoError(dispatcher.Register("test.event", handler))

	var wg sync.WaitGroup
	const n = 100
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			_ = dispatcher.Dispatch(context.Background(), &testEvent{eventType: "test.event", payload: "data"})
		}()
	}
	wg.Wait()
	s.Equal(n, handler.count())
}

func (s *EventDispatcherSuite) TestConcurrentRegisterAndDispatch() {
	dispatcher := NewEventDispatcher()
	var wg sync.WaitGroup
	const n = 50
	wg.Add(n * 2)
	for range n {
		go func() {
			defer wg.Done()
			_ = dispatcher.Register("test.event", &testHandler{})
		}()
		go func() {
			defer wg.Done()
			_ = dispatcher.Dispatch(context.Background(), &testEvent{eventType: "test.event"})
		}()
	}
	wg.Wait()
}

func (s *EventDispatcherSuite) TestConcurrentRegisterRemoveDispatch() {
	dispatcher := NewEventDispatcher()
	handlers := make([]*testHandler, 20)
	for i := range handlers {
		handlers[i] = &testHandler{}
	}

	var wg sync.WaitGroup
	const n = 100
	wg.Add(n * 3)
	for i := range n {
		go func(idx int) {
			defer wg.Done()
			_ = dispatcher.Register("test.event", handlers[idx%len(handlers)])
		}(i)
		go func(idx int) {
			defer wg.Done()
			_ = dispatcher.Remove("test.event", handlers[idx%len(handlers)])
		}(i)
		go func() {
			defer wg.Done()
			_ = dispatcher.Dispatch(context.Background(), &testEvent{eventType: "test.event"})
		}()
	}
	wg.Wait()
}

func (s *EventDispatcherSuite) TestConcurrentHasAndRegister() {
	dispatcher := NewEventDispatcher()
	handler := &testHandler{}

	var wg sync.WaitGroup
	const n = 100
	wg.Add(n * 2)
	for range n {
		go func() {
			defer wg.Done()
			dispatcher.Has("test.event", handler)
		}()
		go func() {
			defer wg.Done()
			_ = dispatcher.Register("test.event", handler)
		}()
	}
	wg.Wait()
}

func (s *EventDispatcherSuite) TestConcurrentClearAndDispatch() {
	dispatcher := NewEventDispatcher()
	s.Require().NoError(dispatcher.Register("test.event", &testHandler{}))

	var wg sync.WaitGroup
	const n = 50
	wg.Add(n * 2)
	for range n {
		go func() {
			defer wg.Done()
			dispatcher.Clear()
		}()
		go func() {
			defer wg.Done()
			_ = dispatcher.Dispatch(context.Background(), &testEvent{eventType: "test.event"})
		}()
	}
	wg.Wait()
}

func (s *EventDispatcherSuite) TestConcurrentMultipleEventTypes() {
	dispatcher := NewEventDispatcher()
	eventTypes := []string{"event1", "event2", "event3", "event4", "event5"}

	var wg sync.WaitGroup
	const n = 50
	wg.Add(len(eventTypes) * n * 3)
	for _, et := range eventTypes {
		for range n {
			go func(t string) {
				defer wg.Done()
				_ = dispatcher.Register(t, &testHandler{})
			}(et)
			go func(t string) {
				defer wg.Done()
				_ = dispatcher.Dispatch(context.Background(), &testEvent{eventType: t})
			}(et)
			go func(t string) {
				defer wg.Done()
				dispatcher.Has(t, &testHandler{})
			}(et)
		}
	}
	wg.Wait()
}

func (s *EventDispatcherSuite) TestStressMassiveParallelOperations() {
	if testing.Short() {
		s.T().Skip("skipping stress test in short mode")
	}

	dispatcher := NewEventDispatcher()
	handlers := make([]*testHandler, 100)
	for i := range handlers {
		handlers[i] = &testHandler{}
	}

	var wg sync.WaitGroup
	const n = 1000
	wg.Add(n * 4)
	for i := range n {
		go func(idx int) {
			defer wg.Done()
			_ = dispatcher.Register("stress.event", handlers[idx%len(handlers)])
		}(i)
		go func() {
			defer wg.Done()
			_ = dispatcher.Dispatch(context.Background(), &testEvent{eventType: "stress.event"})
		}()
		go func(idx int) {
			defer wg.Done()
			dispatcher.Has("stress.event", handlers[idx%len(handlers)])
		}(i)
		go func(idx int) {
			defer wg.Done()
			_ = dispatcher.Remove("stress.event", handlers[idx%len(handlers)])
		}(i)
	}
	wg.Wait()
}

func BenchmarkRegister(b *testing.B) {
	dispatcher := NewEventDispatcher()
	for b.Loop() {
		_ = dispatcher.Register("bench.event", &testHandler{})
	}
}

func BenchmarkDispatch_SingleHandler(b *testing.B) {
	dispatcher := NewEventDispatcher()
	_ = dispatcher.Register("bench.event", &testHandler{})
	event := &testEvent{eventType: "bench.event", payload: "data"}
	ctx := context.Background()
	for b.Loop() {
		_ = dispatcher.Dispatch(ctx, event)
	}
}

func BenchmarkDispatch_MultipleHandlers(b *testing.B) {
	dispatcher := NewEventDispatcher()
	for range 10 {
		_ = dispatcher.Register("bench.event", &testHandler{})
	}
	event := &testEvent{eventType: "bench.event", payload: "data"}
	ctx := context.Background()
	for b.Loop() {
		_ = dispatcher.Dispatch(ctx, event)
	}
}

func BenchmarkHas(b *testing.B) {
	dispatcher := NewEventDispatcher()
	handler := &testHandler{}
	_ = dispatcher.Register("bench.event", handler)
	for b.Loop() {
		dispatcher.Has("bench.event", handler)
	}
}

func BenchmarkRemove(b *testing.B) {
	dispatcher := NewEventDispatcher()
	b.ResetTimer()
	for range b.N {
		b.StopTimer()
		handler := &testHandler{}
		_ = dispatcher.Register("bench.event", handler)
		b.StartTimer()
		_ = dispatcher.Remove("bench.event", handler)
	}
}

func BenchmarkConcurrentDispatch(b *testing.B) {
	dispatcher := NewEventDispatcher()
	_ = dispatcher.Register("bench.event", &testHandler{})
	event := &testEvent{eventType: "bench.event", payload: "data"}
	ctx := context.Background()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = dispatcher.Dispatch(ctx, event)
		}
	})
}

func BenchmarkConcurrentRegister(b *testing.B) {
	dispatcher := NewEventDispatcher()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = dispatcher.Register("bench.event", &testHandler{})
		}
	})
}
