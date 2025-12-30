# Arquitetura e Documentação de Packages - DevKit Go

> **Documentação técnica estratégica e orientada à reutilização**
> Versão: 1.0.0
> Data: 2025-12-30
> Autor: Sistema Automatizado de Documentação

---

# Visão Geral do Projeto

## Filosofia Arquitetural

Este projeto segue rigorosamente os princípios de **Clean Architecture**, **Domain-Driven Design (DDD)** e **SOLID**, com foco absoluto em:

- **Separação de Responsabilidades**: Cada package tem um propósito único e bem definido
- **Inversão de Dependências**: Abstrações (interfaces) governam o fluxo de dependências
- **Baixo Acoplamento**: Packages não conhecem detalhes de implementação uns dos outros
- **Alta Coesão**: Funcionalidades relacionadas vivem juntas dentro de um mesmo package
- **Reutilização**: Cada package pode ser extraído e usado em outros projetos Go

## Princípios Adotados

### 1. Clean Architecture
- **Camadas bem definidas**: Domínio → Aplicação → Infraestrutura → Apresentação
- **Regra de Dependência**: Camadas internas não conhecem camadas externas
- **Boundaries claros**: Transformações acontecem apenas nos limites das camadas

### 2. SOLID
- **Single Responsibility**: Cada package resolve um único problema
- **Open/Closed**: Extensível via composição e interfaces
- **Liskov Substitution**: Implementações são intercambiáveis
- **Interface Segregation**: Interfaces pequenas e focadas
- **Dependency Inversion**: Dependa de abstrações, não de implementações

### 3. Go Idiomático
- **Simplicidade sobre complexidade**: Código claro vence código "inteligente"
- **Composição sobre herança**: Use embedding e interfaces
- **Erros explícitos**: Sempre retorne e trate erros
- **Concorrência segura**: Thread-safety por design
- **Zero magic**: Sem reflexão desnecessária, sem globals mutáveis

## Relação entre Packages

```
┌─────────────────────────────────────────────────────────┐
│                    Application Layer                     │
│              (Use Cases / Business Logic)                │
└───────────────────┬─────────────────────────────────────┘
                    │
        ┌───────────┼───────────┐
        │           │           │
        ▼           ▼           ▼
┌──────────┐  ┌──────────┐  ┌──────────┐
│   VOs    │  │   LINQ   │  │  Entity  │
│ (Domain) │  │(Utility) │  │ (Domain) │
└──────────┘  └──────────┘  └──────────┘
        │
        └───────────┬───────────────────────────────┐
                    │                               │
                    ▼                               ▼
        ┌────────────────────┐        ┌────────────────────┐
        │   Observability    │────────│   HTTP Server      │
        │   (Cross-Cutting)  │        │  (Presentation)    │
        └────────────────────┘        └────────────────────┘
                    │
        ┌───────────┼───────────┐
        │           │           │
        ▼           ▼           ▼
┌──────────┐  ┌──────────┐  ┌──────────┐
│ Database │  │Messaging │  │HttpClient│
│  (Infra) │  │  (Infra) │  │  (Infra) │
└──────────┘  └──────────┘  └──────────┘
```

### Fluxo de Dependências
1. **VOs e Entity**: Núcleo do domínio, sem dependências externas
2. **Observability**: Cross-cutting concern injetado em toda a aplicação
3. **Database, Messaging, HttpClient**: Infraestrutura que implementa interfaces do domínio
4. **HTTP Server**: Camada de apresentação que orquestra use cases
5. **LINQ**: Utilitário funcional independente de qualquer camada

## Nível de Abstração Esperado

### Packages de Domínio (Menor Abstração)
- **pkg/vos**: Representações concretas de conceitos de negócio
- **pkg/entity**: Entidades com identidade e ciclo de vida

### Packages de Aplicação (Média Abstração)
- **pkg/linq**: Operações funcionais genéricas sobre coleções
- **Use Cases**: Orquestração de regras de negócio (não incluído neste doc)

### Packages de Infraestrutura (Alta Abstração)
- **pkg/database**: Abstração sobre persistência relacional
- **pkg/messaging**: Abstração sobre message brokers
- **pkg/httpserver**: Abstração sobre servidores HTTP
- **pkg/observability**: Abstração sobre telemetria

## Objetivo de Tornar Packages Independentes

Cada package foi projetado para ser:

1. **Autocontido**: Todas as dependências são explícitas
2. **Versionável**: Pode evoluir independentemente
3. **Testável**: Possui suas próprias suítes de testes
4. **Documentado**: README.md individual com exemplos
5. **Portável**: Pode ser copiado para outro projeto Go

## Como Usar Esses Packages Fora Deste Projeto

### Opção 1: Go Modules (Recomendado)
```bash
go get github.com/JailtonJunior94/devkit-go/pkg/vos
go get github.com/JailtonJunior94/devkit-go/pkg/observability
```

### Opção 2: Cópia Direta
```bash
# Copiar package completo
cp -r pkg/vos /outro-projeto/pkg/vos
cp -r pkg/linq /outro-projeto/pkg/linq
```

### Opção 3: Vendor (Projetos Legados)
```bash
go mod vendor
```

---

# pkg/database

## Responsabilidade

O package `pkg/database` é responsável por **abstrair e gerenciar conexões com bancos de dados relacionais**, garantindo:

- Gerenciamento seguro do pool de conexões
- Lifecycle completo (criação → uso → shutdown)
- Health checks para monitoramento
- Transações atômicas via Unit of Work
- Thread-safety e resiliência

### O que está dentro do escopo
- Configuração e criação de conexões SQL
- Pool de conexões otimizado para produção
- Abstrações para queries e transações
- Graceful shutdown respeitando contexto
- Suporte a PostgreSQL (via pgx/v5)

### O que **não** é responsabilidade do package
- Mapeamento objeto-relacional (ORM)
- Migrations de schema
- Query builders ou DSLs
- Lógica de negócio ou regras de domínio
- Caching de queries

## Conceitos-Chave

### Gerenciamento de Conexões
- **Pool de conexões**: Reutiliza conexões para reduzir latência
- **Configuração padrão segura**: 25 conexões máximas, 6 idle
- **Rotação automática**: Conexões são renovadas a cada 5 minutos
- **Fail-fast**: Valida conectividade no momento da criação

### Configuração
- **Functional Options Pattern**: Extensível sem quebrar compatibilidade
- **Defaults seguros**: Valores otimizados para produção
- **Validação na construção**: Impossível criar instância inválida

### Ciclo de Vida
- **Criação**: `New(uri, ...options)` com ping imediato
- **Uso**: `DB()` retorna `*sql.DB` thread-safe
- **Health Check**: `Ping(ctx)` para verificar saúde
- **Encerramento**: `Shutdown(ctx)` gracioso respeitando timeout

