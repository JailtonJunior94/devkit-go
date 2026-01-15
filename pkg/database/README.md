# Database Package

Abstrações e utilitários para gerenciamento de conexões PostgreSQL com suporte completo a observabilidade, transações e connection pooling otimizado.

## Visão Geral

### Propósito

O `pkg/database` oferece componentes prontos para produção que facilitam o trabalho com PostgreSQL em aplicações Go. Abstrai a complexidade de connection pooling, transaction management e observability, permitindo que você foque na lógica de negócio.

### Quando Usar

✅ **Use quando você precisa:**
- Gerenciar conexões PostgreSQL com configuração de pool otimizada para produção
- Implementar transações atômicas com Unit of Work pattern
- Observabilidade automática (tracing e métricas) em queries SQL
- Repositories que funcionam tanto com transações quanto sem
- Health checks para Kubernetes/Docker
- Graceful shutdown sem perda de queries em andamento

### Quando NÃO Usar

❌ **Não use quando:**
- Você precisa de suporte a múltiplos bancos de dados (MySQL, SQLite, etc.) - use uma abstração mais genérica
- Você quer um ORM completo (use GORM, ent, sqlc, etc.)
- Você precisa de connection pooling entre microserviços (use PgBouncer)
- Sua aplicação é stateless e efêmera demais para beneficiar-se de pooling

### Problemas que Resolve

1. **Connection Pooling**: Configuração otimizada para evitar exaustão de conexões
2. **Transaction Management**: Unit of Work com rollback automático em erros/panics
3. **Observability**: Tracing e métricas automáticas sem código adicional
4. **Resource Leaks**: Garante fechamento correto de conexões
5. **Repository Pattern**: Interface `DBTX` permite repositories funcionarem com e sem transações
6. **Production Readiness**: Configurações seguras por padrão

## Princípios de Design

### Clean Code
- Nomes descritivos que revelam intenção
- Funções pequenas com responsabilidade única
- Comentários explicam "por quê", não "o quê"
- Validações explícitas com mensagens de erro claras

### SOLID
- **Single Responsibility**: Cada componente tem uma única razão para mudar
  - `postgres.Database`: Gerenciamento de conexões
  - `uow.UnitOfWork`: Gerenciamento de transações
  - `pgxpool_manager`: Pool com observabilidade nativa
  - `postgres_otelsql`: Pool via database/sql com otelsql
- **Open/Closed**: Extensível via options functions
- **Liskov Substitution**: `DBTX` permite substituir `*sql.DB` por `*sql.Tx`
- **Interface Segregation**: Interfaces mínimas (`DBTX` tem apenas 4 métodos)
- **Dependency Inversion**: Repositories dependem de `DBTX`, não de implementações concretas

### DRY
- Configuration patterns reutilizáveis (functional options)
- Lógica de pool centralizada
- Validações consistentes entre implementações

### Baixo Acoplamento
- `DBTX` abstrai `*sql.DB` e `*sql.Tx`
- Repositories não conhecem se estão em transação ou não
- Managers gerenciam lifecycle sem expor internals

### Escalabilidade

#### Projetos Pequenos (< 100 req/s)
- Use `postgres.Database` (simples e direto)
- Pool padrão (25 conexões) é suficiente
- Setup em 5 linhas de código

#### Projetos Médios (100-1000 req/s)
- Use `postgres_otelsql.DBManager` (observability built-in)
- Ajuste pool baseado em métricas
- Implemente Unit of Work para transações complexas

#### Projetos Grandes (> 1000 req/s)
- Use `pgxpool_manager.PgxPoolManager` (melhor performance)
- Tune pool agressivamente (monitore `wait_count`)
- Considere PgBouncer para connection pooling externo
- Implemente read replicas

## Instalação

```bash
go get github.com/JailtonJunior94/devkit-go
```

```go
import (
    "github.com/JailtonJunior94/devkit-go/pkg/database"
    "github.com/JailtonJunior94/devkit-go/pkg/database/postgres"
    "github.com/JailtonJunior94/devkit-go/pkg/database/uow"
)
```

### Dependências

**Core:**
- `github.com/jackc/pgx/v5` (driver PostgreSQL)

**Opcionais (para observability):**
- `github.com/XSAM/otelsql` (para `postgres_otelsql`)
- `go.opentelemetry.io/otel` (para tracing/metrics)

