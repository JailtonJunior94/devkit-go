package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/JailtonJunior94/devkit-go/pkg/httpserver"
	"github.com/JailtonJunior94/devkit-go/pkg/messaging"
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

			err = producer.Publish(r.Context(), OrderQueue, OrderCreated, params, &messaging.Message{
				Body: json,
			})

			if err != nil {
				return err
			}
			return nil
		}),
	}

	server := httpserver.New(
		httpserver.WithPort("8002"),
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