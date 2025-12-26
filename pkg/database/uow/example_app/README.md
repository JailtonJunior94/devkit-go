# Unit of Work + Uber FX - Exemplo Completo

Este diretório contém exemplos práticos de como integrar o Unit of Work com Uber FX para injeção de dependências.

## 📁 Estrutura

```
example_app/
├── main.go           # Aplicação completa com UoW + FX
├── README.md         # Este arquivo
└── main_test.go      # Testes usando FX (a criar)
```

## 🚀 Como Executar

### 1. Pré-requisitos

Certifique-se de ter CockroachDB rodando:

```bash
# Via Docker
docker run -d \
  --name cockroach \
  -p 26257:26257 \
  -p 8080:8080 \
  cockroachdb/cockroach:latest \
  start-single-node --insecure

# Ou via Make (na raiz do projeto)
make start_minimal
```

### 2. Instalar Dependências

```bash
cd pkg/database/uow/example_app
go mod tidy
```

### 3. Executar Aplicação

```bash
go run main.go
```

Você verá:
```
🔌 Conectando ao banco de dados...
✅ Banco de dados conectado
✅ Tabelas criadas/verificadas
🚀 Servidor HTTP iniciando na porta :8080
✅ Servidor HTTP pronto
```

### 4. Testar Endpoints

```bash
# Health check
curl http://localhost:8080/health

# Transferência de ACC001 para ACC002
curl "http://localhost:8080/transfer?from=ACC001&to=ACC002"
```

## 🏗️ Arquitetura

A aplicação demonstra uma arquitetura em camadas com Clean Architecture:

```
┌─────────────────────────────────────────────────────────┐
│                   PRESENTATION LAYER                    │
│                    (HTTP Handlers)                      │
│                    TransferHandler                      │
└─────────────────────┬───────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────┐
│                   APPLICATION LAYER                     │
│                     (Use Cases)                         │
│                 TransferMoneyUseCase                    │
└─────────────────────┬───────────────────────────────────┘
                      │
                      ▼ usa UnitOfWork para atomicidade
┌─────────────────────────────────────────────────────────┐
│                   DOMAIN LAYER                          │
│            (Entities + Repository Interfaces)           │
│          Account, Transfer, AccountRepository           │
└─────────────────────┬───────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────┐
│                INFRASTRUCTURE LAYER                     │
│               (Repository Implementations)              │
│        accountRepository, transferRepository            │
└─────────────────────────────────────────────────────────┘
```

## 🔄 Fluxo de uma Transferência

1. **HTTP Request** → `TransferHandler.HandleTransfer()`
2. **Handler** → `TransferMoneyUseCase.Execute()`
3. **Use Case** → `uow.Do()` - **Inicia transação**
4. **Inside Transaction**:
   ```
   ├─ FindByID(fromAccount)    # SELECT com lock
   ├─ FindByID(toAccount)      # SELECT com lock
   ├─ Validação de saldo
   ├─ UpdateBalance(fromAccount) # UPDATE
   ├─ UpdateBalance(toAccount)   # UPDATE
   └─ Create(transfer)           # INSERT
   ```
5. **UoW** → `tx.Commit()` - **Commit automático**
6. **Response** → HTTP 200 OK

### ⚠️ Se Algo Falhar

- ❌ Saldo insuficiente → **Rollback automático**
- ❌ Conta não encontrada → **Rollback automático**
- ❌ Erro no UPDATE → **Rollback automático**
- ❌ Timeout (30s) → **Rollback automático**
- ❌ Panic → **Rollback automático**

**GARANTIA**: Ou **TODAS** as operações são executadas, ou **NENHUMA** é.

## 🎯 Principais Conceitos Demonstrados

### 1. Injeção de Dependências com FX

```go
app := fx.New(
    // Provedores de dependências
    fx.Provide(
        NewDatabase,        // *sql.DB
        NewUnitOfWork,      // UnitOfWork
        NewAccountRepo,     // AccountRepository
        NewTransferRepo,    // TransferRepository
        NewTransferUseCase, // TransferMoneyUseCase
        NewTransferHandler, // TransferHandler
    ),

    // Consumidores
    fx.Invoke(func(*HTTPServer) {
        // Server é instanciado e iniciado
    }),
)
```