## API Pública

### Pacote `database`

#### Interface `DBTX`
Abstração para operações de banco que funcionam com `*sql.DB` e `*sql.Tx`.

```go
type DBTX interface {
    PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
    QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
    QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
    ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}
```

**Thread-Safety:**
- `*sql.DB`: Thread-safe, compartilhe globalmente
- `*sql.Tx`: **NÃO** é thread-safe, use em uma única goroutine

### Pacote `postgres`

#### `Database`
Gerenciador de conexões PostgreSQL via `database/sql`.

```go
type Database struct { /* private */ }

func New(uri string, opts ...Option) (*Database, error)
func (d *Database) DB() *sql.DB
func (d *Database) Ping(ctx context.Context) error
func (d *Database) Shutdown(ctx context.Context) error
```

**Options:**
- `WithMaxOpenConns(n int)`: Máximo de conexões abertas (padrão: 25)
- `WithMaxIdleConns(n int)`: Máximo de conexões idle (padrão: 6)
- `WithConnMaxLifetime(d time.Duration)`: Tempo máximo de vida (padrão: 5min)
- `WithConnMaxIdleTime(d time.Duration)`: Tempo máximo idle (padrão: 2min)
- `WithPoolConfig(...)`: Configura tudo de uma vez

### Pacote `uow` (Unit of Work)

#### `UnitOfWork`
Gerencia transações atômicas com rollback automático.

```go
type UnitOfWork interface {
    Do(ctx context.Context, fn func(ctx context.Context, db database.DBTX) error) error
}

func NewUnitOfWork(db *sql.DB, opts ...UnitOfWorkOption) UnitOfWork
```

**Options:**
- `WithIsolationLevel(level sql.IsolationLevel)`: Nível de isolamento
- `WithReadOnly(bool)`: Transação somente leitura

**Comportamento:**
- ✅ Commit automático se função retorna `nil`
- ❌ Rollback automático se função retorna erro
- 🔥 Rollback automático se ocorrer panic (re-lança panic após rollback)

### Pacote `postgres_otelsql`

#### `DBManager`
Gerenciador com observability via `otelsql` (database/sql + OpenTelemetry).

```go
type DBManager struct { /* private */ }

func NewDBManager(ctx context.Context, config *Config) (*DBManager, error)
func (m *DBManager) DB() *sql.DB
func (m *DBManager) Ping(ctx context.Context) error
func (m *DBManager) Shutdown(ctx context.Context) error
func (m *DBManager) Stats() sql.DBStats
```

**Config:**
```go
type Config struct {
    DSN                 string
    ServiceName         string
    MaxOpenConns        int
    MaxIdleConns        int
    ConnMaxLifetime     time.Duration
    ConnMaxIdleTime     time.Duration
    EnableMetrics       bool
    EnableTracing       bool
    EnableQueryLogging  bool // ⚠️ NUNCA em produção
    Logger              LogFunc
}

func DefaultConfig(dsn, serviceName string) *Config
```

### Pacote `pgxpool_manager`

#### `PgxPoolManager`
Gerenciador com driver nativo `pgx` e observability built-in.

```go
type PgxPoolManager struct { /* private */ }

func NewPgxPoolManager(ctx context.Context, config *Config) (*PgxPoolManager, error)
func (m *PgxPoolManager) Pool() *pgxpool.Pool
func (m *PgxPoolManager) Ping(ctx context.Context) error
func (m *PgxPoolManager) Shutdown(ctx context.Context) error
func (m *PgxPoolManager) Stats() *pgxpool.Stat
```

**Config:**
```go
type Config struct {
    DSN                 string
    ServiceName         string
    MaxConns            int32
    MinConns            int32
    MaxConnLifetime     time.Duration
    MaxConnIdleTime     time.Duration
    HealthCheckPeriod   time.Duration
    EnableTracing       bool
    EnableMetrics       bool
    EnableQueryLogging  bool // ⚠️ NUNCA em produção
    Logger              LogFunc
}

func DefaultConfig(dsn, serviceName string) *Config
```

## Exemplos de Uso

### Exemplo 1: Setup Básico (postgres)

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/JailtonJunior94/devkit-go/pkg/database/postgres"
)

