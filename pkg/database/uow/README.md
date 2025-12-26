# Unit of Work Pattern - Implementação Completa

Implementação robusta do padrão Unit of Work para Go com suporte completo a transações atômicas, isolation levels configuráveis, e integração com Uber FX.

## 📚 Índice

- [Visão Geral](#visão-geral)
- [Instalação](#instalação)
- [Uso Básico](#uso-básico)
- [Integração com Uber FX](#integração-com-uber-fx)
- [Padrões Avançados](#padrões-avançados)
- [Arquivos do Projeto](#arquivos-do-projeto)
- [Exemplos](#exemplos)
- [Boas Práticas](#boas-práticas)
- [FAQ](#faq)

## 🎯 Visão Geral

O Unit of Work (UoW) é um padrão que mantém uma lista de objetos afetados por uma transação de negócio e coordena a escrita de mudanças e a resolução de problemas de concorrência.

### Características

✅ **Transações Atômicas**: Garante que todas as operações são commitadas juntas ou nenhuma é
✅ **Proteção contra Panic**: Rollback automático em caso de panic
✅ **Timeouts Configuráveis**: Proteção contra transações longas
✅ **Isolation Levels**: Suporte a todos os níveis de isolamento
✅ **Detecção de Transações Aninhadas**: Previne bugs comuns
✅ **Context Cancellation**: Respeita cancelamento de contexto
✅ **Thread-Safe**: Seguro para uso concorrente
✅ **Integração com FX**: Suporte completo a dependency injection

### Bancos Suportados

- ✅ PostgreSQL
- ✅ CockroachDB
- ✅ MySQL
- ⚠️ SQL Server (com limitações - veja documentação)

## 📦 Instalação

```bash
go get github.com/jailtonjunior94/financial/pkg/database/uow
go get go.uber.org/fx
```

## 🚀 Uso Básico

### Exemplo Simples

```go
package main

import (
    "context"
    "database/sql"
    "github.com/jailtonjunior94/financial/pkg/database/uow"
    _ "github.com/lib/pq"
)

func main() {
    db, _ := sql.Open("postgres", "postgres://localhost/mydb?sslmode=disable")
    uow := uow.NewUnitOfWork(db)

    err := uow.Do(context.Background(), func(ctx context.Context, tx any) error {
        // Todas as operações aqui são atômicas
        _, err := tx.(database.DBExecutor).ExecContext(ctx,
            "INSERT INTO users (name) VALUES ($1)", "John")
        if err != nil {
            return err // Rollback automático
        }

        _, err = tx.(database.DBExecutor).ExecContext(ctx,
            "INSERT INTO accounts (user_id, balance) VALUES ($1, $2)", 1, 100.0)
        if err != nil {
            return err // Rollback automático
        }

        return nil // Commit automático
    })

    if err != nil {
        log.Fatal(err)
    }
}
```

### Configurando Isolation Level e Timeout

```go
uow := uow.NewUnitOfWorkWithOptions(
    db,
    sql.LevelReadCommitted,  // Isolation level
    10 * time.Second,        // Timeout
)
```

### Usando Opções Customizadas por Transação

```go
opts := &uow.TxOptions{
    Isolation: sql.LevelSerializable,
    ReadOnly:  false,
    Timeout:   30 * time.Second,
}

err := uow.DoWithOptions(ctx, opts, func(ctx context.Context, tx any) error {
    // Sua lógica aqui
    return nil
})
```

## 🔌 Integração com Uber FX

### Setup Básico

```go
package main

import (
    "database/sql"
    "github.com/jailtonjunior94/financial/pkg/database/uow"
    "go.uber.org/fx"
)

func main() {
    app := fx.New(
        // Prover database
        fx.Provide(func() (*sql.DB, error) {
            return sql.Open("postgres", "postgres://localhost/db?sslmode=disable")
        }),

        // Prover Unit of Work
        fx.Provide(uow.NewUnitOfWorkFromDB),

        // Usar em seus serviços
        fx.Invoke(func(uow uow.UnitOfWork) {
            // Use o UoW aqui
        }),
    )

    app.Run()
}
```

### Estrutura Completa (Clean Architecture)

Veja o exemplo completo em [`example_app/main.go`](example_app/main.go) que demonstra:

- Camadas separadas (Domain, Application, Infrastructure, Presentation)
- Repositórios com interface
- Use Cases com lógica de negócio
- HTTP Handlers
- Lifecycle management
- Atomicidade garantida em operações complexas

```bash
cd example_app
go run main.go
```

## 🎓 Padrões Avançados

### 1. Múltiplos Bancos de Dados

```go
// Banco principal (writes)
type PrimaryUoW UnitOfWork

// Réplica (reads)
type ReplicaUoW UnitOfWork

app := fx.New(
    fx.Provide(
        NewPrimaryDB,
        NewReplicaDB,
        NewPrimaryUoW,
        NewReplicaUoW,
    ),

    fx.Invoke(func(writeUoW PrimaryUoW, readUoW ReplicaUoW) {
        // Use writeUoW para writes
        // Use readUoW para reads
    }),
)
```

### 2. Retry Logic

```go
retryableUoW := uow.NewRetryableUoW(baseUoW, uow.RetryConfig{
    MaxAttempts:  3,
    InitialDelay: 100 * time.Millisecond,
    MaxDelay:     1 * time.Second,
    Multiplier:   2.0,
})

err := retryableUoW.DoWithRetry(ctx, func(ctx context.Context, tx any) error {
    // Retry automático em caso de deadlock ou serialization error
    return nil
})
```

### 3. Circuit Breaker

```go
cbUoW := uow.NewCircuitBreakerUoW(baseUoW, uow.CircuitBreakerConfig{
    MaxFailures:  5,
    ResetTimeout: 30 * time.Second,
})

err := cbUoW.Do(ctx, func(ctx context.Context, tx any) error {
    // Circuit abre após 5 falhas consecutivas
    return nil
})

if err != nil && err.Error() == "circuit breaker is open" {
    // Banco temporariamente indisponível
}
```

### 4. Idempotência

```go
idempotentUoW := uow.NewIdempotentUoW(baseUoW, redisStore)

// Mesma operação com mesma chave executa apenas uma vez
err := idempotentUoW.DoIdempotent(ctx, "transfer-123", 1*time.Hour,
    func(ctx context.Context, tx any) error {
        // Transferência executada apenas uma vez mesmo com múltiplas requisições
        return nil
    },
)
```

### 5. Composição Completa

```go
enhancedUoW := uow.NewEnhancedUoW(
    baseUoW,
    retryConfig,
    circuitBreakerConfig,
    metrics,
)

// UoW com retry + circuit breaker + observability
```

Veja [`fx_advanced.go`](fx_advanced.go) para implementações completas.

## 📁 Arquivos do Projeto

```
pkg/database/uow/
├── uow.go                  # Implementação principal do UoW
├── uow_test.go             # Testes completos (95%+ coverage)
├── fx_example.go           # Exemplos de integração com FX
├── fx_advanced.go          # Padrões avançados (retry, circuit breaker, etc)
├── README.md               # Este arquivo
└── example_app/
    ├── main.go             # Aplicação completa de exemplo
    ├── main_test.go        # Testes usando FX
    └── README.md           # Documentação da aplicação
```

## 📖 Exemplos

### Transferência Bancária (Caso de Uso Real)

```go
type TransferUseCase struct {
    uow          uow.UnitOfWork
    accountRepo  AccountRepository
    transferRepo TransferRepository
}

func (uc *TransferUseCase) Execute(ctx context.Context, from, to string, amount float64) error {
    return uc.uow.Do(ctx, func(ctx context.Context, tx database.DBExecutor) error {
        // 1. Buscar contas (com lock pessimista)
        fromAcc, err := uc.accountRepo.FindByID(ctx, tx, from)
        if err != nil {
            return err
        }

        toAcc, err := uc.accountRepo.FindByID(ctx, tx, to)
        if err != nil {
            return err
        }

        // 2. Validar saldo
        if fromAcc.Balance < amount {
            return errors.New("insufficient balance")
        }

        // 3. Debitar origem
        if err := uc.accountRepo.UpdateBalance(ctx, tx, from, fromAcc.Balance - amount); err != nil {
            return err
        }

        // 4. Creditar destino
        if err := uc.accountRepo.UpdateBalance(ctx, tx, to, toAcc.Balance + amount); err != nil {
            return err
        }

        // 5. Registrar transferência
        transfer := &Transfer{ID: uuid.New(), From: from, To: to, Amount: amount}
        if err := uc.transferRepo.Create(ctx, tx, transfer); err != nil {
            return err
        }

        // Se QUALQUER operação falhar, TODAS são revertidas
        return nil // Commit automático
    })
}
```

### Executar Aplicação de Exemplo

```bash
# 1. Subir CockroachDB
make start_minimal

# 2. Executar aplicação
cd pkg/database/uow/example_app
go run main.go

# 3. Testar endpoints
curl http://localhost:8080/health
curl "http://localhost:8080/transfer?from=ACC001&to=ACC002"
```

## ✨ Boas Práticas

### ✅ Faça

1. **Use Read Committed como padrão**
   ```go
   uow := uow.NewUnitOfWorkWithOptions(db, sql.LevelReadCommitted, 10*time.Second)
   ```

2. **Use SELECT FOR UPDATE para locks pessimistas**
   ```go
   row := tx.QueryRowContext(ctx, "SELECT * FROM accounts WHERE id = $1 FOR UPDATE", id)
   ```

3. **Configure timeouts apropriados**
   - API endpoints: 5-10s
   - Background jobs: 60-120s
   - Batch processes: 5-10 min

4. **Configure connection pool adequadamente**
   ```go
   db.SetMaxOpenConns(25)
   db.SetMaxIdleConns(5)
   db.SetConnMaxLifetime(5 * time.Minute)
   ```

5. **Use lifecycle hooks para gerenciar conexões**
   ```go
   lc.Append(fx.Hook{
       OnStart: func(ctx context.Context) error { return db.PingContext(ctx) },
       OnStop:  func(ctx context.Context) error { return db.Close() },
   })
   ```

### ❌ Não Faça

1. **Não use Serializable como padrão**
   - Causa deadlocks frequentes em alta concorrência
   - Performance 10-100x pior

2. **Não faça queries longas dentro de transações**
   ```go
   // ❌ MAU
   uow.Do(ctx, func(ctx context.Context, tx any) error {
       // Query longa que demora 30s
       time.Sleep(30 * time.Second)
       return nil
   })

   // ✅ BOM
   // Faça queries longas FORA da transação
   data := fetchDataFromAPI() // 30s
   uow.Do(ctx, func(ctx context.Context, tx any) error {
       // Apenas operações rápidas aqui
       return tx.ExecContext(ctx, "INSERT ...", data)
   })
   ```

3. **Não aninha transações**
   ```go
   // ❌ MAU - vai retornar erro
   uow.Do(ctx, func(ctx context.Context, tx1 any) error {
       return uow.Do(ctx, func(ctx context.Context, tx2 any) error {
           // Erro: "nested transactions are not allowed"
       })
   })
   ```

4. **Não ignore erros de commit**
   ```go
   // ❌ MAU
   _ = uow.Do(ctx, fn)

   // ✅ BOM
   if err := uow.Do(ctx, fn); err != nil {
       return fmt.Errorf("transaction failed: %w", err)
   }
   ```

## 🧪 Testes

### Executar Testes

```bash
# Testes unitários
go test -v ./pkg/database/uow/

# Com race detector
go test -race -v ./pkg/database/uow/

# Com coverage
go test -cover -v ./pkg/database/uow/

# Coverage HTML
make cover
```

### Estrutura de Testes

Os testes cobrem:
- ✅ Commit bem-sucedido
- ✅ Rollback em caso de erro
- ✅ Rollback em caso de panic
- ✅ Proteção contra transações aninhadas
- ✅ Timeout de transação
- ✅ Cancelamento de contexto
- ✅ Diferentes isolation levels
- ✅ Transações read-only
- ✅ Concorrência (10+ goroutines simultâneas)
- ✅ Atomicidade
- ✅ Proteção contra panic duplo

## ❓ FAQ

### P: Qual isolation level devo usar?

**R:** Para a maioria dos casos, **Read Committed** é o ideal:
- Bom balanço entre performance e segurança
- Baixa taxa de deadlocks
- Use locks pessimistas (SELECT FOR UPDATE) quando necessário

Use **Serializable** apenas quando:
- Precisa prevenir anomalias complexas (write skew, phantom reads)
- Aceita performance reduzida e deadlocks frequentes
- Tem retry logic robusta

### P: Como lidar com deadlocks?

**R:**
1. Use Read Committed + SELECT FOR UPDATE
2. Implemente retry logic (veja `fx_advanced.go`)
3. Sempre adquira locks na mesma ordem
4. Mantenha transações curtas

### P: Posso usar com múltiplas goroutines?

**R:** Sim! O UoW é thread-safe. Cada chamada a `Do()` cria uma transação independente. Veja teste `TestConcurrentTransactions` para exemplo.

### P: Como fazer operações read-only?

**R:**
```go
opts := &uow.TxOptions{ReadOnly: true}
uow.DoWithOptions(ctx, opts, func(ctx context.Context, tx any) error {
    // Apenas SELECTs aqui
    return nil
})
```

### P: O que acontece se eu tiver um panic?

**R:** O UoW recupera o panic, faz rollback automático, e re-lança o panic para não mascarar o erro.

### P: Como adicionar logging/metrics?

**R:** Use o padrão Observable (veja `fx_advanced.go`):
```go
observableUoW := uow.NewObservableUoW(baseUoW, metrics)
```

### P: Funciona com MySQL?

**R:** Sim, mas com ressalvas:
- MySQL InnoDB não suporta Read Uncommitted verdadeiro
- Serializable é implementado como Repeatable Read
- Deadlocks são mais frequentes que Postgres

### P: Como implementar idempotência?

**R:** Use idempotency keys (veja `fx_advanced.go`):
```go
idempotentUoW.DoIdempotent(ctx, "unique-key", ttl, fn)
```

## 📚 Referências

- [Unit of Work Pattern - Martin Fowler](https://martinfowler.com/eaaCatalog/unitOfWork.html)
- [Database Transaction Isolation Levels](https://www.postgresql.org/docs/current/transaction-iso.html)
- [Uber FX Documentation](https://uber-go.github.io/fx/)
- [Go database/sql Package](https://pkg.go.dev/database/sql)

## 📄 Licença

Este código faz parte do projeto Financial e está sob a mesma licença do projeto principal.

## 🤝 Contribuindo

Contribuições são bem-vindas! Por favor:
1. Adicione testes para novas funcionalidades
2. Mantenha cobertura de testes >90%
3. Siga os padrões de código existentes
4. Atualize a documentação

---

**Dúvidas?** Abra uma issue ou consulte os exemplos em `example_app/`.
