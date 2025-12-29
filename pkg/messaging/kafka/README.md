# Kafka Client - DevKit Go

Camada de conexão com Apache Kafka em Go, segura, resiliente e pronta para produção.

## Características

- **Strategy Pattern para Autenticação**: Plaintext, PLAIN SASL, SCRAM, Confluent Cloud
- **Resiliente por padrão**: Retry automático com backoff exponencial, reconnection automática
- **Thread-safe**: Usa `atomic.Bool`, `sync.RWMutex` e `sync.Once`
- **Health checks**: Monitoramento de conectividade
- **Functional Options Pattern**: Configuração flexível e type-safe
- **Structured Logging**: Interface de logger customizável
- **Producer robusto**: Batch publishing, retry logic, múltiplos acks
- **Consumer flexível**: Worker pool, DLQ support, commit strategies
- **Graceful shutdown**: Context-aware com timeouts configuráveis

---

## Instalação

```bash
go get github.com/JailtonJunior94/devkit-go/pkg/messaging/kafka
```

---

## Início Rápido

### Confluent Cloud (Recomendado para Produção)

```go
package main

import (
    "context"
    "log"

    "github.com/JailtonJunior94/devkit-go/pkg/messaging/kafka"
)

func main() {
    ctx := context.Background()

    // Criar cliente
    client, err := kafka.NewClient(
        kafka.WithBrokers("pkc-xxxxx.us-east-1.aws.confluent.cloud:9092"),
        kafka.WithAuthConfluent("YOUR_API_KEY", "YOUR_API_SECRET"),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Conectar
    if err := client.Connect(ctx); err != nil {
        log.Fatal(err)
    }

    // Usar producer/consumer...
}
```

### Desenvolvimento Local (Docker)

```go
client, err := kafka.NewClient(
    kafka.WithBrokers("localhost:9092"),
    kafka.WithAuthPlaintext(), // ⚠️ Apenas desenvolvimento!
)
```

---

## Arquitetura

### 1. Estrutura de Arquivos

```
pkg/messaging/kafka/
├── client.go              # Client interface + implementação
├── config.go              # Configuração com defaults
├── options.go             # Functional Options Pattern
├── errors.go              # Erros pré-definidos
├── logger.go              # Logger interface
├── new_producer.go        # Producer com retry
├── new_consumer.go        # Consumer com worker pool
├── dlq.go                 # Dead Letter Queue
├── auth/
│   ├── strategy.go        # Strategy interface
│   ├── plaintext.go       # Sem autenticação (dev)
│   ├── plain.go           # SASL PLAIN + TLS
│   ├── scram.go           # SCRAM-SHA-256/512 + TLS
│   └── confluent.go       # Confluent Cloud (padrão)
└── example_complete_test.go
```

### 2. Strategy Pattern para Autenticação

Implementado para desacoplar autenticação da lógica de conexão:

```
                    ┌──────────────┐
                    │   Strategy   │
                    │  (interface) │
                    └──────┬───────┘
                           │
           ┌───────────────┼───────────────┬──────────────┐
           │               │               │              │
    ┌──────▼─────┐  ┌──────▼─────┐ ┌──────▼─────┐ ┌──────▼─────┐
    │ Plaintext  │  │    Plain   │ │    SCRAM   │ │ Confluent  │
    │  (local)   │  │ SASL+TLS   │ │ SCRAM+TLS  │ │ (default)  │
    └────────────┘  └────────────┘ └────────────┘ └────────────┘
```

**Por quê Strategy Pattern?**
- Permite trocar autenticação sem alterar código cliente
- Facilita testes (mock strategies)
- Isola complexidade de cada método de auth
- Suporta adição de novas strategies sem breaking changes

### 3. Padrões Aplicados

#### ✅ Functional Options Pattern

```go
type Option func(*config)

func WithBrokers(brokers ...string) Option {
    return func(c *config) {
        if len(brokers) > 0 {
            c.brokers = brokers
        }
    }
}

// Uso:
client, _ := kafka.NewClient(
    kafka.WithBrokers("broker1:9092"),
    kafka.WithAuthConfluent("key", "secret"),
    kafka.WithMaxRetries(5),
)
```

**Por quê?**
- Type-safe (erros em compile-time)
- Extensível sem quebrar API existente
- Defaults sensatos aplicados antes de options customizadas
- Auto-documentado (cada With* é explícito)

#### ✅ Thread-Safety

```go
type client struct {
    connected   atomic.Bool      // Flags de estado
    closed      atomic.Bool
    mu          sync.RWMutex     // Proteção de recursos compartilhados
    closeOnce   sync.Once        // Garante Close() executa apenas uma vez
}
```

**Por quê?**
- `atomic.Bool`: Leitura/escrita lock-free para flags simples
- `sync.RWMutex`: Permite múltiplas leituras concorrentes
- `sync.Once`: Idempotência em Close() e Shutdown()

#### ✅ Retry com Backoff Exponencial