func main() {
    ctx := context.Background()

    // 1. Criar conexão
    db, err := postgres.New(
        "postgres://user:pass@localhost:5432/mydb?sslmode=disable",
        postgres.WithMaxOpenConns(25),
        postgres.WithMaxIdleConns(10),
        postgres.WithConnMaxLifetime(5*time.Minute),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer db.Shutdown(ctx)

    // 2. Verificar conectividade
    if err := db.Ping(ctx); err != nil {
        log.Fatal(err)
    }

    // 3. Usar em repositories
    repo := NewUserRepository(db.DB())
    users, err := repo.List(ctx)
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Found %d users", len(users))
}
```

### Exemplo 2: Repository Pattern com DBTX

```go
type User struct {
    ID    string
    Name  string
    Email string
}

// Repository aceita DBTX - funciona com e sem transação
type UserRepository struct {
    db database.DBTX
}

func NewUserRepository(db database.DBTX) *UserRepository {
    return &UserRepository{db: db}
}

func (r *UserRepository) FindByID(ctx context.Context, id string) (*User, error) {
    query := `SELECT id, name, email FROM users WHERE id = $1`

    var user User
    err := r.db.QueryRowContext(ctx, query, id).Scan(
        &user.ID,
        &user.Name,
        &user.Email,
    )

    if err == sql.ErrNoRows {
        return nil, fmt.Errorf("user not found")
    }

    if err != nil {
        return nil, fmt.Errorf("failed to query user: %w", err)
    }

    return &user, nil
}

func (r *UserRepository) Create(ctx context.Context, user *User) error {
    query := `INSERT INTO users (id, name, email) VALUES ($1, $2, $3)`

    _, err := r.db.ExecContext(ctx, query, user.ID, user.Name, user.Email)
    if err != nil {
        return fmt.Errorf("failed to create user: %w", err)
    }

    return nil
}

func (r *UserRepository) List(ctx context.Context) ([]*User, error) {
    query := `SELECT id, name, email FROM users ORDER BY name`

    rows, err := r.db.QueryContext(ctx, query)
    if err != nil {
        return nil, fmt.Errorf("failed to query users: %w", err)
    }
    defer rows.Close()

    var users []*User
    for rows.Next() {
        var user User
        if err := rows.Scan(&user.ID, &user.Name, &user.Email); err != nil {
            return nil, fmt.Errorf("failed to scan user: %w", err)
        }
        users = append(users, &user)
    }

    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("rows iteration error: %w", err)
    }

    return users, nil
}
```

### Exemplo 3: Uso sem Transação

```go
func main() {
    db, _ := postgres.New("postgres://...")
    defer db.Shutdown(context.Background())

    // Repository recebe *sql.DB (implementa DBTX)
    repo := NewUserRepository(db.DB())

    // Operação sem transação
    user, err := repo.FindByID(ctx, "user-123")
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("User: %+v", user)
}
```

### Exemplo 4: Uso com Transação (Unit of Work)

```go
func TransferMoney(ctx context.Context, uow uow.UnitOfWork, from, to string, amount float64) error {
    return uow.Do(ctx, func(ctx context.Context, tx database.DBTX) error {
        // Criar repositories com tx (que implementa DBTX)
        accountRepo := NewAccountRepository(tx)

        // Todas as operações abaixo estão na mesma transação
        if err := accountRepo.Debit(ctx, from, amount); err != nil {
            return err // Rollback automático
        }

        if err := accountRepo.Credit(ctx, to, amount); err != nil {
            return err // Rollback automático
        }

        // Retornar nil faz commit automático
        return nil
    })
}

func main() {
    db, _ := postgres.New("postgres://...")
    defer db.Shutdown(context.Background())

    // Criar Unit of Work
    uow := uow.NewUnitOfWork(db.DB())

    // Executar operação transacional
    err := TransferMoney(ctx, uow, "acc-1", "acc-2", 100.00)
    if err != nil {
        log.Printf("Transfer failed: %v", err)
    }
}
```

### Exemplo 5: Unit of Work com Isolation Level

```go
func main() {
    db, _ := postgres.New("postgres://...")
    defer db.Shutdown(context.Background())

    // Criar Unit of Work com nível de isolamento Serializable
    uow := uow.NewUnitOfWork(
        db.DB(),
        uow.WithIsolationLevel(sql.LevelSerializable),
    )

    err := uow.Do(ctx, func(ctx context.Context, tx database.DBTX) error {
        // Transação com isolamento mais alto
        // Previne anomalias como phantom reads
        return processOrder(ctx, tx)
    })
}
```

### Exemplo 6: Setup com Observability (postgres_otelsql)

```go
package main

