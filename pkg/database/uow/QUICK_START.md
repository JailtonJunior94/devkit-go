# Quick Start - Adicionar UoW ao Projeto Atual (SEM migração completa)

Este guia mostra como adicionar Unit of Work ao projeto **SEM** migrar para Uber FX. É uma abordagem incremental e menos invasiva.

## 🎯 Objetivo

Adicionar atomicidade às operações de negócio mantendo a estrutura atual do projeto.

## ⚡ Implementação Rápida (15 minutos)

### Passo 1: Adicionar UoW ao Container

Edite `pkg/bundle/container.go`:

```go
package bundle

import (
    // ... imports existentes
    "github.com/jailtonjunior94/financial/pkg/database/uow"
)

type Container struct {
    DB                     *sql.DB
    UoW                    uow.UnitOfWork  // ← ADICIONAR ESTA LINHA
    Config                 *configs.Config
    Jwt                    auth.JwtAdapter
    Hash                   encrypt.HashAdapter
    Telemetry              o11y.Telemetry
    MiddlewareAuth         middlewares.Authorization
    PanicRecoverMiddleware middlewares.PanicRecoverMiddleware
}

func NewContainer(ctx context.Context) *Container {
    config, err := configs.LoadConfig(".")
    if err != nil {
        log.Fatalf("error loading config: %v", err)
    }

    db, err := postgres.NewPostgresDatabase(config)
    if err != nil {
        log.Fatalf("error connecting to database: %v", err)
    }

    // ========== ADICIONAR ESTAS LINHAS ==========
    uow := uow.NewUnitOfWorkWithOptions(
        db,
        sql.LevelReadCommitted, // Recomendado para produção
        30 * time.Second,       // Timeout padrão
    )
    // ============================================

    // ... resto do código de telemetry, auth, etc

    return &Container{
        DB:                     db,
        UoW:                    uow,  // ← ADICIONAR ESTA LINHA
        Jwt:                    jwt,
        Hash:                   hash,
        Config:                 config,
        MiddlewareAuth:         middlewareAuth,
        Telemetry:              telemetry,
        PanicRecoverMiddleware: panicRecoverMiddleware,
    }
}
```

### Passo 2: Atualizar Módulos para Receber UoW

Edite `internal/budget/module.go` (exemplo):

```go
package budget

import (
    // ... imports existentes
    "github.com/jailtonjunior94/financial/pkg/database/uow"
)

type Module struct {
    Routes []server.Route
}

// ========== ATUALIZAR ASSINATURA ==========
func NewModule(c *bundle.Container) *Module {
    // Repositórios
    budgetRepository := repositories.NewBudgetRepository(c.DB)
    itemRepository := repositories.NewItemRepository(c.DB)

    // Use Cases (passar UoW)
    createBudgetUC := usecase.NewCreateBudgetUseCase(
        c.UoW,           // ← ADICIONAR
        budgetRepository,
        itemRepository,
    )

    updateBudgetUC := usecase.NewUpdateBudgetUseCase(
        c.UoW,           // ← ADICIONAR
        budgetRepository,
        itemRepository,
    )

    deleteBudgetUC := usecase.NewDeleteBudgetUseCase(
        c.UoW,           // ← ADICIONAR
        budgetRepository,
    )

    // ... resto do código
}
```

### Passo 3: Atualizar Use Cases

Edite `internal/budget/application/usecase/create_budget.go`:

