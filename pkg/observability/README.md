# Observability Library

Uma biblioteca de observabilidade robusta e desacoplada para Go, baseada em **Clean Architecture** e encapsulando completamente o **OpenTelemetry**.

## Características

- **Clean Architecture**: Totalmente desacoplada, respeitando o DIP (Dependency Inversion Principle)
- **Interface Única (Facade)**: Uma única interface para injeção em todas as camadas
- **Zero Acoplamento ao OpenTelemetry**: Nenhuma dependência do OTel fora da infraestrutura
- **Três Implementações**:
  - `noop`: Zero overhead para ambientes sem observabilidade
  - `fake`: Para testes com assertions completas
  - `otel`: OpenTelemetry completo para produção
- **Logging Estruturado**: Suporte a formato TEXT e JSON
- **Níveis de Log Completos**: Debug, Info, Warn, Error
- **Alta Performance**: Otimizado para high throughput
- **100% Testável**: Sem necessidade de OpenTelemetry nos testes

## Arquitetura

```
pkg/observability/
├── observability.go    # Interface facade principal
├── tracer.go          # Interface de tracing
├── logger.go          # Interface de logging
├── metrics.go         # Interface de métricas
├── noop/              # Implementação no-op (zero overhead)
├── fake/              # Implementação fake (testes)
└── otel/              # Implementação OpenTelemetry
    ├── config.go
    ├── tracer.go
    ├── logger.go
    └── metrics.go
```

## Instalação

```bash
go get github.com/JailtonJunior94/devkit-go/pkg/observability
```

## 📚 Exemplos Completos

Este pacote inclui exemplos práticos e completos de uso:

- **[HTTP Handler](./examples/http-handler/)** - Exemplo completo de API HTTP com tracing, logging e métricas
- **[Service Layer](./examples/service-layer/)** - Demonstra repository pattern e service layer com observabilidade
- **[Testing](./examples/testing/)** - Como testar serviços usando o fake provider

Para executar os exemplos:

```bash
# HTTP Handler
cd pkg/observability/examples/http-handler
go run main.go

# Service Layer
cd pkg/observability/examples/service-layer
go run main.go

# Testes
cd pkg/observability/examples/testing
go test -v
```

## Uso Básico

### 1. Inicialização (main.go)

```go
package main

import (
    "context"
    "log"

    "github.com/JailtonJunior94/devkit-go/pkg/observability"
    "github.com/JailtonJunior94/devkit-go/pkg/observability/otel"
)

func main() {
    ctx := context.Background()

    // Configuração para produção com OpenTelemetry
    config := &otel.Config{
        ServiceName:     "order-service",
        ServiceVersion:  "1.0.0",
        Environment:     "production",
        OTLPEndpoint:    "localhost:4317",
        TraceSampleRate: 1.0,
        LogLevel:        observability.LogLevelInfo,
        LogFormat:       observability.LogFormatJSON,
    }

    obs, err := otel.NewProvider(ctx, config)
    if err != nil {
        log.Fatalf("failed to initialize observability: %v", err)
    }
    defer obs.Shutdown(ctx)

    // Injetar 'obs' em suas dependências
    app := NewApplication(obs)
    app.Run()
}
```

### 2. Uso em Use Cases / Services

