package cron_worker

import "fmt"

// WorkerError representa um erro do worker
type WorkerError struct {
	Op      string // Operação que causou o erro
	Message string // Mensagem de erro
	Err     error  // Erro subjacente
}

// Error implementa a interface error
func (e *WorkerError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("worker error in %s: %s: %v", e.Op, e.Message, e.Err)
	}
	return fmt.Sprintf("worker error in %s: %s", e.Op, e.Message)
}

// Unwrap retorna o erro subjacente
func (e *WorkerError) Unwrap() error {
	return e.Err
}

// JobError representa um erro específico de um job
type JobError struct {
	Job     string // Nome do job
	Op      string // Operação que causou o erro
	Message string // Mensagem de erro
	Err     error  // Erro subjacente
}

// Error implementa a interface error
func (e *JobError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("job error [%s] in %s: %s: %v", e.Job, e.Op, e.Message, e.Err)
	}
	return fmt.Sprintf("job error [%s] in %s: %s", e.Job, e.Op, e.Message)
}

// Unwrap retorna o erro subjacente
func (e *JobError) Unwrap() error {
	return e.Err
}

// ShutdownError representa um erro durante o shutdown
type ShutdownError struct {
	Message    string // Mensagem de erro
	Timeout    any    // Timeout configurado
	ActiveJobs int    // Número de jobs ativos no momento do timeout
}

// Error implementa a interface error
func (e *ShutdownError) Error() string {
	return fmt.Sprintf("shutdown error: %s (timeout: %v, active jobs: %d)", e.Message, e.Timeout, e.ActiveJobs)
}
