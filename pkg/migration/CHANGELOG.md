# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial release of migration library
- PostgreSQL driver support with Strategy Pattern
- CockroachDB driver support with optimizations for distributed databases
- Structured logging using `log/slog` (Go standard library)
- Option Pattern for flexible configuration
- Comprehensive error handling with typed errors
- Context-aware operations with timeout support
- Graceful shutdown and resource cleanup
- Thread-safe operations with proper synchronization
- Migration operations: Up, Down, Steps, Version
- Helper functions for error checking (IsNoChangeError, IsDirtyError, IsLockError)
- NoopLogger for testing scenarios
- SlogLogger with JSON output for production
- SlogTextLogger for development and debugging
- Comprehensive README with usage examples
- Architecture documentation explaining design decisions
- Basic example with Docker setup
- Unit tests for core functionality
- Support for multi-statement migrations
- Configurable timeouts (global, lock, statement)
- Database name extraction from DSN
- Idempotent migrations support

### Design Patterns
- **Strategy Pattern**: Driver-specific behavior encapsulation
- **Option Pattern**: Flexible and backward-compatible configuration
- **Adapter Pattern**: slog integration for logging
- **Dependency Injection**: Testable and decoupled architecture

### Developer Experience
- Clear and actionable error messages
- Extensive documentation and examples
- Type-safe API
- Intuitive method naming
- Sensible defaults
- No `panic`, `log.Fatal`, or `os.Exit` in library code
- Proper resource cleanup with defer-friendly Close()

### Production Readiness
- Resilient to transient failures
- Lock management for concurrent safety
- Dirty state detection and reporting
- Timeout protection against hanging operations
- Structured logging for observability
- Works in applications, CLI tools, Docker, and Kubernetes

### Documentation
- Comprehensive README.md with examples
- ARCHITECTURE.md explaining design decisions
- CHANGELOG.md for version tracking
- Inline code documentation
- Working example in examples/basic/
- Migration file templates

### Testing
- Unit tests for configuration validation
- Error handling tests
- DSN parsing tests
- 10.1% initial test coverage (focusing on critical paths)

### Compatibility
- Go 1.21+ (for log/slog support)
- PostgreSQL 12+
- CockroachDB 22.1+
- Compatible with golang-migrate/migrate v4

### Future Enhancements (Roadmap)
- MySQL/MariaDB driver support
- SQLite driver support
- Additional migration sources (S3, GCS, GitHub)
- Dry-run mode for testing migrations
- Migration rollback to specific version
- Migration plan preview
- Increased test coverage
- Integration tests with testcontainers
- Performance benchmarks

---

## Release Notes

### v0.1.0 - Initial Release (Unreleased)

First public release of the migration library. This version provides a solid foundation for database migrations in Go applications with focus on:

- **Resilience**: Robust error handling and timeout protection
- **Intuitive API**: Clean, type-safe, and well-documented
- **Production-Ready**: Battle-tested patterns from devkit-go
- **Extensible**: Easy to add new drivers and features

Perfect for:
- Greenfield projects needing reliable migrations
- Existing projects wanting to improve migration tooling
- Teams requiring observability in migration processes
- Kubernetes deployments with init containers
- CI/CD pipelines with deterministic behavior

### Breaking Changes

None - initial release.

### Migration Guide

Not applicable - initial release.

### Contributors

- Initial implementation following devkit-go standards
- Architecture designed for long-term maintainability
- Code review ensuring production quality

---

## Versioning Strategy

This library follows Semantic Versioning:

- **MAJOR**: Incompatible API changes
- **MINOR**: New features (backward-compatible)
- **PATCH**: Bug fixes (backward-compatible)

### Stability Guarantee

Once v1.0.0 is released:
- Public API will remain stable within major versions
- Deprecated features will have migration guides
- Breaking changes will be clearly documented

### Pre-1.0 Note

While in 0.x versions:
- API may change between minor versions
- Feedback is welcome for API improvements
- Production use is safe but expect occasional breaking changes

---

## Support

- **Issues**: GitHub Issues
- **Documentation**: README.md and ARCHITECTURE.md
- **Examples**: examples/ directory
- **Questions**: Open a GitHub Discussion

---

## License

MIT License - See LICENSE file for details