```go
package usecase

import (
    "context"

    "github.com/JailtonJunior94/devkit-go/pkg/observability"
)

type CreateOrderUseCase struct {
    obs        observability.Observability
    repository OrderRepository
}

func NewCreateOrderUseCase(obs observability.Observability, repo OrderRepository) *CreateOrderUseCase {
    return &CreateOrderUseCase{
        obs:        obs,
        repository: repo,
    }
}

func (uc *CreateOrderUseCase) Execute(ctx context.Context, dto CreateOrderDTO) error {
    // Iniciar tracing
    ctx, span := uc.obs.Tracer().Start(ctx, "CreateOrder",
        observability.WithSpanKind(observability.SpanKindServer),
    )
    defer span.End()

    // Log estruturado (automaticamente inclui trace_id)
    uc.obs.Logger().Info(ctx, "creating order",
        observability.String("customer_id", dto.CustomerID),
        observability.Int("items_count", len(dto.Items)),
    )

    // Métricas
    counter := uc.obs.Metrics().Counter(
        "orders.created",
        "Total number of orders created",
        "1",
    )

    // Lógica de negócio
    order, err := uc.repository.Create(ctx, dto)
    if err != nil {
        span.RecordError(err)
        span.SetStatus(observability.StatusCodeError, "failed to create order")

        uc.obs.Logger().Error(ctx, "failed to create order",
            observability.Error(err),
            observability.String("customer_id", dto.CustomerID),
        )

        return err
    }

    // Sucesso
    counter.Add(ctx, 1, observability.String("status", "success"))
    span.SetStatus(observability.StatusCodeOK, "order created successfully")

    uc.obs.Logger().Info(ctx, "order created successfully",
        observability.String("order_id", order.ID),
    )

    return nil
}
```

### 3. Uso em Repositories

```go
package repository

import (
    "context"
    "database/sql"

    "github.com/JailtonJunior94/devkit-go/pkg/observability"
)

type PostgresOrderRepository struct {
    obs observability.Observability
    db  *sql.DB
}

func NewPostgresOrderRepository(obs observability.Observability, db *sql.DB) *PostgresOrderRepository {
    return &PostgresOrderRepository{
        obs: obs,
        db:  db,
    }
}

func (r *PostgresOrderRepository) Create(ctx context.Context, dto CreateOrderDTO) (*Order, error) {
    ctx, span := r.obs.Tracer().Start(ctx, "OrderRepository.Create",
        observability.WithSpanKind(observability.SpanKindClient),
    )
    defer span.End()

    histogram := r.obs.Metrics().Histogram(
        "db.query.duration",
        "Database query duration",
        "ms",
    )

    start := time.Now()

    // Executar query
    result, err := r.db.ExecContext(ctx, query, args...)

    duration := time.Since(start).Milliseconds()
    histogram.Record(ctx, float64(duration),
        observability.String("operation", "insert"),
        observability.String("table", "orders"),
    )

    if err != nil {
        span.RecordError(err)
        r.obs.Logger().Error(ctx, "database error", observability.Error(err))
        return nil, err
    }

    return order, nil
}
```

## Providers

### Provider NoOp (Desenvolvimento/Testes de Integração)

Zero overhead, ideal quando você não quer observabilidade:

```go
import "github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

obs := noop.NewProvider()
```

### Provider Fake (Testes Unitários)

Captura todas as operações para assertions:

```go
import (
    "testing"

    "github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
)

func TestCreateOrder(t *testing.T) {
    obs := fake.NewProvider()
    useCase := NewCreateOrderUseCase(obs, mockRepo)

    // Executar
    err := useCase.Execute(ctx, dto)

    // Assertions
    tracer := obs.Tracer().(*fake.FakeTracer)
    spans := tracer.GetSpans()

    if len(spans) != 1 {
        t.Errorf("expected 1 span, got %d", len(spans))
    }

    logger := obs.Logger().(*fake.FakeLogger)
    entries := logger.GetEntries()

    if len(entries) < 1 {
        t.Error("expected at least 1 log entry")
    }
}
```

### Provider OpenTelemetry (Produção)

Configuração completa:

```go
import "github.com/JailtonJunior94/devkit-go/pkg/observability/otel"

config := &otel.Config{
    ServiceName:     "order-service",
    ServiceVersion:  "1.0.0",
    Environment:     "production",
    OTLPEndpoint:    "otel-collector:4317",
    TraceSampleRate: 1.0, // 100% sampling
    LogLevel:        observability.LogLevelInfo,
    LogFormat:       observability.LogFormatJSON,
    ResourceAttributes: map[string]string{
        "deployment.region": "us-east-1",
        "k8s.cluster.name":  "prod-cluster",
    },
}

obs, err := otel.NewProvider(ctx, config)
if err != nil {
    log.Fatal(err)
}
defer obs.Shutdown(ctx)
```

## Configuração de Logs

### Formato JSON (Produção)

