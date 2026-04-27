# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `pkg/observability/README.md`: Documentação técnica detalhada em PT-BR cobrindo Logger, Metrics, Tracing e propagação de contexto em HTTP e Mensageria.
- `README.md`: Documentação raiz do projeto em PT-BR com mapeamento de todos os pacotes do DevKit.
- `scripts/ci_release`: adicionado suporte a flag `release` e `skip_reason` no output JSON. Agora trata erros de tag existente e seção de changelog ausente como "skippable" para evitar falhas no pipeline de CI.

### Changed
- `.github/workflows/ci.yml`: atualizado para utilizar a nova flag `release` e evitar execuções desnecessárias do passo de publicação.

## [v0.2.0] - 2026-04-27

### Added
- `pkg/observability/otel`: instrumentação HTTP framework-neutral (`http.go`, `propagation.go`, `runtime.go`, `shutdown.go`) com métricas de duração, contagem, requisições ativas e erros
- `pkg/observability/otel`: máquina de estados (`runtime`) e coordenador de shutdown idempotente com ordenação configurável de flush
- `pkg/observability/otel`: propagação W3C TraceContext + Baggage com extração/injeção de headers de correlação customizáveis
- `pkg/observability/mocks`: mocks gerados via mockery para `Observability`, `Tracer`, `Logger` e `Metrics`
- `pkg/observability/examples/lgtm-demo`: demo completo com stack LGTM (Loki, Grafana, Tempo, Mimir)
- `pkg/observability/validation`: value objects `BenchmarkBudget`, `PropagationHeaders`, `ShutdownPolicy` e `ServiceDescriptor` com validação e erros sentinela
- `deployment/observability`: stack Docker Compose local com Grafana, Prometheus, Loki, Tempo e OTel Collector
- Testes unitários e de integração para `pgxpool_manager`, `postgres_otelsql`, `chi_server`, `server_fiber` e `httpserver`

### Changed
- `pkg/observability`: comentários reduzidos e todos traduzidos para PT-BR; removidos comentários que apenas restavam o nome da função/tipo
- `pkg/observability/otel`: logger separado em caminho de produção (sem mutex) e caminho de console (`ConsoleLog=true`), com aviso explícito de p99 sob carga
- `pkg/http_server/chi_server` e `server_fiber`: middlewares de observabilidade atualizados para usar a nova instrumentação HTTP
- `pkg/httpserver`: middlewares e opções de servidor expandidos com testes de cobertura

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
- `pkg/httpserver` legado baseado em Chi
- `pkg/httpclient`, `pkg/events`, `pkg/migration`, `pkg/encrypt`, `pkg/responses`, `pkg/nullable`, `pkg/linq`, `pkg/vos`, `pkg/entity` e `pkg/logger`
- exemplos, testes unitarios e testes de integracao com `testcontainers-go`

### Changed
- baseline inicial do toolkit consolidado como primeira release oficial
- fluxo de observabilidade e componentes HTTP refinados no estado atual do repositorio

### Deprecated
- `pkg/httpserver`: preferir `pkg/http_server/chi_server` ou `pkg/http_server/server_fiber`
- `pkg/logger`: preferir o logger exposto por `pkg/observability`

[v0.2.0]: https://github.com/JailtonJunior94/devkit-go/compare/v0.1.0...v0.2.0
[v0.1.0]: https://github.com/JailtonJunior94/devkit-go/releases/tag/v0.1.0
