package api

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging/rabbitmq"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
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

type apiServer struct {
}

func NewApiServer() *apiServer {
	return &apiServer{}
}

func (s *apiServer) Run() {
	router := chi.NewRouter()
	router.Use(
		middleware.RealIP,
		middleware.RequestID,
		middleware.SetHeader("Content-Type", "application/json"),
		middleware.AllowContentType("application/json", "application/x-www-form-urlencoded"),
	)

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

	router.Post("/orders", func(w http.ResponseWriter, r *http.Request) {
		if err := channel.PublishWithContext(context.Background(), OrdersExchange, OrderCreated, false, false, amqp.Publishing{
			ContentType: "application/json",
			Body:        []byte(`{"id": "1", "status": "created"}`),
		}); err != nil {
			responses.Error(w, http.StatusInternalServerError, err.Error())
		}
		responses.JSON(w, http.StatusCreated, nil)
	})

	/* Graceful shutdown */
	server := http.Server{
		ReadTimeout:       time.Duration(10) * time.Second,
		ReadHeaderTimeout: time.Duration(10) * time.Second,
		Handler:           router,
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", "8001"))
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	s.gracefulShutdown(&server)
}

func (s *apiServer) gracefulShutdown(server *http.Server) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctxShutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctxShutdown); err != nil {
		log.Fatal(err)
	}
}
