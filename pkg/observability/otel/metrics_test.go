package otel

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric"
	noopmeter "go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

func newTestMeterProvider(t *testing.T) *sdkmetric.MeterProvider {
	t.Helper()
	p := sdkmetric.NewMeterProvider()
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })
	return p
}

type failingMeter struct {
	noopmeter.Meter
	err error
}

func (m failingMeter) Int64Counter(_ string, _ ...metric.Int64CounterOption) (metric.Int64Counter, error) {
	return nil, m.err
}

func (m failingMeter) Float64Histogram(_ string, _ ...metric.Float64HistogramOption) (metric.Float64Histogram, error) {
	return nil, m.err
}

func (m failingMeter) Int64UpDownCounter(_ string, _ ...metric.Int64UpDownCounterOption) (metric.Int64UpDownCounter, error) {
	return nil, m.err
}

func (m failingMeter) Float64ObservableGauge(_ string, _ ...metric.Float64ObservableGaugeOption) (metric.Float64ObservableGauge, error) {
	return nil, m.err
}

func TestOtelMetrics_InstrumentCreationFailure_OnErrorCalled(t *testing.T) {
	t.Parallel()

	meterErr := errors.New("meter: synthetic creation failure")
	fm := failingMeter{err: meterErr}

	tests := []struct {
		name    string
		factory func(m *otelMetrics) observability.Counter
		wantOp  string
	}{
		{
			name: "Counter creation failure calls onError and returns noop",
			factory: func(m *otelMetrics) observability.Counter {
				return m.Counter("fail.counter", "desc", "{req}")
			},
			wantOp: "metrics.Counter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var capturedOp string
			var capturedErr error

			m := newOtelMetrics(fm, "", nil, func(op string, err error) {
				capturedOp = op
				capturedErr = err
			})

			instrument := tt.factory(m)

			require.NotNil(t, instrument)
			assert.Equal(t, tt.wantOp, capturedOp)
			require.Error(t, capturedErr)
			assert.ErrorIs(t, capturedErr, meterErr)
		})
	}
}

func TestOtelMetrics_InstrumentCreationFailure_AllInstrumentTypes(t *testing.T) {
	t.Parallel()

	meterErr := errors.New("meter: synthetic failure")
	fm := failingMeter{err: meterErr}

	tests := []struct {
		name   string
		call   func(m *otelMetrics)
		wantOp string
	}{
		{
			name:   "Counter",
			call:   func(m *otelMetrics) { require.NotNil(t, m.Counter("x", "", "")) },
			wantOp: "metrics.Counter",
		},
		{
			name:   "Histogram",
			call:   func(m *otelMetrics) { require.NotNil(t, m.Histogram("x", "", "")) },
			wantOp: "metrics.Histogram",
		},
		{
			name:   "HistogramWithBuckets",
			call:   func(m *otelMetrics) { require.NotNil(t, m.HistogramWithBuckets("x", "", "", nil)) },
			wantOp: "metrics.HistogramWithBuckets",
		},
		{
			name:   "UpDownCounter",
			call:   func(m *otelMetrics) { require.NotNil(t, m.UpDownCounter("x", "", "")) },
			wantOp: "metrics.UpDownCounter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var capturedOp string
			var capturedErr error

			m := newOtelMetrics(fm, "", nil, func(op string, err error) {
				capturedOp = op
				capturedErr = err
			})

			tt.call(m)

			assert.Equal(t, tt.wantOp, capturedOp)
			require.Error(t, capturedErr)
			assert.ErrorIs(t, capturedErr, meterErr)
		})
	}
}

