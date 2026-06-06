# Database Toolkit

![devkit-go banner](https://raw.githubusercontent.com/JailtonJunior94/devkit-go/main/assets/banner.png)

O `pkg/database` é um toolkit de banco de dados agnóstico a driver para Go, projetado para oferecer alta performance, atomicidade através do padrão Unit of Work (UoW) e observabilidade profunda integrada ao OpenTelemetry.

## Índice

- [Segurança](#segurança)
- [Contexto](#contexto)
- [Instalação](#instalação)
- [Uso](#uso)
    - [Inicializando o Manager](#inicializando-o-manager)
    - [Postgres / CockroachDB](#postgres--cockroachdb)
    - [MySQL](#mysql)
    - [SQL Server (MSSQL)](#sql-server-mssql)
    - [Unit of Work (UoW)](#unit-of-work-uow)
    - [Migrações de Startup](#migrações-de-startup)
- [Configuração](#configuração)
- [API](#api)
    - [Interface DBTX](#interface-dbtx)
    - [Propagação de Contexto](#propagação-de-contexto)
- [Observabilidade](#observabilidade)
- [Contribuição](#contribuição)
- [Licença](#licença)

## Segurança

Este toolkit implementa práticas recomendadas de segurança:
- **Parametrização**: Incentiva o uso de queries parametrizadas através da interface `DBTX` para prevenir SQL Injection.
- **Log de SQL Sensível**: O log de consultas SQL é desabilitado por padrão e, quando habilitado, pode ser integrado a provedores de log que suportam sanitização de PII.
- **Isolamento de Transação**: Suporte nativo a diferentes níveis de isolamento e modo *Read-Only* para segurança de dados.
- **Panic Recovery**: A Unit of Work garante que pânicos durante a execução de transações resultem em um rollback imediato antes da re-propagação do pânico.
- **DSN nunca logado**: Adapters substituem mensagens de erro do driver por strings genéricas quando o DSN pode conter credenciais.

## O que este pacote NÃO faz

Para evitar surpresas em produção, o escopo é deliberadamente limitado. As responsabilidades abaixo ficam com o caller:

- **Retry de erros transitórios**: o pacote propaga qualquer erro do driver imediatamente. Aplique política de retry (exponential backoff, jitter) na camada de aplicação ou via biblioteca dedicada (ex.: `cenkalti/backoff/v4`).
- **Circuit breaker**: nenhum corte automático é feito quando o pool ou o banco entram em degradação. Use um circuit breaker externo quando relevante.
- **Query builder / ORM**: a interface `DBTX` recebe SQL parametrizado. Não há geração de SQL, mapeamento de structs ou migrations de esquema fora do `pkg/database/migration`.
- **Cache**: nenhum cache de queries ou de pool é fornecido. Caching deve ser explícito no chamador.
- **Failover entre réplicas**: nada de leader/replica routing automático. Configure um pool por destino.

Essas decisões evitam comportamento mágico que mascara falhas reais em produção.

## Contexto

Diferente de ORMs pesados, este pacote foca em fornecer uma abstração leve sobre o `sql.DB` padrão, adicionando recursos críticos para microsserviços modernos como:
1. **Gestão de Ciclo de Vida**: Startup bloqueante com health checks e shutdown gracioso com drain de transações.
2. **Atomicidade**: Uma implementação genérica de Unit of Work que elimina a necessidade de passar transações manualmente entre repositórios.
3. **Telemetria**: Métricas de pool de conexão, duração de queries e traces distribuídos automáticos.

## Instalação

```bash
go get github.com/JailtonJunior94/devkit-go/pkg/database
```

## Uso

### Inicializando o Manager

O `Manager` é o responsável por gerenciar o pool de conexões.

```go
import (
	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/database/postgres"
)

func main() {
	cfg := postgres.PostgresConfig{
		Host: "localhost",
		Port: 5432,
		User: "user",
		Pass: "pass",
		DB:   "dbname",
	}

	mgr, err := manager.New(cfg, 
		manager.WithSQLLogging(true),
		manager.WithShutdownTimeout(20 * time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer mgr.Shutdown(context.Background())
}
```

### Postgres / CockroachDB

Utilize o driver `postgres` ou `cockroach` para bancos baseados em PostgreSQL. Ambos suportam SSL/TLS e pooling avançado.

### MySQL

Configure o driver `mysql` fornecendo a DSN ou os campos estruturados de configuração.

### Unit of Work (UoW)

A Unit of Work permite executar múltiplas operações em uma única transação atômica.

```go
import "github.com/JailtonJunior94/devkit-go/pkg/database/uow"

// Definindo o UoW (geralmente injetado via DI)
uowProcessor := uow.New[string](mgr)

result, err := uowProcessor.Do(ctx, func(ctx context.Context, tx database.DBTX) (string, error) {
	// Use 'tx' ou apenas chame o repositório que lê do contexto
	// mgr.DBTX(ctx) retornará automaticamente a transação ativa
	err := repo.Create(ctx, data)
	if err != nil {
		return "", err
	}
	return "sucesso", nil
})
```

### Migrações de Startup

O toolkit pode executar migrações automaticamente ao iniciar, suportando arquivos SQL locais ou embutidos via `embed.FS`.

```go
// Exemplo com embed.FS
//go:embed migrations/*.sql
var migrationsFS embed.FS

mgr, err := manager.New(cfg, 
	manager.WithStartupMigrationFS(migrationsFS, "migrations"),
)
```

## Configuração

O `manager.New` aceita diversas opções para customizar o comportamento do pool:

| Opção | Descrição | Padrão |
|-------|-----------|---------|
| `WithShutdownTimeout` | Tempo limite para fechar o pool e drenar transações | `15s` |
| `WithSQLLogging` | Habilita log das queries SQL executadas | `false` |
| `WithObservability` | Injeta provedor OTel para métricas e traces | `noop` |
| `WithReadOnly` | Força todas as transações para modo somente leitura | `false` |
| `WithPoolStatsInterval`| Frequência de coleta de métricas do pool | `10s` |

## API

### Interface DBTX

A interface `DBTX` unifica `sql.DB` e `sql.Tx`, permitindo que seus repositórios sejam agnósticos quanto a estar ou não dentro de uma transação.

```go
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) Row
}
```

### Propagação de Contexto

O `devkit-go` utiliza propagação implícita de transação via `context.Context`. O método `mgr.DBTX(ctx)` verifica se existe uma transação ativa no contexto e a retorna; caso contrário, retorna o pool de conexões padrão.

## Observabilidade

As seguintes métricas são exportadas automaticamente se um provedor de observabilidade for fornecido:
- `db.client.connections.usage`: Uso do pool de conexões.
- `db.client.connections.max`: Limite máximo de conexões.
- `database.tx.duration_ms`: Histograma da duração das transações por desfecho (commit/rollback).
- `database.tx.committed`: Contador de transações confirmadas.

## Contribuição

PRs são bem-vindos para novos adaptadores de drivers ou melhorias nos seletores de métricas.

## Licença

[MIT](LICENSE)
