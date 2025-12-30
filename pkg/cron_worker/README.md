# Cron Worker

Worker resiliente baseado em cron jobs que funciona como entrypoint da aplicação, seguindo o padrão arquitetural do `pkg/http_server`.

## Características

- ✅ **Entrypoint Pattern**: Start() e Shutdown() como pontos de entrada
- ✅ **Graceful Shutdown**: Encerramento gracioso com timeout configurável
- ✅ **Signal Handling**: Tratamento de SIGINT e SIGTERM
- ✅ **Health Checks**: Sistema integrado de health checks
- ✅ **Observability**: Logs estruturados, métricas e tracing
- ✅ **Recovery**: Recuperação automática de panics em jobs
- ✅ **Context-Aware**: Jobs recebem contexto e respeitam cancelamento
- ✅ **Concurrent Jobs**: Suporte para múltiplos jobs concorrentes
- ✅ **Flexible Scheduling**: Suporte para cron expressions padrão e com segundos
- ✅ **Timezone Support**: Configuração de timezone para o scheduler

## Instalação

```bash
go get github.com/robfig/cron/v3
```

## Uso Básico

### 1. Criar Worker

```go
import (
    "github.com/JailtonJunior94/devkit-go/pkg/cron_worker"
    "github.com/JailtonJunior94/devkit-go/pkg/observability"
)

o11y, err := observability.New(/* config */)
if err != nil {
    log.Fatal(err)
}

worker, err := cron_worker.New(
    o11y,
    cron_worker.WithServiceName("my-worker"),
    cron_worker.WithShutdownTimeout(30 * time.Second),
    cron_worker.WithJobTimeout(5 * time.Minute),
)
if err != nil {
    log.Fatal(err)
}
```

### 2. Registrar Jobs

```go
// Job simples usando FuncJob
cleanupJob := cron_worker.NewFuncJob(
    "cleanup",
    "0 * * * *", // A cada hora
    func(ctx context.Context) error {
        return performCleanup(ctx)
    },
)

// Job customizado implementando interface Job
type ReportJob struct{}

func (j *ReportJob) Name() string { return "daily-report" }
func (j *ReportJob) Schedule() string { return "0 0 * * *" } // Todo dia à meia-noite
func (j *ReportJob) Run(ctx context.Context) error {
    return generateReport(ctx)
}

reportJob := &ReportJob{}

// Registrar jobs
if err := worker.RegisterJobs(cleanupJob, reportJob); err != nil {
    log.Fatal(err)
}
```

### 3. Iniciar Worker

```go
// Inicia o worker (bloqueia até receber sinal de shutdown)
if err := worker.Start(context.Background()); err != nil {
    log.Fatal(err)
}
```

## Configuração Avançada

### Cron com Segundos

```go
o11y, _ := observability.New(/* config */)

worker, err := cron_worker.New(
    o11y,
    cron_worker.WithServiceName("precise-worker"),
    cron_worker.WithSeconds(true), // Habilita precisão de segundos
)

// Agora você pode usar schedules com segundos
job := cron_worker.NewFuncJob(
    "frequent",
    "*/30 * * * * *", // A cada 30 segundos
    handler,
)
```

### Timezone Customizado

```go
location, _ := time.LoadLocation("America/Sao_Paulo")
o11y, _ := observability.New(/* config */)

worker, err := cron_worker.New(
    o11y,
    cron_worker.WithServiceName("br-worker"),
    cron_worker.WithLocation(location),
)
```

### Health Checks Customizados

```go
databaseCheck := cron_worker.HealthCheck{
    Name: "database",
    Check: func(ctx context.Context) error {
        return db.Ping(ctx)
    },
    Timeout: 5 * time.Second,
}

o11y, _ := observability.New(/* config */)

worker, err := cron_worker.New(
    o11y,
    cron_worker.WithServiceName("my-worker"),
    cron_worker.WithHealthChecks(databaseCheck),
)
```

### Callbacks de Job

```go
o11y, _ := observability.New(/* config */)

worker, err := cron_worker.New(
    o11y,
    cron_worker.WithServiceName("my-worker"),
    cron_worker.WithOnJobStart(func(jobName string) {
        log.Printf("Job %s started", jobName)
    }),
    cron_worker.WithOnJobComplete(func(jobName string, duration time.Duration, err error) {
        if err != nil {
            log.Printf("Job %s failed after %v: %v", jobName, duration, err)
        } else {
            log.Printf("Job %s completed in %v", jobName, duration)
        }
    }),
    cron_worker.WithOnJobPanic(func(jobName string, recovered interface{}) {
        log.Printf("Job %s panicked: %v", jobName, recovered)
    }),
)
```

