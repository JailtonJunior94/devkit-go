# Event Dispatcher

Sistema de eventos desacoplado baseado no padrão Publish-Subscribe (Observer), com operações thread-safe e suporte completo a contextos.

## Visão Geral

### Propósito

O `pkg/events` oferece um mecanismo leve e type-safe para implementar comunicação assíncrona entre componentes de uma aplicação através do padrão de eventos. Permite que diferentes partes do sistema reajam a mudanças sem criar acoplamento direto.

### Quando Usar

✅ **Use quando você precisa:**
- Desacoplar módulos que precisam reagir a ações do sistema
- Implementar side-effects sem acoplar lógica de negócio (ex: enviar email após criar pedido)
- Criar sistemas extensíveis onde novos handlers podem ser adicionados sem modificar código existente
- Notificar múltiplos componentes sobre a mesma mudança de estado
- Implementar audit logs, métricas ou notificações de forma desacoplada

### Quando NÃO Usar

❌ **Não use quando:**
- Você precisa de comunicação entre microserviços (use message brokers como RabbitMQ, Kafka, NATS)
- Eventos precisam sobreviver a reinicializações da aplicação (use event sourcing com persistência)
- Você precisa de garantias de entrega (at-least-once, exactly-once)
- Handlers precisam processar eventos de forma assíncrona em goroutines separadas (esta lib é síncrona por design)
- Você tem um único consumidor para cada evento (use chamadas diretas)

### Problemas que Resolve

1. **Acoplamento**: Evita dependências diretas entre produtores e consumidores de eventos
2. **Extensibilidade**: Permite adicionar novos handlers sem modificar código existente (Open/Closed Principle)
3. **Manutenibilidade**: Centraliza o gerenciamento de eventos em um único dispatcher
4. **Testabilidade**: Facilita testes isolados de handlers individuais
5. **Thread-Safety**: Garante operações concorrentes seguras sem race conditions

## Princípios de Design

### Clean Code
- Interfaces minimalistas e focadas (Interface Segregation Principle)
- Nomes descritivos que revelam intenção
- Documentação clara sobre thread-safety e cancellation
- Validações explícitas com mensagens de erro específicas

### SOLID
- **Single Responsibility**: Cada handler tem uma única responsabilidade
- **Open/Closed**: Extensível via novos handlers sem modificar dispatcher
- **Liskov Substitution**: Qualquer implementação de `EventHandler` é intercambiável
- **Interface Segregation**: Interfaces mínimas (`Event`, `EventHandler`, `EventDispatcher`)
- **Dependency Inversion**: Depende de abstrações (`EventHandler`), não de implementações concretas

### DRY (Don't Repeat Yourself)
- Lógica de dispatching centralizada no `EventDispatcher`
- Handlers reutilizáveis em diferentes contextos
- Validações consistentes em todas as operações

### Baixo Acoplamento
- Produtores de eventos não conhecem consumidores
- Handlers não conhecem outros handlers
- Comunicação baseada em tipos de eventos (strings), não em tipos Go

### Escalabilidade

#### Projetos Pequenos (1-5 tipos de eventos)
- Overhead mínimo
- Setup simples: criar dispatcher, registrar handlers, disparar eventos
- Ideal para side-effects simples (logging, notificações)

#### Projetos Médios (5-20 tipos de eventos)
- Mantém clareza com organização por domínio
- Use `WithCapacity()` para otimizar alocações de memória
- Recomendado: um dispatcher por domínio (orders, users, payments)

#### Projetos Grandes (20+ tipos de eventos)
- Use múltiplos dispatchers por bounded context
- Implemente factories para padronizar criação de handlers
- Considere event sourcing para histórico completo
- Para processamento assíncrono real, migre para message brokers

## Instalação

```bash
go get github.com/JailtonJunior94/devkit-go
```

```go
import "github.com/JailtonJunior94/devkit-go/pkg/events"
```

### Dependências

Nenhuma dependência externa. Usa apenas a biblioteca padrão do Go:
- `context` (para cancellation e timeouts)
- `sync` (para thread-safety)
- `slices` (para operações em arrays)
- `errors` (para erros customizados)

## API Pública

