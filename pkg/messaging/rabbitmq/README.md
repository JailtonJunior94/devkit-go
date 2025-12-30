# RabbitMQ Client

Cliente RabbitMQ resiliente, seguro e pronto para produção, seguindo rigorosamente os padrões arquiteturais do projeto.

## Características

- **Arquitetura Resiliente**: Reconexão automática com backoff exponencial
- **Segurança por Padrão**: TLS habilitado, publisher confirms, validação de config
- **Thread-Safe**: Todos os componentes são seguros para uso concorrente
- **Context-Aware**: Suporte completo a `context.Context` para cancelamento e timeout
- **Observability**: Integração nativa com o sistema de observability do projeto
- **Strategy Pattern**: Três estratégias de conexão (Plain, TLS, Cloud)
- **Graceful Shutdown**: Encerramento ordenado respeitando operações em andamento

## Arquitetura

A implementação segue fielmente os padrões de:
- `pkg/database/postgres`: Estrutura, lifecycle, options pattern
- `pkg/http_server/server_fiber`: Config, health checks, observability

### Estrutura de Arquivos

```
pkg/messaging/rabbitmq/
├── client.go         # Client principal com New(), Channel(), Ping()
├── config.go         # Config com DefaultConfig() e Validate()
├── options.go        # Functional Options Pattern (WithXXX)
├── strategy.go       # ConnectionStrategy interface + implementações
├── connection.go     # Gerenciamento de conexão e reconexão
├── publisher.go      # Publisher com confirms e batch
├── consumer.go       # Consumer com worker pool e handlers
├── lifecycle.go      # Shutdown gracioso
├── health.go         # Health check para HTTP server
├── errors.go         # Erros customizados
└── example_test.go   # Exemplos de uso
```

## Strategies de Conexão

### PlainStrategy - Desenvolvimento Local

Para uso com Docker local (sem TLS):

```go
client, err := rabbitmq.New(
    o11y,
    rabbitmq.WithPlainConnection("localhost", "guest", "guest", "/"),
    rabbitmq.WithServiceName("dev-service"),
    rabbitmq.WithEnvironment("development"),
)
```

### TLSStrategy - TLS Customizado

Para ambientes com certificados específicos:

```go
client, err := rabbitmq.New(
    o11y,
    rabbitmq.WithTLSConnection(
        "rabbitmq.example.com",
        "user",
        "pass",
        "/production",
        "/path/to/ca.pem",
        "/path/to/client-cert.pem",
        "/path/to/client-key.pem",
    ),
    rabbitmq.WithServiceName("secure-service"),
)
```

### CloudStrategy - Produção (Padrão)

Para CloudAMQP, AWS MQ, Azure Service Bus, Kubernetes:

```go
client, err := rabbitmq.New(
    o11y,
    rabbitmq.WithCloudConnection(os.Getenv("RABBITMQ_URL")),
    rabbitmq.WithServiceName("production-service"),
    rabbitmq.WithServiceVersion("1.0.0"),
    rabbitmq.WithEnvironment("production"),
    rabbitmq.WithPublisherConfirms(true),
    rabbitmq.WithAutoReconnect(true),
)
```

## Uso Básico

### 1. Criar Cliente

```go
import (
    "github.com/JailtonJunior94/devkit-go/pkg/messaging/rabbitmq"
    "github.com/JailtonJunior94/devkit-go/pkg/observability/otel"
)

o11y := otel.NewProvider(...)

client, err := rabbitmq.New(
    o11y,
    rabbitmq.WithCloudConnection(os.Getenv("RABBITMQ_URL")),
    rabbitmq.WithServiceName("my-service"),
    rabbitmq.WithServiceVersion("1.0.0"),
)
if err != nil {
    log.Fatal(err)
}
defer client.Shutdown(context.Background())
```

### 2. Declarar Topologia

```go
// Exchange
client.DeclareExchange(ctx, "events", "topic", true, false, nil)

// Queue com DLQ
dlqArgs := amqp.Table{
    "x-dead-letter-exchange": "events.dlq",
}
client.DeclareQueue(ctx, "user-events", true, false, false, dlqArgs)

// Binding
client.BindQueue(ctx, "user-events", "user.*", "events", nil)
```

### 3. Publisher

```go
publisher := rabbitmq.NewPublisher(client)

err := publisher.Publish(
    ctx,
    "events",                    // exchange
    "user.created",              // routing key
    []byte(`{"user_id": "123"}`), // body
    rabbitmq.WithHeaders(map[string]interface{}{
        "event_type": "user.created",
    }),
    rabbitmq.WithMessageID("msg-123"),
)
```

### 4. Consumer

