package main

import (
	"github.com/JailtonJunior94/devkit-go/examples/order/api"
	"github.com/JailtonJunior94/devkit-go/examples/order/consumer"
	"github.com/JailtonJunior94/devkit-go/examples/order/payment"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "order",
		Short: "Order microservices example with distributed tracing",
		Long: `This example demonstrates distributed tracing across microservices using:
- telemetry (o11y): OpenTelemetry-based tracing, metrics, and logging
- messaging (rabbitmq): Async communication with trace context propagation
- httpclient: HTTP calls between services with trace context injection
- httpserver/fiberserver: HTTP servers with request ID and tracing support

Architecture:
  [Client] -> [Order API] -> [Payment Service]
                   |
                   v
             [RabbitMQ]
                   |
                   v
           [Order Consumer]

All services share the same trace_id for end-to-end visibility.`,
	}

	orderAPI := &cobra.Command{
		Use:   "api",
		Short: "Order API - receives orders, calls payment service, publishes events",
		Long: `The Order API:
- Receives HTTP POST /orders requests
- Creates spans for order processing
- Calls Payment Service via HTTP (trace context injected in headers)
- Publishes order events to RabbitMQ (trace context in message headers)
- Exposes metrics and structured logs with trace correlation

Environment variables:
  PORT                         - HTTP port (default: 8001)
  RABBITMQ_URL                 - RabbitMQ connection URL
  PAYMENT_SERVICE_URL          - Payment service URL (default: http://localhost:8002)
  OTEL_EXPORTER_OTLP_ENDPOINT  - OTLP gRPC endpoint for traces/metrics
  OTEL_EXPORTER_OTLP_LOGS_ENDPOINT - OTLP HTTP endpoint for logs`,
		Run: func(cmd *cobra.Command, args []string) {
			api.NewApiServer().Run()
		},
	}

	paymentService := &cobra.Command{
		Use:   "payment",
		Short: "Payment Service - processes payments with trace continuation",
		Long: `The Payment Service:
- Receives HTTP POST /payments requests from Order API
- Extracts trace context from incoming HTTP headers
- Creates child spans continuing the distributed trace
- Validates and processes payments
- Returns transaction ID

Environment variables:
  PORT                         - HTTP port (default: 8002)
  OTEL_EXPORTER_OTLP_ENDPOINT  - OTLP gRPC endpoint for traces/metrics
  OTEL_EXPORTER_OTLP_LOGS_ENDPOINT - OTLP HTTP endpoint for logs`,
		Run: func(cmd *cobra.Command, args []string) {
			payment.NewPaymentServer().Run()
		},
	}

	consumers := &cobra.Command{
		Use:   "consumer",
		Short: "Order Consumer - processes order events with trace continuation",
		Long: `The Order Consumer:
- Consumes messages from RabbitMQ order queue
- Extracts trace context from message headers
- Creates child spans continuing the distributed trace
- Processes order_created and order_updated events
- Logs and records metrics with trace correlation

Environment variables:
  RABBITMQ_URL                 - RabbitMQ connection URL
  OTEL_EXPORTER_OTLP_ENDPOINT  - OTLP gRPC endpoint for traces/metrics
  OTEL_EXPORTER_OTLP_LOGS_ENDPOINT - OTLP HTTP endpoint for logs`,
		Run: func(cmd *cobra.Command, args []string) {
			consumer.NewConsumer().Run()
		},
	}

	root.AddCommand(orderAPI, paymentService, consumers)
	root.Execute()
}
