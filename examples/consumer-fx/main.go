package main

import (
	"log"
	"time"

	"github.com/JailtonJunior94/devkit-go/examples/consumer-fx/handlers"
	"github.com/JailtonJunior94/devkit-go/pkg/messaging/rabbitmq"
	rabbitmqfx "github.com/JailtonJunior94/devkit-go/pkg/messaging/rabbitmq/fx"
	telemetryfx "github.com/JailtonJunior94/devkit-go/pkg/telemetry/fx"

	"go.uber.org/fx"
)

func main() {
	log.Println("Starting RabbitMQ Consumer with Uber FX...")

	fx.New(
		// ============================================
		// Infrastructure Modules
		// ============================================

		// Telemetry (NoOp for this example)
		telemetryfx.NoOpModule,

		// RabbitMQ Configuration
		fx.Supply(&rabbitmq.Config{
			URL:                  "amqp://guest:guest@localhost:5672/",
			MaxConnections:       5,
			MaxChannels:          50,
			HeartbeatInterval:    60 * time.Second,
			ReconnectDelay:       5 * time.Second,
			MaxReconnectAttempts: 10,
			PrefetchCount:        10,
			PrefetchSize:         0,
		}),

		// RabbitMQ Module (Connection Pool + Publisher)
		rabbitmqfx.Module,

		// Consumer Configuration
		fx.Supply(rabbitmqfx.ConsumerConfig{
			Name:        "order-consumer",
			Queue:       "orders",
			Prefetch:    10,
			AutoAck:     false,
			Exclusive:   false,
			NoLocal:     false,
			NoWait:      false,
			WorkerCount: 3, // Use worker pool with 3 workers
		}),

		// Consumer Module
		rabbitmqfx.ConsumerModule,

		// ============================================
		// Application
		// ============================================

		// Handler
		fx.Provide(handlers.NewOrderHandler),

		// Register message handlers
		fx.Provide(fx.Annotate(
			provideOrderCreatedHandler,
			fx.ResultTags(`group:"consumer_handlers"`),
		)),
		fx.Provide(fx.Annotate(
			provideOrderUpdatedHandler,
			fx.ResultTags(`group:"consumer_handlers"`),
		)),
		fx.Provide(fx.Annotate(
			provideOrderCancelledHandler,
			fx.ResultTags(`group:"consumer_handlers"`),
		)),

		// Log startup
		fx.Invoke(func() {
			log.Println("Consumer started and listening for messages...")
		}),
	).Run()
}

// Handler providers - map event types to handlers
func provideOrderCreatedHandler(h *handlers.OrderHandler) rabbitmqfx.Handler {
	return rabbitmqfx.Handler{
		EventType: "order.created",
		Handler:   h.HandleOrderCreated,
	}
}

func provideOrderUpdatedHandler(h *handlers.OrderHandler) rabbitmqfx.Handler {
	return rabbitmqfx.Handler{
		EventType: "order.updated",
		Handler:   h.HandleOrderUpdated,
	}
}

func provideOrderCancelledHandler(h *handlers.OrderHandler) rabbitmqfx.Handler {
	return rabbitmqfx.Handler{
		EventType: "order.cancelled",
		Handler:   h.HandleOrderCancelled,
	}
}

// ============================================
// Alternative: Using Environment Variables
// ============================================
//
// func main() {
//     fx.New(
//         telemetryfx.ConfigModule,
//         telemetryfx.ModuleInsecure,
//
//         rabbitmqfx.ConfigFromEnvModule,
//         rabbitmqfx.Module,
//         rabbitmqfx.ConsumerModule,
//
//         // Consumer config from env
//         fx.Supply(rabbitmqfx.ConsumerConfig{
//             Name:        os.Getenv("CONSUMER_NAME"),
//             Queue:       os.Getenv("QUEUE_NAME"),
//             Prefetch:    10,
//             WorkerCount: 3,
//         }),
//
//         fx.Provide(handlers.NewOrderHandler),
//         // ... handlers
//     ).Run()
// }
