package cron_worker

import "context"

// Job define a interface para um cron job.
type Job interface {
	// Name retorna o nome do job
	Name() string

	// Schedule retorna o cron schedule do job (formato cron padrão)
	// Exemplos:
	//   - "0 * * * *"        (a cada hora)
	//   - "*/5 * * * *"      (a cada 5 minutos)
	//   - "0 0 * * *"        (todo dia à meia-noite)
	//   - "0 0 * * 0"        (todo domingo à meia-noite)
	//   - "*/30 * * * * *"   (a cada 30 segundos - requer WithSeconds)
	Schedule() string

	// Run executa o job
	// O contexto pode ser cancelado durante o shutdown
	Run(ctx context.Context) error
}

// FuncJob implementa Job usando uma função.
type FuncJob struct {
	name     string
	schedule string
	fn       func(ctx context.Context) error
}

// NewFuncJob cria um novo FuncJob.
func NewFuncJob(name, schedule string, fn func(ctx context.Context) error) *FuncJob {
	return &FuncJob{
		name:     name,
		schedule: schedule,
		fn:       fn,
	}
}

// Name retorna o nome do job.
func (j *FuncJob) Name() string {
	return j.name
}

// Schedule retorna o cron schedule do job.
func (j *FuncJob) Schedule() string {
	return j.schedule
}

// Run executa a função do job.
func (j *FuncJob) Run(ctx context.Context) error {
	return j.fn(ctx)
}