import (
    "context"
    "log"

    "github.com/JailtonJunior94/devkit-go/pkg/database/postgres_otelsql"
    "go.opentelemetry.io/otel"
)

func main() {
    ctx := context.Background()

    // 1. Inicializar OpenTelemetry primeiro
    otelProvider, err := initOTel(ctx)
    if err != nil {
        log.Fatal(err)
    }
    defer otelProvider.Shutdown(ctx)

    // 2. Criar DBManager com observability
    cfg := postgres_otelsql.DefaultConfig(
        "postgres://user:pass@localhost:5432/mydb",
        "my-service",
    )

    dbManager, err := postgres_otelsql.NewDBManager(ctx, cfg)
    if err != nil {
        log.Fatal(err)
    }
    defer dbManager.Shutdown(ctx)

    // 3. Todas as queries serão automaticamente traced e metricsadas
    repo := NewUserRepository(dbManager.DB())
    users, err := repo.List(ctx)
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Found %d users (with full observability)", len(users))
}
```

### Exemplo 7: Setup com PgxPool (melhor performance)

```go
package main

import (
    "context"
    "log"

    "github.com/JailtonJunior94/devkit-go/pkg/database/pgxpool_manager"
)

func main() {
    ctx := context.Background()

    // 1. Criar PgxPoolManager
    cfg := pgxpool_manager.DefaultConfig(
        "postgres://user:pass@localhost:5432/mydb",
        "my-service",
    )

    poolManager, err := pgxpool_manager.NewPgxPoolManager(ctx, cfg)
    if err != nil {
        log.Fatal(err)
    }
    defer poolManager.Shutdown(ctx)

    // 2. Usar com pgx API nativa (melhor performance)
    pool := poolManager.Pool()

    query := `SELECT id, name FROM users WHERE id = $1`
    var id, name string

    err = pool.QueryRow(ctx, query, "user-123").Scan(&id, &name)
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("User: id=%s, name=%s", id, name)
}
```

### Exemplo 8: Health Check para Kubernetes

```go
type HealthHandler struct {
    db *postgres.Database
}