```go
consumer := rabbitmq.NewConsumer(
    client,
    rabbitmq.WithQueue("user-events"),
    rabbitmq.WithPrefetchCount(10),
    rabbitmq.WithWorkerPool(5), // 5 workers concorrentes
)

// Registrar handlers
consumer.RegisterHandler("user.created", func(ctx context.Context, msg rabbitmq.Message) error {
    var event UserCreatedEvent
    if err := json.Unmarshal(msg.Body, &event); err != nil {
        return err
    }

    log.Printf("User created: %s", event.UserID)
    return nil // ACK automático
})

// Iniciar consumo
go func() {
    if err := consumer.Consume(ctx); err != nil {
        log.Printf("Consumer error: %v", err)
    }
}()
```

## Configuração Avançada

### Health Check Integration

```go
healthChecks := map[string]serverfiber.HealthCheckFunc{
    "rabbitmq": client.HealthCheck(),
}

server := serverfiber.New(
    o11y,
    serverfiber.WithHealthChecks(healthChecks),
)
```

### Configuração Completa

```go
client, err := rabbitmq.New(
    o11y,
    rabbitmq.WithCloudConnection(os.Getenv("RABBITMQ_URL")),

    // Serviço
    rabbitmq.WithServiceName("my-service"),
    rabbitmq.WithServiceVersion("1.0.0"),
    rabbitmq.WithEnvironment("production"),

    // Timeouts
    rabbitmq.WithHeartbeat(10 * time.Second),
    rabbitmq.WithConnectionTimeout(30 * time.Second),
    rabbitmq.WithPublishTimeout(5 * time.Second),

    // Reconexão
    rabbitmq.WithReconnectConfig(
        5 * time.Minute,  // timeout
        1 * time.Second,  // initial interval
        30 * time.Second, // max interval
    ),
    rabbitmq.WithAutoReconnect(true),

    // Publisher
    rabbitmq.WithPublisherConfirms(true),

    // Consumer
    rabbitmq.WithDefaultPrefetchCount(10),
)
```

## Padrões Seguidos

### Do `pkg/database/postgres`

- ✅ Struct principal com `New()`
- ✅ Functional Options Pattern
- ✅ `Ping(ctx)` para healthcheck
- ✅ `Shutdown(ctx)` gracioso com `sync.Once`
- ✅ Thread-safety com `sync.RWMutex`
- ✅ Defaults seguros para produção
- ✅ Fail-fast no construtor
- ✅ Validação com panic se config inválida

### Do `pkg/http_server/server_fiber`

- ✅ Config struct com `DefaultConfig()` e `Validate()`
- ✅ Integração com `observability.Observability`
- ✅ Health checks estruturados
- ✅ Lifecycle management
- ✅ Erros customizados
- ✅ Comentários detalhados mas objetivos

## Segurança

- **TLS por Padrão**: CloudStrategy usa TLS 1.2+
- **Publisher Confirms**: Garante entrega de mensagens
- **Validação de Config**: Previne configurações inseguras
- **Sem Variáveis Globais**: Não há estado global mutável
- **Context Timeout**: Todas operações respeitam deadlines

## Resiliência

- **Reconexão Automática**: Backoff exponencial configurável
- **Health Checks**: Monitora conexão continuamente
- **Channel Recreation**: Recria channels automaticamente
- **Publisher Confirms**: Retry em caso de falha
- **Graceful Shutdown**: Aguarda operações em andamento

## Performance

- **Worker Pool**: Processa mensagens em paralelo
- **Prefetch**: Otimiza throughput com QoS
- **Connection Pooling**: Reutiliza conexões
- **Zero Allocations**: Minimiza garbage collection

## Exemplos Completos

Veja `example_test.go` para exemplos de:
- Cliente completo com producer e consumer
- Desenvolvimento local com PlainStrategy
- TLS customizado com TLSStrategy
- Health check integration
- Publisher com retry manual

## Migração do Código Antigo

### Antes (Código Antigo)

```go
conn, shutdownConn, shutdownCh, err := rabbitmq.NewConnection(url)
defer rabbitmq.Cleanup(shutdownConn, shutdownCh)

publisher := rabbitmq.NewRabbitMQPublisher(conn.Channel)
```

### Depois (Código Novo)

```go
client, err := rabbitmq.New(
    o11y,
    rabbitmq.WithCloudConnection(url),
    rabbitmq.WithServiceName("my-service"),
)
defer client.Shutdown(context.Background())

publisher := rabbitmq.NewPublisher(client)
```

## Qualidade de Código

- ✅ `go build` sem erros
- ✅ `go vet` sem warnings
- ✅ `gofmt` formatado
- ✅ `golangci-lint` aprovado (código produção)
- ✅ Thread-safe
- ✅ Context-aware
- ✅ Zero dependências de estado global
