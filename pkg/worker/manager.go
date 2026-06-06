package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/worker/job"
)

const (
	stateIdle int32 = iota
	stateRunning
	stateStopping
	stateStopped
)

type Manager struct {
	cfg       Config
	jobs      []Job
	consumers []Consumer
	obs       observability.Observability
	startMu   sync.RWMutex
	scheduler *job.Scheduler
	cancel    context.CancelFunc
	state     atomic.Int32
	wg        sync.WaitGroup
}

func NewManager(cfg Config, jobs []Job, consumers []Consumer, obs observability.Observability) *Manager {
	if cfg.ShutdownTimeout == 0 {
		cfg.ShutdownTimeout = defaultConfig().ShutdownTimeout
	}
	return &Manager{
		cfg:       cfg,
		jobs:      jobs,
		consumers: consumers,
		obs:       obs,
	}
}

func NewManagerWithSlog(cfg Config, jobs []Job, consumers []Consumer, logger *slog.Logger) *Manager {
	return NewManager(cfg, jobs, consumers, newObsFromSlog(logger))
}

func (m *Manager) Start(ctx context.Context) error {
	m.startMu.Lock()
	defer m.startMu.Unlock()

	if !m.state.CompareAndSwap(stateIdle, stateRunning) {
		return ErrAlreadyStarted
	}

	if err := m.validateNames(); err != nil {
		m.state.Store(stateIdle)
		return err
	}

	runCtx, cancel := context.WithCancel(ctx)
	sched := job.NewScheduler(m.obs)

	for _, j := range m.jobs {
		if err := sched.Register(j); err != nil {
			cancel()
			m.state.Store(stateIdle)
			return fmt.Errorf("worker: register job %s: %w", j.Name(), err)
		}
	}

	m.cancel = cancel
	m.scheduler = sched

	m.wg.Go(func() {
		m.scheduler.Start(runCtx)
	})

	for _, c := range m.consumers {
		m.wg.Go(func() {
			if err := c.Start(runCtx); err != nil &&
				!errors.Is(err, context.Canceled) &&
				!errors.Is(err, context.DeadlineExceeded) {
				m.obs.Logger().Error(runCtx, "consumer start failed",
					observability.String("operation", "worker.consumer.start"),
					observability.String("name", c.Name()),
					observability.Error(err),
				)
			}
		})
	}

	return nil
}

func (m *Manager) Stop(ctx context.Context) error {
	m.startMu.RLock()
	ok := m.state.CompareAndSwap(stateRunning, stateStopping)
	m.startMu.RUnlock()

	if !ok {
		return nil
	}
	defer m.state.Store(stateStopped)

	m.cancel()

	stopCtx, stopCancel := context.WithTimeout(ctx, m.cfg.ShutdownTimeout)
	defer stopCancel()

	errCh := make(chan error, len(m.consumers))
	var stopWg sync.WaitGroup

	for _, c := range m.consumers {
		stopWg.Add(1)
		go func(consumer Consumer) {
			defer stopWg.Done()
			if err := consumer.Stop(stopCtx); err != nil {
				errCh <- err
			}
		}(c)
	}

	stopped := make(chan struct{})
	go func() {
		stopWg.Wait()
		close(stopped)
	}()

	timedOut := false
	select {
	case <-stopped:
	case <-stopCtx.Done():
		timedOut = true
		<-stopped
	}

	m.wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}
	if timedOut {
		errs = append(errs, ErrStopTimeout)
	}
	return errors.Join(errs...)
}

func (m *Manager) validateNames() error {
	seen := make(map[string]struct{}, len(m.jobs)+len(m.consumers))
	for _, j := range m.jobs {
		if _, exists := seen[j.Name()]; exists {
			return fmt.Errorf("%w: %s", ErrDuplicateName, j.Name())
		}
		seen[j.Name()] = struct{}{}
	}
	for _, c := range m.consumers {
		if _, exists := seen[c.Name()]; exists {
			return fmt.Errorf("%w: %s", ErrDuplicateName, c.Name())
		}
		seen[c.Name()] = struct{}{}
	}
	return nil
}