func (h *HealthHandler) Readiness(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
    defer cancel()

    if err := h.db.Ping(ctx); err != nil {
        w.WriteHeader(http.StatusServiceUnavailable)
        json.NewEncoder(w).Encode(map[string]string{
            "status": "unavailable",
            "error":  "database unreachable",
        })
        return
    }

    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{
        "status": "ok",
    })
}
```

### Exemplo 9: Graceful Shutdown

```go
func main() {
    ctx := context.Background()

    db, err := postgres.New("postgres://...")
    if err != nil {
        log.Fatal(err)
    }

    // Setup signal handling
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

    // Start application
    go runServer(db)

    // Wait for shutdown signal
    <-sigChan
    log.Println("Shutdown signal received")

    // Graceful shutdown com timeout
    shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()

    if err := db.Shutdown(shutdownCtx); err != nil {
        log.Printf("Database shutdown error: %v", err)
    }

    log.Println("Database closed gracefully")
}
```

## Casos Comuns

### Caso 1: Batch Insert com Transação

```go
func (r *UserRepository) CreateBatch(ctx context.Context, users []*User) error {
    uow := uow.NewUnitOfWork(r.db.(*sql.DB))

    return uow.Do(ctx, func(ctx context.Context, tx database.DBTX) error {
        stmt, err := tx.PrepareContext(ctx, `
            INSERT INTO users (id, name, email) VALUES ($1, $2, $3)
        `)
        if err != nil {
            return fmt.Errorf("failed to prepare statement: %w", err)
        }
        defer stmt.Close()

        for _, user := range users {
            if _, err := stmt.ExecContext(ctx, user.ID, user.Name, user.Email); err != nil {
                return fmt.Errorf("failed to insert user %s: %w", user.ID, err)
            }
        }

        return nil // Commit todas as inserções
    })
}
```

### Caso 2: Operações com Contexto de Timeout

```go
func (r *UserRepository) FindWithTimeout(ctx context.Context, id string) (*User, error) {
    // Criar contexto com timeout de 3 segundos
    queryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
    defer cancel()

    query := `SELECT id, name, email FROM users WHERE id = $1`

    var user User
    err := r.db.QueryRowContext(queryCtx, query, id).Scan(
        &user.ID,
        &user.Name,
        &user.Email,
    )

    if err == context.DeadlineExceeded {
        return nil, fmt.Errorf("query timeout after 3s")
    }

    if err != nil {
        return nil, err
    }

    return &user, nil
}
```

### Caso 3: Nested Transactions (Savepoints)

```go
func ComplexOperation(ctx context.Context, uow uow.UnitOfWork) error {
    return uow.Do(ctx, func(ctx context.Context, tx database.DBTX) error {
        // Operação 1
        if err := operation1(ctx, tx); err != nil {
            return err
        }

        // Criar savepoint para operação arriscada
        if _, err := tx.ExecContext(ctx, "SAVEPOINT sp1"); err != nil {
            return err
        }

        // Operação 2 (pode falhar)
        if err := riskyOperation(ctx, tx); err != nil {
            // Rollback apenas para o savepoint
            _, _ = tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT sp1")
            log.Printf("Risky operation failed, continuing: %v", err)
        } else {
            _, _ = tx.ExecContext(ctx, "RELEASE SAVEPOINT sp1")
        }

        // Operação 3 (sempre executada)
        if err := operation3(ctx, tx); err != nil {
            return err
        }

        return nil
    })
}
```

### Caso 4: Read-Only Transactions para Performance

```go
func GenerateReport(ctx context.Context, db *sql.DB) (*Report, error) {
    // Transação read-only pode ter melhor performance
    uow := uow.NewUnitOfWork(
        db,
        uow.WithReadOnly(true),
        uow.WithIsolationLevel(sql.LevelRepeatableRead),
    )

    var report Report

    err := uow.Do(ctx, func(ctx context.Context, tx database.DBTX) error {
        // Múltiplas queries com snapshot consistente
        orders, err := queryOrders(ctx, tx)
        if err != nil {
            return err
        }

        stats, err := queryStats(ctx, tx)
        if err != nil {
            return err
        }

        report = Report{
            Orders: orders,
            Stats:  stats,
        }

        return nil
    })

    if err != nil {
        return nil, err
    }

    return &report, nil
}
```

## Padrões Recomendados

### Padrão 1: Service com Unit of Work

```go
type OrderService struct {
    db  *sql.DB
    uow uow.UnitOfWork
}

func NewOrderService(db *sql.DB) *OrderService {
    return &OrderService{
        db:  db,
        uow: uow.NewUnitOfWork(db),
    }
}

func (s *OrderService) CreateOrder(ctx context.Context, order *Order) error {
    return s.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) error {
        orderRepo := NewOrderRepository(tx)
        inventoryRepo := NewInventoryRepository(tx)

        // Criar pedido
        if err := orderRepo.Create(ctx, order); err != nil {
            return err
        }

        // Atualizar estoque
        for _, item := range order.Items {
            if err := inventoryRepo.DecreaseStock(ctx, item.ProductID, item.Quantity); err != nil {
                return err // Rollback automático
            }
        }

        return nil // Commit automático
    })
}
```

### Padrão 2: Dependency Injection

```go
type Container struct {
    DB          *postgres.Database
    UoW         uow.UnitOfWork
    UserRepo    *UserRepository
    OrderRepo   *OrderRepository
    UserService *UserService
}

