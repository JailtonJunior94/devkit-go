# Guia de Migração para Uber FX + Unit of Work

Este guia mostra como migrar o projeto atual do container manual para Uber FX com Unit of Work.

## 📋 Estado Atual

Atualmente o projeto usa um container manual em `pkg/bundle/container.go`:

```go
type Container struct {
    DB                     *sql.DB
    Config                 *configs.Config
    Jwt                    auth.JwtAdapter
    Hash                   encrypt.HashAdapter
    Telemetry              o11y.Telemetry
    MiddlewareAuth         middlewares.Authorization
    PanicRecoverMiddleware middlewares.PanicRecoverMiddleware
}

func NewContainer(ctx context.Context) *Container {
    // Inicialização manual de todas as dependências
}
```

## 🎯 Estado Desejado

Migrar para Uber FX com:
- ✅ Dependency Injection automática
- ✅ Lifecycle management (graceful shutdown)
- ✅ Unit of Work para transações
- ✅ Melhor testabilidade
- ✅ Menos código boilerplate

## 🔄 Passo a Passo da Migração

### Passo 1: Criar Módulo FX para Configuração

Crie `pkg/bundle/fx_config.go`:

```go
package bundle

import (
    "context"
    "github.com/jailtonjunior94/financial/configs"
    "go.uber.org/fx"
)

// ConfigModule fornece configuração via FX
var ConfigModule = fx.Module(
    "config",
    fx.Provide(NewConfig),
)

// NewConfig carrega configuração do ambiente
func NewConfig() (*configs.Config, error) {
    return configs.LoadConfig(".")
}
```

### Passo 2: Criar Módulo FX para Database

Crie `pkg/bundle/fx_database.go`:

```go
package bundle

import (
    "context"
    "database/sql"

    "github.com/jailtonjunior94/financial/configs"
    "github.com/jailtonjunior94/financial/pkg/database/postgres"
    "github.com/jailtonjunior94/financial/pkg/database/uow"
    "go.uber.org/fx"
)

// DatabaseModule fornece database e UoW via FX
var DatabaseModule = fx.Module(
    "database",
    fx.Provide(
        NewDatabase,
        NewUnitOfWork,
    ),
)

// NewDatabase cria e gerencia conexão com database
func NewDatabase(lc fx.Lifecycle, cfg *configs.Config) (*sql.DB, error) {
    db, err := postgres.NewPostgresDatabase(cfg)
    if err != nil {
        return nil, err
    }

    lc.Append(fx.Hook{
        OnStart: func(ctx context.Context) error {
            if err := db.PingContext(ctx); err != nil {
                return err
            }
            return nil
        },
        OnStop: func(ctx context.Context) error {
            return db.Close()
        },
    })

    return db, nil
}

// NewUnitOfWork cria Unit of Work com configurações para produção
func NewUnitOfWork(db *sql.DB) uow.UnitOfWork {
    return uow.NewUnitOfWorkWithOptions(
        db,
        sql.LevelReadCommitted, // Recomendado para produção
        30 * time.Second,       // Timeout padrão
    )
}
```

### Passo 3: Criar Módulo FX para Telemetria

Crie `pkg/bundle/fx_telemetry.go`:

