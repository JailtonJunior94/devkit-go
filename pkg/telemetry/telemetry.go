package o11y

import (
	"context"
	"errors"
	"time"
)

// ShutdownFunc is a function that shuts down a telemetry component.
type ShutdownFunc func(context.Context) error

type telemetry struct {
	tracer          Tracer
	metrics         Metrics
	logger          Logger
	shutdownTracer  ShutdownFunc
	shutdownMetrics ShutdownFunc
	shutdownLogger  ShutdownFunc
}

// NewTelemetry creates a new telemetry instance with all components.
// All components must be non-nil. Use the individual shutdown functions
// returned by NewTracer, NewMetrics, and NewLogger for proper cleanup.
func NewTelemetry(
	tracer Tracer,
	metrics Metrics,
	logger Logger,
	shutdownTracer ShutdownFunc,
	shutdownMetrics ShutdownFunc,
	shutdownLogger ShutdownFunc,
) (Telemetry, error) {
	if tracer == nil {
		return nil, errors.New("tracer cannot be nil")
	}
	if metrics == nil {
		return nil, errors.New("metrics cannot be nil")
	}
	if logger == nil {
		return nil, errors.New("logger cannot be nil")
	}

	return &telemetry{
		tracer:          tracer,
		metrics:         metrics,
		logger:          logger,
		shutdownTracer:  shutdownTracer,
		shutdownMetrics: shutdownMetrics,
		shutdownLogger:  shutdownLogger,
	}, nil
}

func (t *telemetry) Tracer() Tracer {
	return t.tracer
}

func (t *telemetry) Metrics() Metrics {
	return t.metrics
}

func (t *telemetry) Logger() Logger {
	return t.logger
}

// Shutdown gracefully shuts down all telemetry components.
// It attempts to shut down all components even if one fails,
// and returns a combined error of all failures.
func (t *telemetry) Shutdown(ctx context.Context) error {
	var errs []error

	if t.shutdownTracer != nil {
		if err := t.shutdownTracer(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	if t.shutdownMetrics != nil {
		if err := t.shutdownMetrics(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	if t.shutdownLogger != nil {
		if err := t.shutdownLogger(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// ShutdownWithTimeout shuts down all telemetry components with a timeout.
func (t *telemetry) ShutdownWithTimeout(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return t.Shutdown(ctx)
}
