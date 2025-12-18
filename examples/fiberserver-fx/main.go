package main

import (
	"github.com/JailtonJunior94/devkit-go/examples/fiberserver-fx/handlers"
	"github.com/JailtonJunior94/devkit-go/examples/fiberserver-fx/middlewares"
	"github.com/JailtonJunior94/devkit-go/examples/fiberserver-fx/routes"
	fiberserverfx "github.com/JailtonJunior94/devkit-go/pkg/fiberserver/fx"
	"github.com/JailtonJunior94/devkit-go/pkg/httpclient"
	httpclientfx "github.com/JailtonJunior94/devkit-go/pkg/httpclient/fx"
	"github.com/JailtonJunior94/devkit-go/pkg/messaging"
	o11y "github.com/JailtonJunior94/devkit-go/pkg/telemetry"
	telemetryfx "github.com/JailtonJunior94/devkit-go/pkg/telemetry/fx"

	"go.uber.org/fx"
)

func main() {
	fx.New(
		// ============================================
		// Infrastructure Modules
		// ============================================

		// Telemetry (NoOp for this example - use Module for production)
		telemetryfx.NoOpModule,

		// HTTP Client with retry
		httpclientfx.ModuleWithRetry,
		fx.Supply(httpclientfx.Config{
			MaxRetries:  3,
			BackoffTime: 1000000000, // 1 second in nanoseconds
		}),

		// Fiber Server
		fiberserverfx.Module,
		fx.Supply(fiberserverfx.Config{
			Port: "8080",
		}),

		// ============================================
		// Application Modules
		// ============================================

		// Middlewares
		middlewares.Module,

		// Routes
		routes.Module,

		// Handlers
		fx.Provide(
			handlers.NewHealthHandler,
			handlers.NewUserHandler,
			provideOrderHandler,
		),
	).Run()
}

// provideOrderHandler creates OrderHandler with optional publisher.
// Since we're not using RabbitMQ in this example, publisher will be nil.
func provideOrderHandler(
	telemetry o11y.Telemetry,
	httpClient httpclient.HTTPClient,
) *handlers.OrderHandler {
	// In a real app, you would inject the publisher from RabbitMQ or Kafka module
	var publisher messaging.Publisher = nil
	return handlers.NewOrderHandler(telemetry, httpClient, publisher)
}

// ============================================
// Alternative: Production Configuration
// ============================================
//
// For production with real telemetry:
//
// func main() {
//     fx.New(
//         // Real telemetry with OTEL
//         telemetryfx.ModuleInsecureWithConfig(o11y.Config{
//             ServiceName:     "order-api",
//             ServiceVersion:  "1.0.0",
//             Environment:     "development",
//             TracerEndpoint:  "localhost:4317",
//             MetricsEndpoint: "localhost:4317",
//             LoggerEndpoint:  "http://localhost:4318/v1/logs",
//         }),
//
//         // Or use environment variables:
//         // telemetryfx.ConfigModule,
//         // telemetryfx.ModuleInsecure,
//
//         // With RabbitMQ:
//         // rabbitmqfx.ConfigFromEnvModule,
//         // rabbitmqfx.Module,
//
//         // Rest of the modules...
//     ).Run()
// }

// ============================================
// Example with RabbitMQ Publisher
// ============================================
//
// import rabbitmqfx "github.com/JailtonJunior94/devkit-go/pkg/messaging/rabbitmq/fx"
//
// func main() {
//     fx.New(
//         telemetryfx.NoOpModule,
//         httpclientfx.ModuleWithRetry,
//         fiberserverfx.Module,
//
//         // RabbitMQ
//         rabbitmqfx.ConfigFromEnvModule,
//         rabbitmqfx.Module,
//
//         middlewares.Module,
//         routes.Module,
//
//         fx.Provide(
//             handlers.NewHealthHandler,
//             handlers.NewUserHandler,
//             handlers.NewOrderHandler, // Now receives Publisher from RabbitMQ
//         ),
//     ).Run()
// }