### Limitar Jobs Concorrentes

```go
o11y, _ := observability.New(/* config */)

worker, err := cron_worker.New(
    o11y,
    cron_worker.WithServiceName("my-worker"),
    cron_worker.WithMaxConcurrentJobs(5), // Máximo 5 jobs simultâneos
)
```

## Cron Schedules

### Formato Padrão (5 campos)

```
┌───────────── minuto (0 - 59)
│ ┌───────────── hora (0 - 23)
│ │ ┌───────────── dia do mês (1 - 31)
│ │ │ ┌───────────── mês (1 - 12)
│ │ │ │ ┌───────────── dia da semana (0 - 6) (Domingo = 0)
│ │ │ │ │
│ │ │ │ │
* * * * *
```

### Com Segundos (6 campos, requer WithSeconds)

```
┌───────────── segundo (0 - 59)
│ ┌───────────── minuto (0 - 59)
│ │ ┌───────────── hora (0 - 23)
│ │ │ ┌───────────── dia do mês (1 - 31)
│ │ │ │ ┌───────────── mês (1 - 12)
│ │ │ │ │ ┌───────────── dia da semana (0 - 6)
│ │ │ │ │ │
│ │ │ │ │ │
* * * * * *
```

### Exemplos Comuns

| Schedule | Descrição |
|----------|-----------|
| `* * * * *` | A cada minuto |
| `*/5 * * * *` | A cada 5 minutos |
| `0 * * * *` | A cada hora |
| `0 0 * * *` | Todo dia à meia-noite |
| `0 0 * * 0` | Todo domingo à meia-noite |
| `0 0 1 * *` | Todo dia 1 do mês |
| `0 9 * * 1-5` | Dias úteis às 9h |
| `*/30 * * * * *` | A cada 30 segundos (requer WithSeconds) |

### Descritores Especiais

| Descritor | Equivalente | Descrição |
|-----------|-------------|-----------|
| `@yearly` | `0 0 1 1 *` | Uma vez por ano |
| `@monthly` | `0 0 1 * *` | Uma vez por mês |
| `@weekly` | `0 0 * * 0` | Uma vez por semana |
| `@daily` | `0 0 * * *` | Uma vez por dia |
| `@hourly` | `0 * * * *` | Uma vez por hora |

## Health Checks

### Verificar Status

```go
status := worker.Health(context.Background())

fmt.Printf("Status: %s\n", status.Status)           // healthy/unhealthy
fmt.Printf("Running: %v\n", status.IsRunning)        // true/false
fmt.Printf("Jobs: %d\n", status.JobsCount)           // Total de jobs
fmt.Printf("Active: %d\n", status.ActiveJobs)        // Jobs executando
```

### Kubernetes Probes

```go
// Readiness probe
http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
    if worker.Readiness(r.Context()) {
        w.WriteHeader(http.StatusOK)
    } else {
        w.WriteHeader(http.StatusServiceUnavailable)
    }
})

// Liveness probe
http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
    if worker.Liveness(r.Context()) {
        w.WriteHeader(http.StatusOK)
    } else {
        w.WriteHeader(http.StatusServiceUnavailable)
    }
})
```

## Graceful Shutdown

O worker trata automaticamente sinais do sistema operacional:

```go
// Recebe SIGINT (Ctrl+C) ou SIGTERM
// Worker para de aceitar novos jobs
// Aguarda jobs em execução completarem
// Respeita o ShutdownTimeout configurado
```

### Shutdown Programático

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

if err := worker.Shutdown(ctx); err != nil {
    log.Printf("Shutdown error: %v", err)
}
```

## Tratamento de Erros

### Tipos de Erro

```go
// WorkerError - erros gerais do worker
var workerErr *cron_worker.WorkerError
if errors.As(err, &workerErr) {
    log.Printf("Worker error in %s: %s", workerErr.Op, workerErr.Message)
}

// JobError - erros específicos de jobs
var jobErr *cron_worker.JobError
if errors.As(err, &jobErr) {
    log.Printf("Job %s error in %s: %s", jobErr.Job, jobErr.Op, jobErr.Message)
}

// ShutdownError - erros durante shutdown
var shutdownErr *cron_worker.ShutdownError
if errors.As(err, &shutdownErr) {
    log.Printf("Shutdown error: %d jobs still active", shutdownErr.ActiveJobs)
}
```

## Integração com Observability

O worker integra automaticamente com `pkg/observability`:

```go
// Logs estruturados
// - Job started/completed/failed
// - Duração de execução
// - Panics recuperados
// - Health check results

