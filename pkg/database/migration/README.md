# pkg/database/migration

O pacote `migration` envolve o [golang-migrate/v4](https://github.com/golang-migrate/migrate) para fornecer operações de migração de esquema (`Up`, `Down`, `Force`, `Version`) com fontes de sistema de arquivos e embedded-FS, spans do OpenTelemetry e um contrato de erro limpo.

**ADR:** [ADR-003](../../../tasks/prd-database-manager-uow-refactor/adr-003-migration-locking.md)  
**Tech Spec:** [techspec.md](../../../tasks/prd-database-manager-uow-refactor/techspec.md)

---

## Visão Geral

```
sequência de inicialização
───────────────────────────
main()
 │
 ├─ manager.New(cfg)   → Manager
 │
 ├─ migration.New(mgr, src, opts...)  → Migrator
 │
 └─ migrator.Up(ctx)                  ← bloqueia até completar; aborta em erro
        │
        ▼
    servir HTTP / processar eventos
```

Quando existe um diretório convencional `./migrations/<driver>`, o `manager.New(...)` executa o `Up` automaticamente e de forma bloqueante no startup. Use este pacote explicitamente quando precisar de fonte customizada (`EmbedFS`, outro path) ou de operações administrativas como `Down`, `Force` e `Version`.

---

## Convenção de Nomenclatura de Arquivos

Os arquivos de migração devem seguir o padrão de nomenclatura do golang-migrate:

```
NNNNNN_descricao.up.sql
NNNNNN_descricao.down.sql
```

- `NNNNNN` é um inteiro preenchido com zeros (ex: `000001`).
- Cada arquivo `up` deve ter um arquivo `down` correspondente.
- Os arquivos são aplicados em ordem numérica crescente.

Organize as migrações por driver em diretórios separados:

```
migrations/
├── postgres/
│   ├── 000001_create_users.up.sql
│   └── 000001_create_users.down.sql
├── cockroach/
├── mysql/
└── mssql/
```

---

## Início Rápido

### Fonte FSPath (diretório do sistema de arquivos)

```go
import (
    "errors"
    "github.com/JailtonJunior94/devkit-go/pkg/database/migration"
    "github.com/JailtonJunior94/devkit-go/pkg/database/postgres"
)

mgr, _ := manager.New(postgres.PostgresConfig{DSN: dsn})

migrator, err := migration.New(
    mgr,
    migration.FSPath("./migrations/postgres"),
    migration.WithDSN(dsn),
)
if err != nil {
    log.Fatal(err)
}

if err := migrator.Up(ctx); err != nil && !errors.Is(err, migration.ErrNoChange) {
    log.Fatalf("falha na migração: %v", err)
}
```

### Fonte EmbedFS (migrações incorporadas)

```go
import "embed"

//go:embed migrations/postgres
var pgMigrations embed.FS

migrator, err := migration.New(
    mgr,
    migration.EmbedFS{FS: pgMigrations, Root: "migrations/postgres"},
    migration.WithDSN(dsn),
)
```

`EmbedFS.Root` deve corresponder ao caminho do diretório declarado na diretiva `//go:embed`.

---

## Opções

| Opção | Padrão | Descrição |
|-------|--------|-----------|
| `WithDSN(dsn)` | — | **Obrigatório.** URL do banco de dados usada pelo golang-migrate para conectar. Aceita os esquemas `postgres://`, `postgresql://` e `pgx5://`. |
| `WithMigrationTimeout(d)` | 0 (ctx do chamador) | Timeout por operação via `context.WithTimeout`. Zero usa o prazo do chamador. |
| `WithObservability(obs)` | noop | Injeta um provedor `observability.Observability` para spans de migração. |

---

## Operações

### Up — aplica todas as migrações pendentes

```go
err := migrator.Up(ctx)
if errors.Is(err, migration.ErrNoChange) {
    // o banco de dados já está na versão mais recente — trate como sucesso
}
```

### Down — reverte N migrações

```go
// Reverte as últimas 2 migrações:
err := migrator.Down(ctx, 2)
```

### Force — corrige um estado sujo (dirty state)

Use `Force` para marcar uma versão como aplicada sem executar o SQL, resolvendo um estado de esquema sujo deixado por uma migração que falhou:

```go
// Define a versão como 5, limpando a flag dirty:
err := migrator.Force(ctx, 5)
```

**Runbook — estado sujo (dirty state):**

1. Inspecione a tabela `schema_migrations`: `SELECT * FROM schema_migrations;`
2. Identifique a versão que falhou e limpe manualmente quaisquer alterações parciais.
3. Chame `migrator.Force(ctx, version-1)` para resetar para a última versão conhecida como boa.
4. Execute novamente `migrator.Up(ctx)` para aplicar a migração corrigida.

### Version — inspeciona o estado atual

```go
version, dirty, err := migrator.Version(ctx)
if dirty {
    // migração em estado sujo — precisa de Force
}
```

---

## Comportamento de Bloqueio (Lock)

Os bloqueios de migração são delegados inteiramente ao mecanismo de bloqueio nativo do golang-migrate para cada driver (ADR-003). Nenhum bloqueio adicional em nível de aplicação é implementado. Tentativas de migração concorrentes são tratadas pelo bloqueio consultivo (advisory lock) do driver.

A tabela de rastreamento de migração padrão é `schema_migrations` (padrão do golang-migrate). Nenhum nome de tabela customizado ou renomeação é aplicado (RF-30).

---

## Observabilidade

Spans emitidos por operação:

| Operação | Nome do Span |
|-----------|-----------|
| `Up` | `db.{driver}.migration.up` |
| `Down` | `db.{driver}.migration.down` |
| `Force` | `db.{driver}.migration.force` |

Erros são registrados no span via `RecordError` e o status do span é definido como `Error`. `ErrNoChange` não é registrado como um erro (é uma condição normal).

---

## Referência de Erros

| Sentinela | Significado |
|-----------|-------------|
| `migration.ErrNoChange` | Nenhuma migração pendente; o banco de dados está atualizado. |
| `database.ErrMigrationFailed` | Uma operação de migração falhou; o erro encapsulado contém o erro do golang-migrate. |
| `database.ErrInvalidConfig` | `WithDSN` não foi fornecido ou o DSN está vazio. |
