# Integration Tests for Unit of Work

## Overview

The Unit of Work package includes two test suites:

### 1. Unit Tests (uow_test.go)
- **Uses**: SQLite in-memory database
- **Purpose**: Fast, isolated testing of UnitOfWork logic
- **Tests**: Commit, rollback, panic recovery, context handling
- **Run**: `go test ./pkg/database/uow`
- **Speed**: ~100-500ms
- **CI**: Runs on every commit

### 2. Integration Tests (uow_integration_test.go)
- **Uses**: Real PostgreSQL via testcontainers
- **Purpose**: Test actual PostgreSQL behavior (MVCC, deadlocks, SSI)
- **Tests**: PostgreSQL-specific isolation levels, serialization, context cancellation
- **Run**: `go test -tags=integration ./pkg/database/uow`
- **Speed**: ~10-30s (includes container startup)
- **CI**: Runs on PRs and releases

## Why Two Test Suites?

SQLite and PostgreSQL have significant differences:

| Feature | SQLite | PostgreSQL |
|---------|--------|------------|
| Concurrency | File locks | MVCC (Multi-Version Concurrency Control) |
| Isolation | Basic | True SSI (Serializable Snapshot Isolation) |
| Deadlocks | Rare | Common under high concurrency |
| Context Cancellation | Limited | Full wire protocol support |
| Errors | Generic "locked" | Specific: deadlock, serialization failure |

**Unit tests** verify UnitOfWork **logic** is correct.
**Integration tests** verify it works correctly with **PostgreSQL in production**.

## Running Integration Tests

### Prerequisites

1. **Docker** must be installed and running
   ```bash
   docker --version
   ```

2. **Install dependencies**
   ```bash
   go get github.com/testcontainers/testcontainers-go
   go get github.com/testcontainers/testcontainers-go/modules/postgres
   ```

### Local Development

```bash
# Run only unit tests (fast)
go test ./pkg/database/uow

# Run integration tests (slow, requires Docker)
go test -tags=integration -v ./pkg/database/uow

# Run both
go test -tags=integration -v ./pkg/database/uow && go test -v ./pkg/database/uow
```

### CI/CD Integration

#### GitHub Actions
```yaml
name: Integration Tests

on: [pull_request]

jobs:
  integration-tests:
    runs-on: ubuntu-latest

    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Run Integration Tests
        run: |
          go test -tags=integration -v ./pkg/database/uow
```

#### GitLab CI
```yaml
integration-tests:
  image: golang:1.21
  services:
    - docker:dind
  variables:
    DOCKER_HOST: tcp://docker:2375
    DOCKER_TLS_CERTDIR: ""
  script:
    - go test -tags=integration -v ./pkg/database/uow
```

## Test Coverage

### Unit Tests (SQLite)
✅ Successful commit
✅ Rollback on error
✅ Panic recovery
✅ Panic recovery with rollback
✅ Concurrent transactions (limited by SQLite)
✅ Context cancellation
✅ Isolation level configuration
✅ Read-only transactions

### Integration Tests (PostgreSQL)
✅ Successful commit (PostgreSQL MVCC)
✅ Rollback on error
✅ Panic recovery with rollback
✅ Context cancellation (wire protocol)
✅ Context cancellation during execution
✅ Serializable isolation level (true SSI)
✅ Read-only transaction enforcement

## Troubleshooting

### "Cannot connect to Docker daemon"
```bash
# Ensure Docker is running
systemctl start docker  # Linux
# or open Docker Desktop on macOS/Windows
```

### "Container startup timeout"
```bash
# Increase timeout in test
testcontainers.WithWaitStrategy(
    wait.ForLog("ready").WithStartupTimeout(120*time.Second)
)
```

### Tests pass locally but fail in CI
- Ensure CI has Docker service configured
- Check network policies allow container-to-container communication
- Verify sufficient resources (memory, CPU)

## Best Practices

1. **Run unit tests frequently** during development (fast feedback)
2. **Run integration tests before commits** (catch PostgreSQL-specific issues)
3. **Always run integration tests in CI** for PRs and releases
4. **Don't skip integration tests** - they catch real production bugs

## Contributing

When adding new UnitOfWork features:

1. Add unit test in `uow_test.go` (logic verification)
2. Add integration test in `uow_integration_test.go` (PostgreSQL behavior)
3. Update this README if adding new test scenarios

## Resources

- [testcontainers-go Documentation](https://golang.testcontainers.org/)
- [PostgreSQL Transaction Isolation](https://www.postgresql.org/docs/current/transaction-iso.html)
- [Go Build Tags](https://pkg.go.dev/cmd/go#hdr-Build_constraints)
