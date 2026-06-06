package consumer_test

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/JailtonJunior94/devkit-go/pkg/worker/consumer"
	"github.com/stretchr/testify/suite"
	"go.uber.org/goleak"
)

type mockSource struct {
	msgCh    chan consumer.Message
	stopErr  error
	stopped  bool
	startErr error
}

func (m *mockSource) Messages(_ context.Context) (<-chan consumer.Message, error) {
	if m.startErr != nil {
		return nil, m.startErr
	}
	return m.msgCh, nil
}

func (m *mockSource) Stop(_ context.Context) error {
	m.stopped = true
	return m.stopErr
}

type RunnerSuite struct {
	suite.Suite
	obs *noop.Provider
}

func TestRunnerSuite(t *testing.T) {
	suite.Run(t, new(RunnerSuite))
}

func (s *RunnerSuite) SetupTest() {
	s.obs = noop.NewProvider()
}

func (s *RunnerSuite) TestStart_PropagatesSourceError() {
	expected := errors.New("source error")
	src := &mockSource{startErr: expected}
	r, err := consumer.NewRunner("test", src, nil, s.obs)
	s.Require().NoError(err)
	s.Require().ErrorIs(r.Start(context.Background()), expected)
}

func (s *RunnerSuite) TestStart_DispatchesMessages() {
	var handled []string
	msgCh := make(chan consumer.Message, 2)
	msgCh <- consumer.Message{EventType: "evt.a"}
	msgCh <- consumer.Message{EventType: "evt.b"}
	close(msgCh)

	h := consumer.HandlerFunc(func(_ context.Context, msg consumer.Message) error {
		handled = append(handled, msg.EventType)
		return nil
	})

	r, err := consumer.NewRunner("test", &mockSource{msgCh: msgCh}, []consumer.Registration{
		{Name: "a", EventType: "evt.a", Handler: h},
		{Name: "b", EventType: "evt.b", Handler: h},
	}, s.obs)
	s.Require().NoError(err)
	s.Require().NoError(r.Start(context.Background()))
	s.Require().ElementsMatch([]string{"evt.a", "evt.b"}, handled)
}

func (s *RunnerSuite) TestStop_CallsSourceStop() {
	src := &mockSource{msgCh: make(chan consumer.Message)}
	r, err := consumer.NewRunner("test", src, nil, s.obs)
	s.Require().NoError(err)
	s.Require().NoError(r.Stop(context.Background()))
	s.Require().True(src.stopped)
}

func (s *RunnerSuite) TestStop_PropagatesSourceStopError() {
	expected := errors.New("stop error")
	src := &mockSource{msgCh: make(chan consumer.Message), stopErr: expected}
	r, err := consumer.NewRunner("test", src, nil, s.obs)
	s.Require().NoError(err)
	s.Require().ErrorIs(r.Stop(context.Background()), expected)
}

func (s *RunnerSuite) TestStart_NoGoroutineLeak() {
	defer goleak.VerifyNone(s.T())

	msgCh := make(chan consumer.Message)
	src := &mockSource{msgCh: msgCh}
	r, err := consumer.NewRunner("test", src, nil, s.obs)
	s.Require().NoError(err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = r.Start(ctx)
	}()

	cancel()
	close(msgCh)
	<-done
}
