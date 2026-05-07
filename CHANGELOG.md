# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Adicionado

- `pkg/http_server/chi_server`: tipos `Handler` / `ErrorHandler` / `Middleware` + `Server.RegisterHandler(method, path, handler, mws...)` para handlers que retornam `error`; `adaptHandler` armazena o path da requisição no contexto para que `ErrorHandler` construa o campo `instance` do RFC 7807 sem precisar de `*http.Request`.
- `pkg/http_server/common/problem.go`: `ProblemFromError` mapeia qualquer `error` para `ProblemDetail` (RFC 7807) sem vazar detalhes internos; `*fiber.Error` tem código/mensagem preservados, demais erros usam HTTP 500 com detail fixo.
- `pkg/http_server/common/request_id.go`: `ValidateRequestID` / `NewRequestID` — contrato compartilhado de X-Request-ID (charset `[A-Za-z0-9._-]`, máx. 128 chars) usado por ambos os adapters chi e fiber.
- `pkg/http_server/common/cors.go`: `ParseOrigins` — valida origens CORS separadas por vírgula com guarda de exclusividade do wildcard.
- `pkg/http_server/common/health.go`: `HealthCheckFunc`, `HealthStatus` e executor paralelo de health checks extraídos para `pkg/http_server/common`.
- `pkg/http_server/common/shutdowner.go`: interface `Shutdowner` para componentes que precisam de shutdown gracioso (providers OTel, registros de métricas, etc.).
- `pkg/http_server/server_fiber`: `defaultErrorHandler` — handler de erro Fiber centralizado com RFC 7807 via `common.ProblemFromError`; o erro original é logado via `pkg/observability` e nunca exposto ao cliente.
- `pkg/http_server/chi_server`: `leakCounter` (`observability.Counter`) e janela `timeoutCleanup` configurável para rastrear goroutines vazadas por timeouts por rota.
- `pkg/http_server/chi_server/mocks/`, `pkg/http_server/server_fiber/mocks/`: mocks gerados via mockery v3.
- `pkg/http_server/README.md`, `pkg/database/README.md`: documentação a nível de pacote com exemplos de uso.
- Benchmarks: `chi_server/server_bench_test.go`, `server_fiber/server_bench_test.go`.
- Teste de compatibilidade legado: `chi_server/server_legacy_test.go`.
- Nova cobertura de testes: `chi_server/lifecycle_test.go`, `chi_server/timeout_test.go`, `chi_server/request_id_test.go`, `server_fiber/server_test.go`.

### Alterado

- `pkg/http_server/chi_server/middleware.go`: `requestIDMiddleware` agora recebe `observability.Observability` e delega validação a `common.ValidateRequestID`; valores de X-Request-ID rejeitados geram warn com `raw_length`, `remote_addr`, `path` e `method` — o valor bruto jamais é logado nem ecoado (R-SEC-001).
- `pkg/http_server/chi_server/server.go`: `RegisterHandler` aplica timeout por rota via `wrapWithTimeout` no momento do registro quando o path tem duração configurada; interface `routePatternSetter` habilita vinculação tardia de rota OTel.
- `pkg/observability/otel/http.go`: `httpRequestScope.SetRoute` permite atualizar o label de rota após o framework finalizar o roteamento; mantém o gauge de requisições ativas balanceado entre mudanças de label e define o atributo de span `http.route`.
- `pkg/http_server/server_fiber`: tratamento de erros centralizado em `error_handler.go`; `errors.go` removido.

### Removido

- `pkg/http_server/server_fiber/errors.go`: substituído por `error_handler.go` com `common.ProblemFromError`.

## [v0.3.0] - 2026-04-28

### Breaking Changes