```go
package bundle

import (
    "context"

    "github.com/jailtonjunior94/financial/configs"
    "github.com/JailtonJunior94/devkit-go/pkg/o11y"
    "go.uber.org/fx"
)

// TelemetryModule fornece telemetria via FX
var TelemetryModule = fx.Module(
    "telemetry",
    fx.Provide(NewTelemetry),
)

// NewTelemetry cria e gerencia telemetria
func NewTelemetry(lc fx.Lifecycle, cfg *configs.Config) (o11y.Telemetry, error) {
    var telemetry o11y.Telemetry

    lc.Append(fx.Hook{
        OnStart: func(ctx context.Context) error {
            resource, err := o11y.NewServiceResource(
                ctx,
                cfg.O11yConfig.ServiceName,
                cfg.O11yConfig.ServiceVersion,
                "production",
            )
            if err != nil {
                return err
            }

            tracer, tracerShutdown, err := o11y.NewTracerWithOptions(ctx,
                o11y.WithTracerEndpoint(cfg.O11yConfig.ExporterEndpoint),
                o11y.WithTracerServiceName(cfg.O11yConfig.ServiceName),
                o11y.WithTracerResource(resource),
                o11y.WithTracerInsecure(),
            )
            if err != nil {
                return err
            }

            metrics, metricsShutdown, err := o11y.NewMetricsWithOptions(ctx,
                o11y.WithMetricsEndpoint(cfg.O11yConfig.ExporterEndpoint),
                o11y.WithMetricsServiceName(cfg.O11yConfig.ServiceName),
                o11y.WithMetricsResource(resource),
                o11y.WithMetricsInsecure(),
            )
            if err != nil {
                return err
            }

            logger, loggerShutdown, err := o11y.NewLoggerWithOptions(ctx,
                o11y.WithLoggerEndpoint(cfg.O11yConfig.ExporterEndpointHTTP),
                o11y.WithLoggerServiceName(cfg.O11yConfig.ServiceName),
                o11y.WithLoggerResource(resource),
                o11y.WithLoggerInsecure(),
            )
            if err != nil {
                return err
            }

            telemetry, err = o11y.NewTelemetry(
                tracer, metrics, logger,
                tracerShutdown, metricsShutdown, loggerShutdown,
            )
            if err != nil {
                return err
            }

            return nil
        },
        OnStop: func(ctx context.Context) error {
            if telemetry != nil {
                return telemetry.Shutdown(ctx)
            }
            return nil
        },
    })

    return telemetry, nil
}
```

### Passo 4: Criar Módulo FX para Auth e Middlewares

Crie `pkg/bundle/fx_auth.go`:

```go
package bundle

import (
    "github.com/jailtonjunior94/financial/configs"
    "github.com/jailtonjunior94/financial/pkg/api/middlewares"
    "github.com/jailtonjunior94/financial/pkg/auth"
    "github.com/JailtonJunior94/devkit-go/pkg/encrypt"
    "github.com/JailtonJunior94/devkit-go/pkg/o11y"
    "go.uber.org/fx"
)

// AuthModule fornece autenticação e middlewares via FX
var AuthModule = fx.Module(
    "auth",
    fx.Provide(
        NewHashAdapter,
        NewJwtAdapter,
        NewAuthMiddleware,
        NewPanicRecoverMiddleware,
    ),
)

func NewHashAdapter() encrypt.HashAdapter {
    return encrypt.NewHashAdapter()
}

func NewJwtAdapter(cfg *configs.Config, telemetry o11y.Telemetry) auth.JwtAdapter {
    return auth.NewJwtAdapter(cfg, telemetry)
}

func NewAuthMiddleware(cfg *configs.Config, jwt auth.JwtAdapter) middlewares.Authorization {
    return middlewares.NewAuthorization(cfg, jwt)
}

func NewPanicRecoverMiddleware(telemetry o11y.Telemetry) middlewares.PanicRecoverMiddleware {
    return middlewares.NewPanicRecoverMiddleware(telemetry)
}
```

### Passo 5: Criar Módulo Principal

Crie `pkg/bundle/fx_bundle.go`:

```go
package bundle

import (
    "go.uber.org/fx"
)

// AppModule agrupa todos os módulos da aplicação
var AppModule = fx.Options(
    ConfigModule,
    DatabaseModule,
    TelemetryModule,
    AuthModule,
)
```

### Passo 6: Atualizar cmd/main.go

```go
package main

import (
    "context"
    "log"

    "github.com/jailtonjunior94/financial/cmd/server"
    "github.com/jailtonjunior94/financial/pkg/bundle"
    "github.com/jailtonjunior94/financial/internal/user"
    "github.com/jailtonjunior94/financial/internal/category"
    "github.com/jailtonjunior94/financial/internal/budget"
    "go.uber.org/fx"
)

func main() {
    app := fx.New(
        // ========== INFRAESTRUTURA ==========
        bundle.AppModule,

        // ========== MÓDULOS DE DOMÍNIO ==========
        fx.Provide(
            user.NewModule,
            category.NewModule,
            budget.NewModule,
        ),

        // ========== HTTP SERVER ==========
        fx.Provide(server.NewHTTPServer),

        // ========== INVOKE ==========
        fx.Invoke(func(*server.HTTPServer) {
            // Garantir que servidor seja instanciado
        }),
    )

    app.Run()
}
```

