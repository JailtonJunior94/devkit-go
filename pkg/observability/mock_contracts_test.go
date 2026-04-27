package observability_test

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestGeneratedMocks_ObservabilityContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "observability facade exposes tracer logger metrics and shutdown",
			run: func(t *testing.T) {
				t.Helper()

				ctx := context.Background()
				tracer := mocks.NewMockTracer(t)
				logger := mocks.NewMockLogger(t)
				metrics := mocks.NewMockMetrics(t)
				o11y := mocks.NewMockObservability(t)

				o11y.EXPECT().Tracer().Return(tracer)
				o11y.EXPECT().Logger().Return(logger)
				o11y.EXPECT().Metrics().Return(metrics)
				o11y.EXPECT().Shutdown(ctx).Return(nil)

				assert.Same(t, tracer, o11y.Tracer())
				assert.Same(t, logger, o11y.Logger())
				assert.Same(t, metrics, o11y.Metrics())
				require.NoError(t, o11y.Shutdown(ctx))
			},
		},
		{
			name: "tracer preserves context based signatures",
			run: func(t *testing.T) {
				t.Helper()

				ctx := context.Background()
				tracer := mocks.NewMockTracer(t)

				tracer.EXPECT().Start(ctx, "operation").Return(ctx, nil)
				tracer.EXPECT().SpanFromContext(ctx).Return(nil)
				tracer.EXPECT().ContextWithSpan(ctx, nil).Return(ctx)

				gotCtx, gotSpan := tracer.Start(ctx, "operation")
				assert.Equal(t, ctx, gotCtx)
				assert.Nil(t, gotSpan)
				assert.Nil(t, tracer.SpanFromContext(ctx))
				assert.Equal(t, ctx, tracer.ContextWithSpan(ctx, nil))
			},
		},
		{
			name: "logger supports contextual logging and child logger",
			run: func(t *testing.T) {
				t.Helper()

				ctx := context.Background()
				field := observability.String("request_id", "req-123")
				logger := mocks.NewMockLogger(t)

				logger.EXPECT().Debug(ctx, "debug", field)
				logger.EXPECT().Info(ctx, "info", field)
				logger.EXPECT().Warn(ctx, "warn", field)
				logger.EXPECT().Error(ctx, "error", field)
				logger.EXPECT().With(field).Return(logger)

				logger.Debug(ctx, "debug", field)
				logger.Info(ctx, "info", field)
				logger.Warn(ctx, "warn", field)
				logger.Error(ctx, "error", field)
				assert.Same(t, logger, logger.With(field))
			},
		},
		{
			name: "metrics exposes instrument factories and gauge callback context",
			run: func(t *testing.T) {
				t.Helper()

				ctx := context.Background()
				metrics := mocks.NewMockMetrics(t)
				callback := observability.GaugeCallback(func(context.Context) float64 { return 1 })

				metrics.EXPECT().Counter("requests", "request count", "{request}").Return(nil)
				metrics.EXPECT().Histogram("duration", "request duration", "s").Return(nil)
				metrics.EXPECT().HistogramWithBuckets("duration", "request duration", "s", []float64{0.1, 1}).Return(nil)
				metrics.EXPECT().UpDownCounter("active", "active requests", "{request}").Return(nil)
				metrics.EXPECT().Gauge("load", "runtime load", "1", mock.AnythingOfType("observability.GaugeCallback")).Return(nil)

				assert.Nil(t, metrics.Counter("requests", "request count", "{request}"))
				assert.Nil(t, metrics.Histogram("duration", "request duration", "s"))
				assert.Nil(t, metrics.HistogramWithBuckets("duration", "request duration", "s", []float64{0.1, 1}))
				assert.Nil(t, metrics.UpDownCounter("active", "active requests", "{request}"))
				require.NoError(t, metrics.Gauge("load", "runtime load", "1", callback))
				assert.Equal(t, float64(1), callback(ctx))
			},
		},
		{
			name: "observability shutdown propagates errors",
			run: func(t *testing.T) {
				t.Helper()

				ctx := context.Background()
				expected := errors.New("shutdown failed")
				o11y := mocks.NewMockObservability(t)
				o11y.EXPECT().Shutdown(ctx).Return(expected)

				assert.ErrorIs(t, o11y.Shutdown(ctx), expected)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.run(t)
		})
	}
}
