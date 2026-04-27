# DevKit Go - Observabilidade

Uma camada de observabilidade unificada para aplicações Go, fornecendo logs estruturados de alta performance, métricas e rastreamento (tracing) com integração nativa ao OpenTelemetry.

## Objetivo

O pacote `observability` fornece uma interface padronizada para coleta de telemetria em microserviços. Foi projetado para alta performance, utilizando técnicas como campos com união discriminada para evitar alocações de memória (boxing) em caminhos críticos (hot paths).

## Principais Recursos

- **Alta Performance**: Campos com alocação zero para tipos comuns (string, int, bool, etc.).
- **Nativo OpenTelemetry**: Exportadores OTLP integrados para gRPC e HTTP.
- **API Unificada**: Configuração única para Logger, Tracer e Metrics.
- **Foco em Contexto**: Suporte nativo ao `context.Context` do Go para propagação de rastreamento.
- **Agnóstico a Framework**: Fácil de integrar com qualquer framework HTTP ou biblioteca de mensageria.

---

## Início Rápido

### 1. Inicializar o Provider

```go
import (
    "context"
    "log"
    "github.com/JailtonJunior94/devkit-go/pkg/observability/otel"
)

func main() {
    ctx := context.Background()

    // 1. Configurar o provider
    config := otel.DefaultConfig("meu-servico")
    config.OTLPEndpoint = "localhost:4317"
    config.Environment = "production"

    // 2. Criar o provider
    provider, err := otel.NewProvider(ctx, config)
    if err != nil {
        log.Fatal(err)
    }

    // 3. Garantir o shutdown gracioso
    defer func() {
        if err := provider.Shutdown(context.Background()); err != nil {
            log.Printf("falha ao desligar observabilidade: %v", err)
        }
    }()

    // 4. Obter as ferramentas
    logger := provider.Logger()
    tracer := provider.Tracer()
    metrics := provider.Metrics()
}
```

---

## 🪵 Logger (Logs Estruturados)

O logger utiliza uma abordagem estruturada com campos tipados para maximizar a performance.

### Uso

```go
import "github.com/JailtonJunior94/devkit-go/pkg/observability"

// Log com campos tipados
logger.Info(ctx, "usuário autenticado", 
    observability.String("user_id", "123"),
    observability.Int("attempt", 1),
)

// Criar um logger com campos persistentes (contexto)
childLogger := logger.With(observability.String("component", "auth"))
childLogger.Debug(ctx, "processando login")
```

### Melhores Práticas
- **Use Campos Tipados**: Prefira `observability.String()`, `observability.Int()`, etc., em vez de `observability.Any()` para evitar o "boxing" de interfaces e alocações desnecessárias.
- **Contexto é Obrigatório**: Sempre passe o `context.Context` para vincular os logs aos traces ativos.
- **Evite ConsoleLog em Produção**: A opção `ConsoleLog: true` utiliza travas síncronas que podem impactar a performance. Use apenas para desenvolvimento local.

---

## 📊 Métricas

Suporta os instrumentos padrão do OpenTelemetry: Counters, Histograms e Gauges.

### Uso

```go
// 1. Criar instrumentos
requestCounter := metrics.Counter("http_requests_total", "Total de requisições", "{request}")
durationHistogram := metrics.Histogram("request_duration", "Latência da requisição", "s")

// 2. Registrar dados
requestCounter.Increment(ctx, observability.String("method", "GET"))
durationHistogram.Record(ctx, 0.45, observability.String("method", "GET"))
```

---

## 🔍 Tracing (Rastreamento)

API de rastreamento simplificada construída sobre o OpenTelemetry.

### Uso

```go
// Iniciar um novo span
ctx, span := tracer.Start(ctx, "ProcessarPedido")
defer span.End()

// Adicionar atributos/eventos
span.SetAttributes(observability.String("order_id", "ABC"))
span.AddEvent("validacao_concluida")

if err := validateOrder(); err != nil {
    span.RecordError(err)
    span.SetStatus(observability.StatusCodeError, err.Error())
}
```

---

## 🌐 Microserviços HTTP

Para manter o rastreamento em chamadas HTTP, utilize a `HTTPInstrumentation` integrada.

### Requisições de Entrada (Servidor)

Envolva seus handlers usando a instrumentação para extrair automaticamente o contexto do trace e registrar métricas HTTP.

```go
instr := provider.HTTP()

// Dentro do seu middleware ou handler:
req := otel.HTTPRequest{
    Method: r.Method,
    Route:  "/users/:id",
    Target: r.URL.Path,
}

ctx, scope := instr.StartRequest(r.Context(), req)
defer scope.Finish(otel.HTTPResponse{StatusCode: 200})

// Continue com a lógica usando o contexto atualizado
logger.Info(ctx, "processando requisição") 
```

---

## 📩 Mensageria (Propagação de Contexto)

Para sistemas de mensageria (Kafka, RabbitMQ, etc.), você deve realizar a **Injeção** e **Extração** do contexto manualmente.

### Produtor (Injetar)

```go
// Criar um transportador (ex: baseado em mapa de headers)
carrier := make(otel.TextMapCarrier)

// Injetar o contexto atual no carrier
provider.Inject(ctx, carrier)

// Envie os headers do carrier junto com sua mensagem
// ex: kafkaMsg.Headers = carrierToKafkaHeaders(carrier)
```

### Consumidor (Extrair)

```go
// Extrair os headers da mensagem recebida para um carrier
carrier := otel.TextMapCarrier(headersExtraidos)

// Extrair o contexto do carrier
ctx, correlation := provider.Extract(ctx, carrier)

// Use o novo contexto para todas as operações subsequentes
ctx, span := tracer.Start(ctx, "processar_mensagem")
defer span.End()
```

---

## 🛠 Melhores Práticas e Recomendações

1. **Shutdown Gracioso**: Sempre chame `provider.Shutdown(ctx)` quando a aplicação for encerrada para garantir que todos os spans e métricas pendentes sejam enviados ao coletor.
2. **Tipo de Span**: Ao iniciar spans, use `WithSpanKind(observability.SpanKindClient)` para chamadas externas e `SpanKindServer` para fluxos de entrada.
3. **Tratamento de Erros**: Use `span.RecordError(err)` para garantir que exceções sejam rastreadas corretamente em seu backend (ex: Jaeger, Tempo).
4. **Cardinalidade de Atributos**: Evite usar valores de alta cardinalidade (como IDs de usuários ou e-mails) como **Labels de Métricas**. Use-os em **Campos de Log** ou **Atributos de Trace**.
5. **Sanitização**: Ative `config.Sanitize = true` se seus logs puderem conter dados sensíveis (PII), ciente do pequeno impacto na performance.

---

## Documentação Relacionada

- [Documentação OpenTelemetry Go](https://opentelemetry.io/docs/instrumentation/go/)
- [Especificação OTLP](https://opentelemetry.io/docs/specs/otlp/)
