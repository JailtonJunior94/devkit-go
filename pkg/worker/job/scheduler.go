package job

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/robfig/cron/v3"
)

var ErrSchedulerStarted = errors.New("worker: scheduler already started")

type registeredJob struct {
	name     string
	schedule string
	run      func(ctx context.Context) error
	policy   OverlapPolicy
}

type Scheduler struct {
	cron        *cron.Cron
	validateFn  func(string) error
	obs         observability.Observability
	mu          sync.Mutex
	started     bool
	jobs        []registeredJob
	allowWg     sync.WaitGroup
	execCounter observability.Counter
	errCounter  observability.Counter
}

func NewScheduler(obs observability.Observability) *Scheduler {
	return newSchedulerWith(obs, cron.New(), func(s string) error {
		_, err := cron.ParseStandard(s)
		return err
	})
}

func newSchedulerWith(obs observability.Observability, c *cron.Cron, validateFn func(string) error) *Scheduler {
	return &Scheduler{
		cron:        c,
		validateFn:  validateFn,
		obs:         obs,
		execCounter: obs.Metrics().Counter("worker.jobs.executions_total", "Total job executions", "1"),
		errCounter:  obs.Metrics().Counter("worker.jobs.errors_total", "Total job errors", "1"),
	}
}

func (s *Scheduler) Register(j Runner) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return ErrSchedulerStarted
	}

	if err := s.validateFn(j.Schedule()); err != nil {
		return fmt.Errorf("worker: invalid schedule for %s: %w", j.Name(), err)
	}

	s.jobs = append(s.jobs, registeredJob{
		name:     j.Name(),
		schedule: j.Schedule(),
		run:      j.Run,
		policy:   j.OverlapPolicy(),
	})
	return nil
}

func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	s.started = true
	jobs := make([]registeredJob, len(s.jobs))
	copy(jobs, s.jobs)
	s.mu.Unlock()

	for _, rj := range jobs {
		var inFlight atomic.Bool

		switch rj.policy {
		case OverlapAllow:
			s.cron.AddFunc(rj.schedule, func() {
				s.allowWg.Go(func() {
					s.executeJob(ctx, rj)
				})
			})
		default:
			s.cron.AddFunc(rj.schedule, func() {
				if !inFlight.CompareAndSwap(false, true) {
					return
				}
				defer inFlight.Store(false)
				s.executeJob(ctx, rj)
			})
		}
	}

	s.cron.Start()
	<-ctx.Done()
	stopCtx := s.cron.Stop()
	<-stopCtx.Done()
	s.allowWg.Wait()
}

func (s *Scheduler) Stop() {}

func (s *Scheduler) executeJob(ctx context.Context, rj registeredJob) {
	ctx, span := s.obs.Tracer().Start(ctx, "worker.job.run",
		observability.WithAttributes(observability.String("job.name", rj.name)))
	defer span.End()

	if err := rj.run(ctx); err != nil {
		span.RecordError(err)
		span.SetStatus(observability.StatusCodeError, err.Error())
		s.errCounter.Increment(ctx, observability.String("job", rj.name))
		s.execCounter.Increment(ctx,
			observability.String("job", rj.name),
			observability.String("result", "error"),
		)
		s.obs.Logger().Error(ctx, "job execution failed",
			observability.String("operation", "worker.job.run"),
			observability.String("name", rj.name),
			observability.Error(err),
		)
		return
	}

	s.execCounter.Increment(ctx,
		observability.String("job", rj.name),
		observability.String("result", "success"),
	)
}
