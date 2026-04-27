package otel

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShutdownCoordinator(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		policy     observability.ShutdownPolicy
		register   func(*shutdownCoordinator, *[]string)
		run        func(*testing.T, *shutdownCoordinator) error
		assertions func(*testing.T, error, []string)
	}{
		{
			name:   "runs force flush before shutdown in policy order",
			policy: newTestShutdownPolicy(t, time.Second, []string{"tracer_provider", "meter_provider", "logger_provider"}),
			register: func(c *shutdownCoordinator, calls *[]string) {
				c.register(newRecordingShutdownStep("logger_provider", calls, nil, nil))
				c.register(newRecordingShutdownStep("tracer_provider", calls, nil, nil))
				c.register(newRecordingShutdownStep("meter_provider", calls, nil, nil))
			},
			run: func(t *testing.T, c *shutdownCoordinator) error {
				t.Helper()
				return c.Shutdown(context.Background())
			},
			assertions: func(t *testing.T, err error, calls []string) {
				t.Helper()
				require.NoError(t, err)
				assert.Equal(t, []string{
					"force_flush:tracer_provider",
					"force_flush:meter_provider",
					"force_flush:logger_provider",
					"shutdown:tracer_provider",
					"shutdown:meter_provider",
					"shutdown:logger_provider",
				}, calls)
			},
		},
		{
			name:   "is idempotent",
			policy: newTestShutdownPolicy(t, time.Second, []string{"tracer_provider"}),
			register: func(c *shutdownCoordinator, calls *[]string) {
				c.register(newRecordingShutdownStep("tracer_provider", calls, nil, nil))
			},
			run: func(t *testing.T, c *shutdownCoordinator) error {
				t.Helper()
				firstErr := c.Shutdown(context.Background())
				secondErr := c.Shutdown(context.Background())
				require.NoError(t, firstErr)
				return secondErr
			},
			assertions: func(t *testing.T, err error, calls []string) {
				t.Helper()
				require.NoError(t, err)
				assert.Equal(t, []string{
					"force_flush:tracer_provider",
					"shutdown:tracer_provider",
				}, calls)
			},
		},
		{
			name:   "aggregates force flush and shutdown errors",
			policy: newTestShutdownPolicy(t, time.Second, []string{"tracer_provider", "meter_provider"}),
			register: func(c *shutdownCoordinator, calls *[]string) {
				c.register(newRecordingShutdownStep("tracer_provider", calls, errors.New("flush failed"), nil))
				c.register(newRecordingShutdownStep("meter_provider", calls, nil, errors.New("shutdown failed")))
			},
			run: func(t *testing.T, c *shutdownCoordinator) error {
				t.Helper()
				return c.Shutdown(context.Background())
			},
			assertions: func(t *testing.T, err error, calls []string) {
				t.Helper()
				require.Error(t, err)
				assert.ErrorIs(t, err, observability.ErrShutdownFailed)
				assert.Contains(t, err.Error(), "tracer_provider force flush")
				assert.Contains(t, err.Error(), "meter_provider shutdown")
				assert.Equal(t, []string{
					"force_flush:tracer_provider",
					"force_flush:meter_provider",
					"shutdown:tracer_provider",
					"shutdown:meter_provider",
				}, calls)
			},
		},
		{
			name:   "returns timeout errors from force flush",
			policy: newTestShutdownPolicy(t, time.Nanosecond, []string{"tracer_provider"}),
			register: func(c *shutdownCoordinator, calls *[]string) {
				c.register(shutdownStep{
					name: "tracer_provider",
					forceFlush: func(ctx context.Context) error {
						*calls = append(*calls, "force_flush:tracer_provider")
						<-ctx.Done()
						return ctx.Err()
					},
					shutdown: func(context.Context) error {
						*calls = append(*calls, "shutdown:tracer_provider")
						return nil
					},
				})
			},
			run: func(t *testing.T, c *shutdownCoordinator) error {
				t.Helper()
				return c.Shutdown(context.Background())
			},
			assertions: func(t *testing.T, err error, calls []string) {
				t.Helper()
				require.Error(t, err)
				assert.ErrorIs(t, err, observability.ErrShutdownFailed)
				assert.ErrorIs(t, err, context.DeadlineExceeded)
				assert.Equal(t, []string{
					"force_flush:tracer_provider",
					"shutdown:tracer_provider",
				}, calls)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			calls := make([]string, 0)
			coordinator := newShutdownCoordinator(tt.policy)
			tt.register(coordinator, &calls)

			err := tt.run(t, coordinator)

			tt.assertions(t, err, calls)
		})
	}
}

func TestRuntimeShutdownTransitions(t *testing.T) {
	t.Parallel()

	policy := newTestShutdownPolicy(t, time.Second, []string{"tracer_provider"})
	calls := make([]string, 0)
	rt := &runtime{
		shutdown: newShutdownCoordinator(policy),
	}
	rt.state.Store(uint32(runtimeStateRunning))
	rt.shutdown.register(newRecordingShutdownStep("tracer_provider", &calls, nil, nil))

	err := rt.Shutdown(context.Background())
	require.NoError(t, err)

	assert.Equal(t, runtimeStateStopped, rt.currentState())
	assert.Equal(t, []string{
		"force_flush:tracer_provider",
		"shutdown:tracer_provider",
	}, calls)

	err = rt.Shutdown(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{
		"force_flush:tracer_provider",
		"shutdown:tracer_provider",
	}, calls)
}

func newTestShutdownPolicy(t *testing.T, timeout time.Duration, order []string) observability.ShutdownPolicy {
	t.Helper()
	policy, err := observability.NewShutdownPolicy(timeout, order)
	require.NoError(t, err)
	return policy
}

func newRecordingShutdownStep(name string, calls *[]string, flushErr, shutdownErr error) shutdownStep {
	return shutdownStep{
		name: name,
		forceFlush: func(context.Context) error {
			*calls = append(*calls, "force_flush:"+name)
			return flushErr
		},
		shutdown: func(context.Context) error {
			*calls = append(*calls, "shutdown:"+name)
			return shutdownErr
		},
	}
}
