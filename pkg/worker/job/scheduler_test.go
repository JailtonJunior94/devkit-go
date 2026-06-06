package job

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/suite"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func newSecondsScheduler(obs *noop.Provider) *Scheduler {
	secondsParser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	return newSchedulerWith(obs, cron.New(cron.WithParser(secondsParser)), func(s string) error {
		_, err := secondsParser.Parse(s)
		return err
	})
}

type SchedulerSuite struct {
	suite.Suite
	obs *noop.Provider
}

func TestSchedulerSuite(t *testing.T) {
	suite.Run(t, new(SchedulerSuite))
}

func (s *SchedulerSuite) SetupTest() {
	s.obs = noop.NewProvider()
}

func (s *SchedulerSuite) TestRegister_InvalidSchedule() {
	sc := NewScheduler(s.obs)
	err := sc.Register(NewAdapter("job", "invalid-schedule", func(ctx context.Context) error { return nil }))
	s.Require().Error(err)
}

func (s *SchedulerSuite) TestRegister_ValidSchedule() {
	sc := NewScheduler(s.obs)
	err := sc.Register(NewAdapter("job", "* * * * *", func(ctx context.Context) error { return nil }))
	s.Require().NoError(err)
}

func (s *SchedulerSuite) TestRegister_AfterStartReturnsError() {
	sc := newSecondsScheduler(s.obs)
	s.Require().NoError(sc.Register(NewAdapter("job", "* * * * * *", func(ctx context.Context) error { return nil })))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		sc.Start(ctx)
	}()

	time.Sleep(50 * time.Millisecond)
	err := sc.Register(NewAdapter("job2", "* * * * * *", func(ctx context.Context) error { return nil }))
	s.Require().ErrorIs(err, ErrSchedulerStarted)

	cancel()
	<-done
}

func (s *SchedulerSuite) TestOverlapSkip_NoConcurrentExecution() {
	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32

	sc := newSecondsScheduler(s.obs)
	s.Require().NoError(sc.Register(NewAdapterWithPolicy("job", "* * * * * *", func(ctx context.Context) error {
		current := concurrent.Add(1)
		if current > maxConcurrent.Load() {
			maxConcurrent.Store(current)
		}
		time.Sleep(1500 * time.Millisecond)
		concurrent.Add(-1)
		return nil
	}, OverlapSkip)))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		sc.Start(ctx)
	}()

	time.Sleep(3 * time.Second)
	cancel()
	<-done

	s.Require().LessOrEqual(maxConcurrent.Load(), int32(1))
}

func (s *SchedulerSuite) TestOverlapAllow_NoGoroutineLeak() {
	started := make(chan struct{})
	block := make(chan struct{})

	sc := newSecondsScheduler(s.obs)
	s.Require().NoError(sc.Register(NewAdapterWithPolicy("job", "* * * * * *", func(ctx context.Context) error {
		select {
		case started <- struct{}{}:
		default:
		}
		select {
		case <-block:
		case <-ctx.Done():
		}
		return nil
	}, OverlapAllow)))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		sc.Start(ctx)
	}()

	<-started
	cancel()
	close(block)
	<-done
}

func (s *SchedulerSuite) TestCancellation_StopsExecution() {
	sc := newSecondsScheduler(s.obs)
	s.Require().NoError(sc.Register(NewAdapter("job", "* * * * * *", func(ctx context.Context) error { return nil })))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		sc.Start(ctx)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		s.Fail("scheduler did not stop after context cancellation")
	}
}
