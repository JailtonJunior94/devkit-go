package chiserver

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/http_server/common"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

// Start starts the HTTP server and blocks until a shutdown signal is received.
func (s *Server) Start(ctx context.Context) error {
	s.observability.Logger().Info(ctx, "starting HTTP server",
		observability.String("address", s.config.Address),
		observability.String("service", s.config.ServiceName),
		observability.String("version", s.config.ServiceVersion),
		observability.String("environment", s.config.Environment),
	)

	serverErr := make(chan error, 1)

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		s.observability.Logger().Error(ctx, "server failed to start", observability.Error(err))
		return err
	case <-ctx.Done():
		s.observability.Logger().Info(ctx, "context cancelled, initiating shutdown")
	case sig := <-sigChan:
		s.observability.Logger().Info(ctx, "signal received, initiating shutdown",
			observability.String("signal", sig.String()))
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return s.Shutdown(shutdownCtx)
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	var shutdownErr error

	s.shutdownOnce.Do(func() {
		s.observability.Logger().Info(ctx, "initiating graceful shutdown")

		if err := s.httpServer.Shutdown(ctx); err != nil {
			s.observability.Logger().Error(ctx, "error shutting down HTTP server", observability.Error(err))
			shutdownErr = err
		}

		provider, ok := s.observability.(common.Shutdowner)
		if !ok {
			s.observability.Logger().Info(ctx, "graceful shutdown completed")
			return
		}

		if err := provider.Shutdown(ctx); err != nil {
			s.observability.Logger().Error(ctx, "error shutting down observability provider", observability.Error(err))
			shutdownErr = errors.Join(shutdownErr, err)
			return
		}

		s.observability.Logger().Info(ctx, "graceful shutdown completed")
	})

	return shutdownErr
}