func NewContainer(dsn string) (*Container, error) {
    // 1. Criar database
    db, err := postgres.New(dsn)
    if err != nil {
        return nil, err
    }

    // 2. Criar Unit of Work
    unitOfWork := uow.NewUnitOfWork(db.DB())

    // 3. Criar repositories (sem transação para queries simples)
    userRepo := NewUserRepository(db.DB())
    orderRepo := NewOrderRepository(db.DB())

    // 4. Criar services (com UoW para operações complexas)
    userService := NewUserService(userRepo, unitOfWork)

    return &Container{
        DB:          db,
        UoW:         unitOfWork,
        UserRepo:    userRepo,
        OrderRepo:   orderRepo,
        UserService: userService,
    }, nil
}
```

### Padrão 3: Testing com Transaction Rollback

```go
func TestUserRepository_Create(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()

    uow := uow.NewUnitOfWork(db)

    err := uow.Do(context.Background(), func(ctx context.Context, tx database.DBTX) error {
        repo := NewUserRepository(tx)

        // Test dentro da transação
        user := &User{ID: "test-1", Name: "Test", Email: "test@example.com"}
        if err := repo.Create(ctx, user); err != nil {
            t.Fatalf("Create failed: %v", err)
        }

        // Verificar que foi criado
        found, err := repo.FindByID(ctx, user.ID)
        if err != nil {
            t.Fatalf("FindByID failed: %v", err)
        }

        if found.Email != user.Email {
            t.Errorf("Expected email %s, got %s", user.Email, found.Email)
        }

        // Forçar rollback para limpar
        return errors.New("rollback test transaction")
    })

    // Esperado que falhe (rollback intencional)
    if err == nil {
        t.Error("Expected rollback error")
    }
}
```

## Anti-Patterns a Evitar

### ❌ Anti-Pattern 1: Criar Conexão por Request

```go
// ERRADO - cria pool novo a cada request
func HandleRequest(w http.ResponseWriter, r *http.Request) {
    db, _ := postgres.New("postgres://...") // NÃO FAÇA ISSO
    defer db.Shutdown(r.Context())

    // ... usar db
}
```

**Solução:**
```go
// CORRETO - criar uma vez no main
func main() {
    db, _ := postgres.New("postgres://...")
    defer db.Shutdown(context.Background())

    handler := &Handler{db: db}
    http.HandleFunc("/", handler.HandleRequest)
}
```

### ❌ Anti-Pattern 2: Ignorar Erros de Rows.Close()

```go
// ERRADO
rows, _ := db.QueryContext(ctx, query)
defer rows.Close() // Ignora erro

for rows.Next() {
    // ...
}
```

**Solução:**
```go
// CORRETO
rows, err := db.QueryContext(ctx, query)
if err != nil {
    return err
}
defer func() {
    if closeErr := rows.Close(); closeErr != nil {
        log.Printf("Failed to close rows: %v", closeErr)
    }
}()

for rows.Next() {
    // ...
}

if err := rows.Err(); err != nil {
    return err
}
```

### ❌ Anti-Pattern 3: Repository com *sql.DB Hardcoded

```go
// ERRADO - não funciona com transações
type UserRepository struct {
    db *sql.DB // Específico demais
}
```

**Solução:**
```go
// CORRETO - aceita DBTX (funciona com DB e Tx)
type UserRepository struct {
    db database.DBTX
}
```

### ❌ Anti-Pattern 4: Transação sem Timeout

```go
// ERRADO - pode bloquear indefinidamente
func LongOperation() error {
    return uow.Do(context.Background(), func(ctx context.Context, tx database.DBTX) error {
        // Operação longa sem timeout
        return heavyQuery(ctx, tx)
    })
}
```

**Solução:**
```go
// CORRETO
func LongOperation() error {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    return uow.Do(ctx, func(ctx context.Context, tx database.DBTX) error {
        return heavyQuery(ctx, tx)
    })
}
```

### ❌ Anti-Pattern 5: Não Verificar sql.ErrNoRows

```go
// ERRADO - sql.ErrNoRows não é erro fatal
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*User, error) {
    var user User
    err := r.db.QueryRowContext(ctx, query, email).Scan(&user.ID, &user.Name)
    return &user, err // Retorna sql.ErrNoRows como erro genérico
}
```

**Solução:**
```go
// CORRETO
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*User, error) {
    var user User
    err := r.db.QueryRowContext(ctx, query, email).Scan(&user.ID, &user.Name)

    if err == sql.ErrNoRows {
        return nil, nil // Ou erro customizado: ErrUserNotFound
    }

    if err != nil {
        return nil, fmt.Errorf("failed to query user: %w", err)
    }

    return &user, nil
}
```

## Escalabilidade

### Projetos Pequenos (< 50 queries/s)
- Use `postgres.Database` (mais simples)
- Pool padrão (25 conexões) é suficiente
- Não precisa de métricas detalhadas

**Setup:**
```go
db, _ := postgres.New("postgres://...")
defer db.Shutdown(ctx)
```

### Projetos Médios (50-500 queries/s)
- Use `postgres_otelsql.DBManager` (observability)
- Ajuste pool baseado em métricas:
  - Monitore `db.client.connections.wait_count`
  - Aumente `MaxOpenConns` se wait_count > 0
  - Reduza `MaxIdleConns` se `idle > usage * 0.5`
- Implemente query timeout (3-5s)

**Setup:**
```go
cfg := postgres_otelsql.DefaultConfig(dsn, "service")
cfg.MaxOpenConns = 50
cfg.MaxIdleConns = 20