func TestOtelMetrics_CardinalityEnforced_Counter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		fields        []observability.Field
		wantError     bool
		errorContains string
	}{
		{
			name:          "high cardinality user_id blocked",
			fields:        []observability.Field{observability.String("user_id", "12345")},
			wantError:     true,
			errorContains: "user_id",
		},
		{
			name:          "high cardinality session_id blocked",
			fields:        []observability.Field{observability.String("session_id", "abc")},
			wantError:     true,
			errorContains: "session_id",
		},
		{
			name: "low cardinality labels allowed",
			fields: []observability.Field{
				observability.String("status", "success"),
				observability.String("method", "GET"),
			},
			wantError: false,
		},
		{
			name:      "no fields allowed",
			fields:    nil,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var capturedOp string
			var capturedErr error

			p := newTestMeterProvider(t)
			validator := observability.NewCardinalityValidator(true)
			m := newOtelMetrics(p.Meter("test"), "", validator, func(op string, err error) {
				capturedOp = op
				capturedErr = err
			})

			counter := m.Counter("test.counter", "test", "{req}")
			require.NotNil(t, counter)

			counter.Add(context.Background(), 1, tt.fields...)

			if tt.wantError {
				assert.Equal(t, "counter.Add", capturedOp)
				require.Error(t, capturedErr)
				assert.True(t, errors.Is(capturedErr, observability.ErrCardinalityViolation))
				if tt.errorContains != "" {
					assert.Contains(t, capturedErr.Error(), tt.errorContains)
				}
			} else {
				assert.Empty(t, capturedOp)
				assert.NoError(t, capturedErr)
			}
		})
	}
}

func TestOtelMetrics_CardinalityEnforced_Histogram(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		fields    []observability.Field
		wantError bool
	}{
		{
			name:      "trace_id blocked",
			fields:    []observability.Field{observability.String("trace_id", "trace-123")},
			wantError: true,
		},
		{
			name:      "route label allowed",
			fields:    []observability.Field{observability.String("http.route", "/api/v1/orders")},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var capturedErr error

			p := newTestMeterProvider(t)
			validator := observability.NewCardinalityValidator(true)
			m := newOtelMetrics(p.Meter("test"), "", validator, func(_ string, err error) {
				capturedErr = err
			})

			h := m.Histogram("test.histogram", "test", "ms")
			require.NotNil(t, h)

			h.Record(context.Background(), 100.0, tt.fields...)

			if tt.wantError {
				require.Error(t, capturedErr)
				assert.True(t, errors.Is(capturedErr, observability.ErrCardinalityViolation))
			} else {
				assert.NoError(t, capturedErr)
			}
		})
	}
}

func TestOtelMetrics_CardinalityEnforced_UpDownCounter(t *testing.T) {
	t.Parallel()

	var capturedErr error

	p := newTestMeterProvider(t)
	validator := observability.NewCardinalityValidator(true)
	m := newOtelMetrics(p.Meter("test"), "", validator, func(_ string, err error) {
		capturedErr = err
	})

	updown := m.UpDownCounter("test.updown", "test", "{conn}")
	require.NotNil(t, updown)

	updown.Add(context.Background(), 1, observability.String("request_id", "req-001"))

	require.Error(t, capturedErr)
	assert.True(t, errors.Is(capturedErr, observability.ErrCardinalityViolation))
	assert.Contains(t, capturedErr.Error(), "request_id")
}

func TestOtelMetrics_CardinalityDisabled_AllLabelsAllowed(t *testing.T) {
	t.Parallel()

	var errorCalled bool

	p := newTestMeterProvider(t)
	m := newOtelMetrics(p.Meter("test"), "", nil, func(_ string, _ error) {
		errorCalled = true
	})

	counter := m.Counter("test.counter", "test", "{req}")
	counter.Add(context.Background(), 1,
		observability.String("user_id", "12345"),
		observability.String("session_id", "abc"),
	)

	assert.False(t, errorCalled, "onError must not be called when cardinality check is disabled")
}

