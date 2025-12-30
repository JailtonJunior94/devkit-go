package cron_worker

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

// Start inicia o worker e o scheduler.
func (s *Server) Start(ctx context.Context) error {
	if s.running.Load() {
		return &WorkerError{
			Op:      "start",
			Message: "worker already running",
		}
	}

	s.observability.Logger().Info(ctx,
		"starting cron worker",
		observability.String("worker", s.config.ServiceName),
		observability.Int("jobs", s.GetJobCount()),
	)

	// Registra jobs no scheduler
	if err := s.scheduleJobs(ctx); err != nil {
		return &WorkerError{
			Op:      "start",
			Message: "failed to schedule jobs",
			Err:     err,
		}
	}

	// Inicia o scheduler
	s.scheduler.Start()
	s.running.Store(true)

	s.observability.Logger().Info(ctx,
		"cron worker started",
		observability.String("worker", s.config.ServiceName),
		observability.Bool("with_seconds", s.config.WithSeconds),
		observability.String("location", s.config.Location.String()),
	)

	// Setup signal handler
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Espera por sinal de término ou contexto cancelado
	select {
	case sig := <-sigChan:
		s.observability.Logger().Info(ctx,
			"received shutdown signal",
			observability.String("worker", s.config.ServiceName),
			observability.String("signal", sig.String()),
		)
	case <-ctx.Done():
		s.observability.Logger().Info(ctx,
			"context cancelled",
			observability.String("worker", s.config.ServiceName),
		)
	}

	// Executa shutdown gracioso
	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
	defer cancel()

	return s.Shutdown(shutdownCtx)
}

// Shutdown encerra graciosamente o worker.
func (s *Server) Shutdown(ctx context.Context) error {
	s.shutdownRun.Do(func() {
		s.shutdownMu.Lock()
		defer s.shutdownMu.Unlock()

		if !s.running.Load() {
			s.shutdownErr = &WorkerError{
				Op:      "shutdown",
				Message: "worker not running",
			}
			return
		}

		s.observability.Logger().Info(ctx,
			"shutting down cron worker",
			observability.String("worker", s.config.ServiceName),
			observability.Int("active_jobs", int(s.activeJobs.Load())),
		)

		// Para o scheduler (não aceita mais jobs)
		stopCtx := s.scheduler.Stop()

		// Aguarda jobs em execução ou timeout
		done := make(chan struct{})
		go func() {
			<-stopCtx.Done()
			close(done)
		}()

		select {
		case <-done:
			s.observability.Logger().Info(ctx,
				"all jobs completed",
				observability.String("worker", s.config.ServiceName),
			)
		case <-ctx.Done():
			s.observability.Logger().Warn(ctx,
				"shutdown timeout reached, some jobs may still be running",
				observability.String("worker", s.config.ServiceName),
				observability.Int("active_jobs", int(s.activeJobs.Load())),
			)
			s.shutdownErr = &ShutdownError{
				Message:    "shutdown timeout",
				Timeout:    s.config.ShutdownTimeout,
				ActiveJobs: int(s.activeJobs.Load()),
			}
		}

		s.running.Store(false)

		s.observability.Logger().Info(ctx,
			"cron worker stopped",
			observability.String("worker", s.config.ServiceName),
		)
	})

	s.shutdownMu.Lock()
	defer s.shutdownMu.Unlock()
	return s.shutdownErr
}

// scheduleJobs registra todos os jobs no scheduler.
func (s *Server) scheduleJobs(ctx context.Context) error {
	s.jobsMu.RLock()
	defer s.jobsMu.RUnlock()

	if len(s.jobs) == 0 {
		return &WorkerError{
			Op:      "schedule_jobs",
			Message: "no jobs registered",
		}
	}

	for name, job := range s.jobs {
		if err := s.scheduleJob(ctx, name, job); err != nil {
			return err
		}
	}

	return nil
}

// scheduleJob registra um job específico no scheduler.
func (s *Server) scheduleJob(ctx context.Context, name string, job Job) error {
	schedule := job.Schedule()

	_, err := s.scheduler.AddFunc(schedule, func() {
		s.runJob(ctx, name, job)
	})

	if err != nil {
		return &JobError{
			Job:     name,
			Op:      "schedule",
			Message: "failed to schedule job",
			Err:     err,
		}
	}

	s.observability.Logger().Info(ctx,
		"job scheduled",
		observability.String("worker", s.config.ServiceName),
		observability.String("job", name),
		observability.String("schedule", schedule),
	)

	return nil
}

// runJob executa um job com timeout e tracking.
func (s *Server) runJob(parentCtx context.Context, name string, job Job) {
	// Incrementa contador de jobs ativos
	s.activeJobs.Add(1)
	defer s.activeJobs.Add(-1)

	// Callback de início
	if s.config.OnJobStart != nil {
		s.config.OnJobStart(name)
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(parentCtx, s.config.JobTimeout)
	defer cancel()

	s.observability.Logger().Info(ctx,
		"job started",
		observability.String("worker", s.config.ServiceName),
		observability.String("job", name),
	)

	// Executa job com recovery
	var jobErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				s.observability.Logger().Error(ctx,
					"job panic recovered",
					observability.String("worker", s.config.ServiceName),
					observability.String("job", name),
					observability.Any("panic", r),
				)
				jobErr = &JobError{
					Job:     name,
					Op:      "run",
					Message: fmt.Sprintf("job panicked: %v", r),
				}
				if s.config.OnJobPanic != nil {
					s.config.OnJobPanic(name, r)
				}
			}
		}()

		jobErr = job.Run(ctx)
	}()

	duration := time.Since(start)

	// Callback de conclusão
	if s.config.OnJobComplete != nil {
		s.config.OnJobComplete(name, duration, jobErr)
	}

	// Log resultado
	if jobErr != nil {
		s.observability.Logger().Error(ctx,
			"job failed",
			observability.String("worker", s.config.ServiceName),
			observability.String("job", name),
			observability.String("duration", duration.String()),
			observability.Error(jobErr),
		)
	} else {
		s.observability.Logger().Info(ctx,
			"job completed",
			observability.String("worker", s.config.ServiceName),
			observability.String("job", name),
			observability.String("duration", duration.String()),
		)
	}
}