### 2. Atomicidade com Unit of Work

```go
func (uc *TransferMoneyUseCase) Execute(ctx context.Context, input Input) error {
    return uc.uow.Do(ctx, func(ctx context.Context, tx database.DBExecutor) error {
        // Todas as operações aqui são atômicas
        // Se qualquer uma falhar, TODAS são revertidas

        // 1. Debitar origem
        if err := uc.accountRepo.UpdateBalance(ctx, tx, from, newBalance); err != nil {
            return err // ← ROLLBACK automático
        }

        // 2. Creditar destino
        if err := uc.accountRepo.UpdateBalance(ctx, tx, to, newBalance); err != nil {
            return err // ← ROLLBACK automático
        }

        // 3. Registrar transferência
        if err := uc.transferRepo.Create(ctx, tx, transfer); err != nil {
            return err // ← ROLLBACK automático
        }

        return nil // ← COMMIT automático
    })
}
```

### 3. Lifecycle Hooks

```go
func NewDatabase(lc fx.Lifecycle, cfg AppConfig) (*sql.DB, error) {
    db, err := sql.Open("postgres", cfg.DatabaseURL)

    lc.Append(fx.Hook{
        OnStart: func(ctx context.Context) error {
            // Executado quando app.Run() é chamado
            return db.PingContext(ctx)
        },
        OnStop: func(ctx context.Context) error {
            // Executado quando app recebe SIGTERM/SIGINT
            return db.Close()
        },
    })

    return db, nil
}
```

## 📊 Vantagens desta Arquitetura

### ✅ Testabilidade

Cada camada pode ser testada isoladamente:
- Use Cases podem ser testados com repositórios mockados
- Handlers podem ser testados com use cases mockados
- Repositórios podem ser testados com banco de testes

### ✅ Manutenibilidade

- Dependências explícitas (não há `new` espalhado pelo código)
- Fácil de adicionar novas features (apenas adicionar providers)
- Fácil de trocar implementações (ex: trocar Postgres por MySQL)

### ✅ Atomicidade Garantida

- Unit of Work garante que operações complexas sejam atômicas
- Rollback automático em caso de erro
- Proteção contra transações aninhadas

### ✅ Production-Ready

- Lifecycle management (startup/shutdown gracioso)
- Connection pooling configurado
- Timeouts configurados
- Health checks

## 🧪 Como Testar

```bash
# Testes unitários (com mocks)
go test -v ./...

# Testes de integração (com banco real)
go test -v -tags=integration ./...

# Testes com race detector
go test -race -v ./...
```

## 🔧 Customização

### Alterar Isolation Level

```go
func NewUoWConfig(cfg AppConfig) uow.UoWConfig {
    return uow.UoWConfig{
        DefaultIsolation: sql.LevelReadCommitted,  // ← Mude aqui
        DefaultTimeout:   10 * time.Second,        // ← Ou aqui
    }
}
```

### Adicionar Logging

```go
fx.Provide(
    NewLogger,  // Seu logger (zap, logrus, etc)
)

func NewTransferMoneyUseCase(
    uow uow.UnitOfWork,
    accountRepo AccountRepository,
    transferRepo TransferRepository,
    logger Logger,  // ← Injetado automaticamente
) *TransferMoneyUseCase {
    // ...
}
```

### Adicionar Metrics

```go
fx.Provide(
    NewMetrics,  // Prometheus, StatsD, etc
)

func (uc *TransferMoneyUseCase) Execute(...) error {
    start := time.Now()
    defer func() {
        uc.metrics.RecordDuration("transfer.duration", time.Since(start))
    }()

    // ...
}
```

## 📚 Referências

- [Uber FX Documentation](https://uber-go.github.io/fx/)
- [Clean Architecture](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)
- [Unit of Work Pattern](https://martinfowler.com/eaaCatalog/unitOfWork.html)
- [Repository Pattern](https://martinfowler.com/eaaCatalog/repository.html)

## 💡 Próximos Passos

1. Adicionar observabilidade (OpenTelemetry)
2. Implementar retry logic para erros retriáveis
3. Adicionar circuit breaker
4. Implementar idempotency keys
5. Adicionar rate limiting
6. Implementar audit log