- **`pkg/httpserver` removed.** Use `pkg/http_server/chi_server` instead. Migration: replace `httpserver.New(...)` with `chiserver.New(...)` and use `Server.RegisterHandler(method, path, handler, mws...)` for handlers that return error.
- **`pkg/database/pgxpool_manager` removed.** Replace with `pkg/database/manager` + `pkg/database/postgres`. See [migration guide](docs/database/migration-guide.md#1-replacing-pgxpool_manager-with-managernewpostgresconfig).
- **`pkg/database/postgres_otelsql` removed.** Replace with `manager.WithObservability(obs)`. See [migration guide](docs/database/migration-guide.md#2-replacing-postgres_otelsql).
- **`pkg/migration` removed.** Replace with `pkg/database/migration`. See [migration guide](docs/database/migration-guide.md#3-replacing-pkgmigration-with-pkgdatabasemigration).
- **`database.DBTX` interface redesigned.** Methods renamed from `Exec`/`Query`/`QueryRow` to `ExecContext`/`QueryContext`/`QueryRowContext`. All methods now accept `context.Context`, and `PrepareContext` is no longer part of the public driver-agnostic contract. See [migration guide](docs/database/migration-guide.md#4-dbtx-interface-changes).
- **`database.Result.LastInsertId` removed.** Use `RETURNING id` for Postgres/CockroachDB. See [migration guide](docs/database/migration-guide.md#5-resultlastinsertid-removed).
- **`uow.UnitOfWork` is now generic (`UnitOfWork[T]`).** `Do` returns `(T, error)` instead of `error`. Use `uow.NewVoid(mgr)` for flows without a typed return. See [migration guide](docs/database/migration-guide.md#6-unit-of-work--generic-signature).

### Added

- `pkg/database/manager`: driver-agnostic `Manager` interface with Factory `New(cfg DriverConfig, opts ...Option)`, lifecycle (`Ping`, `Shutdown`), graceful shutdown with configurable timeout (default 15 s), pool stats scraping every 10 s, and OTel instrumentation via `pkg/observability`.
- `pkg/database/uow`: generic `UnitOfWork[T]` with atomic commit/rollback, panic safety (`recover` → rollback → `panic(r)`), isolation level per call (`WithIsolation`), read-only mode (`WithReadOnly`) and nested transaction guard (`ErrNestedTransaction`).
- `pkg/database/migration`: `Migrator` wrapping `golang-migrate/v4` with `Up`, `Down`, `Force` and `Version` operations, `FSPath` and `EmbedFS` sources, per-operation context timeout and OTel spans.
- `pkg/database/postgres`: native `pgx/v5` adapter with structured config (`PostgresConfig`), DSN precedence, pool defaults (`MaxOpen=25`, `MaxIdle=6`, `ConnMaxLife=30m`) and DSN sanitisation in logs.
- `pkg/database/cockroach`: dedicated CockroachDB adapter (reuses `pgxpool` internally) with calibrated pool defaults (`MaxOpen=50`, `ConnMaxLife=15m`) and `db.system=cockroach` in spans/metrics.
- `pkg/database/mysql`: `database/sql` + `go-sql-driver/mysql` adapter with `MySQLConfig` and pool defaults.
- `pkg/database/mssql`: `database/sql` + `microsoft/go-mssqldb` adapter with `MSSQLConfig`, `DefaultSchema` support and pool defaults.
- `pkg/database/internal/pool`: shared pool utilities (DSN sanitisation, OTel pool stats, per-driver defaults).
- `pkg/database/mocks`: generated mocks for `DBTX`, `Result`, `Rows`, `Row`, `Manager`, `DriverConfig`, `Migrator` and `Source` via mockery v3.
- `pkg/database/manager/README.md`, `pkg/database/uow/README.md`, `pkg/database/migration/README.md`: per-subpackage documentation with usage examples, options tables and error reference.
- `docs/database/migration-guide.md`: before/after migration guide covering all breaking changes.
- `pkg/database/manager/bench_test.go`, `pkg/database/uow/bench_test.go`: benchmarks with absolute thresholds and CI gate (`make bench-check`).
- `Makefile`: added `bench` and `bench-check` targets for running and gating benchmarks.
- `docs/testing/unit_test.md`: canonical test conventions document (framework, structure, naming, mocks).

### Changed

- `pkg/database/db.go` rewritten: `DBTX`, `Result`, `Rows`, `Row` redesigned as standard-library-friendly interfaces; `Driver` constants expanded to include `cockroach`, `mysql`, `mssql`; `WithTx`/`FromContext` helpers for implicit transaction propagation (ADR-004).
- `.mockery.yml`: extended to generate mocks for all `pkg/database` interfaces.
- `Makefile` `mocks-clean`: now also removes mocks under `pkg/database`.

## [v0.2.0] - 2026-04-27

### Added
- `pkg/observability/otel`: instrumentação HTTP framework-neutral (`http.go`, `propagation.go`, `runtime.go`, `shutdown.go`) com métricas de duração, contagem, requisições ativas e erros
- `pkg/observability/otel`: máquina de estados (`runtime`) e coordenador de shutdown idempotente com ordenação configurável de flush
- `pkg/observability/otel`: propagação W3C TraceContext + Baggage com extração/injeção de headers de correlação customizáveis
- `pkg/observability/mocks`: mocks gerados via mockery para `Observability`, `Tracer`, `Logger` e `Metrics`
- `pkg/observability/examples/lgtm-demo`: demo completo com stack LGTM (Loki, Grafana, Tempo, Mimir)
- `pkg/observability/validation`: value objects `BenchmarkBudget`, `PropagationHeaders`, `ShutdownPolicy` e `ServiceDescriptor` com validação e erros sentinela
- `deployment/observability`: stack Docker Compose local com Grafana, Prometheus, Loki, Tempo e OTel Collector
- Testes unitários e de integração para `pgxpool_manager`, `postgres_otelsql`, `chi_server`, `server_fiber` e `httpserver` (legado)

### Changed
- `pkg/observability`: comentários reduzidos e todos traduzidos para PT-BR; removidos comentários que apenas restavam o nome da função/tipo
- `pkg/observability/otel`: logger separado em caminho de produção (sem mutex) e caminho de console (`ConsoleLog=true`), com aviso explícito de p99 sob carga
- `pkg/http_server/chi_server` e `server_fiber`: middlewares de observabilidade atualizados para usar a nova instrumentação HTTP
- `httpserver` package: middlewares e opções de servidor expandidos com testes de cobertura (removido em v0.3.0)

### Removed
- `docs/prompts/setup-ci-release.md`: prompt de setup removido (supersedido pela skill de CI/release)

### Fixed
- `scripts/ci_release`: corrigido o parser da CLI para aceitar o separador `--` usado pelo workflow de release em `go run`, eliminando a falha do passo `Plan release`
- `scripts/ci_release`: corrigida a instabilidade do teste `TestGitCLIIntegration` ao isolar o repositório Git temporário por subtest e evitar contaminação de tags entre cenários paralelos

## [v0.1.0] - 2026-04-24

### Added
- `pkg/observability` com implementacoes `otel`, `noop` e `fake`
- `pkg/database` com `DBTX`, `postgres`, `postgres_otelsql`, `pgxpool_manager` e `uow`
- `pkg/messaging` com adapters para Kafka e RabbitMQ
- `pkg/http_server` com adapters `chi_server` e `server_fiber`
- `httpserver` package legado baseado em Chi (deprecated, removido em v0.3.0)
- `pkg/httpclient`, `pkg/events`, `pkg/migration`, `pkg/encrypt`, `pkg/responses`, `pkg/nullable`, `pkg/linq`, `pkg/vos`, `pkg/entity` e `pkg/logger`
- exemplos, testes unitarios e testes de integracao com `testcontainers-go`

### Changed
- baseline inicial do toolkit consolidado como primeira release oficial
- fluxo de observabilidade e componentes HTTP refinados no estado atual do repositorio

### Deprecated
- `httpserver` package deprecated: usar `pkg/http_server/chi_server` ou `pkg/http_server/server_fiber` (removido em v0.3.0)
- `pkg/logger`: preferir o logger exposto por `pkg/observability`

[v0.3.0]: https://github.com/JailtonJunior94/devkit-go/compare/v0.2.0...v0.3.0
[v0.2.0]: https://github.com/JailtonJunior94/devkit-go/compare/v0.1.0...v0.2.0
[v0.1.0]: https://github.com/JailtonJunior94/devkit-go/releases/tag/v0.1.0
