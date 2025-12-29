# Dead Letter Queue (DLQ) - Guia Completo

## 📋 Índice

1. [Visão Geral](#visão-geral)
2. [Arquitetura](#arquitetura)
3. [Quick Start](#quick-start)
4. [Configuração](#configuração)
5. [Strategies](#strategies)
6. [DLQ Message Format](#dlq-message-format)
7. [Retry Logic](#retry-logic)
8. [Best Practices](#best-practices)
9. [Observabilidade](#observabilidade)
10. [Exemplos Completos](#exemplos-completos)

---

## Visão Geral

O sistema de **Dead Letter Queue (DLQ)** captura automaticamente mensagens que falharam no processamento após várias tentativas, preservando contexto completo para análise e reprocessamento.

### Features

- ✅ **Retry Automático** com backoff exponencial
- ✅ **Metadata Enriquecida** (erro, tentativas, timestamps, histórico)
- ✅ **Strategy Pattern** para diferentes tratamentos
- ✅ **Context Preservation** completo
- ✅ **Observabilidade** via logs estruturados
- ✅ **Configuração Flexível** via functional options
- ✅ **Production-Ready** e testado

---

## Arquitetura

```
┌─────────────┐
│   Message   │
└──────┬──────┘
       │
       ▼
┌─────────────────┐
│  Handler Exec   │
└──────┬──────────┘
       │
    Success? ────Yes───> Commit & Continue
       │
       No
       │
       ▼
┌─────────────────┐
│ Retry Attempt   │
│ (with backoff)  │
└──────┬──────────┘
       │
    Max Retries? ──No──> Retry Again
       │
       Yes
       │
       ▼
┌─────────────────┐
│   Send to DLQ   │
│  (with metadata)│
└──────┬──────────┘
       │
       ▼
┌─────────────────┐
│  Commit Original│
│    Message      │
└─────────────────┘
```

---

## Quick Start

### 1. Configuração Básica

```go
client, err := kafka.NewClient(
    kafka.WithBrokers("localhost:9092"),
    kafka.WithAuthPlaintext(),

    // Enable DLQ
    kafka.WithDLQEnabled(true),
    kafka.WithDLQTopic("events-dlq"),
    kafka.WithDLQMaxRetries(3),
)
```

### 2. Consumer com DLQ

```go
consumer, err := client.NewConsumer(
    kafka.WithGroupID("my-group"),
    kafka.WithTopics("events"),
    kafka.ConsumerWithDLQEnabled(true),
)

consumer.RegisterHandler("order.created", func(ctx context.Context, headers map[string]string, body []byte) error {
    // Se falhar após 3 tentativas, vai automaticamente para DLQ
    return processOrder(body)
})

consumer.Consume(ctx)
```

### 3. Processar DLQ

```go
dlqConsumer, err := client.NewConsumer(
    kafka.WithGroupID("dlq-processor"),
    kafka.WithTopics("events-dlq"),
)

dlqConsumer.RegisterHandler("", func(ctx context.Context, headers map[string]string, body []byte) error {
    var dlqMsg kafka.DLQMessage
    json.Unmarshal(body, &dlqMsg)

    // Análise, alerta, reprocessamento manual, etc.
    log.Printf("Failed: %s - Error: %s", dlqMsg.Topic, dlqMsg.Error)
    return nil
})
```

---

## Configuração

### Opções Globais (Client Level)

```go
kafka.NewClient(
    // Enable/disable DLQ
    kafka.WithDLQEnabled(true),

    // DLQ topic name
    kafka.WithDLQTopic("my-app-dlq"),

    // Max retry attempts (default: 3)
    kafka.WithDLQMaxRetries(5),

    // Base backoff duration (default: 2s)
    kafka.WithDLQRetryBackoff(2*time.Second),

    // Max backoff duration (default: 30s)
    kafka.WithDLQMaxRetryBackoff(1*time.Minute),

    // Service identifier
    kafka.WithDLQServiceName("order-service"),

    // Environment identifier
    kafka.WithDLQEnvironment("production"),

    // Include stack traces (default: false)
    kafka.WithDLQIncludeStackTrace(true),

    // Custom DLQ strategy
    kafka.WithDLQStrategy(myCustomStrategy),
)
```

### Opções de Consumer (Consumer Level)

```go
consumer, err := client.NewConsumer(
    kafka.WithGroupID("my-group"),
    kafka.WithTopics("events"),

    // Enable DLQ for this consumer
    kafka.ConsumerWithDLQEnabled(true),

    // Override DLQ topic for this consumer
    kafka.ConsumerWithDLQTopic("events-dlq-v2"),
)
```

---

## Strategies

### 1. **PublishToDLQStrategy** (Default)

Publica mensagens falhadas para um tópico DLQ.

```go
strategy := kafka.NewPublishToDLQStrategy(
    dlqProducer,
    "events-dlq",
    logger,
)

client, err := kafka.NewClient(
    kafka.WithDLQStrategy(strategy),
)
```

**Uso:** Produção - captura completa de falhas

### 2. **LogOnlyStrategy**

Apenas loga mensagens falhadas (não publica).

```go
strategy := kafka.NewLogOnlyStrategy(logger)

client, err := kafka.NewClient(
    kafka.WithDLQStrategy(strategy),
)
```

**Uso:** Desenvolvimento, testes, debug

### 3. **DiscardStrategy**

Descarta mensagens falhadas silenciosamente.

```go
strategy := kafka.NewDiscardStrategy()

client, err := kafka.NewClient(
    kafka.WithDLQStrategy(strategy),
)
```

**Uso:** Casos onde falhas são aceitáveis/esperadas

### 4. **Custom Strategy**

Implemente sua própria estratégia:

```go
type MyCustomStrategy struct {
    alerter Alerter
}

func (s *MyCustomStrategy) HandleFailure(ctx context.Context, msg *kafka.DLQMessage) error {
    // Send alert
    s.alerter.Send(fmt.Sprintf("Message failed: %s", msg.Error))

    // Store in database
    db.SaveFailedMessage(msg)

    return nil
}

func (s *MyCustomStrategy) Name() string {
    return "custom_alert_and_store"
}
```

---

## DLQ Message Format

Mensagens no DLQ contêm metadata enriquecida:

```json
{
  "topic": "orders",
  "partition": 0,
  "offset": 12345,
  "key": "order-123",
  "value": "eyJvcmRlcl9pZCI6MTIzfQ==",
  "headers": {
    "event_type": "order.created",
    "correlation_id": "abc-123"
  },
  "original_event": "order.created",

  "error": "database connection failed",
  "error_type": "*sql.DBError",
  "error_timestamp": "2025-12-29T10:30:45Z",

  "attempts": 5,
  "max_attempts": 5,
  "first_attempt": "2025-12-29T10:30:00Z",
  "last_attempt": "2025-12-29T10:30:45Z",

  "retry_history": [
    {
      "attempt": 1,
      "timestamp": "2025-12-29T10:30:00Z",
      "error": "database connection failed",
      "backoff": "2s"
    },
    {
      "attempt": 2,
      "timestamp": "2025-12-29T10:30:05Z",
      "error": "database connection failed",
      "backoff": "4s"
    }
  ],

  "consumer_group": "order-processor",
  "service_name": "order-service",
  "environment": "production",
  "metadata": {
    "host": "app-server-01",
    "version": "1.2.3"
  }
}
```

---

## Retry Logic

### Backoff Exponencial

```
Attempt 1: Backoff = 2s
Attempt 2: Backoff = 4s  (2s * 2)
Attempt 3: Backoff = 8s  (4s * 2)
Attempt 4: Backoff = 16s (8s * 2)
Attempt 5: Backoff = 30s (max reached)
```

### Fluxo Completo

1. **Primeira Falha**: Handler retorna erro
2. **Retry 1**: Aguarda 2s → tenta novamente
3. **Retry 2**: Aguarda 4s → tenta novamente
4. **Retry 3**: Aguarda 8s → tenta novamente
5. **Max Retries**: Envia para DLQ com histórico completo
6. **Commit**: Commita mensagem original para evitar reprocessamento

---

## Best Practices

### 1. **Configuração de Retries**

```go
// Desenvolvimento
kafka.WithDLQMaxRetries(1)           // Falha rápido
kafka.WithDLQRetryBackoff(1*time.Second)

// Produção
kafka.WithDLQMaxRetries(5)            // Mais tentativas
kafka.WithDLQRetryBackoff(2*time.Second)
kafka.WithDLQMaxRetryBackoff(1*time.Minute)
```

### 2. **Naming Convention**

```go
// Padrão: {original-topic}-dlq
"orders" → "orders-dlq"
"events" → "events-dlq"

// Com ambiente
"orders" → "orders-dlq-prod"
"events" → "events-dlq-staging"
```

### 3. **Monitoramento**

```go
// Sempre use logger estruturado
kafka.WithLogger(yourLogger)

// Identifique o serviço
kafka.WithDLQServiceName("order-service")
kafka.WithDLQEnvironment("production")
```

### 4. **Alertas**

Configure alertas para:
- Taxa de mensagens no DLQ
- Picos de falhas
- Tipos de erro recorrentes

### 5. **Reprocessamento**

```go
// Consumer dedicado para DLQ
dlqConsumer, err := client.NewConsumer(
    kafka.WithGroupID("dlq-reprocessor"),
    kafka.WithTopics("events-dlq"),
)

// Lógica de reprocessamento manual
dlqConsumer.RegisterHandler("", func(ctx context.Context, headers map[string]string, body []byte) error {
    var dlqMsg kafka.DLQMessage
    json.Unmarshal(body, &dlqMsg)

    // Análise
    if isRetryable(dlqMsg.Error) {
        return republishToOriginalTopic(dlqMsg)
    }

    // Alertar equipe
    alertTeam(dlqMsg)
    return nil
})
```

---

## Observabilidade

### Logs Estruturados

```go
// Ao enviar para DLQ
INFO: message sent to DLQ
  dlq_topic: "events-dlq"
  original_topic: "events"
  attempts: 5
  error: "database connection failed"
  consumer_group: "order-processor"
```

### Métricas Sugeridas

```
kafka_dlq_messages_total{topic, error_type, service}
kafka_dlq_retry_attempts{topic, attempt}
kafka_dlq_processing_time_seconds{topic}
```

### Headers DLQ

Todas as mensagens DLQ incluem headers:

```
dlq_version: "1.0"
dlq_original_topic: "orders"
dlq_error: "database connection failed"
dlq_attempts: "5"
dlq_service: "order-service"
dlq_environment: "production"
dlq_timestamp: "2025-12-29T10:30:45Z"
original_event_type: "order.created"
original_correlation_id: "abc-123"
```

---

## Exemplos Completos

### Exemplo 1: E-commerce com DLQ

```go
func setupOrderProcessor() {
    client, _ := kafka.NewClient(
        kafka.WithBrokers("kafka:9092"),
        kafka.WithAuthScram("app", "pass", auth.ScramSHA512),
        kafka.WithDLQEnabled(true),
        kafka.WithDLQTopic("orders-dlq"),
        kafka.WithDLQMaxRetries(5),
        kafka.WithDLQServiceName("order-service"),
        kafka.WithDLQEnvironment("production"),
    )

    consumer, _ := client.NewConsumer(
        kafka.WithGroupID("order-processor"),
        kafka.WithTopics("orders"),
    )

    consumer.RegisterHandler("order.created", handleOrderCreated)
    consumer.RegisterHandler("order.updated", handleOrderUpdated)

    consumer.ConsumeWithWorkerPool(context.Background(), 10)
}

func handleOrderCreated(ctx context.Context, headers map[string]string, body []byte) error {
    // Se falhar, tentará 5x antes de ir para DLQ
    return orderService.Create(ctx, body)
}
```

### Exemplo 2: Monitoramento de DLQ

```go
func setupDLQMonitor() {
    client, _ := kafka.NewClient(
        kafka.WithBrokers("kafka:9092"),
    )

    dlqConsumer, _ := client.NewConsumer(
        kafka.WithGroupID("dlq-monitor"),
        kafka.WithTopics("orders-dlq", "payments-dlq", "events-dlq"),
    )

    dlqConsumer.RegisterHandler("", func(ctx context.Context, headers map[string]string, body []byte) error {
        var dlqMsg kafka.DLQMessage
        json.Unmarshal(body, &dlqMsg)

        // Enviar para sistema de alertas
        alerting.SendAlert(fmt.Sprintf(
            "DLQ Alert - Service: %s, Topic: %s, Error: %s",
            dlqMsg.ServiceName,
            dlqMsg.Topic,
            dlqMsg.Error,
        ))

        // Armazenar para análise
        db.SaveDLQMessage(dlqMsg)

        // Métricas
        metrics.IncDLQMessages(dlqMsg.Topic, dlqMsg.ErrorType)

        return nil
    })

    dlqConsumer.Consume(context.Background())
}
```

---

## Troubleshooting

### Mensagens não vão para DLQ

✅ Verifique se DLQ está habilitado:
```go
kafka.WithDLQEnabled(true)
```

✅ Verifique se o tópico DLQ existe:
```bash
kafka-topics --list --bootstrap-server localhost:9092 | grep dlq
```

✅ Verifique logs do consumer

### Muitas mensagens no DLQ

❌ **Problema**: Alta taxa de falhas

✅ **Soluções**:
1. Analise tipos de erro no DLQ
2. Corrija bugs no handler
3. Aumente timeout de operações
4. Adicione circuit breaker

### DLQ crescendo infinitamente

✅ Configure um consumer dedicado para DLQ

✅ Implemente reprocessamento ou arquivamento

✅ Configure TTL no tópico DLQ (Kafka retention)

---

## Migration Guide

### Sem DLQ → Com DLQ

```go
// Antes
consumer.RegisterHandler("event", func(ctx context.Context, headers map[string]string, body []byte) error {
    return process(body) // Falhas perdidas
})

// Depois
client, _ := kafka.NewClient(
    kafka.WithDLQEnabled(true),
    kafka.WithDLQTopic("events-dlq"),
)

consumer.RegisterHandler("event", func(ctx context.Context, headers map[string]string, body []byte) error {
    return process(body) // Falhas vão para DLQ automaticamente
})
```

---

## Conclusão

O sistema DLQ fornece uma solução robusta e production-ready para:

✅ **Resiliência**: Não perder mensagens importantes
✅ **Observabilidade**: Contexto completo de falhas
✅ **Flexibilidade**: Múltiplas estratégias de tratamento
✅ **Produtividade**: Retry automático sem código boilerplate

**Recomendação**: Sempre habilite DLQ em produção! 🚀
