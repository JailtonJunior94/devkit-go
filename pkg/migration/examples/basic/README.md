# Basic Migration Example

Este exemplo demonstra o uso básico da biblioteca de migrations.

## Pré-requisitos

- PostgreSQL rodando localmente ou via Docker
- Go 1.21+

## Setup do Banco de Dados

### Usando Docker

```bash
docker run --name postgres-dev \
  -e POSTGRES_USER=user \
  -e POSTGRES_PASSWORD=pass \
  -e POSTGRES_DB=mydb \
  -p 5432:5432 \
  -d postgres:16-alpine
```

## Executar o Exemplo

```bash
cd pkg/migration/examples/basic
go run main.go
```

## O que o exemplo faz

1. Cria um logger estruturado (slog) com output no console
2. Configura o migrator com PostgreSQL
3. Executa migrations UP (cria tabelas users e posts)
4. Exibe a versão atual do schema
5. Trata erros gracefully (incluindo "no changes")

## Estrutura de Migrations

```
migrations/
├── 000001_create_users_table.up.sql
├── 000001_create_users_table.down.sql
├── 000002_create_posts_table.up.sql
└── 000002_create_posts_table.down.sql
```

## Output Esperado

```
time=2024-01-15T10:30:00.000Z level=INFO msg="initializing migrator" driver=postgres database=mydb source=file://migrations
time=2024-01-15T10:30:00.100Z level=INFO msg="migrator initialized successfully" database=mydb
Running migrations...
time=2024-01-15T10:30:00.200Z level=INFO msg="starting migration UP" database=mydb
time=2024-01-15T10:30:01.500Z level=INFO msg="migration UP completed successfully" database=mydb current_version=2 dirty=false duration=1.3s
Migration completed successfully! Current version: 2 (dirty: false)
time=2024-01-15T10:30:01.600Z level=INFO msg="closing migrator" database=mydb
time=2024-01-15T10:30:01.650Z level=INFO msg="migrator closed successfully" database=mydb
```

## Rollback (DOWN)

Para fazer rollback das migrations, modifique o main.go:

```go
// Em vez de:
if err := migrator.Up(ctx); err != nil {

// Use:
if err := migrator.Down(ctx); err != nil {
```

## Testando Idempotência

Execute o exemplo múltiplas vezes. Você verá:

```
No migrations to apply - database is up to date
```

Isso demonstra que a biblioteca é idempotente - pode ser executada múltiplas vezes com segurança.
