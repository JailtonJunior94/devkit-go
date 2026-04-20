# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `pkg/nullable`: nullable value objects for int, int64, float32, float64, string and time

### Changed
- `pkg/observability`: extend Span interface with TraceID/SpanID/IsSampled; replace closure SpanOptions with concrete types
- `pkg/observability`: fix race conditions, add Field union type and pools

### Deprecated
- `pkg/httpserver`: use `pkg/http_server/chi_server` or `pkg/http_server/server_fiber` instead
- `pkg/logger`: use `pkg/observability` Logger instead

## [v2.0.0] - 2025-12-26

### Changed
- Breaking: module upgraded to v2 (`github.com/JailtonJunior94/devkit-go/v2`)
- Metrics improvements
- Module reorganization

## [v1.7.8] - 2026-01-26

### Changed
- Observability upgrades
- Unit of Work refactoring
- `pkg/linq` and `pkg/vos` improvements

### Added
- `pkg/httpclient`, `pkg/events`, `pkg/database` packages
- Database + OTel integration

### Fixed
- CodeRabbit review corrections

## [v1.7.0] - 2025-12-30

### Added
- RabbitMQ module with full producer/consumer support
- `pkg/vos` value objects
- go-chi HTTP server (`pkg/http_server/chi_server`)
- PostgreSQL support

### Changed
- Fiber, database and Kafka refactoring
- Observability critical and medium issue fixes
- Unit of Work pattern

### Removed
- uber/fx dependency

## [v1.6.0] - 2025-12-20

### Added
- New observability package (`pkg/observability`) with OpenTelemetry

## [v1.5.0] - 2025-12-18

### Added
- Distributed tracing example with order microservices
- HTTP server security features (CORS, security headers, body limit)

### Changed
- Refactored `pkg/observability` with OpenTelemetry abstraction
- Fixed race conditions in HTTP server

## [v1.4.0] - 2025-12-01

### Added
- Initial Kafka consumer/producer (`pkg/messaging/kafka`)
- HTTP server with Fiber (`pkg/http_server/server_fiber`)
- Database migration support (`pkg/migration`)
- Encryption utilities (`pkg/encrypt`)

[Unreleased]: https://github.com/JailtonJunior94/devkit-go/compare/v2.0.0...HEAD
[v2.0.0]: https://github.com/JailtonJunior94/devkit-go/compare/v1.7.8...v2.0.0
[v1.7.8]: https://github.com/JailtonJunior94/devkit-go/compare/v1.7.0...v1.7.8
[v1.7.0]: https://github.com/JailtonJunior94/devkit-go/compare/v1.6.0...v1.7.0
[v1.6.0]: https://github.com/JailtonJunior94/devkit-go/compare/v1.5.0...v1.6.0
[v1.5.0]: https://github.com/JailtonJunior94/devkit-go/compare/v1.4.0...v1.5.0
[v1.4.0]: https://github.com/JailtonJunior94/devkit-go/releases/tag/v1.4.0
