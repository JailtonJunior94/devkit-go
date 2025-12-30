package cron_worker

import (
	"context"
	"sync"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

// HealthStatus representa o status de saúde do worker
type HealthStatus struct {
	// Status indica se o worker está saudável
	Status string `json:"status"`

	// IsRunning indica se o worker está em execução
	IsRunning bool `json:"is_running"`

	// JobsCount é o número total de jobs registrados
	JobsCount int `json:"jobs_count"`

	// ActiveJobs é o número de jobs atualmente em execução
	ActiveJobs int32 `json:"active_jobs"`

	// Checks contém os resultados dos health checks customizados
	Checks map[string]CheckResult `json:"checks,omitempty"`

	// Message é uma mensagem adicional sobre o status
	Message string `json:"message,omitempty"`

	// Timestamp é o momento da verificação
	Timestamp time.Time `json:"timestamp"`
}

// CheckResult representa o resultado de um health check
type CheckResult struct {
	// Status indica se o check passou
	Status string `json:"status"`

	// Message é uma mensagem adicional sobre o check
	Message string `json:"message,omitempty"`

	// Duration é o tempo que levou para executar o check
	Duration time.Duration `json:"duration"`

	// Error contém o erro se o check falhou
	Error string `json:"error,omitempty"`
}

// HealthCheck é uma função de verificação de saúde customizada
type HealthCheck struct {
	// Name é o nome do health check
	Name string

	// Check é a função que executa a verificação
	Check func(ctx context.Context) error

	// Timeout é o tempo máximo para o check (opcional)
	Timeout time.Duration
}

// Health retorna o status de saúde atual do worker
func (s *Server) Health(ctx context.Context) HealthStatus {
	status := HealthStatus{
		Status:     "healthy",
		IsRunning:  s.running.Load(),
		JobsCount:  s.GetJobCount(),
		ActiveJobs: s.activeJobs.Load(),
		Checks:     make(map[string]CheckResult),
		Timestamp:  time.Now(),
	}

	// Se não está rodando, está unhealthy
	if !status.IsRunning {
		status.Status = "unhealthy"
		status.Message = "worker not running"
		return status
	}

	// Executa health checks customizados se habilitados
	if s.config.EnableHealthCheck && len(s.config.HealthChecks) > 0 {
		checkResults := s.executeHealthChecks(ctx)
		status.Checks = checkResults

		// Se algum check falhou, o worker está unhealthy
		for name, result := range checkResults {
			if result.Status != "pass" {
				status.Status = "unhealthy"
				status.Message = "health check failed: " + name
				break
			}
		}
	}

	return status
}

// executeHealthChecks executa todos os health checks customizados
func (s *Server) executeHealthChecks(ctx context.Context) map[string]CheckResult {
	results := make(map[string]CheckResult)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, hc := range s.config.HealthChecks {
		wg.Add(1)
		go func(check HealthCheck) {
			defer wg.Done()

			result := s.runHealthCheck(ctx, check)

			mu.Lock()
			results[check.Name] = result
			mu.Unlock()
		}(hc)
	}

	wg.Wait()
	return results
}

// runHealthCheck executa um health check individual
func (s *Server) runHealthCheck(ctx context.Context, hc HealthCheck) CheckResult {
	start := time.Now()

	// Usa timeout do check se especificado, senão usa timeout padrão
	timeout := hc.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Executa check em goroutine separada
	errChan := make(chan error, 1)
	go func() {
		errChan <- hc.Check(checkCtx)
	}()

	// Aguarda resultado ou timeout
	var err error
	select {
	case err = <-errChan:
	case <-checkCtx.Done():
		err = checkCtx.Err()
	}

	duration := time.Since(start)

	result := CheckResult{
		Duration: duration,
	}

	if err != nil {
		result.Status = "fail"
		result.Error = err.Error()
		s.observability.Logger().Warn(ctx,
			"health check failed",
			observability.String("worker", s.config.ServiceName),
			observability.String("check", hc.Name),
			observability.Error(err),
		)
	} else {
		result.Status = "pass"
	}

	return result
}

// Readiness verifica se o worker está pronto para processar jobs
func (s *Server) Readiness(ctx context.Context) bool {
	return s.running.Load() && s.GetJobCount() > 0
}

// Liveness verifica se o worker está vivo
func (s *Server) Liveness(ctx context.Context) bool {
	return s.running.Load()
}