```go
package usecase

import (
    // ... imports existentes
    "github.com/jailtonjunior94/financial/pkg/database"
    "github.com/jailtonjunior94/financial/pkg/database/uow"
)

type CreateBudgetUseCase struct {
    uow              uow.UnitOfWork  // ← ADICIONAR
    budgetRepository budgetDomain.BudgetRepository
    itemRepository   budgetDomain.ItemRepository
}

func NewCreateBudgetUseCase(
    uow uow.UnitOfWork,  // ← ADICIONAR
    budgetRepository budgetDomain.BudgetRepository,
    itemRepository budgetDomain.ItemRepository,
) *CreateBudgetUseCase {
    return &CreateBudgetUseCase{
        uow:              uow,  // ← ADICIONAR
        budgetRepository: budgetRepository,
        itemRepository:   itemRepository,
    }
}

func (uc *CreateBudgetUseCase) Execute(ctx context.Context, input *dtos.CreateBudgetInput) (*dtos.BudgetOutput, error) {
    // Criar entidades (FORA da transação)
    budget, err := factories.NewBudget(input.UserID, input.Month, input.Year, input.Income)
    if err != nil {
        return nil, err
    }

    items, err := uc.createItems(input.Items, budget.ID)
    if err != nil {
        return nil, err
    }

    // ========== USAR UoW PARA ATOMICIDADE ==========
    err = uc.uow.Do(ctx, func(ctx context.Context, tx database.DBExecutor) error {
        // 1. Criar budget
        if err := uc.budgetRepository.Create(ctx, tx, budget); err != nil {
            return fmt.Errorf("failed to create budget: %w", err)
        }

        // 2. Criar items
        for _, item := range items {
            if err := uc.itemRepository.Create(ctx, tx, item); err != nil {
                return fmt.Errorf("failed to create item: %w", err)
            }
        }

        // Se QUALQUER operação falhar, TODAS são revertidas
        return nil // Commit automático se tudo OK
    })
    // ===============================================

    if err != nil {
        return nil, err
    }

    return &dtos.BudgetOutput{
        ID:     budget.ID.String(),
        UserID: budget.UserID.String(),
        Month:  budget.Month,
        Year:   budget.Year,
        Income: budget.Income.Value(),
    }, nil
}

// createItems permanece igual (helper privado)
```

## 📋 Quais Use Cases Precisam de UoW?

### ✅ PRECISAM de UoW (operações múltiplas)

1. **CreateBudgetUseCase**
   - Cria Budget + múltiplos Items
   - Se criar Budget mas falhar ao criar Item → ROLLBACK

2. **UpdateBudgetUseCase**
   - Atualiza Budget + recria Items
   - Precisa garantir atomicidade

3. **DeleteBudgetUseCase**
   - Deleta Budget (soft delete)
   - Pode precisar deletar items relacionados
   - Atomicidade garantida

### ❌ NÃO PRECISAM de UoW (operação única)

1. **AuthenticationUseCase**
   - Apenas lê usuário
   - Não faz writes

2. **CreateUserUseCase**
   - Apenas insere 1 registro
   - Banco garante atomicidade de 1 INSERT
   - **NOTA**: Se futuramente criar conta junto, precisará de UoW!

3. **GetBudgetByIDUseCase**
   - Apenas leitura
   - Não precisa de transação

## 🔄 Exemplo Completo: Budget Use Case

### ANTES (sem UoW)

```go
func (uc *CreateBudgetUseCase) Execute(ctx context.Context, input *dtos.CreateBudgetInput) (*dtos.BudgetOutput, error) {
    budget, err := factories.NewBudget(...)
    if err != nil {
        return nil, err
    }

    // ❌ PROBLEMA: Se criar budget OK mas items falhar,
    // budget fica órfão no banco!
    if err := uc.budgetRepository.Create(ctx, uc.db, budget); err != nil {
        return nil, err
    }

    for _, item := range items {
        // Se ESTE insert falhar, budget já foi criado!
        if err := uc.itemRepository.Create(ctx, uc.db, item); err != nil {
            return nil, err // ❌ Budget órfão!
        }
    }

    return output, nil
}
```

### DEPOIS (com UoW)

```go
func (uc *CreateBudgetUseCase) Execute(ctx context.Context, input *dtos.CreateBudgetInput) (*dtos.BudgetOutput, error) {
    budget, err := factories.NewBudget(...)
    if err != nil {
        return nil, err
    }

    // ✅ SOLUÇÃO: Tudo dentro de transação atômica
    err = uc.uow.Do(ctx, func(ctx context.Context, tx database.DBExecutor) error {
        if err := uc.budgetRepository.Create(ctx, tx, budget); err != nil {
            return err // ← ROLLBACK automático
        }

        for _, item := range items {
            if err := uc.itemRepository.Create(ctx, tx, item); err != nil {
                return err // ← ROLLBACK automático (budget também)
            }
        }

        return nil // ← COMMIT automático
    })

    if err != nil {
        return nil, err
    }

    return output, nil
}
```

## 🧪 Testando

### Teste Manual

