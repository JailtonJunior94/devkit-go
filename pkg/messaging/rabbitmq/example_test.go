package rabbitmq_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging/rabbitmq"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Example_completeWorkflow demonstra uso completo do cliente RabbitMQ.
// Inclui: client, publisher, consumer, exchanges, queues, bindings.
func Example_completeWorkflow() {
	ctx := context.Background()
	o11y := noop.NewProvider()

	// 1. Criar cliente com CloudStrategy (produção)
	client, err := rabbitmq.New(
		o11y,
		rabbitmq.WithCloudConnection(os.Getenv("RABBITMQ_URL")),
		rabbitmq.WithServiceName("example-service"),
		rabbitmq.WithServiceVersion("1.0.0"),
		rabbitmq.WithEnvironment("production"),
		rabbitmq.WithPublisherConfirms(true),
		rabbitmq.WithAutoReconnect(true),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Shutdown(context.Background())

	// 2. Declarar topologia (exchanges, queues, bindings)
	if err := setupTopology(ctx, client); err != nil {
		log.Fatal(err)
	}

	// 3. Criar e configurar publisher
	publisher := rabbitmq.NewPublisher(client)

	// 4. Publicar mensagem
	event := UserCreatedEvent{
		UserID:    "123",
		Email:     "user@example.com",
		CreatedAt: time.Now(),
	}

	body, _ := json.Marshal(event)
	if err := publisher.Publish(
		ctx,
		"events",
		"user.created",
		body,
		rabbitmq.WithHeaders(map[string]interface{}{
			"event_type": "user.created",
			"version":    "1.0",
		}),
		rabbitmq.WithMessageID("msg-123"),
	); err != nil {
		log.Fatal(err)
	}

	// 5. Criar e configurar consumer
	consumer := rabbitmq.NewConsumer(
		client,
		rabbitmq.WithQueue("user-events"),
		rabbitmq.WithPrefetchCount(10),
		rabbitmq.WithWorkerPool(5),
	)

	// 6. Registrar handlers
	consumer.RegisterHandler("user.created", handleUserCreated)
	consumer.RegisterHandler("user.updated", handleUserUpdated)

	// 7. Iniciar consumo em goroutine
	go func() {
		if err := consumer.Consume(ctx); err != nil {
			log.Printf("Consumer error: %v", err)
		}
	}()

	// 8. Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := consumer.Close(); err != nil {
		log.Printf("Consumer close error: %v", err)
	}

	if err := client.Shutdown(shutdownCtx); err != nil {
		log.Printf("Client shutdown error: %v", err)
	}
}

// Example_developmentLocal demonstra uso com PlainStrategy (desenvolvimento local).
func Example_developmentLocal() {
	ctx := context.Background()
	o11y := noop.NewProvider()

	// Cliente para desenvolvimento local (Docker)
	client, err := rabbitmq.New(
		o11y,
		rabbitmq.WithPlainConnection("localhost", "guest", "guest", "/"),
		rabbitmq.WithServiceName("dev-service"),
		rabbitmq.WithServiceVersion("dev"),
		rabbitmq.WithEnvironment("development"),
		rabbitmq.WithAutoReconnect(false), // Desabilita reconexão em dev
	)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Shutdown(ctx)

	fmt.Println("Connected to local RabbitMQ")
}

// Example_tlsConnection demonstra uso com TLSStrategy (certificados customizados).
func Example_tlsConnection() {
	ctx := context.Background()
	o11y := noop.NewProvider()

	// Cliente com TLS customizado
	client, err := rabbitmq.New(
		o11y,
		rabbitmq.WithTLSConnection(
			"rabbitmq.example.com",
			"user",
			"pass",
			"/production",
			"/path/to/ca.pem",
			"/path/to/client-cert.pem",
			"/path/to/client-key.pem",
		),
		rabbitmq.WithServiceName("secure-service"),
		rabbitmq.WithServiceVersion("1.0.0"),
		rabbitmq.WithEnvironment("production"),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Shutdown(ctx)

	fmt.Println("Connected with TLS")
}

// Example_healthCheckIntegration demonstra integração com HTTP server health checks.
func Example_healthCheckIntegration() {
	o11y := noop.NewProvider()

	client, err := rabbitmq.New(
		o11y,
		rabbitmq.WithCloudConnection(os.Getenv("RABBITMQ_URL")),
		rabbitmq.WithServiceName("api-service"),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Shutdown(context.Background())

	// Integração com HTTP server (pkg/http_server/server_fiber)
	healthChecks := map[string]func(context.Context) error{
		"rabbitmq": client.HealthCheck(),
	}

	// Usar healthChecks no servidor HTTP
	_ = healthChecks

	fmt.Println("Health check registered")
}

// Example_publisherWithRetry demonstra publish com retry manual.
func Example_publisherWithRetry() {
	ctx := context.Background()
	o11y := noop.NewProvider()

	client, err := rabbitmq.New(
		o11y,
		rabbitmq.WithCloudConnection(os.Getenv("RABBITMQ_URL")),
		rabbitmq.WithServiceName("publisher-service"),
		rabbitmq.WithPublisherConfirms(true),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Shutdown(ctx)

	publisher := rabbitmq.NewPublisher(client)

	// Publish com retry manual
	maxRetries := 3
	var publishErr error

	for i := 0; i < maxRetries; i++ {
		publishErr = publisher.Publish(
			ctx,
			"events",
			"user.created",
			[]byte(`{"user_id": "123"}`),
		)

		if publishErr == nil {
			break
		}

		log.Printf("Publish failed (attempt %d/%d): %v", i+1, maxRetries, publishErr)
		time.Sleep(time.Second * time.Duration(i+1))
	}

	if publishErr != nil {
		log.Fatal("Failed to publish after retries")
	}

	fmt.Println("Message published successfully")
}

// setupTopology configura exchanges, queues e bindings.
func setupTopology(ctx context.Context, client *rabbitmq.Client) error {
	// Declarar exchange
	if err := client.DeclareExchange(ctx, "events", "topic", true, false, nil); err != nil {
		return err
	}

	// Declarar queue com DLQ
	dlqArgs := amqp.Table{
		"x-dead-letter-exchange":    "events.dlq",
		"x-dead-letter-routing-key": "user.failed",
	}

	if _, err := client.DeclareQueue(ctx, "user-events", true, false, false, dlqArgs); err != nil {
		return err
	}

	// Declarar DLQ
	if _, err := client.DeclareQueue(ctx, "user-events.dlq", true, false, false, nil); err != nil {
		return err
	}

	// Binding queue -> exchange
	if err := client.BindQueue(ctx, "user-events", "user.*", "events", nil); err != nil {
		return err
	}

	return nil
}

// UserCreatedEvent representa evento de criação de usuário.
type UserCreatedEvent struct {
	UserID    string    `json:"user_id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// handleUserCreated processa evento user.created.
func handleUserCreated(ctx context.Context, msg rabbitmq.Message) error {
	var event UserCreatedEvent
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	log.Printf("User created: %s (%s)", event.UserID, event.Email)
	return nil
}

// handleUserUpdated processa evento user.updated.
func handleUserUpdated(ctx context.Context, msg rabbitmq.Message) error {
	log.Printf("User updated: routing_key=%s", msg.RoutingKey)
	return nil
}