```go
func calculateBackoff(current, max time.Duration) time.Duration {
    next := current * 2
    if next > max {
        return max
    }
    return next
}
```

**Por quê?**
- Evita sobrecarga de brokers Kafka em falhas
- Aumenta chances de sucesso em problemas transitórios
- Respeita context.Context para cancelamento

#### ✅ Context-Aware

Todos os métodos de I/O aceitam `context.Context`:

```go
func (c *client) Connect(ctx context.Context) error
func (c *client) HealthCheck(ctx context.Context) error
func (p *producer) Publish(ctx context.Context, ...) error
```

**Por quê?**
- Permite cancelamento externo
- Propagação de deadlines
- Request-scoped logging
- Graceful shutdown

---

## Estratégias de Autenticação

### 🏆 Confluent (Padrão - RECOMENDADO)

**Quando usar:** Confluent Cloud ou Confluent Platform em produção

```go
kafka.WithAuthConfluent("API_KEY", "API_SECRET")
```

**Configuração:**
- SASL_SSL security protocol
- SCRAM-SHA-512 por padrão
- TLS 1.2+ obrigatório
- Certificados validados automaticamente

**Vantagens:**
- Máxima segurança out-of-the-box
- Compatível com Confluent Cloud
- Não precisa configurar TLS manualmente

---

### 🔒 SCRAM (Kafka Auto-gerenciado)

**Quando usar:** Kafka clusters com SASL/SCRAM habilitado

```go
kafka.WithAuthScram("username", "password", auth.ScramSHA512)
```

**Configuração:**
- SCRAM-SHA-256 ou SCRAM-SHA-512
- TLS configurável
- Mais seguro que PLAIN

**Vantagens:**
- Não envia senha em texto claro
- Suportado por Kafka nativamente
- Challenge-response authentication

---

### 🔓 PLAIN (Compatibilidade)

**Quando usar:** Kafka clusters legados com SASL/PLAIN

```go
kafka.WithAuthPlain("username", "password")
```

**Configuração:**
- PLAIN SASL + TLS
- TLS obrigatório (sem plaintext)

**⚠️ Atenção:** Menos seguro que SCRAM. Use apenas se SCRAM não estiver disponível.

---

### 🚫 Plaintext (Apenas Desenvolvimento)

**Quando usar:** APENAS desenvolvimento local via Docker

```go
kafka.WithAuthPlaintext()
```

**⚠️ NUNCA use em produção!** Sem autenticação e sem criptografia.

---

## Configurações Importantes

### Timeouts

```go
kafka.WithDialTimeout(10*time.Second)        // Timeout para estabelecer conexão TCP
kafka.WithConnectTimeout(30*time.Second)     // Timeout total de conexão (com retries)
kafka.WithHealthCheckTimeout(5*time.Second)  // Timeout para health checks
```

**Por quê esses valores?**
- `DialTimeout (10s)`: Adequado para conexões cloud/datacenter
- `ConnectTimeout (30s)`: Permite 3 retries de 10s cada
- `HealthCheckTimeout (5s)`: Rápido o suficiente para alertas

### Retry & Reconnect

```go
kafka.WithMaxRetries(5)                          // Máximo de tentativas
kafka.WithRetryBackoff(2*time.Second)            // Backoff inicial
kafka.WithMaxRetryBackoff(1*time.Minute)         // Backoff máximo
kafka.WithReconnectEnabled(true)                 // Auto-reconnect
kafka.WithReconnectInterval(10*time.Second)      // Intervalo entre checks
```

**Por quê?**
- **MaxRetries (5)**: Balanceia resiliência vs fail-fast
- **Backoff (2s → 60s)**: Evita flood de requests
- **Reconnect**: Auto-recuperação sem restart manual

### Producer

```go
kafka.WithProducerBatchSize(100)                 // Mensagens por batch
kafka.WithProducerBatchTimeout(time.Second)      // Timeout do batch
kafka.WithProducerMaxAttempts(3)                 // Tentativas de envio
kafka.WithProducerRequiredAcks(-1)               // -1=all, 0=none, 1=leader
kafka.WithProducerCompression(3)                 // 0=none, 1=gzip, 2=snappy, 3=lz4, 4=zstd
```

**Por quê?**
- **BatchSize (100)**: Balance throughput vs latência
- **RequiredAcks (-1)**: Máxima durabilidade (all replicas)
- **Compression (LZ4)**: Melhor balance CPU vs compressão

### Consumer

```go
kafka.WithConsumerGroupID("my-service")          // Grupo de consumidores
kafka.WithConsumerTopics("events.orders")        // Tópicos
kafka.WithConsumerStartOffset(-1)                // -1=newest, -2=oldest
kafka.WithConsumerCommitInterval(5*time.Second)  // Auto-commit interval
kafka.WithConsumerMaxBytes(10e6)                 // 10MB por fetch
```