### Interfaces

#### `Event`
Representa um evento com tipo e payload.

```go
type Event interface {
    GetEventType() string  // Identificador único do evento
    GetPayload() any       // Dados do evento (qualquer tipo)
}
```

**Importante:**
- `GetPayload()` retorna `any` por flexibilidade máxima
- Handlers **DEVEM** validar o tipo do payload com type assertion
- Use o padrão `payload, ok := event.GetPayload().(ExpectedType)` sempre

#### `EventHandler`
Processa eventos de um tipo específico.

```go
type EventHandler interface {
    Handle(ctx context.Context, event Event) error
}
```

**Importante:**
- Handlers são comparados por identidade (ponteiros)
- **SEMPRE** use pointer receivers (`*MyHandler`) e registre ponteiros
- Handlers **DEVEM** respeitar cancellation via `ctx.Done()`
- Retorne erro para interromper processamento de outros handlers

#### `EventDispatcher`
Gerencia registro e envio de eventos para handlers.

```go
type EventDispatcher interface {
    Register(eventType string, handler EventHandler) error
    Dispatch(ctx context.Context, event Event) error
    Remove(eventType string, handler EventHandler) error
    Has(eventType string, handler EventHandler) bool
    Clear()
}
```

**Importante:**
- **Thread-safe**: Todas as operações podem ser chamadas concorrentemente
- Handlers são executados **sincronamente** na ordem de registro
- Dispatch para no primeiro erro de handler

### Funções Públicas

#### `NewEventDispatcher`
Cria um novo dispatcher.

```go
func NewEventDispatcher(opts ...DispatcherOption) EventDispatcher
```

**Opções:**
- `WithCapacity(n int)`: Pré-aloca capacidade para `n` tipos de eventos

### Erros Públicos

```go
var (
    ErrHandlerAlreadyRegistered = errors.New("handler already registered")
    ErrEventNil                 = errors.New("event cannot be nil")
    ErrHandlerNil               = errors.New("handler cannot be nil")
    ErrEventTypeEmpty           = errors.New("event type cannot be empty")
)
```

## Exemplos de Uso

### Exemplo 1: Quick Start (Básico)

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/JailtonJunior94/devkit-go/pkg/events"
)

// 1. Definir evento
type OrderCreated struct {
    OrderID   string
    CustomerID string
    Amount    float64
}

func (e *OrderCreated) GetEventType() string { return "order.created" }
func (e *OrderCreated) GetPayload() any      { return e }

// 2. Criar handler
type EmailNotificationHandler struct{}

func (h *EmailNotificationHandler) Handle(ctx context.Context, event events.Event) error {
    // Validar tipo do payload
    order, ok := event.GetPayload().(*OrderCreated)
    if !ok {
        return fmt.Errorf("unexpected payload type: %T", event.GetPayload())
    }

    // Processar evento
    fmt.Printf("Enviando email para cliente %s sobre pedido %s\n",
        order.CustomerID, order.OrderID)
    return nil
}

func main() {
    ctx := context.Background()

    // 3. Criar dispatcher
    dispatcher := events.NewEventDispatcher()

    // 4. Registrar handler (use ponteiro!)
    emailHandler := &EmailNotificationHandler{}
    if err := dispatcher.Register("order.created", emailHandler); err != nil {
        log.Fatal(err)
    }

    // 5. Disparar evento
    event := &OrderCreated{
        OrderID:    "ORD-123",
        CustomerID: "CUST-456",
        Amount:     99.90,
    }

    if err := dispatcher.Dispatch(ctx, event); err != nil {
        log.Fatal(err)
    }
}
```

### Exemplo 2: Múltiplos Handlers para o Mesmo Evento

```go
type AuditLogHandler struct {
    logger *log.Logger
}

func (h *AuditLogHandler) Handle(ctx context.Context, event events.Event) error {
    order, ok := event.GetPayload().(*OrderCreated)
    if !ok {
        return fmt.Errorf("unexpected payload type: %T", event.GetPayload())
    }

    h.logger.Printf("AUDIT: Order %s created for customer %s",
        order.OrderID, order.CustomerID)
    return nil
}