### Passo 7: Atualizar Módulos de Domínio

Exemplo para `internal/user/module.go`:

```go
package user

import (
    "database/sql"
    "net/http"

    "github.com/jailtonjunior94/financial/pkg/api/middlewares"
    "github.com/jailtonjunior94/financial/pkg/auth"
    "github.com/jailtonjunior94/financial/pkg/database/uow"
    userDomain "github.com/jailtonjunior94/financial/internal/user/domain/interfaces"
    "github.com/jailtonjunior94/financial/internal/user/infrastructure/http/handlers"
    "github.com/jailtonjunior94/financial/internal/user/infrastructure/repositories"
    "github.com/jailtonjunior94/financial/internal/user/application/usecase"
    "go.uber.org/fx"
)

// UserModuleDeps dependências do módulo User
type UserModuleDeps struct {
    fx.In

    DB         *sql.DB
    UoW        uow.UnitOfWork  // ← NOVO!
    JWT        auth.JwtAdapter
    Hash       encrypt.HashAdapter
    AuthMW     middlewares.Authorization
}

// UserModule retorna o módulo User para FX
var UserModule = fx.Module(
    "user",
    fx.Provide(
        NewUserRepository,
        NewAuthenticationUseCase,
        NewCreateUserUseCase,
        NewUserHandler,
        NewUserRoutes,
    ),
)

func NewUserRepository(db *sql.DB) userDomain.UserRepository {
    return repositories.NewUserRepository(db)
}

func NewAuthenticationUseCase(
    repo userDomain.UserRepository,
    jwt auth.JwtAdapter,
    hash encrypt.HashAdapter,
) *usecase.AuthenticationUseCase {
    return usecase.NewAuthenticationUseCase(repo, jwt, hash)
}

func NewCreateUserUseCase(
    uow uow.UnitOfWork,  // ← USAR UoW!
    repo userDomain.UserRepository,
    hash encrypt.HashAdapter,
) *usecase.CreateUserUseCase {
    return usecase.NewCreateUserUseCase(uow, repo, hash)
}

func NewUserHandler(
    authUC *usecase.AuthenticationUseCase,
    createUC *usecase.CreateUserUseCase,
) *handlers.UserHandler {
    return handlers.NewUserHandler(authUC, createUC)
}

func NewUserRoutes(
    handler *handlers.UserHandler,
    authMW middlewares.Authorization,
) []server.Route {
    return []server.Route{
        {Method: http.MethodPost, Path: "/auth", Handler: handler.Authentication},
        {Method: http.MethodPost, Path: "/users", Handler: handler.Create},
    }
}
```

### Passo 8: Atualizar Use Cases para Usar UoW

Exemplo `internal/user/application/usecase/create_user.go`:

```go
package usecase

import (
    "context"
    "fmt"

    "github.com/jailtonjunior94/financial/pkg/database"
    "github.com/jailtonjunior94/financial/pkg/database/uow"
    "github.com/jailtonjunior94/financial/internal/user/application/dtos"
    "github.com/jailtonjunior94/financial/internal/user/domain/factories"
    "github.com/jailtonjunior94/financial/internal/user/domain/interfaces"
    "github.com/JailtonJunior94/devkit-go/pkg/encrypt"
)

type CreateUserUseCase struct {
    uow  uow.UnitOfWork           // ← NOVO!
    repo interfaces.UserRepository
    hash encrypt.HashAdapter
}

func NewCreateUserUseCase(
    uow uow.UnitOfWork,
    repo interfaces.UserRepository,
    hash encrypt.HashAdapter,
) *CreateUserUseCase {
    return &CreateUserUseCase{
        uow:  uow,
        repo: repo,
        hash: hash,
    }
}

func (uc *CreateUserUseCase) Execute(ctx context.Context, input *dtos.CreateUserInput) (*dtos.UserOutput, error) {
    // Hash da senha
    hashedPassword, err := uc.hash.Hash(input.Password)
    if err != nil {
        return nil, fmt.Errorf("failed to hash password: %w", err)
    }

    // Criar entidade
    user, err := factories.NewUser(input.Name, input.Email, hashedPassword)
    if err != nil {
        return nil, err
    }

    // Executar dentro de transação
    err = uc.uow.Do(ctx, func(ctx context.Context, tx database.DBExecutor) error {
        // Todas as operações aqui são atômicas
        if err := uc.repo.Create(ctx, tx, user); err != nil {
            return fmt.Errorf("failed to create user: %w", err)
        }

        // Se houver mais operações (ex: criar conta, enviar email, etc)
        // adicione aqui e todas serão atômicas

        return nil // Commit automático
    })

    if err != nil {
        return nil, err
    }

    return &dtos.UserOutput{
        ID:    user.ID.String(),
        Name:  user.Name.Value(),
        Email: user.Email.Value(),
    }, nil
}
```

