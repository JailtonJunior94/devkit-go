package otel

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
)

type runtimeState uint32

const (
	runtimeStateUninitialized runtimeState = iota
	runtimeStateRunning
	runtimeStateShuttingDown
	runtimeStateStopped
)

type runtime struct {
	config      *Config
	provider    *Provider
	propagator  propagation.TextMapPropagator
	propagation *propagationRuntime
	http        HTTPInstrumentation
	shutdown    *shutdownCoordinator
	state       atomic.Uint32
}

func newRuntime(ctx context.Context, config *Config) (*runtime, error) {
	policy := observability.DefaultShutdownPolicy()
	rt := &runtime{
		config:   config,
		shutdown: newShutdownCoordinator(policy),
	}
	rt.state.Store(uint32(runtimeStateUninitialized))

	provider := &Provider{
		config:  config,
		runtime: rt,
	}
	rt.provider = provider

	res, err := provider.createResource(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	if err := rt.initializeProviders(ctx, res); err != nil {
		return nil, err
	}
	rt.initializePropagation()
	rt.initializeComponents()
	rt.state.Store(uint32(runtimeStateRunning))

	return rt, nil
}

func (r *runtime) initializeProviders(ctx context.Context, res *resource.Resource) error {
	if err := r.provider.initTracerProvider(ctx, res); err != nil {
		return fmt.Errorf("failed to initialize tracer provider: %w", err)
	}
	if err := r.provider.initMeterProvider(ctx, res); err != nil {
		return fmt.Errorf("failed to initialize meter provider: %w", err)
	}
	if err := r.provider.initLoggerProvider(ctx, res); err != nil {
		return fmt.Errorf("failed to initialize logger provider: %w", err)
	}
	return nil
}

func (r *runtime) initializePropagation() {
	r.propagator = propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	r.propagation = newPropagationRuntime(r.propagator, configuredPropagationHeaders(r.config))
	if r.config.RegisterGlobal {
		otel.SetTextMapPropagator(r.propagator)
	}
}

func (r *runtime) initializeComponents() {
	cfg := r.config
	r.provider.tracer = newOtelTracer(r.provider.tracerProvider.Tracer(cfg.ServiceName))
	r.provider.logger = newOtelLogger(
		cfg.LogLevel,
		cfg.LogFormat,
		cfg.ServiceName,
		cfg.Environment,
		r.provider.loggerProvider,
		cfg.Sanitize,
		cfg.ConsoleLog,
	)
	r.provider.metrics = newOtelMetrics(
		r.provider.meterProvider.Meter(cfg.ServiceName),
		cfg.MetricNamespace,
		buildCardinalityValidator(cfg),
		_defaultMetricsErrorHandler,
	)
	r.http = newHTTPInstrumentation(r.provider.tracer, r.provider.metrics)
}

func buildCardinalityValidator(cfg *Config) *observability.CardinalityValidator {
	if !cfg.EnableCardinalityCheck {
		return nil
	}
	if len(cfg.CustomBlockedLabels) > 0 {
		return observability.NewCardinalityValidatorWithCustomLabels(true, cfg.CustomBlockedLabels)
	}
	return observability.NewCardinalityValidator(true)
}

var _defaultMetricsErrorHandler = func(op string, err error) {
	slog.Default().Error("observability metrics error", "operation", op, "error", err)
}

func (r *runtime) observability() *Provider {
	return r.provider
}

func (r *runtime) Shutdown(ctx context.Context) error {
	if !r.state.CompareAndSwap(uint32(runtimeStateRunning), uint32(runtimeStateShuttingDown)) {
		return r.shutdown.Shutdown(ctx)
	}

	err := r.shutdown.Shutdown(ctx)
	r.state.Store(uint32(runtimeStateStopped))
	return err
}

func (r *runtime) currentState() runtimeState {
	return runtimeState(r.state.Load())
}
