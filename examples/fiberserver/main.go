package main

import (
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/JailtonJunior94/devkit-go/pkg/fiberserver"
	"github.com/gofiber/fiber/v2"
)

func main() {
	routes := []fiberserver.Route{
		fiberserver.NewRoute(fiber.MethodGet, "/hello", func(c *fiber.Ctx) error {
			requestID := fiberserver.GetRequestID(c)
			return c.Status(fiber.StatusOK).JSON(fiber.Map{
				"message":    "Hello, World!",
				"request_id": requestID,
			})
		}),
		fiberserver.NewRoute(fiber.MethodGet, "/error", func(c *fiber.Ctx) error {
			return errors.New("something went wrong")
		}),
		fiberserver.NewRoute(fiber.MethodPost, "/users", func(c *fiber.Ctx) error {
			type CreateUserRequest struct {
				Name  string `json:"name"`
				Email string `json:"email"`
			}

			var req CreateUserRequest
			if err := c.BodyParser(&req); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": "Invalid request body",
				})
			}

			return c.Status(fiber.StatusCreated).JSON(fiber.Map{
				"id":    "123",
				"name":  req.Name,
				"email": req.Email,
			})
		}),
		fiberserver.NewRoute(fiber.MethodGet, "/users/:id", func(c *fiber.Ctx) error {
			id := c.Params("id")
			return c.Status(fiber.StatusOK).JSON(fiber.Map{
				"id":   id,
				"name": "John Doe",
			})
		}),
		fiberserver.NewRoute(fiber.MethodGet, "/health", fiberserver.HealthCheck),
		fiberserver.NewRoute(fiber.MethodGet, "/ready", fiberserver.ReadinessCheck),
	}

	server := fiberserver.New(
		fiberserver.WithPort("8002"),
		fiberserver.WithRoutes(routes...),
		fiberserver.WithMiddlewares(
			fiberserver.RequestID,
			fiberserver.Recovery,
			fiberserver.SecurityHeaders,
			fiberserver.Logger,
		),
	)

	shutdown := server.Run()
	log.Println("Server started on port 8002")

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := <-server.ShutdownListener(); err != nil {
			log.Printf("Server error: %v", err)
			interrupt <- syscall.SIGTERM
		}
	}()

	<-interrupt
	log.Println("Shutting down server...")

	ctx, cancel := fiberserver.GetShutdownTimeout()
	defer cancel()

	if err := shutdown(ctx); err != nil {
		log.Fatal(err)
	}

	log.Println("Server stopped gracefully")
}
