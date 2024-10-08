package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/JailtonJunior94/devkit-go/pkg/httpserver"
)

func main() {
	routes := []httpserver.Route{
		httpserver.NewRoute(http.MethodGet, "/helloaaa", func(w http.ResponseWriter, r *http.Request) error {
			requestID := r.Context().Value(httpserver.ContextKeyRequestID).(string)
			w.Write([]byte(requestID))

			return errors.New("error")
		}),
		httpserver.NewRoute(http.MethodPost, "/hello", func(w http.ResponseWriter, r *http.Request) error {
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
