package otel

import (
	"context"
	"errors"
	"sync"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

type shutdownFunc func(context.Context) error

type shutdownStep struct {
	name       string
	forceFlush shutdownFunc
	shutdown   shutdownFunc
}

type shutdownCoordinator struct {
	policy observability.ShutdownPolicy

	mu      sync.Mutex
	steps   []shutdownStep
	err     error
	done    chan struct{}
	stopped bool
}

func newShutdownCoordinator(policy observability.ShutdownPolicy) *shutdownCoordinator {
	return &shutdownCoordinator{policy: policy}
}

func (c *shutdownCoordinator) register(step shutdownStep) {
	if step.name == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped {
		return
	}
	c.steps = append(c.steps, step)
}

func (c *shutdownCoordinator) Shutdown(ctx context.Context) error {
	steps, alreadyStopped, previousErr := c.beginShutdown()
	if alreadyStopped {
		return previousErr
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, c.policy.Timeout())
	defer cancel()

	err := c.execute(shutdownCtx, steps)
	c.finishShutdown(err)
	return err
}

func (c *shutdownCoordinator) beginShutdown() ([]shutdownStep, bool, error) {
	c.mu.Lock()
	if c.stopped {
		done := c.done
		c.mu.Unlock()
		<-done
		c.mu.Lock()
		defer c.mu.Unlock()
		return nil, true, c.err
	}
	c.stopped = true
	c.done = make(chan struct{})
	steps := append([]shutdownStep(nil), c.orderedSteps()...)
	c.mu.Unlock()
	return steps, false, nil
}

func (c *shutdownCoordinator) finishShutdown(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.err = err
	close(c.done)
}

func (c *shutdownCoordinator) execute(ctx context.Context, steps []shutdownStep) error {
	var errs []error

	for _, step := range steps {
		if step.forceFlush == nil {
			continue
		}
		if err := step.forceFlush(ctx); err != nil {
			errs = append(errs, observability.NewShutdownError(step.name+" force flush", err))
		}
	}

	for _, step := range steps {
		if step.shutdown == nil {
			continue
		}
		if err := step.shutdown(ctx); err != nil {
			errs = append(errs, observability.NewShutdownError(step.name+" shutdown", err))
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return observability.NewShutdownError("coordinator", errors.Join(errs...))
}

func (c *shutdownCoordinator) orderedSteps() []shutdownStep {
	order := c.policy.FlushOrder()
	if len(order) == 0 || len(c.steps) < 2 {
		return append([]shutdownStep(nil), c.steps...)
	}

	byName := make(map[string][]shutdownStep, len(c.steps))
	for _, step := range c.steps {
		byName[step.name] = append(byName[step.name], step)
	}

	ordered := make([]shutdownStep, 0, len(c.steps))
	seen := make(map[string]bool, len(order))
	for _, name := range order {
		seen[name] = true
		ordered = append(ordered, byName[name]...)
		delete(byName, name)
	}

	for _, step := range c.steps {
		if seen[step.name] {
			continue
		}
		ordered = append(ordered, step)
	}
	return ordered
}
