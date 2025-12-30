// Package cron_worker fornece um worker resiliente baseado em cron jobs.
// que funciona como entrypoint da aplicação, seguindo o mesmo padrão.
// arquitetural do pkg/http_server.
//
// O worker utiliza github.com/robfig/cron/v3 para agendamento de jobs
// e oferece recursos completos para produção:.
//   - Graceful shutdown com timeout configurável.
//   - Health checks integrados.
//   - Observability completa (logs, métricas, tracing).
//   - Recuperação de panics.
//   - Execução concorrente de jobs.
//   - Context-aware job execution.
//   - Signal handling (SIGINT, SIGTERM).
//
// Exemplo básico:.
//
//	o11y, err := observability.New(/* config */)
//	if err != nil {.
//	    log.Fatal(err)
//	}.
//
//	worker, err := cron_worker.New(
//	    o11y,.
//	    cron_worker.WithServiceName("my-worker"),
//	    cron_worker.WithShutdownTimeout(30 * time.Second),
//	).
//	if err != nil {.
//	    log.Fatal(err)
//	}.
//
//	// Registra um job.
//	job := cron_worker.NewFuncJob("cleanup", "0 * * * *", func(ctx context.Context) error {
//	    // Executa limpeza a cada hora.
//	    return performCleanup(ctx).
//	}).
//
//	if err := worker.RegisterJobs(job); err != nil {
//	    log.Fatal(err)
//	}.
//
//	// Inicia o worker (bloqueia até shutdown).
//	if err := worker.Start(context.Background()); err != nil {
//	    log.Fatal(err)
//	}.
package cron_worker