```go
config := &otel.Config{
    LogLevel:  observability.LogLevelInfo,
    LogFormat: observability.LogFormatJSON,
}
```

Saída:
```json
{"time":"2025-12-27T10:00:00Z","level":"INFO","msg":"order created","service":"order-service","trace_id":"abc123","span_id":"def456","order_id":"12345"}
```

### Formato TEXT (Desenvolvimento)

```go
config := &otel.Config{
    LogLevel:  observability.LogLevelDebug,
    LogFormat: observability.LogFormatText,
}
```

Saída:
```
time=2025-12-27T10:00:00Z level=INFO msg="order created" service=order-service trace_id=abc123 span_id=def456 order_id=12345
```

## Níveis de Log

```go
logger := obs.Logger()

logger.Debug(ctx, "detailed debug information", fields...)
logger.Info(ctx, "informational message", fields...)
logger.Warn(ctx, "warning message", fields...)
logger.Error(ctx, "error occurred", observability.Error(err))
```

## Métricas

### Counter (Monotonicamente Crescente)

```go
counter := obs.Metrics().Counter("requests.total", "Total requests", "1")
counter.Add(ctx, 1,
    observability.String("method", "POST"),
    observability.String("endpoint", "/orders"),
)
```

### Histogram (Distribuição de Valores)

```go
histogram := obs.Metrics().Histogram("request.duration", "Request duration", "ms")
histogram.Record(ctx, 245.5,
    observability.String("endpoint", "/orders"),
)
```

### UpDownCounter (Pode Crescer e Decrecer)

```go
activeConns := obs.Metrics().UpDownCounter("connections.active", "Active connections", "1")
activeConns.Add(ctx, 1)  // Conexão aberta
// ...
activeConns.Add(ctx, -1) // Conexão fechada
```

### Gauge (Valor Atual)

```go
obs.Metrics().Gauge("memory.usage", "Memory usage", "bytes",
    func(ctx context.Context) float64 {
        var m runtime.MemStats
        runtime.ReadMemStats(&m)
        return float64(m.Alloc)
    },
)
```

## Tracing Distribuído

### Propagação de Contexto

```go
// HTTP Server
func (h *OrderHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
    // Contexto já contém trace propagado do header HTTP
    ctx := r.Context()

    ctx, span := h.obs.Tracer().Start(ctx, "HTTP POST /orders",
        observability.WithSpanKind(observability.SpanKindServer),
    )
    defer span.End()

    // Passar contexto adiante
    err := h.useCase.Execute(ctx, dto)
}
```

### HTTP Client

```go
func (c *PaymentClient) ProcessPayment(ctx context.Context, order Order) error {
    ctx, span := c.obs.Tracer().Start(ctx, "PaymentClient.ProcessPayment",
        observability.WithSpanKind(observability.SpanKindClient),
    )
    defer span.End()

    // O trace será propagado automaticamente via headers HTTP
    req, _ := http.NewRequestWithContext(ctx, "POST", c.url, body)
    resp, err := c.httpClient.Do(req)

    if err != nil {
        span.RecordError(err)
        span.SetStatus(observability.StatusCodeError, "payment failed")
        return err
    }

    span.SetStatus(observability.StatusCodeOK, "payment processed")
    return nil
}
```

## Eventos e Atributos

```go
ctx, span := obs.Tracer().Start(ctx, "ProcessPayment")
defer span.End()

// Adicionar atributos
span.SetAttributes(
    observability.String("payment.method", "credit_card"),
    observability.Float64("amount", 99.99),
    observability.String("currency", "USD"),
)

// Adicionar eventos
span.AddEvent("payment.authorized",
    observability.String("auth_code", "ABC123"),
)

span.AddEvent("payment.captured",
    observability.String("transaction_id", "TXN456"),
)
```

## Logger com Campos Permanentes

```go
// Logger base
baseLogger := obs.Logger()

// Child logger com campos permanentes
userLogger := baseLogger.With(
    observability.String("user_id", userID),
    observability.String("session_id", sessionID),
)

// Todos os logs terão user_id e session_id
userLogger.Info(ctx, "user action performed",
    observability.String("action", "create_order"),
)
```

