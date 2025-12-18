package telemetryfx

import (
	"context"

	o11y "github.com/JailtonJunior94/devkit-go/pkg/telemetry"
	"go.uber.org/fx"
)

// TracerModule provides only the Tracer component.
var TracerModule = fx.Module("telemetry-tracer",
	fx.Provide(ProvideTracer),
)

// MetricsModule provides only the Metrics component.
var MetricsModule = fx.Module("telemetry-metrics",
	fx.Provide(ProvideMetrics),
)

// LoggerModule provides only the Logger component.
var LoggerModule = fx.Module("telemetry-logger",
	fx.Provide(ProvideLogger),
)

// TracerParams contains dependencies for creating a standalone Tracer.
type TracerParams struct {
	fx.In

	Config  o11y.Config
	LC      fx.Lifecycle
	Options []o11y.TracerOption `group:"tracer_options" optional:"true"`
}

// TracerResult contains the Tracer output.
type TracerResult struct {
	fx.Out

	Tracer o11y.Tracer
}

// ProvideTracer creates a Tracer with lifecycle management.
func ProvideTracer(p TracerParams) (TracerResult, error) {
	ctx := context.Background()

	res, err := o11y.NewServiceResource(
		ctx,
		p.Config.ServiceName,
		p.Config.ServiceVersion,
		p.Config.Environment,
	)
	if err != nil {
		return TracerResult{}, err
	}

	tracer, shutdown, err := o11y.NewTracer(
		ctx,
		p.Config.TracerEndpoint,
		p.Config.ServiceName,
		res,
		p.Options...,
	)
	if err != nil {
		return TracerResult{}, err
	}

	p.LC.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return shutdown(ctx)
		},
	})

	return TracerResult{Tracer: tracer}, nil
}

// MetricsParams contains dependencies for creating a standalone Metrics.
type MetricsParams struct {
	fx.In

	Config  o11y.Config
	LC      fx.Lifecycle
	Options []o11y.MetricsOption `group:"metrics_options" optional:"true"`
}

// MetricsResult contains the Metrics output.
type MetricsResult struct {
	fx.Out

	Metrics o11y.Metrics
}

// ProvideMetrics creates Metrics with lifecycle management.
func ProvideMetrics(p MetricsParams) (MetricsResult, error) {
	ctx := context.Background()

	res, err := o11y.NewServiceResource(
		ctx,
		p.Config.ServiceName,
		p.Config.ServiceVersion,
		p.Config.Environment,
	)
	if err != nil {
		return MetricsResult{}, err
	}

	metrics, shutdown, err := o11y.NewMetrics(
		ctx,
		p.Config.MetricsEndpoint,
		p.Config.ServiceName,
		res,
		p.Options...,
	)
	if err != nil {
		return MetricsResult{}, err
	}

	p.LC.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return shutdown(ctx)
		},
	})

	return MetricsResult{Metrics: metrics}, nil
}

// LoggerParams contains dependencies for creating a standalone Logger.
type LoggerParams struct {
	fx.In

	Config  o11y.Config
	LC      fx.Lifecycle
	Tracer  o11y.Tracer          `optional:"true"`
	Options []o11y.LoggerOption `group:"logger_options" optional:"true"`
}

// LoggerResult contains the Logger output.
type LoggerResult struct {
	fx.Out

	Logger o11y.Logger
}

// ProvideLogger creates Logger with lifecycle management.
func ProvideLogger(p LoggerParams) (LoggerResult, error) {
	ctx := context.Background()

	res, err := o11y.NewServiceResource(
		ctx,
		p.Config.ServiceName,
		p.Config.ServiceVersion,
		p.Config.Environment,
	)
	if err != nil {
		return LoggerResult{}, err
	}

	tracer := p.Tracer
	if tracer == nil {
		tracer = o11y.NewNoOpTracer()
	}

	logger, shutdown, err := o11y.NewLogger(
		ctx,
		tracer,
		p.Config.LoggerEndpoint,
		p.Config.ServiceName,
		res,
		p.Options...,
	)
	if err != nil {
		return LoggerResult{}, err
	}

	p.LC.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return shutdown(ctx)
		},
	})

	return LoggerResult{Logger: logger}, nil
}

// ProvideTracerOption is a helper to provide tracer options.
// Usage:
//
//	fx.Provide(fx.Annotate(
//	    telemetryfx.ProvideTracerOption(o11y.WithTracerInsecure()),
//	    fx.ResultTags(`group:"tracer_options"`),
//	))
func ProvideTracerOption(opt o11y.TracerOption) func() o11y.TracerOption {
	return func() o11y.TracerOption {
		return opt
	}
}

// ProvideMetricsOption is a helper to provide metrics options.
// Usage:
//
//	fx.Provide(fx.Annotate(
//	    telemetryfx.ProvideMetricsOption(o11y.WithMetricsInsecure()),
//	    fx.ResultTags(`group:"metrics_options"`),
//	))
func ProvideMetricsOption(opt o11y.MetricsOption) func() o11y.MetricsOption {
	return func() o11y.MetricsOption {
		return opt
	}
}

// ProvideLoggerOption is a helper to provide logger options.
// Usage:
//
//	fx.Provide(fx.Annotate(
//	    telemetryfx.ProvideLoggerOption(o11y.WithLoggerInsecure()),
//	    fx.ResultTags(`group:"logger_options"`),
//	))
func ProvideLoggerOption(opt o11y.LoggerOption) func() o11y.LoggerOption {
	return func() o11y.LoggerOption {
		return opt
	}
}