// Métricas (se habilitado)
worker, err := cron_worker.New(
    cron_worker.WithMetrics(true, "my_worker"),
)

// Tracing (se habilitado)
worker, err := cron_worker.New(
    cron_worker.WithTracing(true),
)
```

## Exemplo Completo

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/JailtonJunior94/devkit-go/pkg/cron_worker"
    "github.com/JailtonJunior94/devkit-go/pkg/observability"
)

func main() {
    // Setup observability
    o11y, err := observability.New(/* config */)
    if err != nil {
        log.Fatal(err)
    }

    // Criar worker
    worker, err := cron_worker.New(
        o11y,
        cron_worker.WithServiceName("example-worker"),
        cron_worker.WithShutdownTimeout(30*time.Second),
        cron_worker.WithJobTimeout(5*time.Minute),
        cron_worker.WithHealthCheck(true),
    )
    if err != nil {
        log.Fatal(err)
    }

    // Registrar jobs
    cleanupJob := cron_worker.NewFuncJob(
        "cleanup",
        "0 * * * *", // A cada hora
        func(ctx context.Context) error {
            log.Println("Running cleanup...")
            // Implementar lógica de limpeza
            return nil
        },
    )

    reportJob := cron_worker.NewFuncJob(
        "report",
        "0 0 * * *", // Todo dia à meia-noite
        func(ctx context.Context) error {
            log.Println("Generating report...")
            // Implementar geração de relatório
            return nil
        },
    )

    if err := worker.RegisterJobs(cleanupJob, reportJob); err != nil {
        log.Fatal(err)
    }

    // Iniciar worker (bloqueia até shutdown)
    log.Println("Starting worker...")
    if err := worker.Start(context.Background()); err != nil {
        log.Fatal(err)
    }

    log.Println("Worker stopped")
}
```

## Testes

```go
func TestWorker(t *testing.T) {
    o11y, err := observability.New(/* config */)
    require.NoError(t, err)

    worker, err := cron_worker.New(
        o11y,
        cron_worker.WithServiceName("test-worker"),
        cron_worker.WithShutdownTimeout(5*time.Second),
    )
    require.NoError(t, err)

    // Registrar job de teste
    executed := false
    job := cron_worker.NewFuncJob(
        "test",
        "* * * * *",
        func(ctx context.Context) error {
            executed = true
            return nil
        },
    )

    err = worker.RegisterJobs(job)
    require.NoError(t, err)

    // Verificar estado inicial
    assert.False(t, worker.IsRunning())
    assert.Equal(t, 1, worker.GetJobCount())
    assert.Equal(t, int32(0), worker.GetActiveJobCount())
}
```

## Padrões de Uso

### Worker como Entrypoint

```go
func main() {
    worker := setupWorker()
    if err := worker.Start(context.Background()); err != nil {
        log.Fatal(err)
    }
}
```

### Worker com HTTP Server

```go
func main() {
    worker := setupWorker()
    httpServer := setupHTTPServer(worker)

    // Iniciar HTTP server em goroutine
    go func() {
        if err := httpServer.Start(context.Background()); err != nil {
            log.Fatal(err)
        }
    }()

    // Worker como processo principal
    if err := worker.Start(context.Background()); err != nil {
        log.Fatal(err)
    }
}
```

## Boas Práticas

1. **Sempre use contexto**: Jobs devem respeitar o contexto para cancelamento gracioso
2. **Configure timeouts apropriados**: JobTimeout e ShutdownTimeout adequados ao seu caso
3. **Implemente health checks**: Para monitoramento e readiness probes
4. **Use callbacks para métricas**: OnJobStart, OnJobComplete para instrumentação
5. **Trate erros de jobs**: Retorne erros dos jobs para logging apropriado
6. **Teste schedules**: Valide cron expressions antes de deployment
7. **Monitor active jobs**: Use GetActiveJobCount() para observabilidade
8. **Cuidado com overlapping**: Jobs longos podem sobrepor execuções

## Comparação com http_server

| Aspecto | http_server | cron_worker |
|---------|-------------|-------------|
| Trigger | HTTP requests | Cron schedule |
| Start() | Sim | Sim |
| Shutdown() | Sim | Sim |
| Health checks | Sim | Sim |
| Middleware | Sim | Não |
| Routes | Sim | Jobs |
| Signal handling | Sim | Sim |
| Graceful shutdown | Sim | Sim |
| Observability | Sim | Sim |

## Referências

- [robfig/cron](https://github.com/robfig/cron)
- [Cron Expression Format](https://en.wikipedia.org/wiki/Cron)
- [pkg/http_server](../http_server/README.md)
- [pkg/observability](../observability/README.md)