## High Throughput / Performance

### Batching Automático

O provider OpenTelemetry usa batch span processor por padrão:

```go
// Configurado automaticamente em otel/config.go
sdktrace.WithBatcher(exporter) // Batch para alta performance
```

### Sampling

Controle a taxa de sampling para reduzir overhead:

```go
config := &otel.Config{
    TraceSampleRate: 0.1, // 10% de sampling
}
```

### Zero Allocation Logging

Use slog internamente para performance otimizada:

```go
// Implementado em otel/logger.go usando slog
logger.Info(ctx, "message", fields...) // Zero allocations para campos primitivos
```

## Integração com Coralogix

```go
config := &otel.Config{
    ServiceName:    "order-service",
    OTLPEndpoint:   "ingress.coralogix.com:443",
    LogFormat:      observability.LogFormatJSON,
    // Adicione headers de autenticação via environment variables
}
```

## Testes

### Executar todos os testes

```bash
go test ./pkg/observability/... -v
```

### Benchmarks

```bash
go test ./pkg/observability/noop -bench=. -benchmem
```

Resultado esperado para NoOp:
```
BenchmarkNoopTracer-8    1000000000    0.5 ns/op    0 B/op    0 allocs/op
```

## Boas Práticas

1. **Sempre passe o contexto**: O trace ID é propagado via context
2. **Defer span.End()**: Garante que o span sempre será finalizado
3. **Use span kinds apropriados**: Server, Client, Internal, Producer, Consumer
4. **Log com contexto**: Sempre use `ctx` para correlação automática de traces
5. **Métricas com labels**: Use labels para segmentação de métricas
6. **Shutdown gracioso**: Sempre chame `obs.Shutdown(ctx)` no main

## Exemplo Completo

Veja o diretório `examples/` para exemplos completos de uso.

## Licença

MIT

## ✨ Moderno e Idiomático (Go 1.18+)

Este pacote foi modernizado para aproveitar os recursos mais recentes do Go:

### Melhorias de Linguagem

- ✅ **Usa `any` ao invés de `interface{}`** - Melhor legibilidade e alinhamento com Go moderno
- ✅ **Early returns** - Sem else/else if para código mais limpo e fácil de ler
- ✅ **Type-safe field helpers** - Funções `String()`, `Int()`, `Float64()`, etc.

### Melhorias de Performance

- ✅ **Zero allocations** para slices vazios (casos comuns otimizados)
- ✅ **Pre-alocação exata** de capacidade quando necessário
- ✅ **Funções de conversão centralizadas** (DRY - Don't Repeat Yourself)

### Melhorias de Observabilidade

- ✅ **Logging em todas as falhas** - Sem silent failures
- ✅ **Mensagens de erro descritivas** - Facilita debugging em produção
- ✅ **Redação automática de dados sensíveis** - Segurança por padrão

### Melhorias de Qualidade

- ✅ **Cobertura de testes > 38%** - Testes unitários abrangentes
- ✅ **Exemplos completos** - HTTP handlers, services, testes
- ✅ **Zero warnings** - staticcheck, go vet, golangci-lint

## 📖 Documentação Adicional

- [MIGRATION.md](./MIGRATION.md) - Guia de migração do pacote `o11y` antigo
- [Exemplos HTTP](./examples/http-handler/) - API REST completa
- [Exemplos Service Layer](./examples/service-layer/) - Padrões arquiteturais
- [Exemplos de Testes](./examples/testing/) - Testes com fake provider

## 🤝 Contribuindo

Contribuições são bem-vindas! Por favor:

1. Fork o projeto
2. Crie uma branch para sua feature (`git checkout -b feature/AmazingFeature`)
3. Commit suas mudanças (`git commit -m 'feat: Add some AmazingFeature'`)
4. Push para a branch (`git push origin feature/AmazingFeature`)
5. Abra um Pull Request

## 📝 Licença

Este projeto está sob a licença MIT. Veja o arquivo LICENSE para mais detalhes.
