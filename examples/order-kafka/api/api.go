package api

import (
	"context"
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/JailtonJunior94/devkit-go/pkg/httpserver"
	"github.com/JailtonJunior94/devkit-go/pkg/messaging"
	"github.com/JailtonJunior94/devkit-go/pkg/messaging/kafka"
	"github.com/JailtonJunior94/devkit-go/pkg/vos"
)

type apiServer struct {
}

func NewApiServer() *apiServer {
	return &apiServer{}
}

type ParametrosConclusaoCorte struct {
	LogCorte int    `json:"logCorte"`
	Status   string `json:"status"`
}

func (s *apiServer) Run() {
	ctx := context.Background()

	broker, err := kafka.NewBroker(ctx, []string{"localhost:9092"}, vos.PlainText, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer broker.Close()

	producer, err := broker.NewProducerFromBroker()
	if err != nil {
		log.Fatal(err)
	}
	defer producer.Close()

	params, err := os.ReadFile("parametros.json")
	if err != nil {
		log.Fatal(err)
	}

	if err := producer.Publish(ctx, "selecao_corte", "", nil, &messaging.Message{Body: params}); err != nil {
		log.Fatal(err)
	}

	lotes := []int{12, 13, 16, 17, 18, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45}
	status := []string{"Sucesso", "ComErro", "SemMovimento", "JaCortadoAnteriormente"}

	for lote := range lotes {
		messages := []*messaging.Message{}

		for i := 1; i <= 1_000; i++ {
			message := ParametrosConclusaoCorte{
				LogCorte: lotes[lote],
				Status:   status[rand.Intn(len(status))],
			}

			jsonMessage, err := json.Marshal(message)
			if err != nil {
				log.Printf("Error marshaling message: %v", err)
				continue
			}

			msg := &messaging.Message{
				Headers: []messaging.Header{},
				Body:    jsonMessage,
			}

			messages = append(messages, msg)
		}

		if err := producer.PublishBatch(ctx, "conclusao_corte", "", map[string]string{}, messages); err != nil {
			log.Fatal(err)
		}
	}

	routes := []httpserver.Route{
		httpserver.NewRoute(http.MethodPost, "/message", func(w http.ResponseWriter, r *http.Request) error {
			requestID := r.Context().Value(httpserver.ContextKeyRequestID).(string)
			params := map[string]string{
				"content_type": "application/json",
				"event_type":   "order_created",
				"request_id":   requestID,
			}

			order := "mensagem via golang"
			json, err := json.Marshal(order)
			if err != nil {
				return err
			}

			err = producer.Publish(r.Context(), "orders", "order_created", params, &messaging.Message{
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