type MetricsHandler struct {
    metricsClient *MetricsClient
}

func (h *MetricsHandler) Handle(ctx context.Context, event events.Event) error {
    order, ok := event.GetPayload().(*OrderCreated)
    if !ok {
        return fmt.Errorf("unexpected payload type: %T", event.GetPayload())
    }

    h.metricsClient.IncrementCounter("orders.created", 1)
    h.metricsClient.RecordValue("orders.amount", order.Amount)
    return nil
}

func main() {
    dispatcher := events.NewEventDispatcher()

    // Registrar múltiplos handlers para o mesmo evento
    dispatcher.Register("order.created", &EmailNotificationHandler{})
    dispatcher.Register("order.created", &AuditLogHandler{logger: log.Default()})
    dispatcher.Register("order.created", &MetricsHandler{metricsClient: metricsClient})

    // Todos os handlers serão executados na ordem de registro
    dispatcher.Dispatch(ctx, &OrderCreated{...})
}
```

### Exemplo 3: Uso com Context para Timeout

```go
func ProcessOrderWithTimeout(dispatcher events.EventDispatcher, order *OrderCreated) error {
    // Criar contexto com timeout de 5 segundos
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    // Se algum handler demorar mais de 5s, será cancelado
    if err := dispatcher.Dispatch(ctx, order); err != nil {
        if errors.Is(err, context.DeadlineExceeded) {
            return fmt.Errorf("event processing timed out after 5s")
        }
        return fmt.Errorf("failed to dispatch event: %w", err)
    }

    return nil
}
```

### Exemplo 4: Handler que Respeita Cancellation

```go
type SlowProcessingHandler struct{}

func (h *SlowProcessingHandler) Handle(ctx context.Context, event events.Event) error {
    // Simular processamento em múltiplos passos
    for i := 0; i < 10; i++ {
        // SEMPRE verificar cancellation em operações longas
        select {
        case <-ctx.Done():
            return ctx.Err() // Retorna context.Canceled ou context.DeadlineExceeded
        default:
        }

        // Processar passo
        time.Sleep(500 * time.Millisecond)
        fmt.Printf("Processando passo %d/10\n", i+1)
    }

    return nil
}
```

### Exemplo 5: Tratamento Avançado de Erros

```go
type ResilientHandler struct {
    maxRetries int
    backoff    time.Duration
}

func (h *ResilientHandler) Handle(ctx context.Context, event events.Event) error {
    var lastErr error

    for attempt := 1; attempt <= h.maxRetries; attempt++ {
        // Verificar cancellation antes de cada tentativa
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        // Tentar processar
        if err := h.doProcess(ctx, event); err != nil {
            lastErr = err
            log.Printf("Attempt %d/%d failed: %v", attempt, h.maxRetries, err)

            // Aguardar antes de retry (com backoff exponencial)
            backoff := h.backoff * time.Duration(attempt)
            time.Sleep(backoff)
            continue
        }

        return nil // Sucesso
    }

    return fmt.Errorf("failed after %d attempts: %w", h.maxRetries, lastErr)
}

func (h *ResilientHandler) doProcess(ctx context.Context, event events.Event) error {
    // Lógica de processamento aqui
    return nil
}
```

### Exemplo 6: Uso em Aplicações Reais (Repository Pattern)

```go
// Domain Event
type UserRegistered struct {
    UserID    string
    Email     string
    Name      string
    CreatedAt time.Time
}

func (e *UserRegistered) GetEventType() string { return "user.registered" }
func (e *UserRegistered) GetPayload() any      { return e }

// Repository
type UserRepository struct {
    db         *sql.DB
    dispatcher events.EventDispatcher
}

func (r *UserRepository) Create(ctx context.Context, user *User) error {
    // 1. Persistir usuário
    if err := r.db.ExecContext(ctx, "INSERT INTO users..."); err != nil {
        return fmt.Errorf("failed to create user: %w", err)
    }

    // 2. Disparar evento (side-effects são tratados por handlers)
    event := &UserRegistered{
        UserID:    user.ID,
        Email:     user.Email,
        Name:      user.Name,
        CreatedAt: time.Now(),
    }

    if err := r.dispatcher.Dispatch(ctx, event); err != nil {
        // Log erro mas não falha a operação principal
        log.Printf("Failed to dispatch user.registered event: %v", err)
    }

    return nil
}