dbManager, _ := postgres_otelsql.NewDBManager(ctx, cfg)
defer dbManager.Shutdown(ctx)
```

### Projetos Grandes (> 500 queries/s)
- Use `pgxpool_manager.PgxPoolManager` (melhor performance)
- Tune agressivo do pool:
  - MaxConns = 100-200 (ou 60-80% de `max_connections` do PostgreSQL)
  - MinConns = 20-50 (mantém warm connections)
- Considere PgBouncer para connection pooling externo
- Implemente read replicas para queries read-only

**Setup:**
```go
cfg := pgxpool_manager.DefaultConfig(dsn, "service")
cfg.MaxConns = 150
cfg.MinConns = 30
cfg.MaxConnLifetime = 10 * time.Minute
cfg.HealthCheckPeriod = 30 * time.Second

poolManager, _ := pgxpool_manager.NewPgxPoolManager(ctx, cfg)
defer poolManager.Shutdown(ctx)
```

**Métricas para monitorar:**
- `wait_count`: Quantas vezes esperou por conexão (deve ser ~0)
- `wait_duration`: Tempo de espera (deve ser < 10ms)
- `max_lifetime_closed`: Conexões fechadas por max lifetime
- `idle_closed`: Conexões fechadas por idle timeout

## Testabilidade

### Mock de DBTX

```go
type MockDB struct {
    QueryRowFunc  func(ctx context.Context, query string, args ...any) *sql.Row
    QueryFunc     func(ctx context.Context, query string, args ...any) (*sql.Rows, error)
    ExecFunc      func(ctx context.Context, query string, args ...any) (sql.Result, error)
    PrepareFunc   func(ctx context.Context, query string) (*sql.Stmt, error)
}

func (m *MockDB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
    if m.QueryRowFunc != nil {
        return m.QueryRowFunc(ctx, query, args...)
    }
    return nil
}

// ... outros métodos

// Uso
func TestUserRepository_FindByID(t *testing.T) {
    mockDB := &MockDB{
        QueryRowFunc: func(ctx context.Context, query string, args ...any) *sql.Row {
            // Retornar row mockado
        },
    }

    repo := NewUserRepository(mockDB)
    user, err := repo.FindByID(context.Background(), "test-id")

    // Assertions
}
```

### Testes com Banco Real (Docker)

```go
func setupTestDB(t *testing.T) *sql.DB {
    dsn := os.Getenv("TEST_DATABASE_URL")
    if dsn == "" {
        t.Skip("TEST_DATABASE_URL not set")
    }

    db, err := postgres.New(dsn)
    if err != nil {
        t.Fatalf("Failed to connect to test database: %v", err)
    }

    // Limpar dados antes de cada teste
    db.DB().ExecContext(context.Background(), "TRUNCATE users CASCADE")

    return db.DB()
}
```

**Docker Compose para testes:**
```yaml
version: '3.8'
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: test
      POSTGRES_PASSWORD: test
      POSTGRES_DB: testdb
    ports:
      - "5432:5432"
```

## FAQ / Troubleshooting

### P: Qual a diferença entre `postgres`, `postgres_otelsql` e `pgxpool_manager`?

**R:**
- **`postgres.Database`**: Simples, sem observability. Use para projetos pequenos ou que já têm observability na camada de aplicação.
- **`postgres_otelsql.DBManager`**: database/sql + otelsql. Use quando você quer observability mas precisa de compatibilidade com database/sql.
- **`pgxpool_manager.PgxPoolManager`**: Driver nativo pgx. Melhor performance, observability nativa, features avançadas do PostgreSQL.

**Recomendação:** Use `pgxpool_manager` para projetos novos de médio/grande porte.

### P: Como ajustar o pool de conexões?

**R:** Monitore métricas e ajuste baseado em:

1. **`wait_count` > 0**: Aumente `MaxOpenConns`
2. **`idle > usage * 0.5`**: Reduza `MaxIdleConns`
3. **Latência alta**: Aumente `MaxIdleConns` (mais warm connections)
4. **Memória alta**: Reduza `MaxIdleConns` ou `ConnMaxIdleTime`

**Fórmula básica:**
```
MaxOpenConns = (concurrent_requests * avg_query_time) / target_response_time

