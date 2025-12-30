package consumer

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

// Start begins consuming messages and blocks until the context is cancelled,
// an error occurs, or an OS signal (SIGINT, SIGTERM) is received.
// It follows the same lifecycle pattern as pkg/http_server.
func (s *Server) Start(ctx context.Context) error {
	if s.isRunning.Load() {
		return &ConsumerError{
			Op:      "start",
			Message: "consumer is already running",
			Err:     errors.New("already running"),
		}
	}

	// Log startup with metadata
	s.observability.Logger().Info(ctx, "starting consumer server",
		observability.String("service", s.config.ServiceName),
		observability.String("version", s.config.ServiceVersion),
		observability.String("environment", s.config.Environment),
		observability.Any("topics", s.config.Topics),
		observability.Int("workers", s.config.WorkerCount),
		observability.Int("batch_size", s.config.BatchSize),
	)

	// Create worker context with cancellation
	s.workerCtx, s.stopWorkers = context.WithCancel(context.Background())

	// Mark as running
	s.isRunning.Store(true)

	// Start consumption in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := s.consume(s.workerCtx); err != nil {
			errChan <- err
		}
	}()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Triple select pattern (same as http_server)
	select {
	case err := <-errChan:
		// Consumer startup/runtime error
		s.observability.Logger().Error(ctx, "consumer error occurred",
			observability.String("error", err.Error()))
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
		defer cancel()
		if shutdownErr := s.Shutdown(shutdownCtx); shutdownErr != nil {
			return errors.Join(err, shutdownErr)
		}
		return err

	case <-ctx.Done():
		// Context cancellation
		s.observability.Logger().Info(ctx, "context cancelled, initiating shutdown")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
		defer cancel()
		return s.Shutdown(shutdownCtx)

	case sig := <-sigChan:
		// OS signal received
		s.observability.Logger().Info(ctx, "signal received, initiating graceful shutdown",
			observability.String("signal", sig.String()))
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
		defer cancel()
		return s.Shutdown(shutdownCtx)
	}
}

// Shutdown gracefully stops the consumer, waiting for in-flight messages
// to complete processing within the provided context timeout.
// It uses sync.Once to ensure shutdown is only executed once.
func (s *Server) Shutdown(ctx context.Context) error {
	var shutdownErr error

	s.shutdownOnce.Do(func() {
		s.observability.Logger().Info(ctx, "initiating graceful shutdown")

		// Stop accepting new messages
		if s.stopWorkers != nil {
			s.stopWorkers()
		}

		// Wait for workers to finish with timeout
		workersDone := make(chan struct{})
		go func() {
			s.workers.Wait()
			close(workersDone)
		}()

		select {
		case <-workersDone:
			s.observability.Logger().Info(ctx, "all workers finished gracefully")
		case <-ctx.Done():
			shutdownErr = &ShutdownError{
				Message: "shutdown timeout exceeded, some workers may not have finished",
				Err:     ctx.Err(),
			}
			s.observability.Logger().Warn(ctx, "shutdown timeout exceeded",
				observability.String("error", shutdownErr.Error()))
		}

		// Mark as not running
		s.isRunning.Store(false)

		// Shutdown observability provider (same pattern as http_server)
		type shutdowner interface {
			Shutdown(context.Context) error
		}

		if provider, ok := s.observability.(shutdowner); ok {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if err := provider.Shutdown(shutdownCtx); err != nil {
				s.observability.Logger().Error(ctx, "failed to shutdown observability provider",
					observability.String("error", err.Error()))
				shutdownErr = errors.Join(shutdownErr, err)
			}
		}

		s.observability.Logger().Info(ctx, "graceful shutdown completed")
	})

	return shutdownErr
}

// consume is the main consumption loop that processes messages.
// It manages the worker pool and coordinates message processing.
func (s *Server) consume(ctx context.Context) error {
	s.observability.Logger().Info(ctx, "starting message consumption",
		observability.Int("workers", s.config.WorkerCount))

	// Start worker pool
	for i := 0; i < s.config.WorkerCount; i++ {
		s.workers.Add(1)
		go s.worker(ctx, i)
	}

	// Wait for context cancellation
	<-ctx.Done()

	s.observability.Logger().Info(ctx, "consumption stopped, waiting for workers to finish")
	return nil
}

// worker is a single worker goroutine that processes messages.
// This is a simplified implementation - in real usage, this would
// integrate with the actual message broker (Kafka, RabbitMQ, etc.).
func (s *Server) worker(ctx context.Context, workerID int) {
	defer s.workers.Done()

	s.observability.Logger().Info(ctx, "worker started",
		observability.Int("worker_id", workerID))

	// Worker processing loop
	for {
		select {
		case <-ctx.Done():
			s.observability.Logger().Info(ctx, "worker stopping",
				observability.Int("worker_id", workerID))
			return
		default:
			// In a real implementation, this would:
			// 1. Fetch messages from the broker
			// 2. Process each message through the handler chain
			// 3. Handle retries and errors
			// 4. Commit offsets
			//
			// For now, we just check if context is cancelled
			// to allow graceful shutdown to work correctly.
			time.Sleep(100 * time.Millisecond)
		}
	}
}