// Handlers (side-effects desacoplados)
type WelcomeEmailHandler struct {
    emailService *EmailService
}

func (h *WelcomeEmailHandler) Handle(ctx context.Context, event events.Event) error {
    user, ok := event.GetPayload().(*UserRegistered)
    if !ok {
        return fmt.Errorf("unexpected payload type")
    }

    return h.emailService.SendWelcomeEmail(ctx, user.Email, user.Name)
}

type UserMetricsHandler struct {
    metricsService *MetricsService
}

func (h *UserMetricsHandler) Handle(ctx context.Context, event events.Event) error {
    h.metricsService.IncrementCounter("users.registered")
    return nil
}

// Setup
func NewUserRepository(db *sql.DB) *UserRepository {
    dispatcher := events.NewEventDispatcher()

    // Registrar todos os side-effects
    dispatcher.Register("user.registered", &WelcomeEmailHandler{emailService})
    dispatcher.Register("user.registered", &UserMetricsHandler{metricsService})

    return &UserRepository{
        db:         db,
        dispatcher: dispatcher,
    }
}
```

## Casos Comuns

### Caso 1: Audit Log Centralizado

```go
type AuditLogger struct {
    storage AuditStorage
}

func (a *AuditLogger) Handle(ctx context.Context, event events.Event) error {
    entry := AuditEntry{
        EventType: event.GetEventType(),
        Timestamp: time.Now(),
        Payload:   event.GetPayload(),
    }
    return a.storage.Save(ctx, entry)
}

// Registrar para TODOS os eventos
dispatcher.Register("order.created", auditLogger)
dispatcher.Register("order.cancelled", auditLogger)
dispatcher.Register("user.registered", auditLogger)
// ... etc
```

### Caso 2: Notificações Multi-Canal

```go
type NotificationHub struct {
    emailService *EmailService
    smsService   *SMSService
    pushService  *PushService
}

func (h *NotificationHub) Handle(ctx context.Context, event events.Event) error {
    notification, ok := event.GetPayload().(Notification)
    if !ok {
        return fmt.Errorf("expected Notification payload")
    }

    // Enviar por múltiplos canais em paralelo
    var wg sync.WaitGroup
    errors := make(chan error, 3)

    wg.Add(3)
    go func() {
        defer wg.Done()
        if err := h.emailService.Send(ctx, notification); err != nil {
            errors <- fmt.Errorf("email: %w", err)
        }
    }()

    go func() {
        defer wg.Done()
        if err := h.smsService.Send(ctx, notification); err != nil {
            errors <- fmt.Errorf("sms: %w", err)
        }
    }()

    go func() {
        defer wg.Done()
        if err := h.pushService.Send(ctx, notification); err != nil {
            errors <- fmt.Errorf("push: %w", err)
        }
    }()

    wg.Wait()
    close(errors)

    // Coletar erros
    var errs []error
    for err := range errors {
        errs = append(errs, err)
    }

    if len(errs) > 0 {
        return fmt.Errorf("notification failures: %v", errs)
    }

    return nil
}
```

### Caso 3: Cache Invalidation

```go
type CacheInvalidationHandler struct {
    cache Cache
}

func (h *CacheInvalidationHandler) Handle(ctx context.Context, event events.Event) error {
    switch event.GetEventType() {
    case "user.updated":
        user := event.GetPayload().(*User)
        return h.cache.Delete(ctx, fmt.Sprintf("user:%s", user.ID))

    case "product.updated":
        product := event.GetPayload().(*Product)
        return h.cache.Delete(ctx, fmt.Sprintf("product:%s", product.ID))

    default:
        return nil // Ignorar eventos desconhecidos
    }
}
```

## Casos de Uso Frequentes

### Padrão 1: Event Sourcing Simplificado

```go
type EventStore struct {
    events []events.Event
    mu     sync.RWMutex
}

type EventStoreHandler struct {
    store *EventStore
}