### Segurança
- **Thread-safety**: Mutex protege estado durante shutdown
- **Context-aware**: Respeita cancelamento e timeouts
- **Prevenção de leaks**: Fecha conexões mesmo em caso de erro

### Resiliência
- **Graceful shutdown**: Aguarda queries ativas finalizarem
- **Timeouts configuráveis**: Evita deadlocks
- **Idempotência**: `Shutdown()` pode ser chamado múltiplas vezes

### Confiabilidade
- **Unit of Work**: Garante atomicidade de transações
- **Isolation Levels**: Configurável por transação
- **Panic Recovery**: Rollback automático em caso de panic
- **Validação de contexto**: Verifica cancelamento antes e depois de transações

## Como reutilizar em outras aplicações

### Quando utilizar
- Aplicações que precisam de conexão com PostgreSQL
- Serviços que exigem transações atômicas
- Sistemas com alta concorrência de queries
- Aplicações que precisam de health checks confiáveis

### Quando evitar
- Bancos NoSQL (MongoDB, Redis, etc.)
- Aplicações que precisam de ORM completo (use GORM/SQLBoiler separadamente)
- Sistemas que não usam PostgreSQL (atualmente limitado a pgx)

### Boas práticas de integração
1. **Injete a interface DBTX, não *sql.DB**:
```go
type Repository interface {
    FindByID(ctx context.Context, db database.DBTX, id string) (*Entity, error)
}
```

2. **Use Unit of Work para transações**:
```go
uow := uow.NewUnitOfWork(db.DB())
err := uow.Do(ctx, func(ctx context.Context, tx database.DBTX) error {
    // Todas as operações aqui são atômicas
    return nil
})
```

3. **Configure pool para seu workload**:
```go
db, err := postgres.New(
    uri,
    postgres.WithMaxOpenConns(100),  // Alta concorrência
    postgres.WithConnMaxLifetime(10 * time.Minute),
)
```

### Cuidados com concorrência e ciclo de vida
- **Nunca** chame `Close()` diretamente no `*sql.DB` retornado
- **Sempre** use `Shutdown(ctx)` para encerrar
- **Evite** criar múltiplas instâncias de Database apontando para o mesmo banco
- **Compartilhe** a mesma instância de Database entre goroutines

## Exemplo conceitual de uso

```go
// 1. Criar conexão (main.go ou inicialização)
db, err := postgres.New(
    "postgres://user:pass@host:5432/dbname",
    postgres.WithMaxOpenConns(50),
    postgres.WithConnMaxLifetime(5 * time.Minute),
)
if err != nil {
    log.Fatal(err)
}
defer db.Shutdown(context.Background())

// 2. Injetar em repositórios
userRepo := NewUserRepository(db.DB())

// 3. Usar em queries simples
user, err := userRepo.FindByID(ctx, userID)

// 4. Usar em transações complexas
uow := uow.NewUnitOfWork(db.DB())
err = uow.Do(ctx, func(ctx context.Context, tx database.DBTX) error {
    // Criar usuário
    if err := userRepo.Create(ctx, tx, user); err != nil {
        return err
    }

    // Criar perfil associado
    if err := profileRepo.Create(ctx, tx, profile); err != nil {
        return err
    }

    // Commit automático se não houver erro
    return nil
})

// 5. Health check para HTTP server
http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
    if err := db.Ping(r.Context()); err != nil {
        w.WriteHeader(http.StatusServiceUnavailable)
        return
    }
    w.WriteHeader(http.StatusOK)
})
```

## Garantias do package

### Thread-safety
- ✅ Todos os métodos são thread-safe
- ✅ `*sql.DB` subjacente é concorrente por design
- ✅ Mutex protege estado durante shutdown

### Comportamento previsível em falhas
- ✅ Erros sempre retornados, nunca panic (exceto validação)
- ✅ Rollback automático em caso de erro ou panic
- ✅ Conexões fechadas mesmo em caso de erro de ping

### Estabilidade da API pública
- ✅ Functional Options garantem extensibilidade sem breaking changes
- ✅ Interfaces estáveis (`DBTX`, `UnitOfWork`)
- ✅ Compatível com `database/sql` padrão do Go

---

# pkg/http_server

## Responsabilidade

O package `pkg/httpserver` é responsável por **abstrair e gerenciar servidores HTTP**, fornecendo:

- Inicialização e configuração de servidor HTTP/HTTPS
- Sistema de rotas e middlewares componível
- Graceful shutdown respeitando conexões ativas
- Error handling centralizado
- Extensibilidade via functional options

### Papel do package na aplicação
- **Camada de Apresentação**: Recebe requisições externas
- **Orquestração**: Conecta handlers a use cases
- **Cross-cutting**: Aplica middlewares (logging, auth, CORS)
- **Lifecycle**: Gerencia startup e shutdown do servidor

### Limites claros de responsabilidade
**Responsável por**:
- Gerenciar servidor HTTP (go-chi)
- Registrar rotas e middlewares
- Graceful shutdown
- Error handling de handlers

**Não responsável por**:
- Lógica de negócio (vive em use cases)
- Validação de domínio (vive em entities/vos)
- Serialização de payload (responsabilidade do handler)
- Autenticação/Autorização (vive em middlewares dedicados)

### O que ele abstrai do framework HTTP subjacente
- **Chi Router**: Esconde detalhes de implementação do go-chi
- **Middlewares**: Padroniza assinatura de middlewares
- **Error Handling**: Unifica tratamento de erros via `ErrorHandler`
- **Lifecycle**: Abstrai `ListenAndServe` e `Shutdown`

## Conceitos-Chave

### Inicialização do servidor
- **Configuração padrão segura**: Timeouts conservadores para produção
- **Functional Options**: Customização sem breaking changes
- **Fail-fast**: Valida configuração antes de iniciar
- **Non-blocking**: Servidor roda em goroutine separada

### Middlewares
- **Composição**: Middlewares são compostos de forma encadeada
- **Ordem de execução**: Ordem de registro importa
- **Globais vs. Locais**: Middlewares podem ser aplicados globalmente ou por rota
- **Stateless**: Middlewares não devem manter estado mutável

### Rotas
- **Registro dinâmico**: Rotas podem ser adicionadas mesmo após `Run()`
- **Thread-safe**: Mutex protege registro concorrente
- **RESTful**: Suporte a todos os métodos HTTP
- **Error-aware**: Handlers retornam erro, não panic

### Graceful shutdown
- **Context-aware**: Respeita timeout do contexto
- **Connection draining**: Aguarda requisições ativas finalizarem
- **Shutdown listener**: Canal para notificar término
- **Idempotente**: Pode ser chamado múltiplas vezes

### Configuração e extensibilidade
- **Defaults seguros**: ReadTimeout, WriteTimeout, IdleTimeout
- **Customização total**: Todos os aspectos configuráveis via options
- **Testável**: Implementa `http.Handler` para testes

