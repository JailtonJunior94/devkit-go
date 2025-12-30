package cron_worker

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/robfig/cron/v3"
)

// CronWorker define a interface para um worker baseado em cron jobs.
type CronWorker interface {
	// Start inicia o worker e o scheduler de cron jobs
	Start(ctx context.Context) error

	// Shutdown encerra graciosamente o worker
	Shutdown(ctx context.Context) error

	// Health retorna o status de saúde do worker
	Health(ctx context.Context) HealthStatus

	// RegisterJobs registra jobs no scheduler
	RegisterJobs(jobs ...Job) error
}

// Server implementa a interface CronWorker.
type Server struct {
	config        *Config
	observability observability.Observability
	scheduler     *cron.Cron
	jobs          map[string]Job
	jobsMu        sync.RWMutex
	running       atomic.Bool
	activeJobs    atomic.Int32
	shutdownMu    sync.Mutex
	shutdownErr   error
	shutdownRun   sync.Once
}

// New cria uma nova instância do worker de cron.
func New(o11y observability.Observability, opts ...Option) (*Server, error) {
	config := DefaultConfig()
	for _, opt := range opts {
		opt(config)
	}

	if err := config.Validate(); err != nil {
		return nil, &WorkerError{
			Op:      "new",
			Message: "invalid configuration",
			Err:     err,
		}
	}

	cronOpts := []cron.Option{
		cron.WithLogger(newCronLogger(config.ServiceName, o11y)),
		cron.WithChain(cron.Recover(newCronLogger(config.ServiceName, o11y))),
	}

	if config.WithSeconds {
		cronOpts = append(cronOpts, cron.WithSeconds())
	}

	if config.Location != nil {
		cronOpts = append(cronOpts, cron.WithLocation(config.Location))
	}

	scheduler := cron.New(cronOpts...)

	srv := &Server{
		config:        config,
		observability: o11y,
		scheduler:     scheduler,
		jobs:          make(map[string]Job),
	}

	srv.running.Store(false)
	srv.activeJobs.Store(0)

	return srv, nil
}

// RegisterJobs registra um ou mais jobs no scheduler.
func (s *Server) RegisterJobs(jobs ...Job) error {
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()

	if s.running.Load() {
		return &WorkerError{
			Op:      "register_jobs",
			Message: "cannot register jobs while worker is running",
		}
	}

	for _, job := range jobs {
		if job == nil {
			continue
		}

		name := job.Name()
		if name == "" {
			return &JobError{
				Job:     "<unnamed>",
				Op:      "register",
				Message: "job name cannot be empty",
			}
		}

		if _, exists := s.jobs[name]; exists {
			return &JobError{
				Job:     name,
				Op:      "register",
				Message: "job already registered",
			}
		}

		schedule := job.Schedule()
		if schedule == "" {
			return &JobError{
				Job:     name,
				Op:      "register",
				Message: "job schedule cannot be empty",
			}
		}

		s.jobs[name] = job

		s.observability.Logger().Info(context.Background(),
			"job registered",
			observability.String("worker", s.config.ServiceName),
			observability.String("job", name),
			observability.String("schedule", schedule),
		)
	}

	return nil
}

// GetJobCount retorna o número total de jobs registrados.
func (s *Server) GetJobCount() int {
	s.jobsMu.RLock()
	defer s.jobsMu.RUnlock()
	return len(s.jobs)
}

// GetActiveJobCount retorna o número de jobs atualmente em execução.
func (s *Server) GetActiveJobCount() int32 {
	return s.activeJobs.Load()
}

// IsRunning verifica se o worker está em execução.
func (s *Server) IsRunning() bool {
	return s.running.Load()
}