func TestOtelMetrics_CustomBlockedLabels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		label         string
		wantError     bool
		errorContains string
	}{
		{
			name:          "custom label customer_id blocked",
			label:         "customer_id",
			wantError:     true,
			errorContains: "customer_id",
		},
		{
			name:      "standard label status allowed",
			label:     "status",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var capturedErr error

			p := newTestMeterProvider(t)
			validator := observability.NewCardinalityValidatorWithCustomLabels(true, []string{"customer_id", "order_id"})
			m := newOtelMetrics(p.Meter("test"), "", validator, func(_ string, err error) {
				capturedErr = err
			})

			counter := m.Counter("test.counter", "test", "{req}")
			counter.Add(context.Background(), 1, observability.String(tt.label, "value"))

			if tt.wantError {
				require.Error(t, capturedErr)
				assert.True(t, errors.Is(capturedErr, observability.ErrCardinalityViolation))
				assert.Contains(t, capturedErr.Error(), tt.errorContains)
			} else {
				assert.NoError(t, capturedErr)
			}
		})
	}
}

func TestOtelMetrics_HistogramWithBuckets_CardinalityEnforced(t *testing.T) {
	t.Parallel()

	var capturedErr error

	p := newTestMeterProvider(t)
	validator := observability.NewCardinalityValidator(true)
	m := newOtelMetrics(p.Meter("test"), "", validator, func(_ string, err error) {
		capturedErr = err
	})

	h := m.HistogramWithBuckets("test.latency", "request latency", "s", []float64{0.001, 0.01, 0.1, 0.5, 1.0, 5.0})
	require.NotNil(t, h)

	h.Record(context.Background(), 0.05, observability.String("ip_address", "192.168.1.1"))

	require.Error(t, capturedErr)
	assert.True(t, errors.Is(capturedErr, observability.ErrCardinalityViolation))
}

func TestOtelMetrics_InstrumentCreation_AlwaysReturnsNonNil(t *testing.T) {
	t.Parallel()

	p := newTestMeterProvider(t)
	m := newOtelMetrics(p.Meter("test"), "", nil, nil)

	assert.NotNil(t, m.Counter("nonnull.counter", "desc", "{req}"))
	assert.NotNil(t, m.Histogram("nonnull.hist", "desc", "ms"))
	assert.NotNil(t, m.HistogramWithBuckets("nonnull.hist.b", "desc", "ms", []float64{1, 5, 10}))
	assert.NotNil(t, m.UpDownCounter("nonnull.updown", "desc", "{conn}"))
}

func TestOtelMetrics_DefaultOnError_UsedWhenNil(t *testing.T) {
	t.Parallel()

	p := newTestMeterProvider(t)
	m := newOtelMetrics(p.Meter("test"), "", nil, nil)

	assert.NotNil(t, m.onError, "default onError must be set when nil is passed")
}

func TestOtelMetrics_Namespace_PrependedToName(t *testing.T) {
	t.Parallel()

	p := newTestMeterProvider(t)
	meter := p.Meter("test")

	assert.Equal(t, "myservice.requests", newOtelMetrics(meter, "myservice", nil, nil).addNamespace("requests"))
	assert.Equal(t, "latency", newOtelMetrics(meter, "", nil, nil).addNamespace("latency"))
}

func TestBuildCardinalityValidator(t *testing.T) {
	t.Parallel()

	t.Run("disabled returns nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, buildCardinalityValidator(&Config{EnableCardinalityCheck: false}))
	})

	t.Run("enabled returns default validator", func(t *testing.T) {
		t.Parallel()
		v := buildCardinalityValidator(&Config{EnableCardinalityCheck: true})
		require.NotNil(t, v)
		assert.True(t, v.IsBlocked("user_id"))
	})

	t.Run("enabled with custom labels", func(t *testing.T) {
		t.Parallel()
		v := buildCardinalityValidator(&Config{
			EnableCardinalityCheck: true,
			CustomBlockedLabels:    []string{"tenant_id", "account_id"},
		})
		require.NotNil(t, v)
		assert.True(t, v.IsBlocked("tenant_id"))
		assert.True(t, v.IsBlocked("account_id"))
		assert.True(t, v.IsBlocked("user_id"))
		assert.False(t, v.IsBlocked("status"))
	})
}
