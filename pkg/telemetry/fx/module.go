package telemetryfx

import (
	"context"

	o11y "github.com/JailtonJunior94/devkit-go/pkg/telemetry"
	"go.uber.org/fx"
)

// Module provides all telemetry components for FX dependency injection.
// Usage:
//
//	fx.New(
//	    telemetryfx.Module,
//	    fx.Provide(func() o11y.Config { return config }),
//	)
var Module = fx.Module("telemetry",
	fx.Provide(
		ProvideTelemetry,
	),
)

// ModuleWithConfig provides telemetry with inline config.
// Usage:
//
//	fx.New(
//	    telemetryfx.ModuleWithConfig(o11y.Config{
//	        ServiceName: "my-service",
//	        ...
//	    }),
//	)
func ModuleWithConfig(cfg o11y.Config) fx.Option {
	return fx.Module("telemetry",
		fx.Supply(cfg),
		fx.Provide(ProvideTelemetry),
	)
}

// ModuleInsecure provides telemetry with TLS disabled for development.
// Usage:
//
//	fx.New(
//	    telemetryfx.ModuleInsecure,
//	    fx.Provide(func() o11y.Config { return config }),
//	)
var ModuleInsecure = fx.Module("telemetry-insecure",
	fx.Provide(ProvideTelemetryInsecure),
)

// ModuleInsecureWithConfig provides telemetry with TLS disabled and inline config.
func ModuleInsecureWithConfig(cfg o11y.Config) fx.Option {
	return fx.Module("telemetry-insecure",
		fx.Supply(cfg),
		fx.Provide(ProvideTelemetryInsecure),
	)
}

// NoOpModule provides NoOp telemetry for testing.
// Usage:
//
//	fx.New(
//	    telemetryfx.NoOpModule,
//	)
var NoOpModule = fx.Module("telemetry-noop",
	fx.Provide(
		ProvideNoOpTelemetry,
		ProvideNoOpTracer,
		ProvideNoOpMetrics,
		ProvideNoOpLogger,
	),
)

// TelemetryParams contains dependencies for creating Telemetry.
type TelemetryParams struct {
	fx.In

	Config o11y.Config
	LC     fx.Lifecycle
}

// TelemetryResult contains the Telemetry output with individual components.
type TelemetryResult struct {
	fx.Out

	Telemetry o11y.Telemetry
	Tracer    o11y.Tracer
	Metrics   o11y.Metrics
	Logger    o11y.Logger
}

// ProvideTelemetry creates a new Telemetry instance with lifecycle management.
func ProvideTelemetry(p TelemetryParams) (TelemetryResult, error) {
	ctx := context.Background()

	tel, err := o11y.SetupTelemetry(
		ctx,
		p.Config,
		[]o11y.TracerOption{},
		[]o11y.MetricsOption{},
		[]o11y.LoggerOption{},
	)
	if err != nil {
		return TelemetryResult{}, err
	}

	p.LC.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return tel.Shutdown(ctx)
		},
	})

	return TelemetryResult{
		Telemetry: tel,
		Tracer:    tel.Tracer(),
		Metrics:   tel.Metrics(),
		Logger:    tel.Logger(),
	}, nil
}

// ProvideTelemetryInsecure creates a new Telemetry instance with TLS disabled.
func ProvideTelemetryInsecure(p TelemetryParams) (TelemetryResult, error) {
	ctx := context.Background()

	tel, err := o11y.SetupTelemetryInsecure(ctx, p.Config)
	if err != nil {
		return TelemetryResult{}, err
	}

	p.LC.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return tel.Shutdown(ctx)
		},
	})

	return TelemetryResult{
		Telemetry: tel,
		Tracer:    tel.Tracer(),
		Metrics:   tel.Metrics(),
		Logger:    tel.Logger(),
	}, nil
}

// ProvideNoOpTelemetry creates a no-op telemetry instance for testing.
func ProvideNoOpTelemetry() o11y.Telemetry {
	return o11y.NewNoOpTelemetry()
}

// ProvideNoOpTracer creates a no-op tracer instance for testing.
func ProvideNoOpTracer() o11y.Tracer {
	return o11y.NewNoOpTracer()
}

// ProvideNoOpMetrics creates a no-op metrics instance for testing.
func ProvideNoOpMetrics() o11y.Metrics {
	return o11y.NewNoOpMetrics()
}

// ProvideNoOpLogger creates a no-op logger instance for testing.
func ProvideNoOpLogger() o11y.Logger {
	return o11y.NewNoOpLogger()
}