## Integração com outras aplicações

### Passos conceituais de integração
1. **Criar servidor**: `New(...options)`
2. **Registrar middlewares globais**: `WithGlobalMiddlewares(...)`
3. **Registrar rotas**: `RegisterRoute(route)` ou `WithRoutes(...)`
4. **Iniciar**: `shutdown := server.Run()`
5. **Graceful shutdown**: `shutdown(ctx)`

### Pontos de atenção
- **Ordem de middlewares**: Recovery → RequestID → Logging → Auth → Handler
- **Error handler**: Customize via `WithErrorHandler()` para controle fino
- **Timeouts**: Ajuste para seu workload (APIs de streaming precisam de timeouts maiores)
- **Context propagation**: Request ID é injetado automaticamente no contexto

### Customizações esperadas
- **ErrorHandler**: Implementar lógica de mapeamento erro→status HTTP
- **Middlewares**: Criar middlewares específicos do domínio (auth, rate limit)
- **Rotas**: Definir estrutura de rotas da aplicação
- **Observability**: Integrar com sistema de observabilidade

## Padrões adotados

### Composição
- Servidor compõe `http.Server` + `chi.Mux`
- Middlewares são funções que recebem e retornam `http.Handler`
- Rotas são structs com configuração isolada

### Inversão de dependência
- Servidor depende de interfaces (`ErrorHandler`, `Middleware`)
- Handlers retornam erro, não status HTTP
- Error handling é injetado, não hardcoded

### Baixo acoplamento
- Servidor não conhece lógica de negócio
- Handlers são funções puras que recebem dependências
- Middlewares não dependem uns dos outros

### Código idiomático Go
- Functional Options Pattern
- Context propagation
- Error handling explícito
- Graceful shutdown com context

## Exemplo conceitual de uso

```go
// 1. Criar servidor com configuração
server := httpserver.New(
    httpserver.WithPort("8080"),
    httpserver.WithReadTimeout(15 * time.Second),
    httpserver.WithGlobalMiddlewares(
        httpserver.Recovery,
        httpserver.RequestID,
        httpserver.JSONContentType,
        httpserver.SecurityHeaders,
    ),
    httpserver.WithErrorHandler(customErrorHandler),
)

// 2. Registrar rotas dinamicamente
userHandler := NewUserHandler(userUseCase, obs)
server.RegisterRoute(httpserver.NewRoute(
    "POST", "/users",
    userHandler.Create,
    httpserver.CORS("*", "POST,GET", "Content-Type"),
    httpserver.Timeout(5 * time.Second),
))

server.RegisterRoute(httpserver.NewRoute(
    "GET", "/users/{id}",
    userHandler.GetByID,
))

// 3. Iniciar servidor
shutdown := server.Run()

// 4. Aguardar sinal de término
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
<-sigCh

// 5. Graceful shutdown
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
if err := shutdown(ctx); err != nil {
    log.Printf("Shutdown error: %v", err)
}
```

## Garantias do package

### Thread-safety
- ✅ Registro de rotas é protegido por mutex
- ✅ `chi.Mux` é thread-safe por design
- ✅ Servidor pode ser usado concorrentemente

### Comportamento previsível em falhas
- ✅ Erros de handlers são capturados pelo ErrorHandler
- ✅ Panic em handlers é recuperado pelo Recovery middleware
- ✅ Shutdown respeita timeout ou retorna erro

### Estabilidade da API pública
- ✅ Interfaces estáveis (`Server`, `Handler`, `Middleware`)
- ✅ Functional Options garantem extensibilidade
- ✅ Compatível com `http.Handler` padrão

---

# pkg/observability

## Responsabilidade

O package `pkg/observability` é responsável por fornecer **observabilidade completa para aplicações Go**, encapsulando:

- **Logging estruturado**: Logs com contexto e níveis
- **Métricas**: Contadores, histogramas, gauges
- **Tracing distribuído**: Spans e propagação de contexto
- **Abstração total**: Zero dependência de OpenTelemetry fora da infraestrutura

### O que é observado
- **Logs**: Eventos da aplicação (info, warn, error, debug)
- **Métricas**: Números quantitativos (requests/s, latência, memória)
- **Traces**: Jornada de uma requisição através de sistemas distribuídos

### Por que esse package existe
- **Desacoplar**: Domínio não deve depender de biblioteca de telemetria
- **Testabilidade**: Fake provider permite assertions em testes
- **Performance**: NoOp provider tem zero overhead
- **Flexibilidade**: Trocar backend sem alterar código de negócio

### Qual problema resolve em produção
- **Debugging distribuído**: Rastrear requisições entre microsserviços
- **Análise de performance**: Identificar gargalos via traces e métricas
- **Alertas proativos**: Métricas alimentam sistemas de alerta
- **Correlação de eventos**: Logs automaticamente incluem trace_id

## Conceitos-Chave

### Logging estruturado
- **Campos tipados**: String, Int, Float64, Error
- **Níveis**: Debug, Info, Warn, Error
- **Context-aware**: Trace ID automaticamente injetado
- **Formatos**: JSON (produção) ou TEXT (desenvolvimento)

### Métricas
- **Counter**: Valores monotonicamente crescentes (ex: total de requests)
- **Histogram**: Distribuição de valores (ex: latência)
- **UpDownCounter**: Pode crescer e decrecer (ex: conexões ativas)
- **Gauge**: Valor atual (ex: uso de memória)

### Tracing
- **Span**: Unidade de trabalho (função, query, HTTP call)
- **Context propagation**: Spans filhos herdam trace_id do pai
- **Atributos**: Metadados adicionados ao span
- **Eventos**: Marcos importantes durante execução

### Context propagation
- **Injeção automática**: Trace ID adicionado aos logs
- **Propagação entre serviços**: Via HTTP headers
- **Hierarquia de spans**: Parent-child relationships

## Como outras aplicações e IAs devem usar

### O que instrumentar
1. **Boundaries (obrigatório)**:
   - HTTP handlers (início e fim de requests)
   - Repository queries (latência de database)
   - Chamadas externas (APIs, message queues)

2. **Use Cases (recomendado)**:
   - Lógica de negócio complexa
   - Operações assíncronas
   - Processamento em batch

3. **Funções puras (opcional)**:
   - Apenas se forem computacionalmente caras
   - Evite over-instrumentation

### O que evitar
- ❌ Instrumentar getters/setters triviais
- ❌ Logs em loops (use métricas)
- ❌ Traces em funções síncronas simples
- ❌ Métricas sem labels (perda de granularidade)

### Boas práticas de observabilidade
1. **Sempre passe contexto**: `logger.Info(ctx, "message")`
2. **Defer span.End()**: Garante finalização mesmo com panic
3. **Use span kinds apropriados**: Server, Client, Internal
4. **Log com erro incluído**: `logger.Error(ctx, "msg", obs.Error(err))`
5. **Métricas com labels**: Segmente por status, endpoint, etc.