## ✅ Benefícios da Migração

### Antes (Container Manual)

```go
func main() {
    ctx := context.Background()

    // Inicialização manual
    container := bundle.NewContainer(ctx)
    defer container.DB.Close() // Fácil esquecer!

    // Passar container para tudo
    userModule := user.NewModule(container)
    categoryModule := category.NewModule(container)
    budgetModule := budget.NewModule(container)

    // Setup manual do servidor
    server := server.NewServer(container, userModule, categoryModule, budgetModule)

    // Sem graceful shutdown automático
    if err := server.Start(); err != nil {
        log.Fatal(err)
    }
}
```

### Depois (Uber FX)

```go
func main() {
    app := fx.New(
        bundle.AppModule,
        fx.Provide(
            user.NewModule,
            category.NewModule,
            budget.NewModule,
        ),
        fx.Provide(server.NewHTTPServer),
        fx.Invoke(func(*server.HTTPServer) {}),
    )

    app.Run() // ← Graceful shutdown automático!
}
```

### Vantagens:

1. ✅ **Graceful Shutdown Automático**: FX gerencia lifecycle
2. ✅ **Menos Código**: Não precisa passar container manualmente
3. ✅ **Melhor Testabilidade**: Fácil criar app de testes
4. ✅ **Type-Safe**: Erros de dependência em compile-time
5. ✅ **Atomicidade**: UoW garante transações atômicas
6. ✅ **Mais Fácil de Estender**: Apenas adicionar fx.Provide()

## 🧪 Testes com FX

### Antes (Manual)

```go
func TestCreateUser(t *testing.T) {
    // Setup manual complexo
    db, _ := setupTestDB()
    defer db.Close()

    config := &configs.Config{...}
    hash := encrypt.NewHashAdapter()
    jwt := auth.NewJwtAdapter(config, nil)

    repo := repositories.NewUserRepository(db)
    uc := usecase.NewCreateUserUseCase(repo, hash)

    // Testar...
}
```

### Depois (FX)

```go
func TestCreateUser(t *testing.T) {
    var uc *usecase.CreateUserUseCase

    app := fxtest.New(
        t,
        bundle.AppModule,
        fx.Provide(
            user.NewCreateUserUseCase,
        ),
        fx.Populate(&uc),
        fx.NopLogger,
    )

    app.RequireStart()
    defer app.RequireStop()

    // Testar com uc injetado automaticamente!
}
```

## 📊 Checklist de Migração

- [ ] Criar módulos FX (config, database, telemetry, auth)
- [ ] Adicionar UoW ao DatabaseModule
- [ ] Atualizar módulos de domínio (user, category, budget)
- [ ] Atualizar use cases para usar UoW
- [ ] Atualizar repositórios para aceitar DBExecutor
- [ ] Atualizar cmd/main.go para usar FX
- [ ] Atualizar testes para usar fxtest
- [ ] Testar aplicação completa
- [ ] Remover container.go antigo
- [ ] Atualizar documentação

## 🚀 Executar Aplicação Migrada

```bash
# Instalar dependências
go mod tidy

# Executar aplicação
go run cmd/main.go api

# Testar
curl http://localhost:8080/health
```

## 📚 Próximos Passos

Após migração básica, considere adicionar:

1. **Retry Logic**: Para operações com deadlock
2. **Circuit Breaker**: Para proteger banco em caso de falhas
3. **Observabilidade**: Metrics de transações
4. **Idempotência**: Para operações financeiras críticas

Veja [`fx_advanced.go`](fx_advanced.go) para exemplos.

---

**Dúvidas?** Consulte [`README.md`](README.md) ou os exemplos em [`example_app/`](example_app/).
