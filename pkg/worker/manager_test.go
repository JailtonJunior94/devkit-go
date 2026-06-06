package worker_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/JailtonJunior94/devkit-go/pkg/worker"
	"github.com/JailtonJunior94/devkit-go/pkg/worker/job"
	"github.com/stretchr/testify/suite"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

type mockJob struct {
	name     string
	schedule string
	runFn    func(ctx context.Context) error
	policy   job.OverlapPolicy
}

func (m *mockJob) Name() string                     { return m.name }
func (m *mockJob) Schedule() string                 { return m.schedule }
func (m *mockJob) OverlapPolicy() job.OverlapPolicy { return m.policy }
func (m *mockJob) Run(ctx context.Context) error {
	if m.runFn != nil {
		return m.runFn(ctx)
	}
	return nil
}

func newTestJob(name string) *mockJob {
	return &mockJob{name: name, schedule: "* * * * *", policy: job.OverlapSkip}
}

type mockConsumer struct {
	name    string
	startFn func(ctx context.Context) error
	stopFn  func(ctx context.Context) error
}

func (m *mockConsumer) Name() string { return m.name }
func (m *mockConsumer) Start(ctx context.Context) error {
	if m.startFn != nil {
		return m.startFn(ctx)
	}
	<-ctx.Done()
	return nil
}
func (m *mockConsumer) Stop(ctx context.Context) error {
	if m.stopFn != nil {
		return m.stopFn(ctx)
	}
	return nil
}

func newTestConsumer(name string) *mockConsumer {
	return &mockConsumer{name: name}
}

type ManagerSuite struct {
	suite.Suite
	obs *noop.Provider
	cfg worker.Config
}

func TestManagerSuite(t *testing.T) {
	suite.Run(t, new(ManagerSuite))
}

func (s *ManagerSuite) SetupTest() {
	s.obs = noop.NewProvider()
	s.cfg = worker.Config{ShutdownTimeout: 3 * time.Second}
}

func (s *ManagerSuite) TestStartStop_Success() {
	m := worker.NewManager(s.cfg, nil, nil, s.obs)
	s.Require().NoError(m.Start(context.Background()))
	s.Require().NoError(m.Stop(context.Background()))
}

func (s *ManagerSuite) TestStart_DuplicateNamesInJobs() {
	m := worker.NewManager(s.cfg, []worker.Job{
		newTestJob("dup"),
		newTestJob("dup"),
	}, nil, s.obs)
	err := m.Start(context.Background())
	s.Require().ErrorIs(err, worker.ErrDuplicateName)
}

func (s *ManagerSuite) TestStart_DuplicateNamesInConsumers() {
	m := worker.NewManager(s.cfg, nil, []worker.Consumer{
		newTestConsumer("dup"),
		newTestConsumer("dup"),
	}, s.obs)
	err := m.Start(context.Background())
	s.Require().ErrorIs(err, worker.ErrDuplicateName)
}

func (s *ManagerSuite) TestStart_DuplicateAcrossJobAndConsumer() {
	m := worker.NewManager(s.cfg,
		[]worker.Job{newTestJob("shared-name")},
		[]worker.Consumer{newTestConsumer("shared-name")},
		s.obs,
	)
	err := m.Start(context.Background())
	s.Require().ErrorIs(err, worker.ErrDuplicateName)
}

func (s *ManagerSuite) TestStart_IsIdempotent() {
	m := worker.NewManager(s.cfg, nil, nil, s.obs)
	s.Require().NoError(m.Start(context.Background()))
	s.Require().ErrorIs(m.Start(context.Background()), worker.ErrAlreadyStarted)
	s.Require().NoError(m.Stop(context.Background()))
}

func (s *ManagerSuite) TestStop_IsIdempotent() {
	m := worker.NewManager(s.cfg, nil, nil, s.obs)
	s.Require().NoError(m.Start(context.Background()))
	s.Require().NoError(m.Stop(context.Background()))
	s.Require().NoError(m.Stop(context.Background()))
}

func (s *ManagerSuite) TestStop_AggregatesConsumerErrorsWithoutRace() {
	stopErr := errors.New("consumer stop failed")
	consumers := make([]worker.Consumer, 5)
	for i := range consumers {
		consumers[i] = &mockConsumer{
			name: "consumer-" + string(rune('a'+i)),
			stopFn: func(_ context.Context) error {
				return stopErr
			},
		}
	}

	m := worker.NewManager(s.cfg, nil, consumers, s.obs)
	s.Require().NoError(m.Start(context.Background()))

	err := m.Stop(context.Background())
	s.Require().Error(err)
	s.Require().ErrorIs(err, stopErr)
}

func (s *ManagerSuite) TestStop_TimeoutAppendsSentinel() {
	blocked := make(chan struct{})
	c := &mockConsumer{
		name: "blocked-consumer",
		stopFn: func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-blocked:
				return nil
			}
		},
	}

	m := worker.NewManager(worker.Config{ShutdownTimeout: 100 * time.Millisecond}, nil, []worker.Consumer{c}, s.obs)
	s.Require().NoError(m.Start(context.Background()))

	err := m.Stop(context.Background())
	close(blocked)
	s.Require().ErrorIs(err, worker.ErrStopTimeout)
}

func (s *ManagerSuite) TestNewManagerWithSlog_Works() {
	logger := slog.Default()
	m := worker.NewManagerWithSlog(s.cfg, nil, nil, logger)
	s.Require().NoError(m.Start(context.Background()))
	s.Require().NoError(m.Stop(context.Background()))
}

func (s *ManagerSuite) TestStop_NoGoroutineLeak() {
	m := worker.NewManager(s.cfg, nil, []worker.Consumer{newTestConsumer("c1")}, s.obs)
	s.Require().NoError(m.Start(context.Background()))
	s.Require().NoError(m.Stop(context.Background()))
}