## Benefícios por porte de projeto

### Pequeno (1-3 serviços)
- **NoOp provider**: Zero overhead durante desenvolvimento
- **Fake provider**: Testes unitários simples
- **Logs estruturados**: Facilita debugging local

### Médio (4-10 serviços)
- **OpenTelemetry provider**: Traces entre serviços
- **Métricas básicas**: Dashboards de saúde
- **Correlação de logs**: Trace ID conecta eventos

### Grande (10+ serviços)
- **Tracing distribuído completo**: Identifica gargalos cross-service
- **Métricas detalhadas**: SLIs, SLOs, SLAs
- **Alertas automatizados**: Baseados em métricas
- **Análise de tendências**: Histórico de performance

## Exemplo conceitual de uso

```go
// 1. Inicializar observability (main.go)
obs, err := otel.NewProvider(ctx, &otel.Config{
    ServiceName:    "order-service",
    ServiceVersion: "1.0.0",
    Environment:    "production",
    OTLPEndpoint:   "otel-collector:4317",
    LogLevel:       observability.LogLevelInfo,
    LogFormat:      observability.LogFormatJSON,
})
defer obs.Shutdown(ctx)

// 2. Injetar em use cases
orderUseCase := NewCreateOrderUseCase(obs, orderRepo)

// 3. Usar em use cases
func (uc *CreateOrderUseCase) Execute(ctx context.Context, dto DTO) error {
    // Tracing
    ctx, span := uc.obs.Tracer().Start(ctx, "CreateOrder")
    defer span.End()

    // Logging
    uc.obs.Logger().Info(ctx, "creating order",
        obs.String("customer_id", dto.CustomerID),
    )

    // Métricas
    counter := uc.obs.Metrics().Counter("orders.created", "Total orders", "1")

    // Lógica de negócio
    order, err := uc.orderRepo.Create(ctx, dto)
    if err != nil {
        span.RecordError(err)
        uc.obs.Logger().Error(ctx, "failed to create order", obs.Error(err))
        return err
    }

    counter.Add(ctx, 1, obs.String("status", "success"))
    return nil
}

// 4. Usar em repositories
func (r *OrderRepository) Create(ctx context.Context, dto DTO) (*Order, error) {
    ctx, span := r.obs.Tracer().Start(ctx, "OrderRepository.Create")
    defer span.End()

    histogram := r.obs.Metrics().Histogram("db.query.duration", "DB latency", "ms")
    start := time.Now()

    // Query database
    result, err := r.db.ExecContext(ctx, query, args...)

    histogram.Record(ctx, float64(time.Since(start).Milliseconds()),
        obs.String("operation", "insert"),
    )

    if err != nil {
        span.RecordError(err)
        return nil, err
    }

    return order, nil
}
```

## Garantias do package

### Thread-safety
- ✅ Todos os providers são thread-safe
- ✅ Logger, Tracer, Metrics podem ser usados concorrentemente

### Comportamento previsível em falhas
- ✅ NoOp provider nunca falha
- ✅ Fake provider captura tudo para assertions
- ✅ OTEL provider: falhas de export não quebram aplicação

### Estabilidade da API pública
- ✅ Interface `Observability` é estável
- ✅ Trocar provider não requer mudança de código
- ✅ Compatível com OpenTelemetry 1.x

---

# pkg/vos (Value Objects)

## Responsabilidade

O package `pkg/vos` é responsável por fornecer **Value Objects do domínio**, garantindo:

- Representação precisa de conceitos de negócio
- Validação em tempo de construção
- Imutabilidade total
- Integração com infraestrutura (DB, JSON)

### O que são Value Objects
Value Objects são **objetos imutáveis sem identidade**, definidos apenas por seus valores. Dois VOs são iguais se todos os seus atributos forem iguais.

**Exemplo**: Dois objetos `Money(1000, BRL)` são considerados iguais, independente de onde foram criados.

### Por que existem neste projeto
1. **Expressividade**: `Money` é mais claro que `int64`
2. **Segurança de tipo**: Impossível somar `Money` com `Percentage` por engano
3. **Validação centralizada**: Regras de negócio encapsuladas
4. **Precisão**: Evita problemas de arredondamento com `float64`
5. **Imutabilidade**: Thread-safe por design

### O papel deles no domínio
- **Semântica clara**: Código se lê como linguagem de negócio
- **Prevenção de bugs**: Impossível criar VOs inválidos
- **Consistência**: Mesmas regras em todo o sistema
- **Auditabilidade**: Histórico de valores precisos

## Características

### Imutabilidade
- Valores não podem ser modificados após criação
- Operações retornam novos VOs
- Thread-safe sem necessidade de locks

### Validação na criação
- Construtores retornam `(VO, error)`
- Impossível criar VO inválido
- Validação acontece uma única vez

### Segurança de domínio
- Type safety evita misturar conceitos
- Operações aritméticas validadas
- Conversões explícitas obrigatórias

### Clareza semântica
- Código autodocumentado
- Alinhamento com linguagem ubíqua (DDD)
- Reduz cognitive load

## Reutilização em outros projetos

### APIs (input/output)
```go
// Request
type CreateProductRequest struct {
    Price    vos.Money      `json:"price"`
    Discount vos.Percentage `json:"discount"`
}

// Response
type ProductResponse struct {
    ID    vos.UUID  `json:"id"`
    Price vos.Money `json:"price"`
}
```

### Banco de dados
```go
// Money implementa sql.Scanner e driver.Valuer
var price vos.Money
db.QueryRow("SELECT price FROM products WHERE id = $1", id).Scan(&price)

db.Exec("INSERT INTO products (price) VALUES ($1)", price)
```

### Regras de negócio
```go
func CalculateTotal(price vos.Money, discount vos.Percentage) (vos.Money, error) {
    discountAmount, err := discount.Apply(price)
    if err != nil {
        return vos.Money{}, err
    }

    return price.Subtract(discountAmount)
}
```

### Mensageria
```go
type OrderCreatedEvent struct {
    OrderID vos.ULID  `json:"order_id"`
    Amount  vos.Money `json:"amount"`
}

// Serialização automática para JSON
json.Marshal(event)
```

## Benefícios para automação e IA

### Análise automática de domínio
- **Tipos explícitos**: IA pode inferir relacionamentos entre entidades
- **Validações centralizadas**: IA identifica regras de negócio facilmente
- **Nomenclatura ubíqua**: Alinhamento entre código e documentação

### Geração de código
- **Construtores padronizados**: IA pode gerar factories automaticamente
- **Serialização previsível**: IA pode gerar DTOs consistentes
- **Testes determinísticos**: IA pode gerar casos de teste baseados em regras

