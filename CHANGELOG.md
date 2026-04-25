# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/JailtonJunior94/devkit-go/compare/v0.1.0...HEAD
[v0.1.0]: https://github.com/JailtonJunior94/devkit-go/releases/tag/v0.1.0
