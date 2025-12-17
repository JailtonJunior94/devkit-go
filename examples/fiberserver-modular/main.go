package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/JailtonJunior94/devkit-go/examples/fiberserver-modular/handlers"
	"github.com/JailtonJunior94/devkit-go/examples/fiberserver-modular/routes"
	"github.com/JailtonJunior94/devkit-go/pkg/fiberserver"
)

func main() {
	// Initialize handlers (in real app, inject dependencies here)
	accountHandler := handlers.NewAccountHandler()
	transactionHandler := handlers.NewTransactionHandler()
	userHandler := handlers.NewUserHandler()

	// Create server
	server := fiberserver.New(
		fiberserver.WithPort("8080"),
		fiberserver.WithMiddlewares(
			fiberserver.RequestID,
			fiberserver.Recovery,
			fiberserver.SecurityHeaders,
			fiberserver.Logger,
		),
	)

	// Register routes
	routes.RegisterHealthRoutes(server)
	routes.RegisterV1Routes(server, accountHandler, transactionHandler, userHandler)
	routes.RegisterV2Routes(server, accountHandler, transactionHandler, userHandler)

	// Start server
	shutdown := server.Run()
	log.Println("Server started on :8080")

	// Graceful shutdown
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := <-server.ShutdownListener(); err != nil {
			interrupt <- syscall.SIGTERM
		}
	}()

	<-interrupt
	log.Println("Shutting down...")

	ctx, cancel := fiberserver.GetShutdownTimeout()
	defer cancel()
	shutdown(ctx)
}