### Validações consistentes
- **Regras encapsuladas**: Uma fonte de verdade para validação
- **Erros tipados**: IA pode mapear erros para mensagens de usuário
- **Previsibilidade**: IA pode simular comportamento sem executar código

### Redução de ambiguidade semântica
- **Semântica clara**: `Money` vs `int64` não deixa dúvidas
- **Operações explícitas**: `money.Add(other)` é óbvio
- **Domínio explícito**: IA pode gerar diagramas UML automaticamente

## Exemplo conceitual de uso

```go
// 1. Criação segura com validação
price, err := vos.NewMoney(10000, vos.CurrencyBRL) // R$ 100,00
if err != nil {
    return err
}

discount, err := vos.NewPercentageFromFloat(10.0) // 10%
if err != nil {
    return err
}

// 2. Operações validadas
discountAmount, err := discount.Apply(price) // R$ 10,00
if err != nil {
    return err
}

total, err := price.Subtract(discountAmount) // R$ 90,00
if err != nil {
    return err
}

// 3. Comparações seguras
if total.LessThan(price) {
    fmt.Println("Desconto aplicado com sucesso")
}

// 4. Integração com banco de dados
db.Exec("INSERT INTO orders (total) VALUES ($1)", total)

// 5. Serialização JSON automática
type Order struct {
    Total vos.Money `json:"total"`
}
json.Marshal(Order{Total: total})
// {"total":{"amount":"90.00","currency":"BRL"}}
```

## Garantias do package

### Imutabilidade
- ✅ Campos privados
- ✅ Operações retornam novos VOs
- ✅ Sem setters

### Validação
- ✅ Impossível criar VO inválido
- ✅ Erros explícitos na criação
- ✅ Regras de negócio encapsuladas

### Integração
- ✅ `json.Marshaler` / `json.Unmarshaler`
- ✅ `sql.Scanner` / `driver.Valuer`
- ✅ Compatível com bibliotecas padrão

---

# pkg/linq

## Responsabilidade

