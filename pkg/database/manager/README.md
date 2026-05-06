# pkg/database/manager

O pacote `manager` fornece a interface `Manager` e sua Factory (`New`) para gerenciar o ciclo de vida do pool de conexões de banco de dados em múltiplos drivers.

**ADRs:** [ADR-001](../../../tasks/prd-database-manager-uow-refactor/adr-001-driver-base-layer.md) · [ADR-004](../../../tasks/prd-database-manager-uow-refactor/adr-004-tx-propagation.md)  
**Tech Spec:** [techspec.md](../../../tasks/prd-database-manager-uow-refactor/techspec.md)

---

## Visão Geral

O `Manager` abstrai o pool de conexões de banco de dados para Postgres, CockroachDB, MySQL e MSSQL. Os consumidores dependem apenas da interface `Manager`; o adaptador concreto é resolvido pela factory `New` com base no tipo de `DriverConfig` passado.

Quando existe um diretório convencional `./migrations/<driver>`, o `manager.New(...)` executa essas migrações de forma síncrona antes de retornar. Falhas nessa etapa abortam a construção do manager com `database.ErrMigrationFailed`.

```
main                       consumidor (use case / repository)
 │                                    │
 ▼                                    ▼
New(cfg, opts...)  ──────────►  Interface Manager
                                 ├─ Driver() Driver
                                 ├─ DBTX(ctx) DBTX          ← pool ou tx ativa
                                 ├─ BeginTx(ctx, opts) Tx
                                 ├─ Ping(ctx) error
                                 └─ Shutdown(ctx) error
```

---

## Início Rápido

### Postgres (DSN)

```go
import (
    "github.com/JailtonJunior94/devkit-go/pkg/database/manager"
    "github.com/JailtonJunior94/devkit-go/pkg/database/postgres"
)

mgr, err := manager.New(postgres.PostgresConfig{
    DSN: "postgres://user:pass@localhost:5432/mydb?sslmode=disable",
})
if err != nil {
    log.Fatal(err)
}
defer mgr.Shutdown(context.Background())
```

### Postgres (campos estruturados)

```go
mgr, err := manager.New(postgres.PostgresConfig{
    Host:     "localhost",
    Port:     5432,
    User:     "app",
    Password: "secret",
    Database: "mydb",
    SSLMode:  "require",
})
```

Quando ambos `DSN` e campos individuais são definidos, o **DSN tem precedência**.

### CockroachDB

```go
import "github.com/JailtonJunior94/devkit-go/pkg/database/cockroach"

mgr, err := manager.New(cockroach.CockroachConfig{
    DSN: "postgres://root@localhost:26257/defaultdb?sslmode=disable",
})
```

### MySQL

```go
import "github.com/JailtonJunior94/devkit-go/pkg/database/mysql"

mgr, err := manager.New(mysql.MySQLConfig{
    DSN: "app:secret@tcp(localhost:3306)/mydb?parseTime=true",
})
```

### MSSQL

```go
import "github.com/JailtonJunior94/devkit-go/pkg/database/mssql"

mgr, err := manager.New(mssql.MSSQLConfig{
    DSN: "sqlserver://app:secret@localhost:1433?database=mydb",
})
```

`DefaultSchema` em `MSSQLConfig` é aceito por paridade contratual, mas o adapter não executa `ALTER USER` nem muta o principal do banco para aplicá-lo. Em MSSQL, use um login já configurado fora da aplicação ou SQL qualificado com schema.

---

## Opções

| Opção | Padrão | Descrição |
|-------|--------|-----------|
| `WithShutdownTimeout(d)` | 15s | Tempo máximo de espera para o fechamento do pool no `Shutdown`. |
| `WithSQLLogging(true)` | false | Loga consultas SQL no nível debug com parâmetros higienizados. Reverte para `slog.Default()` quando o provedor de observabilidade é noop. |
| `WithObservability(obs)` | noop | Injeta um provedor `observability.Observability` para spans e métricas. |
| `WithReadOnly(true)` | false | Sinaliza que o Manager é usado em modo somente leitura (propagado para o UoW). |
| `WithPoolStatsInterval(d)` | 10s | Intervalo entre as coletas de estatísticas do pool emitidas como gauges OTel. |

```go
mgr, err := manager.New(
    postgres.PostgresConfig{DSN: dsn},
    manager.WithShutdownTimeout(30*time.Second),
    manager.WithObservability(obs),
    manager.WithSQLLogging(false),
)
```

---

## DBTX e Propagação de Transação

`Manager.DBTX(ctx)` retorna a transação ativa carregada pelo `ctx` quando uma existe (propagação implícita ADR-004), ou um `DBTX` baseado no pool caso contrário.

```go
// Execução direta no pool (sem transação):
dbtx := mgr.DBTX(ctx)
_, err := dbtx.ExecContext(ctx, "INSERT INTO events(name) VALUES($1)", "startup")

// Após uow.Do injetar a tx no ctx, a mesma chamada retorna a tx ativa:
// (o UoW lida com isso automaticamente — veja pkg/database/uow)
```

---

## Ciclo de Vida

```go
// inicialização
mgr, err := manager.New(cfg, opts...)
if err != nil { /* erro de configuração ou conexão */ }

if err := mgr.Ping(ctx); err != nil { /* não saudável */ }

// encerramento gracioso (ex: no SIGTERM)
shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
defer cancel()
if err := mgr.Shutdown(shutdownCtx); err != nil {
    // err == database.ErrShutdownTimeout quando o ctx expira antes do pool fechar
}
```

O `Shutdown` é idempotente — chamá-lo múltiplas vezes é seguro.  
Após o `Shutdown`, `DBTX(ctx)` retorna um `closedDBTX` que reporta `database.ErrManagerClosed` em cada operação.
`Ping(ctx)` também passa a retornar `database.ErrManagerClosed` após o encerramento, evitando readiness inconsistente.

Se houver migrações em `./migrations/<driver>`, elas rodam dentro do `manager.New(...)` e o manager só fica disponível após a conclusão bem-sucedida desse passo.

---

## Padrões de Pool por Driver

| Driver | MaxOpen | MaxIdle | ConnMaxLife | ConnMaxIdle |
|--------|---------|---------|-------------|-------------|
| Postgres | 25 | 6 | 30m | 5m |
| CockroachDB | 50 | 10 | 15m | 5m |
| MySQL | 20 | 5 | 10m | 5m |
| MSSQL | 20 | 5 | 10m | 5m |

Sobrescreva via os campos da struct de configuração do driver (`MaxOpenConns`, `MaxIdleConns`, `ConnMaxLife`, `ConnMaxIdle`).

---

## Observabilidade

Spans: `db.{driver}.ping`, `db.{driver}.exec`, `db.{driver}.query`, `db.{driver}.query_row`.  
Métricas (prefixo `database.`): `pool.connections_open`, `pool.connections_idle`, `pool.wait_count`, `pool.wait_duration_ms`.

Nenhum DSN, senha ou parâmetro de consulta é escrito nos spans ou logs (R-SEC-001 / R-O11Y-001).

---

## Referência de Erros

| Sentinela | Significado |
|-----------|-------------|
| `database.ErrManagerClosed` | Operação tentada após o `Shutdown`. |
| `database.ErrShutdownTimeout` | O pool não fechou antes do contexto de `Shutdown` expirar. |
| `database.ErrInvalidConfig` | Configuração ausente ou inválida (retornada por `New`). |