Exemplo:
- 100 req/s concorrentes
- Queries levam 50ms em média
- Quero resposta em 100ms

MaxOpenConns = (100 * 0.05) / 0.1 = 50 conexões
```

### P: Unit of Work funciona com pgxpool?

**R:** Não diretamente. `uow.UnitOfWork` foi projetado para `database/sql`. Para pgxpool, use transações nativas:

```go
err := pool.BeginFunc(ctx, func(tx pgx.Tx) error {
    // Operações transacionais aqui
    return nil // Commit automático
})
```

### P: Como fazer DEBUG de queries SQL?

**R:**

**Opção 1: Log queries via config (DEV APENAS)**
```go
cfg := postgres_otelsql.DefaultConfig(dsn, "service")
cfg.EnableQueryLogging = true
cfg.Logger = func(format string, args ...any) {
    log.Printf("[SQL] "+format, args...)
}
```

**Opção 2: PostgreSQL log (produção)**
```sql
-- No PostgreSQL
ALTER SYSTEM SET log_statement = 'all';
SELECT pg_reload_conf();

-- Ver logs
tail -f /var/log/postgresql/postgresql.log
```

**Opção 3: Tracing com OpenTelemetry**
```go
// Queries aparecem como spans no Jaeger/Tempo
dbManager, _ := postgres_otelsql.NewDBManager(ctx, cfg)
```

### P: Como prevenir SQL injection?

**R:** **SEMPRE** use placeholders ($1, $2, etc.):

```go
// ERRADO - vulnerável a SQL injection
query := fmt.Sprintf("SELECT * FROM users WHERE email = '%s'", email)
db.QueryContext(ctx, query)

// CORRETO - usa placeholders
query := "SELECT * FROM users WHERE email = $1"
db.QueryContext(ctx, query, email)
```

### P: Erro "too many connections" - o que fazer?

**R:**

1. **Verificar pool config:**
```go
// Se MaxOpenConns > max_connections do PostgreSQL
cfg.MaxOpenConns = 50 // Reduzir
```

2. **Verificar connection leaks:**
```go
// SEMPRE fechar rows
rows, _ := db.QueryContext(ctx, query)
defer rows.Close() // Crítico!
```

3. **Verificar PostgreSQL max_connections:**
```sql
SHOW max_connections; -- Ver limite atual
ALTER SYSTEM SET max_connections = 200; -- Aumentar (requer restart)
```

4. **Considerar PgBouncer** para pooling externo

### P: Context cancelado mas query continua executando?

**R:** Queries SQL **NÃO** podem ser canceladas depois de iniciadas. O context apenas previne que novas queries sejam enviadas. Para cancelar queries longas:

**PostgreSQL:**
```sql
-- Cancelar query específica
SELECT pg_cancel_backend(pid);

-- Ver queries ativas
SELECT pid, query, state FROM pg_stat_activity WHERE state = 'active';
```

**No código (timeout agressivo):**
```go
ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()

// Se timeout, pelo menos não fica esperando indefinidamente
err := db.QueryRowContext(ctx, slowQuery).Scan(&result)
```

### P: Como fazer migrations?

**R:** Use ferramentas especializadas:

**golang-migrate:**
```bash
migrate -database "postgres://..." -path ./migrations up
```

**goose:**
```bash
goose postgres "postgres://..." up
```

**Integração no código:**
```go
import "github.com/golang-migrate/migrate/v4"

func RunMigrations(dsn string) error {
    m, err := migrate.New(
        "file://migrations",
        dsn,
    )
    if err != nil {
        return err
    }

    return m.Up()
}
```

### P: Performance degradou após migração para pgx?

**R:** Verifique configurações:

1. **Pool muito pequeno:** Aumente MaxConns
2. **Health checks muito frequentes:** Aumente HealthCheckPeriod
3. **Prepared statements não estão sendo cacheados:** Use `pool.Exec` ao invés de `QueryRow` para commands

**Benchmark:**
```bash
go test -bench=. -benchmem ./...
```

## Licença

Este pacote faz parte do projeto devkit-go.