O package `pkg/linq` é responsável por fornecer **operações funcionais sobre coleções**, inspirado em LINQ (C#) e streams (Java), permitindo:

- Transformações declarativas de slices
- Operações imutáveis sobre coleções
- Pipeline de dados legível
- Redução de código imperativo

### Qual problema resolve
- **Boilerplate**: Elimina loops repetitivos
- **Legibilidade**: Código declarativo é mais claro que imperativo
- **Composição**: Operações podem ser encadeadas
- **Type-safety**: Generics garantem segurança de tipos

### Motivação do uso
- **Expressividade**: `Filter`, `Map`, `GroupBy` são mais claros que loops
- **Manutenibilidade**: Menos código, menos bugs
- **Testabilidade**: Funções puras são fáceis de testar
- **Reutilização**: Operações comuns centralizadas

## Conceitos-Chave

### Operações funcionais
- **Filter**: Seleciona elementos que satisfazem condição
- **Map**: Transforma elementos de um tipo para outro
- **Find**: Retorna primeiro elemento que satisfaz condição
- **Remove**: Exclui elementos que satisfazem condição
- **GroupBy**: Agrupa elementos por chave
- **Sum**: Soma valores numéricos de elementos

### Imutabilidade
- Operações **não modificam** slice original
- Retornam **novos slices**
- Seguro para uso concorrente (desde que slice original não mude)

### Pipelines de dados
```go
result := linq.Map(
    linq.Filter(numbers, isEven),
    double,
)
```

### Leitura expressiva
```go
// Imperativo
var evens []int
for _, n := range numbers {
    if n % 2 == 0 {
        evens = append(evens, n)
    }
}

// Funcional
evens := linq.Filter(numbers, func(n int) bool {
    return n % 2 == 0
})
```

## Quando usar

### Casos ideais
1. **Filtrar coleções**: Selecionar subset de elementos
2. **Transformar dados**: Mapear de um tipo para outro
3. **Agrupar dados**: Organizar por categoria
4. **Agregações**: Somar, contar, achar máximo/mínimo
5. **Pipelines de transformação**: Múltiplas operações encadeadas

### Casos a evitar
1. **Alta performance crítica**: Loops nativos são mais rápidos
2. **Slices gigantes**: Allocations podem impactar memória
3. **Operações complexas**: Se callback fica muito complexo, use loop
4. **Efeitos colaterais**: LINQ assume funções puras

## Benefícios

### Legibilidade
- Código declarativo ("o que") vs imperativo ("como")
- Nomes de função autoexplicativos
- Reduz cognitive load

### Manutenção
- Menos linhas de código
- Operações centralizadas
- Bugs mais difíceis de introduzir

### Redução de código imperativo
```go
// Antes: 10 linhas
var result []Product
for _, p := range products {
    if p.Price > 100 {
        result = append(result, p)
    }
}

// Depois: 1 linha
result := linq.Filter(products, func(p Product) bool {
    return p.Price > 100
})
```

### Clareza em transformações de dados
```go
// Complexo de ler
var names []string
for _, user := range users {
    if user.Active {
        names = append(names, user.Name)
    }
}

// Claro e direto
activeUsers := linq.Filter(users, func(u User) bool { return u.Active })
names := linq.Map(activeUsers, func(u User) string { return u.Name })
```

## Exemplo conceitual de uso

```go
// 1. Filter: Selecionar produtos caros
expensiveProducts := linq.Filter(products, func(p Product) bool {
    return p.Price.GreaterThan(vos.NewMoney(10000, vos.CurrencyBRL))
})

// 2. Map: Extrair IDs
productIDs := linq.Map(products, func(p Product) vos.UUID {
    return p.ID
})

// 3. GroupBy: Agrupar por categoria
byCategory := linq.GroupBy(products, func(p Product) string {
    return p.Category
})

// 4. Sum: Calcular total
total := linq.Sum(products, func(p Product) float64 {
    return p.Price.Float()
})

// 5. Pipeline: Transformações encadeadas
result := linq.Map(
    linq.Filter(products, isExpensive),
    extractName,
)

// 6. Find: Primeiro elemento
firstActive := linq.Find(users, func(u User) bool {
    return u.Active
})

// 7. Remove: Excluir elementos
withoutDeleted := linq.Remove(products, func(p Product) bool {
    return p.DeletedAt.IsValid()
})
```

## Garantias do package

### Imutabilidade
- ✅ Slice original nunca é modificado
- ✅ Retorna novos slices

### Thread-safety
- ✅ Seguro para uso concorrente (se slice não mudar)
- ✅ Sem estado global mutável

### Type-safety
- ✅ Generics garantem tipos corretos
- ✅ Erros de tipo capturados em compile-time

---

# pkg/messaging

## Responsabilidade

O package `pkg/messaging` é responsável por **abstrair comunicação assíncrona via message brokers**, fornecendo:

- Interface unificada para RabbitMQ e Kafka
- Publishers e Consumers resilientes
- Reconexão automática
- Observabilidade integrada
- Graceful shutdown

### Papel do package na comunicação assíncrona
- **Desacoplamento**: Serviços se comunicam sem conhecer uns aos outros
- **Resiliência**: Mensagens não são perdidas se consumer estiver offline
- **Escalabilidade**: Múltiplos consumers processam em paralelo
- **Auditabilidade**: Mensagens podem ser reprocessadas

### O que ele abstrai
- **Brokers**: RabbitMQ vs Kafka são intercambiáveis
- **Protocolos**: AMQP, TLS, SASL abstraídos
- **Drivers**: `amqp091-go`, `kafka-go` encapsulados
- **Reconexão**: Backoff exponencial automático

### Limites claros de responsabilidade
**Responsável por**:
- Conectar a brokers
- Publicar e consumir mensagens
- Reconexão automática
- Health checks

**Não responsável por**:
- Serialização de payload (use JSON, Protobuf, etc.)
- Lógica de negócio (vive em handlers)
- Roteamento complexo (configure no broker)
- Reprocessamento de mensagens (configure DLQ)

## Conceitos-Chave

### Producers e Consumers
- **Producer**: Publica mensagens em exchange/topic
- **Consumer**: Consome mensagens de queue/topic
- **Publisher Confirms**: Garante que mensagem foi aceita pelo broker
- **ACK/NACK**: Consumer confirma ou rejeita processamento

### Mensagens e eventos
- **Estrutura mínima**: Headers + Body + Routing Key
- **Headers**: Metadados (timestamp, message_id, trace_id)
- **Body**: Payload serializado (JSON recomendado)
- **Routing Key**: Define destino da mensagem

### Serialização / deserialização
- **Responsabilidade do usuário**: Package não impõe formato
- **Recomendação**: JSON para interoperabilidade
- **Alternativas**: Protobuf, Avro, MessagePack

### Garantias de entrega
- **RabbitMQ**: Publisher confirms + persistent messages
- **Kafka**: Acknowledgements configuráveis (0, 1, all)
- **DLQ**: Mensagens com erro vão para Dead Letter Queue

### Resiliência e reconexão
- **Backoff exponencial**: Intervalo cresce a cada tentativa
- **Reconexão automática**: Transparente para aplicação
- **Health checks**: Monitora saúde da conexão
- **Circuit breaker**: Evita sobrecarga durante indisponibilidade

## Como reutilizar em outras aplicações

### Quando utilizar mensageria
1. **Comunicação assíncrona**: Request não precisa esperar resposta
2. **Desacoplamento**: Serviços não devem conhecer uns aos outros
3. **Processamento em background**: Jobs, emails, notificações
4. **Event sourcing**: Registro de eventos de domínio
5. **Load balancing**: Distribuir trabalho entre workers

### Boas práticas de integração
1. **Injete Observability**: Sempre passe `obs` ao criar client
2. **Use DLQ**: Configure Dead Letter Queue para mensagens com erro
3. **Idempotência**: Handlers devem ser idempotentes
4. **Timeout**: Defina timeout para processamento
5. **Graceful shutdown**: Aguarde mensagens em processamento

### Cuidados com idempotência e duplicidade
- **Mensagens podem chegar duplicadas**: Use deduplicação
- **Idempotência é essencial**: Processar 2x não deve causar efeito colateral
- **Message ID**: Use para detectar duplicatas
- **Database constraints**: UNIQUE evita duplicação

### Uso conjunto com pkg/vos
```go
type OrderCreatedEvent struct {
    OrderID   vos.ULID  `json:"order_id"`
    Amount    vos.Money `json:"amount"`
    CreatedAt vos.NullableTime `json:"created_at"`
}

// Serializar
payload, _ := json.Marshal(event)
publisher.Publish(ctx, "orders", "order.created", payload)

// Deserializar
var event OrderCreatedEvent
json.Unmarshal(msg.Body, &event)
// Validação automática dos VOs
```

## Integração com outros packages

### Com pkg/observability
```go
// Client RabbitMQ recebe observability
client, err := rabbitmq.New(
    obs,  // Injeta observability
    rabbitmq.WithCloudConnection(url),
)

// Traces e logs automáticos
publisher.Publish(ctx, exchange, routingKey, body)
// → Cria span "rabbitmq.publish"
// → Log "publishing message to exchange=orders"
```

### Com pkg/database
```go
// Handler de mensagem persiste no banco
func (h *OrderHandler) HandleOrderCreated(ctx context.Context, msg messaging.Message) error {
    var event OrderCreatedEvent
    json.Unmarshal(msg.Body, &event)

    // Usar Unit of Work para transação
    return h.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) error {
        return h.orderRepo.Create(ctx, tx, event.ToOrder())
    })
}
```

### Com pkg/vos
```go
// Mensagens usam Value Objects
type PaymentProcessedEvent struct {
    PaymentID vos.UUID       `json:"payment_id"`
    Amount    vos.Money      `json:"amount"`
    Fee       vos.Percentage `json:"fee"`
}

// Validação automática ao deserializar
var event PaymentProcessedEvent
if err := json.Unmarshal(msg.Body, &event); err != nil {
    // VO inválido retorna erro
    return err
}
```

## Garantias do package

### Confiabilidade
- ✅ Publisher confirms (RabbitMQ)
- ✅ ACK/NACK explícito
- ✅ DLQ para mensagens com erro
- ✅ Reconexão automática

### Segurança
- ✅ TLS habilitado por padrão (CloudStrategy)
- ✅ Autenticação configurável
- ✅ Validação de configuração

### Comportamento previsível em falhas
- ✅ Mensagens não confirmadas retornam ao broker
- ✅ Reconexão transparente
- ✅ Health check detecta desconexões
- ✅ Graceful shutdown aguarda mensagens em processamento

## Exemplo conceitual de uso

```go
// 1. RabbitMQ Client
client, err := rabbitmq.New(
    obs,
    rabbitmq.WithCloudConnection(os.Getenv("RABBITMQ_URL")),
    rabbitmq.WithServiceName("order-service"),
    rabbitmq.WithPublisherConfirms(true),
    rabbitmq.WithAutoReconnect(true),
)
defer client.Shutdown(ctx)

// 2. Declarar topologia
client.DeclareExchange(ctx, "orders", "topic", true, false, nil)
client.DeclareQueue(ctx, "order-processing", true, false, false, nil)
client.BindQueue(ctx, "order-processing", "order.*", "orders", nil)

// 3. Publisher
publisher := rabbitmq.NewPublisher(client)
event := OrderCreatedEvent{
    OrderID: orderID,
    Amount:  amount,
}
payload, _ := json.Marshal(event)

err = publisher.Publish(
    ctx,
    "orders",        // exchange
    "order.created", // routing key
    payload,
    rabbitmq.WithMessageID(orderID.String()),
)

// 4. Consumer
consumer := rabbitmq.NewConsumer(
    client,
    rabbitmq.WithQueue("order-processing"),
    rabbitmq.WithPrefetchCount(10),
    rabbitmq.WithWorkerPool(5),
)

consumer.RegisterHandler("order.created", func(ctx context.Context, msg rabbitmq.Message) error {
    var event OrderCreatedEvent
    if err := json.Unmarshal(msg.Body, &event); err != nil {
        return err // NACK, mensagem vai para DLQ
    }

    // Processar evento (idempotente!)
    return h.processOrder(ctx, event)
    // Retornar nil → ACK automático
    // Retornar erro → NACK, requeue ou DLQ
})

go consumer.Consume(ctx)
```

---

# Convenções Gerais do Projeto

## Padrões de nomenclatura

### Packages
- **Lowercase, singular**: `database`, `httpserver`, `observability`
- **Descritivo**: Nome deve indicar responsabilidade
- **Evite**: `utils`, `common`, `helpers` (muito genérico)

### Funções e Métodos
- **CamelCase**: `NewUser()`, `FindByID()`
- **Construtores**: `New`, `NewFrom`, `NewWith`
- **Predicados**: `Is`, `Has`, `Can` (retornam bool)
- **Conversões**: `To`, `From`, `String()`

### Interfaces
- **Substantivo ou adjetivo**: `Repository`, `Publisher`, `Closeable`
- **Sem prefixo `I`**: Use `Database`, não `IDatabase`
- **Pequenas e focadas**: 1-3 métodos idealmente

### Variáveis
- **camelCase**: `userID`, `orderRepo`
- **Descritivas**: Evite `x`, `tmp`, `data`
- **Curtas em escopo curto**: `i` em loop está OK

### Constantes
- **CamelCase ou SCREAMING_SNAKE_CASE**:
  - Exportadas: `MaxRetries`, `DefaultTimeout`
  - Privadas: `maxConnections`, `defaultPort`

## Organização de pastas

```
pkg/
├── database/
│   ├── db.go              # Interface DBTX
│   ├── postgres/
│   │   ├── postgres.go    # Implementação PostgreSQL
│   │   └── options.go     # Functional Options
│   └── uow/
│       └── uow.go         # Unit of Work
├── httpserver/
│   ├── server.go          # Interface Server
│   ├── server_options.go  # Functional Options
│   └── middlewares.go     # Middlewares comuns
├── observability/
│   ├── observability.go   # Interface Observability
│   ├── otel/              # Implementação OpenTelemetry
│   ├── noop/              # Implementação NoOp
│   └── fake/              # Implementação Fake (testes)
├── vos/
│   ├── money.go           # Value Object Money
│   ├── uuid.go            # Value Object UUID
│   └── currency.go        # Value Object Currency
├── linq/
│   └── slices.go          # Operações funcionais
└── messaging/
    ├── rabbitmq/
    │   ├── client.go      # Client RabbitMQ
    │   ├── publisher.go   # Publisher
    │   └── consumer.go    # Consumer
    └── kafka/
        ├── client.go      # Client Kafka
        ├── producer.go    # Producer
        └── consumer.go    # Consumer
```

### Convenções de arquivos
- **Um tipo por arquivo**: `user.go` contém `type User struct`
- **Testes ao lado**: `user.go` + `user_test.go`
- **Exemplos**: `example_test.go` para exemplos executáveis
- **Interfaces**: Podem viver em arquivo separado ou junto com implementação

## Convenções de visibilidade

### Exportado (público)
- **Inicial maiúscula**: `Database`, `NewUser()`, `ID`
- **Quando usar**: API pública do package
- **Documentação obrigatória**: Godoc para tudo exportado

### Não exportado (privado)
- **Inicial minúscula**: `connection`, `validateConfig()`, `maxRetries`
- **Quando usar**: Detalhes de implementação
- **Documentação opcional**: Mas recomendada

### Regra de ouro
> "Exporte o mínimo necessário. É fácil tornar algo público, difícil tornar privado."

## Expectativas de estabilidade da API

### Packages estáveis (v1.0+)
- **pkg/vos**: API estável, breaking changes raros
- **pkg/database**: API estável, novas opções adicionadas
- **pkg/observability**: Interface `Observability` estável

### Packages em evolução
- **pkg/messaging**: Pode adicionar novos brokers
- **pkg/httpserver**: Pode adicionar novos middlewares

### Garantias de compatibilidade
- **Versionamento semântico**: MAJOR.MINOR.PATCH
- **Functional Options**: Permitem adicionar features sem breaking changes
- **Interfaces segregadas**: Adicionar métodos cria nova interface

## Regras de versionamento

### Semantic Versioning (SemVer)
- **MAJOR**: Breaking changes (1.0.0 → 2.0.0)
- **MINOR**: Novas features compatíveis (1.0.0 → 1.1.0)
- **PATCH**: Bug fixes compatíveis (1.0.0 → 1.0.1)

### Quando incrementar
- **MAJOR**:
  - Mudar assinatura de função pública
  - Remover campo de struct exportada
  - Mudar comportamento existente

- **MINOR**:
  - Adicionar nova função pública
  - Adicionar novo functional option
  - Adicionar campo a struct (com zero value compatível)

- **PATCH**:
  - Corrigir bug sem alterar API
  - Melhorar performance
  - Atualizar documentação

### Branches
- **main**: Código estável, pronto para produção
- **develop**: Integração de features
- **feature/**: Novas funcionalidades
- **fix/**: Correções de bugs

---

# Diretrizes para Outras IAs

## Como interpretar corretamente cada package

### pkg/database
- **Propósito**: Acesso a dados relacionais
- **Não confundir com**: ORM completo (não gera queries)
- **Escopo**: Connection pooling + Unit of Work
- **Limitações**: Apenas PostgreSQL atualmente

### pkg/httpserver
- **Propósito**: Servidor HTTP com rotas e middlewares
- **Não confundir com**: Framework web completo
- **Escopo**: HTTP server + error handling + middlewares
- **Limitações**: Baseado em go-chi, não suporta GraphQL nativamente

### pkg/observability
- **Propósito**: Telemetria (logs, métricas, traces)
- **Não confundir com**: APM completo (sem análise automática)
- **Escopo**: Abstração sobre OpenTelemetry
- **Limitações**: Não inclui backend (precisa de collector externo)

### pkg/vos
- **Propósito**: Value Objects de domínio
- **Não confundir com**: DTOs ou entities
- **Escopo**: Valores imutáveis com validação
- **Limitações**: Conjunto fixo de VOs (extensível por cópia)

### pkg/linq
- **Propósito**: Operações funcionais sobre slices
- **Não confundir com**: LINQ completo do C# (mais limitado)
- **Escopo**: Filter, Map, GroupBy, Sum, Find, Remove
- **Limitações**: Não suporta lazy evaluation

### pkg/messaging
- **Propósito**: Message brokers (RabbitMQ, Kafka)
- **Não confundir com**: Event bus in-memory
- **Escopo**: Pub/Sub distribuído
- **Limitações**: Não inclui serialização (use JSON/Protobuf)

## O que pode ser reutilizado automaticamente

### Copiar e usar diretamente
- ✅ `pkg/vos`: Independente de qualquer infraestrutura
- ✅ `pkg/linq`: Zero dependências externas
- ✅ `pkg/entity`: Base para entidades de domínio

### Requer configuração mínima
- ⚙️ `pkg/database`: Ajustar connection string e pool
- ⚙️ `pkg/httpserver`: Configurar porta e middlewares
- ⚙️ `pkg/messaging`: Configurar broker URL

### Requer dependências externas
- 🔗 `pkg/observability`: Precisa de OpenTelemetry collector
- 🔗 `pkg/messaging`: Precisa de RabbitMQ ou Kafka rodando

## O que exige validação ou contexto humano

### Decisões arquiteturais
- **Qual banco usar**: PostgreSQL vs MySQL vs MongoDB
- **Qual broker usar**: RabbitMQ vs Kafka vs NATS
- **Estratégia de observabilidade**: Self-hosted vs Cloud (Coralogix, Datadog)

### Modelagem de domínio
- **Quais VOs criar**: Dependem das regras de negócio
- **Estrutura de entidades**: Dependem do domínio
- **Eventos de mensageria**: Dependem do fluxo de negócio

### Requisitos não-funcionais
- **Pool de conexões**: Depende de carga esperada
- **Timeouts**: Dependem de SLAs
- **Retry policies**: Dependem de tolerância a falhas

## O que não deve ser modificado sem entendimento profundo

### Núcleo dos packages
- ❌ **Interfaces públicas**: Quebra compatibilidade
- ❌ **Lógica de reconexão**: Testada extensivamente
- ❌ **Thread-safety**: Mutex e atomic posicionados cuidadosamente
- ❌ **Validação de VOs**: Regras de negócio centralizadas

### Padrões estabelecidos
- ❌ **Functional Options**: Padrão do projeto
- ❌ **Error handling**: Sempre retornar erro, nunca panic
- ❌ **Context propagation**: Sempre passar contexto

### Segurança
- ❌ **TLS configs**: Testadas para compliance
- ❌ **Timeout defaults**: Balanceados para produção
- ❌ **Validação de input**: Previne injection attacks

## Como usar essa documentação como base para geração de código

### 1. Entender o domínio
```
Input: "Preciso criar um sistema de pagamentos"
IA deve:
1. Identificar VOs necessários (Money, Currency, PaymentMethod)
2. Mapear entidades (Payment, Transaction)
3. Definir eventos (PaymentProcessed, PaymentFailed)
```

### 2. Gerar estrutura de projeto
```
IA pode gerar:
- pkg/payment/domain/payment.go (entidade)
- pkg/payment/domain/vos/payment_method.go (novo VO)
- pkg/payment/application/process_payment.go (use case)
- pkg/payment/infrastructure/repository.go (persistência)
```

### 3. Aplicar padrões consistentes
```
IA deve:
- Usar Functional Options para construtores
- Injetar observability em todos os componentes
- Retornar (value, error), nunca panic
- Implementar graceful shutdown
```

### 4. Gerar testes
```
IA pode gerar:
- Testes unitários com fake providers
- Testes de integração com testcontainers
- Benchmarks para operações críticas
```

### 5. Validar contra esta documentação
```
IA deve verificar:
- Todos os packages seguem convenções de nomenclatura
- Interfaces são pequenas e focadas
- Dependências apontam para abstrações
- VOs têm validação na construção
```

### Templates de código

#### Template de Use Case
```go
type UseCase struct {
    obs  observability.Observability
    repo Repository
}

func NewUseCase(obs observability.Observability, repo Repository) *UseCase {
    return &UseCase{obs: obs, repo: repo}
}

func (uc *UseCase) Execute(ctx context.Context, input Input) (Output, error) {
    ctx, span := uc.obs.Tracer().Start(ctx, "UseCase.Execute")
    defer span.End()

    uc.obs.Logger().Info(ctx, "executing use case", obs.String("input", input.String()))

    // Lógica de negócio
    result, err := uc.repo.FindByID(ctx, input.ID)
    if err != nil {
        span.RecordError(err)
        uc.obs.Logger().Error(ctx, "failed to execute", obs.Error(err))
        return Output{}, err
    }

    return result, nil
}
```

#### Template de Repository
```go
type Repository struct {
    obs observability.Observability
    db  *sql.DB
}

func NewRepository(obs observability.Observability, db *sql.DB) *Repository {
    return &Repository{obs: obs, db: db}
}

func (r *Repository) FindByID(ctx context.Context, id vos.UUID) (*Entity, error) {
    ctx, span := r.obs.Tracer().Start(ctx, "Repository.FindByID")
    defer span.End()

    histogram := r.obs.Metrics().Histogram("db.query.duration", "DB latency", "ms")
    start := time.Now()

    var entity Entity
    err := r.db.QueryRowContext(ctx, query, id.String()).Scan(&entity)

    histogram.Record(ctx, float64(time.Since(start).Milliseconds()))

    if err != nil {
        span.RecordError(err)
        return nil, err
    }

    return &entity, nil
}
```

#### Template de Handler HTTP
```go
type Handler struct {
    obs     observability.Observability
    useCase *UseCase
}

func NewHandler(obs observability.Observability, uc *UseCase) *Handler {
    return &Handler{obs: obs, useCase: uc}
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) error {
    ctx, span := h.obs.Tracer().Start(r.Context(), "Handler.Handle")
    defer span.End()

    var input Input
    if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
        return err
    }

    output, err := h.useCase.Execute(ctx, input)
    if err != nil {
        return err
    }

    w.Header().Set("Content-Type", "application/json")
    return json.NewEncoder(w).Encode(output)
}
```

---

## Conclusão

Este documento serve como **fonte única de verdade** para a arquitetura e uso dos packages do projeto **devkit-go**.

### Para desenvolvedores humanos
- Use como referência ao integrar packages
- Siga os padrões estabelecidos
- Contribua com melhorias via Pull Request

### Para IAs
- Use como base para geração de código consistente
- Valide código gerado contra essas diretrizes
- Sugira melhorias alinhadas com os princípios arquiteturais

### Manutenção deste documento
- Atualizar quando novos packages forem adicionados
- Revisar quando breaking changes ocorrerem
- Sincronizar com READMEs individuais dos packages

---

**Data de criação**: 2025-12-30
**Última atualização**: 2025-12-30
**Versão**: 1.0.0
**Mantenedor**: Jailton Junior
**Licença**: MIT
