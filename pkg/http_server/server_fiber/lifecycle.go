package serverfiber

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/JailtonJunior94/devkit-go/pkg/http_server/common"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

func (s *Server) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	s.observability.Logger().Info(ctx, "starting HTTP server",
		observability.String("address", s.config.Address),
		observability.String("service", s.config.ServiceName),
		observability.String("version", s.config.ServiceVersion),
		observability.String("environment", s.config.Environment),
	)

	serverErr := make(chan error, 1)

	go func() {
		if err := s.app.Listen(s.config.Address); err != nil {
			serverErr <- err
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		s.observability.Logger().Error(ctx, "server error",
			observability.Error(err),
		)
		return err

	case <-ctx.Done():
		s.observability.Logger().Info(ctx, "shutdown signal received via context")

	case sig := <-sigChan:
		s.observability.Logger().Info(ctx, "shutdown signal received",
			observability.String("signal", sig.String()),
		)
	}

	// Derive shutdown ctx from the parent ctx so that an externally provided
	// deadline (tests, orchestration) is honored; ShutdownTimeout caps the
	// upper bound to avoid hangs (RF-8.3).
	shutdownCtx, cancel := context.WithTimeout(ctx, s.config.ShutdownTimeout)
	defer cancel()

	return s.Shutdown(shutdownCtx)
}

func (s *Server) Shutdown(ctx context.Context) error {
	var shutdownErr error

	s.shutdownOnce.Do(func() {
		s.observability.Logger().Info(ctx, "initiating graceful shutdown")

		if err := s.app.ShutdownWithContext(ctx); err != nil {
			s.observability.Logger().Error(ctx, "error shutting down HTTP server",
				observability.Error(err),
			)
			shutdownErr = err
		} else {
			s.observability.Logger().Info(ctx, "HTTP server shutdown complete")
		}

		if provider, ok := s.observability.(common.Shutdowner); ok {
			if err := provider.Shutdown(ctx); err != nil {
				s.observability.Logger().Error(ctx, "error shutting down observability",
					observability.Error(err),
				)
				shutdownErr = errors.Join(shutdownErr, err)
			} else {
				s.observability.Logger().Info(ctx, "observability shutdown complete")
			}
		}

		if shutdownErr == nil {
			s.observability.Logger().Info(ctx, "graceful shutdown completed successfully")
		}
	})

	return shutdownErr
}