**Por quê?**
- **StartOffset (-1)**: Evita reprocessar histórico inteiro
- **CommitInterval (5s)**: Balance entre performance e at-least-once
- **MaxBytes (10MB)**: Limita memória usada

---

## Exemplos Avançados

### Producer com Retry e Headers

```go
producer, _ := client.NewProducer("orders")

msg := &messaging.Message{
    Body: []byte(`{"order_id":"12345"}`),
    Headers: []messaging.Header{
        {Key: "event_type", Value: []byte("order.created")},
        {Key: "version", Value: []byte("v1")},
    },
}

// Retry automático em caso de falha
err := producer.Publish(ctx, "orders", "order-12345", map[string]string{
    "correlation_id": "req-abc",
}, msg)
```

### Consumer com Worker Pool

```go
consumer, _ := client.NewConsumer(
    kafka.WithGroupID("order-processor"),
    kafka.WithTopics("orders"),
)

consumer.RegisterHandler("order.created", func(ctx context.Context, headers map[string]string, body []byte) error {
    // Thread-safe processing
    return processOrder(body)
})

// 10 workers processando em paralelo
consumer.ConsumeWithWorkerPool(ctx, 10)

// Monitorar erros
go func() {
    for err := range consumer.Errors() {
        log.Printf("Error: %v", err)
    }
}()
```

### Health Check Periódico

```go
ticker := time.NewTicker(30 * time.Second)
defer ticker.Stop()

for range ticker.C {
    if err := client.HealthCheck(ctx); err != nil {
        // Alertar equipe, registrar métrica, etc.
        log.Printf("Kafka unhealthy: %v", err)
    }
}
```

### Graceful Shutdown

```go
// Capturar sinais de sistema
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

<-sigChan

// Shutdown gracioso com timeout
shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

if err := producer.Close(); err != nil {
    log.Printf("Producer close error: %v", err)
}

if err := consumer.Close(); err != nil {
    log.Printf("Consumer close error: %v", err)
}

if err := client.Close(); err != nil {
    log.Printf("Client close error: %v", err)
}
```

---

## Boas Práticas

### ✅ DO

- Use `WithAuthConfluent` para produção
- Configure retry e reconnect
- Monitore health checks periodicamente
- Use structured logging
- Feche resources com `defer`
- Use context.Context para cancelamento
- Configure timeouts apropriados
- Use worker pool para alto throughput

### ❌ DON'T

- Não use `WithAuthPlaintext()` em produção
- Não ignore erros de Close()
- Não crie múltiplos clients para o mesmo cluster
- Não compartilhe producers entre goroutines sem sincronização
- Não use `InsecureSkipVerify` em produção
- Não bloqueie handlers de consumer por muito tempo
- Não ignore canal de erros do consumer

---

## Erros Comuns

### ErrClientNotConnected

**Causa:** Tentou criar producer/consumer antes de chamar `Connect()`

**Solução:**
```go
client.Connect(ctx)  // ← Chame antes de NewProducer/NewConsumer
producer, _ := client.NewProducer("topic")
```

### ErrConnectionFailed

**Causa:** Não conseguiu conectar após todas as retries

**Solução:**
- Verifique brokers estão acessíveis
- Verifique credenciais estão corretas
- Aumente `WithMaxRetries` se necessário

### ErrHealthCheckFailed

**Causa:** Conexão foi perdida

**Solução:**
- Se `reconnectEnabled=true`, aguarde auto-reconnect
- Caso contrário, chame `Connect()` novamente

---

## Troubleshooting

### Logs Estruturados

Implemente a interface `Logger` e passe com `WithLogger`:

```go
type myLogger struct{}

func (l *myLogger) Info(ctx context.Context, msg string, fields ...kafka.Field) {
    // Seu logger estruturado aqui
}

// Use:
kafka.WithLogger(&myLogger{})
```

### Métricas

Monitore:
- `client.IsConnected()` - Conectividade
- `client.HealthCheck(ctx)` - Latência/disponibilidade
- Erros do canal `consumer.Errors()`
- Taxa de retry de producer

---

## Comparação com Padrões do Projeto

Este pacote replica fielmente os padrões de:

### `pkg/database/postgres`
- ✅ Functional Options Pattern
- ✅ Comentários detalhados com "Por quê"
- ✅ Defaults sensatos documentados
- ✅ Fail-fast validation
- ✅ Context-aware methods

### `pkg/http_server/server_fiber`
- ✅ Structured logging
- ✅ Health checks
- ✅ Graceful shutdown com `sync.Once`
- ✅ Middleware pattern (strategies)
- ✅ Error handling consistente

---

## Roadmap

- [ ] Suporte a transações (Kafka 0.11+)
- [ ] Schema Registry integration
- [ ] Exactly-once semantics
- [ ] Compression benchmarks
- [ ] Observability hooks (OpenTelemetry)

---

## Licença

Este pacote faz parte do DevKit Go e segue a mesma licença do projeto.
