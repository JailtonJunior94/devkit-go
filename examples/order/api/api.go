package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/httpserver"
	"github.com/JailtonJunior94/devkit-go/pkg/messaging"
	"github.com/JailtonJunior94/devkit-go/pkg/messaging/rabbitmq"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	OrdersExchange = "order"
	OrderQueue     = "order"
	OrderCreated   = "order_created"
	OrderUpdated   = "order_updated"
	FinanceQueue   = "finance_order"
)

var (
	Exchanges = []*rabbitmq.Exchange{
		rabbitmq.NewExchange(OrdersExchange, "direct"),
	}

	Bindings = []*rabbitmq.Binding{
		rabbitmq.NewBindingRouting(OrderQueue, OrdersExchange, OrderCreated),
		rabbitmq.NewBindingRouting(OrderQueue, OrdersExchange, OrderUpdated),
		rabbitmq.NewBindingRouting(FinanceQueue, OrdersExchange, OrderCreated),
	}
)

type order struct {
	ID     int     `json:"id"`
	Status string  `json:"status"`
	Value  float64 `json:"value"`
}

type apiServer struct {
}

func NewApiServer() *apiServer {
	return &apiServer{}
}

func (s *apiServer) Run() {
	connection, err := amqp.Dial("amqp://guest:pass@rabbitmq@localhost:5672")
	if err != nil {
		log.Fatal(err)
	}
	defer connection.Close()

	channel, err := connection.Channel()
	if err != nil {
		log.Fatal(err)
	}
	defer channel.Close()

	_, err = rabbitmq.NewAmqpBuilder(channel).
		DeclareExchanges(Exchanges...).
		DeclareBindings(Bindings...).
		DeclarePrefetchCount(5).
		WithDLQ().
		WithRetry().
		DeclareTTL(3 * time.Second).
		Apply()

	if err != nil {
		log.Fatal(err)
	}
	producer := rabbitmq.NewRabbitMQ(channel)

	routes := []httpserver.Route{
		httpserver.NewRoute(http.MethodPost, "/message", func(w http.ResponseWriter, r *http.Request) error {
			requestID := r.Context().Value(httpserver.ContextKeyRequestID).(string)
			params := map[string]string{
				"content_type": "application/json",
				"event_type":   OrderCreated,
				"request_id":   requestID,
			}

			order := &order{ID: 1, Status: "created", Value: 100.0}
			json, err := json.Marshal(order)
			if err != nil {
				return err
			}

			err = producer.Produce(r.Context(), OrderQueue, OrderCreated, params, &messaging.Message{
				Body: json,
			})

			if err != nil {
				return err
			}
			return nil
		}),
	}

	server := httpserver.New(
		httpserver.WithPort("8001"),
		httpserver.WithRoutes(routes...),
		httpserver.WithMiddlewares(
			httpserver.RequestID,
		),
	)

	shutdown := server.Run()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := <-server.ShutdownListener(); err != nil && err != http.ErrServerClosed {
			interrupt <- syscall.SIGTERM
		}
	}()

	<-interrupt
	if err := shutdown(context.Background()); err != nil {
		log.Fatal(err)
	}
}