```bash
# 1. Iniciar aplicação
make start_docker
go run cmd/main.go api

# 2. Criar budget com erro forçado
# (ex: item com category_id inválido)
curl -X POST http://localhost:8080/budgets \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "...",
    "month": 1,
    "year": 2024,
    "income": 5000,
    "items": [
      {"category_id": "INVALID", "amount": 100}
    ]
  }'

# 3. Verificar que NENHUM dado foi criado
# (sem UoW, budget seria criado mesmo com item falhando)
```

### Teste Unitário

```go
func TestCreateBudgetUseCase_Rollback(t *testing.T) {
    // Arrange
    db, _ := setupTestDB(t)
    uow := uow.NewUnitOfWork(db)
    budgetRepo := repositories.NewBudgetRepository(db)
    itemRepo := repositories.NewItemRepository(db)
    uc := usecase.NewCreateBudgetUseCase(uow, budgetRepo, itemRepo)

    input := &dtos.CreateBudgetInput{
        // ... dados que causam erro no item
    }

    // Act
    _, err := uc.Execute(context.Background(), input)

    // Assert
    assert.Error(t, err)

    // Verificar que NENHUM budget foi criado (rollback funcionou)
    var count int
    db.QueryRow("SELECT COUNT(*) FROM budgets").Scan(&count)
    assert.Equal(t, 0, count, "Nenhum budget deve existir após rollback")
}
```

## 📊 Checklist de Implementação

### Use Cases que DEVEM ser atualizados:

- [ ] `internal/budget/application/usecase/create_budget.go`
- [ ] `internal/budget/application/usecase/update_budget.go`
- [ ] `internal/budget/application/usecase/delete_budget.go`

### Opcionais (se criar operações compostas no futuro):

- [ ] `internal/user/application/usecase/create_user.go` (se criar conta junto)
- [ ] `internal/category/application/usecase/*` (se tiver operações múltiplas)

## ⚠️ Pontos de Atenção

### 1. Não fazer operações longas dentro de transação

```go
// ❌ MAU
uc.uow.Do(ctx, func(ctx context.Context, tx database.DBExecutor) error {
    // Chamar API externa (5 segundos)
    externalData := callExternalAPI()

    // Processar dados (10 segundos)
    processedData := heavyProcessing(externalData)

    // Inserir (transação fica aberta por 15+ segundos!)
    return repo.Create(ctx, tx, processedData)
})

// ✅ BOM
// Fazer operações lentas FORA da transação
externalData := callExternalAPI()
processedData := heavyProcessing(externalData)

// Transação rápida (apenas writes)
uc.uow.Do(ctx, func(ctx context.Context, tx database.DBExecutor) error {
    return repo.Create(ctx, tx, processedData)
})
```

### 2. Validações ANTES da transação

```go
// ✅ BOM
func (uc *CreateBudgetUseCase) Execute(ctx context.Context, input *dtos.CreateBudgetInput) error {
    // Validações primeiro (FORA da transação)
    if input.Income <= 0 {
        return errors.New("income must be positive")
    }

    budget, err := factories.NewBudget(...) // Validações de domínio
    if err != nil {
        return err
    }

    // Só abrir transação após todas validações passarem
    return uc.uow.Do(ctx, func(ctx context.Context, tx database.DBExecutor) error {
        // Apenas operações de banco aqui
        return uc.budgetRepository.Create(ctx, tx, budget)
    })
}
```

### 3. Timeouts apropriados

```go
// Para operações API (rápidas)
uow := uow.NewUnitOfWorkWithOptions(db, sql.LevelReadCommitted, 10*time.Second)

// Para batch/background jobs (lentas)
uow := uow.NewUnitOfWorkWithOptions(db, sql.LevelReadCommitted, 5*time.Minute)
```

## 🚀 Próximos Passos (Opcional)

Após implementação básica funcionar, considere:

1. **Adicionar Retry Logic** (para deadlocks)
2. **Adicionar Metrics** (duração de transações, rollback rate)
3. **Migrar para Uber FX** (quando time estiver confortável)

## 📚 Recursos

- [README.md](README.md) - Documentação completa
- [example_app/](example_app/) - Aplicação completa de exemplo
- [MIGRATION_GUIDE.md](MIGRATION_GUIDE.md) - Migração para Uber FX (opcional)
- [fx_advanced.go](fx_advanced.go) - Padrões avançados

---

**Tempo estimado de implementação**: 15-30 minutos
**Complexidade**: Baixa
**Risco**: Muito baixo (apenas adiciona atomicidade, não quebra código existente)