func (h *EventStoreHandler) Handle(ctx context.Context, event events.Event) error {
    h.store.mu.Lock()
    defer h.store.mu.Unlock()

    h.store.events = append(h.store.events, event)
    return nil
}

// Registrar para todos os eventos
dispatcher.Register("*", &EventStoreHandler{store: eventStore})
```

### Padrão 2: Conditional Handlers

```go
type ConditionalHandler struct {
    condition func(events.Event) bool
    handler   events.EventHandler
}

func (h *ConditionalHandler) Handle(ctx context.Context, event events.Event) error {
    if !h.condition(event) {
        return nil // Skip
    }
    return h.handler.Handle(ctx, event)
}

// Exemplo: só processar pedidos acima de R$ 100
highValueOnly := &ConditionalHandler{
    condition: func(e events.Event) bool {
        order := e.GetPayload().(*OrderCreated)
        return order.Amount >= 100.00
    },
    handler: &HighValueOrderHandler{},
}

dispatcher.Register("order.created", highValueOnly)
```

### Padrão 3: Handler Composição

```go
type CompositeHandler struct {
    handlers []events.EventHandler
}

func (h *CompositeHandler) Handle(ctx context.Context, event events.Event) error {
    for _, handler := range h.handlers {
        if err := handler.Handle(ctx, event); err != nil {
            return fmt.Errorf("composite handler failed: %w", err)
        }
    }
    return nil
}

// Agrupar múltiplos handlers
composite := &CompositeHandler{
    handlers: []events.EventHandler{
        &EmailHandler{},
        &SMSHandler{},
        &PushHandler{},
    },
}

dispatcher.Register("urgent.notification", composite)
```

## Anti-Patterns a Evitar

### ❌ Anti-Pattern 1: Handler sem Pointer Receiver

```go
// ERRADO
type MyHandler struct{}

func (h MyHandler) Handle(ctx context.Context, event events.Event) error {
    return nil
}

handler := MyHandler{} // Não é ponteiro
dispatcher.Register("event", handler) // Vai funcionar mas Remove/Has não funcionarão corretamente
```

**Solução:**
```go
// CORRETO
type MyHandler struct{}

func (h *MyHandler) Handle(ctx context.Context, event events.Event) error { // Pointer receiver
    return nil
}

