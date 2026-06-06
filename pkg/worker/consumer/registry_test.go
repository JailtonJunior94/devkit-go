package consumer

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"
)

type mockHandlerImpl struct {
	handleFn func(ctx context.Context, msg Message) error
}

func (m *mockHandlerImpl) Handle(ctx context.Context, msg Message) error {
	if m.handleFn != nil {
		return m.handleFn(ctx, msg)
	}
	return nil
}

type RegistrySuite struct {
	suite.Suite
	reg *registry
}

func TestRegistrySuite(t *testing.T) {
	suite.Run(t, new(RegistrySuite))
}

func (s *RegistrySuite) SetupTest() {
	s.reg = newRegistry()
}

func (s *RegistrySuite) TestRegister_Success() {
	err := s.reg.register("order.created", &mockHandlerImpl{})
	s.Require().NoError(err)
}

func (s *RegistrySuite) TestRegister_NilHandler() {
	err := s.reg.register("order.created", nil)
	s.Require().ErrorIs(err, ErrNilHandler)
}

func (s *RegistrySuite) TestRegister_DuplicateEventType() {
	s.Require().NoError(s.reg.register("order.created", &mockHandlerImpl{}))
	err := s.reg.register("order.created", &mockHandlerImpl{})
	s.Require().ErrorIs(err, ErrDuplicateEventType)
}

func (s *RegistrySuite) TestDispatch_Success_PassesParamsAndBody() {
	var received Message
	h := &mockHandlerImpl{handleFn: func(_ context.Context, msg Message) error {
		received = msg
		return nil
	}}
	s.Require().NoError(s.reg.register("order.created", h))

	sent := Message{EventType: "order.created", Params: map[string]string{"id": "42"}, Body: []byte("payload")}
	err := s.reg.dispatch(context.Background(), sent)
	s.Require().NoError(err)
	s.Require().Equal(sent.Params, received.Params)
	s.Require().Equal(sent.Body, received.Body)
}

func (s *RegistrySuite) TestDispatch_UnknownEventType() {
	err := s.reg.dispatch(context.Background(), Message{EventType: "unknown"})
	s.Require().ErrorIs(err, ErrUnknownEventType)
}

func (s *RegistrySuite) TestDispatch_PropagatesHandlerError() {
	expected := errors.New("handler error")
	h := &mockHandlerImpl{handleFn: func(_ context.Context, _ Message) error { return expected }}
	s.Require().NoError(s.reg.register("order.created", h))

	err := s.reg.dispatch(context.Background(), Message{EventType: "order.created"})
	s.Require().ErrorIs(err, expected)
}

func (s *RegistrySuite) TestRegistry_ConcurrentRegisterAndDispatch() {
	s.Require().NoError(s.reg.register("concurrent.event", &mockHandlerImpl{}))

	const goroutines = 50
	errs := make([]error, goroutines)
	var wg sync.WaitGroup
	for i := range goroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errs[idx] = s.reg.dispatch(context.Background(), Message{EventType: "concurrent.event"})
		}(i)
	}
	wg.Wait()
	for _, err := range errs {
		s.Require().NoError(err)
	}
}