handler := &MyHandler{} // Ponteiro
dispatcher.Register("event", handler)
```

### ❌ Anti-Pattern 2: Ignorar Context Cancellation

```go
// ERRADO - pode bloquear indefinidamente
func (h *MyHandler) Handle(ctx context.Context, event events.Event) error {
    time.Sleep(10 * time.Second) // Não verifica ctx.Done()
    return nil
}
```

**Solução:**
```go
// CORRETO
func (h *MyHandler) Handle(ctx context.Context, event events.Event) error {
    select {
    case <-time.After(10 * time.Second):
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

### ❌ Anti-Pattern 3: Handler sem Validação de Tipo

```go
// ERRADO - panic se payload for tipo inesperado
func (h *MyHandler) Handle(ctx context.Context, event events.Event) error {
    order := event.GetPayload().(*OrderCreated) // PANIC!
    return h.process(order)
}
```

**Solução:**
```go
// CORRETO
func (h *MyHandler) Handle(ctx context.Context, event events.Event) error {
    order, ok := event.GetPayload().(*OrderCreated)
    if !ok {
        return fmt.Errorf("unexpected payload type: %T", event.GetPayload())
    }
    return h.process(order)
}
```

### ❌ Anti-Pattern 4: Criar Dispatcher por Request

```go
// ERRADO - recria dispatcher a cada request
func HandleRequest(w http.ResponseWriter, r *http.Request) {
    dispatcher := events.NewEventDispatcher() // NÃO FAÇA ISSO
    dispatcher.Register("event", handler)
    dispatcher.Dispatch(r.Context(), event)
}
```

**Solução:**
```go
// CORRETO - criar uma vez no início da aplicação
type Service struct {
    dispatcher events.EventDispatcher
}

func NewService() *Service {
    dispatcher := events.NewEventDispatcher()
    dispatcher.Register("event", handler)
    return &Service{dispatcher: dispatcher}
}

func (s *Service) HandleRequest(w http.ResponseWriter, r *http.Request) {
    s.dispatcher.Dispatch(r.Context(), event) // Reusar
}
```

### ❌ Anti-Pattern 5: Handlers com Side-Effects Não-Idempotentes

```go
// ERRADO - se handler for registrado duas vezes, duplica
func (h *CounterHandler) Handle(ctx context.Context, event events.Event) error {
    h.counter++ // Não-idempotente
    return nil
}

dispatcher.Register("event", counterHandler)
dispatcher.Register("event", counterHandler) // Registrado 2x acidentalmente
dispatcher.Dispatch(ctx, event) // counter++ executado 2x
```

**Solução:**
```go
// CORRETO - verificar se já está registrado
if !dispatcher.Has("event", counterHandler) {
    dispatcher.Register("event", counterHandler)
}

// Ou: usar Register que retorna erro se já existir
if err := dispatcher.Register("event", counterHandler); err != nil {
    if errors.Is(err, events.ErrHandlerAlreadyRegistered) {
        // Já registrado, tudo bem
    } else {
        return err
    }
}
```

## Escalabilidade

### Projetos Pequenos (< 10 eventos/segundo)
- Setup padrão é suficiente
- Um único dispatcher global funciona bem
- Handlers síncronos não causam problemas de performance

**Exemplo:**
```go
var GlobalDispatcher = events.NewEventDispatcher()

func init() {
    GlobalDispatcher.Register("user.created", &EmailHandler{})
    GlobalDispatcher.Register("order.created", &NotificationHandler{})
}
```

### Projetos Médios (10-100 eventos/segundo)
- Use `WithCapacity()` para otimizar alocações
- Organize por bounded contexts (um dispatcher por domínio)
- Considere handlers assíncronos para operações longas

**Exemplo:**
```go
// Dispatcher por domínio
type EventBus struct {
    Orders   events.EventDispatcher
    Users    events.EventDispatcher
    Payments events.EventDispatcher
}

func NewEventBus() *EventBus {
    return &EventBus{
        Orders:   events.NewEventDispatcher(events.WithCapacity(10)),
        Users:    events.NewEventDispatcher(events.WithCapacity(5)),
        Payments: events.NewEventDispatcher(events.WithCapacity(8)),
    }
}

// Handler assíncrono para operações longas
type AsyncHandler struct {
    handler events.EventHandler
}

func (h *AsyncHandler) Handle(ctx context.Context, event events.Event) error {
    go func() {
        ctx := context.Background() // Não herda cancellation do request
        if err := h.handler.Handle(ctx, event); err != nil {
            log.Printf("Async handler error: %v", err)
        }
    }()
    return nil // Retorna imediatamente
}
```

### Projetos Grandes (> 100 eventos/segundo)

**Limitações desta biblioteca:**
- Handlers síncronos podem se tornar gargalo
- Sem garantias de entrega
- Sem persistência de eventos
- Sem backpressure

**Migração Recomendada:**

Use message brokers para alto throughput:

```go
// Fase 1: Adicionar adapter
type RabbitMQAdapter struct {
    channel *amqp.Channel
    dispatcher events.EventDispatcher
}

func (a *RabbitMQAdapter) Handle(ctx context.Context, event events.Event) error {
    // Publicar para RabbitMQ
    return a.channel.Publish(...)
}

// Fase 2: Migração gradual
dispatcher.Register("order.created", &RabbitMQAdapter{...})
dispatcher.Register("order.created", &LegacyHandler{}) // Mantém temporariamente

// Fase 3: Remover handlers locais após verificar que RabbitMQ funciona
dispatcher.Remove("order.created", legacyHandler)
```

**Alternativas para Alta Escala:**
- **RabbitMQ**: Para workloads complexos com roteamento avançado
- **NATS**: Para comunicação ultra-rápida e leve
- **Kafka**: Para event streaming e event sourcing
- **EventStoreDB**: Para event sourcing completo com projeções

## Testabilidade

### Mock de Dispatcher

```go
type MockDispatcher struct {
    DispatchedEvents []events.Event
    DispatchError    error
}

func (m *MockDispatcher) Dispatch(ctx context.Context, event events.Event) error {
    m.DispatchedEvents = append(m.DispatchedEvents, event)
    return m.DispatchError
}

func (m *MockDispatcher) Register(eventType string, handler events.EventHandler) error {
    return nil
}

// ... outros métodos

// Teste
func TestUserRepository_Create(t *testing.T) {
    mockDispatcher := &MockDispatcher{}
    repo := &UserRepository{dispatcher: mockDispatcher}

    repo.Create(ctx, user)

    // Verificar que evento foi disparado
    if len(mockDispatcher.DispatchedEvents) != 1 {
        t.Errorf("Expected 1 event, got %d", len(mockDispatcher.DispatchedEvents))
    }

    event := mockDispatcher.DispatchedEvents[0]
    if event.GetEventType() != "user.created" {
        t.Errorf("Expected user.created event, got %s", event.GetEventType())
    }
}
```

### Testar Handlers Isoladamente

```go
func TestEmailHandler_Handle(t *testing.T) {
    handler := &EmailNotificationHandler{
        emailService: &MockEmailService{},
    }

    event := &OrderCreated{
        OrderID: "ORD-123",
        Email:   "customer@example.com",
    }

    err := handler.Handle(context.Background(), event)

    if err != nil {
        t.Errorf("Handler failed: %v", err)
    }

    // Verificar que email foi enviado
    // ...
}
```

### Testar Integração Completa

```go
func TestEventFlow(t *testing.T) {
    // Criar dispatcher real
    dispatcher := events.NewEventDispatcher()

    // Spy handler para verificar execução
    var executed bool
    spyHandler := &SpyHandler{
        onHandle: func(ctx context.Context, event events.Event) error {
            executed = true
            return nil
        },
    }

    dispatcher.Register("test.event", spyHandler)

    // Disparar evento
    event := &TestEvent{}
    err := dispatcher.Dispatch(context.Background(), event)

    if err != nil {
        t.Errorf("Dispatch failed: %v", err)
    }

    if !executed {
        t.Error("Handler was not executed")
    }
}
```

## FAQ / Troubleshooting

### P: Por que `Remove()` ou `Has()` não funciona?

**R:** Você provavelmente não está usando ponteiros consistentemente.

```go
// ERRADO
handler := MyHandler{} // Value
dispatcher.Register("event", handler)
dispatcher.Has("event", handler) // false - cria nova cópia

// CORRETO
handler := &MyHandler{} // Ponteiro
dispatcher.Register("event", handler)
dispatcher.Has("event", handler) // true - mesmo ponteiro
```

### P: Handlers são executados em paralelo?

**R:** Não. Handlers são executados **sincronamente** na ordem de registro. Se você precisa de execução paralela, crie handlers assíncronos:

```go
type AsyncHandler struct {
    handler events.EventHandler
}

func (h *AsyncHandler) Handle(ctx context.Context, event events.Event) error {
    go h.handler.Handle(context.Background(), event)
    return nil // Retorna imediatamente
}
```

**Cuidado:** Handlers assíncronos não propagam erros e não respeitam cancellation do contexto original.

### P: Como garantir que um evento foi processado com sucesso?

**R:** Por design, `Dispatch()` retorna erro no primeiro handler que falha. Se você quer garantir processamento completo:

```go
err := dispatcher.Dispatch(ctx, event)
if err != nil {
    log.Printf("Event processing failed: %v", err)
    // Decidir: retry, compensar, alertar, etc.
}
```

### P: Posso usar o mesmo handler para múltiplos tipos de eventos?

**R:** Sim! Basta registrar a mesma instância para múltiplos tipos:

```go
auditHandler := &AuditHandler{}
dispatcher.Register("order.created", auditHandler)
dispatcher.Register("order.cancelled", auditHandler)
dispatcher.Register("user.registered", auditHandler)
```

O handler deve fazer type switch no payload se precisar de comportamento diferente:

```go
func (h *AuditHandler) Handle(ctx context.Context, event events.Event) error {
    switch event.GetEventType() {
    case "order.created":
        // ...
    case "order.cancelled":
        // ...
    case "user.registered":
        // ...
    }
    return nil
}
```

### P: Como debugar qual handler está causando erro?

**R:** Adicione logging dentro dos handlers:

```go
func (h *MyHandler) Handle(ctx context.Context, event events.Event) error {
    log.Printf("[%s] Processing event %s", reflect.TypeOf(h).Name(), event.GetEventType())

    err := h.doActualWork(ctx, event)

    if err != nil {
        log.Printf("[%s] Failed: %v", reflect.TypeOf(h).Name(), err)
    }

    return err
}
```

Ou crie um decorator:

```go
type LoggingHandler struct {
    name    string
    handler events.EventHandler
}

func (h *LoggingHandler) Handle(ctx context.Context, event events.Event) error {
    log.Printf("[%s] START processing %s", h.name, event.GetEventType())

    err := h.handler.Handle(ctx, event)

    if err != nil {
        log.Printf("[%s] ERROR: %v", h.name, err)
    } else {
        log.Printf("[%s] SUCCESS", h.name)
    }

    return err
}

// Uso
dispatcher.Register("event", &LoggingHandler{
    name: "EmailHandler",
    handler: &EmailHandler{},
})
```

### P: Como implementar retry em handlers?

**R:** Crie um wrapper de retry:

```go
type RetryHandler struct {
    handler    events.EventHandler
    maxRetries int
    backoff    time.Duration
}

func (h *RetryHandler) Handle(ctx context.Context, event events.Event) error {
    var lastErr error

    for attempt := 1; attempt <= h.maxRetries; attempt++ {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        if err := h.handler.Handle(ctx, event); err != nil {
            lastErr = err
            log.Printf("Attempt %d/%d failed: %v", attempt, h.maxRetries, err)
            time.Sleep(h.backoff * time.Duration(attempt))
            continue
        }

        return nil
    }

    return fmt.Errorf("failed after %d attempts: %w", h.maxRetries, lastErr)
}

// Uso
dispatcher.Register("event", &RetryHandler{
    handler:    &EmailHandler{},
    maxRetries: 3,
    backoff:    time.Second,
})
```

### P: Como limpar handlers registrados em testes?

**R:** Use `Clear()` no setup/teardown:

```go
func TestMyFeature(t *testing.T) {
    dispatcher := events.NewEventDispatcher()

    // Setup
    dispatcher.Register("test.event", handler1)
    dispatcher.Register("test.event", handler2)

    // Teste
    // ...

    // Cleanup
    dispatcher.Clear()
}
```

Ou crie um novo dispatcher para cada teste:

```go
func TestMyFeature(t *testing.T) {
    dispatcher := events.NewEventDispatcher() // Novo para cada teste
    // ...
}
```

### P: Performance degrada com muitos handlers?

**R:** Depende. Alguns cenários:

- **Poucos handlers (< 10 por evento)**: Impacto mínimo
- **Muitos handlers (> 50 por evento)**: Considere otimizações:
  - Use `WithCapacity()` no dispatcher
  - Handlers assíncronos para side-effects
  - Agrupe handlers relacionados em composite handlers

**Benchmark:**
```bash
go test -bench=. -benchmem ./pkg/events/
```

### P: Posso usar generics para type-safety?

**R:** Não diretamente, mas você pode criar wrappers type-safe:

```go
type TypedEvent[T any] struct {
    Type    string
    Payload T
}

func (e *TypedEvent[T]) GetEventType() string { return e.Type }
func (e *TypedEvent[T]) GetPayload() any      { return e.Payload }

type TypedHandler[T any] struct {
    handleFunc func(context.Context, T) error
}

func (h *TypedHandler[T]) Handle(ctx context.Context, event events.Event) error {
    payload, ok := event.GetPayload().(T)
    if !ok {
        return fmt.Errorf("unexpected payload type")
    }
    return h.handleFunc(ctx, payload)
}

// Uso
event := &TypedEvent[*OrderCreated]{
    Type:    "order.created",
    Payload: order,
}

handler := &TypedHandler[*OrderCreated]{
    handleFunc: func(ctx context.Context, order *OrderCreated) error {
        // Type-safe!
        return nil
    },
}

dispatcher.Register("order.created", handler)
```

## Licença

Este pacote faz parte do projeto devkit-go.
