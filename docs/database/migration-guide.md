# Migration Guide — pkg/database v0.3.0

This guide covers the breaking changes introduced in `v0.3.0` of `pkg/database` and provides before/after examples for each affected area. The changes affect consumers of:

- `pkg/database/pgxpool_manager` (removed)
- `pkg/database/postgres_otelsql` (removed)
- `pkg/migration` (removed)
- `pkg/database` (`DBTX` and `Result` contract changed)
- `pkg/database/uow` (generic signature changed)

**Reference:** [CHANGELOG.md](../../CHANGELOG.md) · [techspec.md](../../tasks/prd-database-manager-uow-refactor/techspec.md)

---

## 1. Replacing `pgxpool_manager` with `manager.New(PostgresConfig{...})`

### Before

```go
import "github.com/JailtonJunior94/devkit-go/pkg/database/pgxpool_manager"

mgr, err := pgxpool_manager.New(pgxpool_manager.Config{
    DSN:          "postgres://user:pass@localhost:5432/mydb",
    MaxOpenConns: 20,
})
```

### After

```go
import (
    "github.com/JailtonJunior94/devkit-go/pkg/database/manager"
    "github.com/JailtonJunior94/devkit-go/pkg/database/postgres"
)

mgr, err := manager.New(postgres.PostgresConfig{
    DSN:          "postgres://user:pass@localhost:5432/mydb",
    MaxOpenConns: 20,
})
```

**Key differences:**
- `pgxpool_manager.Config` → `postgres.PostgresConfig` (fields are identical).
- `pgxpool_manager.New` → `manager.New` (returns the `Manager` interface, not a concrete type).
- `manager.New` validates the config eagerly and returns an error on invalid or missing fields.
- The initial `Ping` (5 s timeout) now happens inside the adapter constructor called by `manager.New`.

---

## 2. Replacing `postgres_otelsql`

`postgres_otelsql` provided a Postgres pool with OTel instrumentation via `otelsql`. It is replaced by the Postgres adapter inside `manager`, which uses `pgx/v5` natively with OTel spans emitted through `pkg/observability`.

### Before

```go
import "github.com/JailtonJunior94/devkit-go/pkg/database/postgres_otelsql"

pool, err := postgres_otelsql.New(postgres_otelsql.Config{
    DSN: dsn,
}, obs)
// pool exposed *pgxpool.Pool or a similar concrete type
```

### After

```go
mgr, err := manager.New(
    postgres.PostgresConfig{DSN: dsn},
    manager.WithObservability(obs),
)
// mgr exposes Manager interface; spans are emitted automatically
```

---

## 3. Replacing `pkg/migration` with `pkg/database/migration`

The top-level `pkg/migration` package is removed. Use `pkg/database/migration` instead.

### Before

```go
import "github.com/JailtonJunior94/devkit-go/pkg/migration"

m, err := migration.New(migration.Config{
    DSN:    dsn,
    Source: "./migrations",
    Driver: migration.DriverPostgres,
})
if err != nil { log.Fatal(err) }
if err := m.Up(); err != nil { log.Fatal(err) }
```

### After

```go
import (
    "errors"
    "github.com/JailtonJunior94/devkit-go/pkg/database/migration"
)

migrator, err := migration.New(
    mgr,
    migration.FSPath("./migrations/postgres"),
    migration.WithDSN(dsn),
)
if err != nil { log.Fatal(err) }

if err := migrator.Up(ctx); err != nil && !errors.Is(err, migration.ErrNoChange) {
    log.Fatalf("migration failed: %v", err)
}
```

**Key differences:**
- `migration.New` now receives a `manager.Manager` as first argument.
- The source is typed: `migration.FSPath("./path")` or `migration.EmbedFS{FS: fs, Root: "dir"}`.
- All operations receive a `context.Context`.
- `ErrNoChange` is a wrapped sentinel from the `migration` package (not golang-migrate); check it with `errors.Is`.

---

## 4. DBTX interface changes

`database.DBTX` is now a pure interface based on standard Go types, removing all `pgx`-specific types from the public contract.

### Before

```go
// DBTX was previously coupled to pgx types or database/sql types
type DBTX interface {
    Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
    Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
    QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
```

### After

```go
// Standard-library-friendly contract; works for pgx, database/sql and mocks
type DBTX interface {
    ExecContext(ctx context.Context, query string, args ...any) (database.Result, error)
    QueryContext(ctx context.Context, query string, args ...any) (database.Rows, error)
    QueryRowContext(ctx context.Context, query string, args ...any) database.Row
}
```

`PrepareContext` was removed from the public `DBTX` contract. Consumers that previously prepared statements directly through `database.DBTX` must move that logic behind a driver-specific adapter or execute parametrized queries through `ExecContext` / `QueryContext`. Prepared statements remain enabled by default inside the supported drivers; the explicit prepare API is no longer part of the driver-agnostic surface.

**Update your repositories:**

```go
// Before (pgx-specific)
func (r *repo) Insert(ctx context.Context, db pgx.Tx, name string) error {
    _, err := db.Exec(ctx, "INSERT INTO items(name) VALUES($1)", name)
    return err
}

// After (driver-agnostic)
func (r *repo) Insert(ctx context.Context, db database.DBTX, name string) error {
    _, err := db.ExecContext(ctx, "INSERT INTO items(name) VALUES($1)", name)
    return err
}
```

---

## 5. `Result.LastInsertId` removed

`database.Result` no longer exposes `LastInsertId`. This method was unreliable for Postgres (always returned 0) and is not supported by `pgx/v5`.

### Before

```go
res, _ := db.ExecContext(ctx, "INSERT INTO items(name) VALUES($1)", name)
id, _ := res.LastInsertId() // unreliable with Postgres
```

### After

Use `RETURNING id` for Postgres and CockroachDB:

```go
row := db.QueryRowContext(ctx,
    "INSERT INTO items(name) VALUES($1) RETURNING id", name)
var id int64
if err := row.Scan(&id); err != nil {
    return 0, err
}
```

For MySQL/MSSQL, `RowsAffected()` remains available via `database.Result`. `LastInsertId` is available via a separate query if needed.

---

## 6. Unit of Work — generic signature

`UoW` is now generic (`UnitOfWork[T]`); the return type of `Do` is determined by the type parameter.

### Before

```go
import "github.com/JailtonJunior94/devkit-go/pkg/database/uow"

u := uow.New(mgr)
err := u.Do(ctx, func(ctx context.Context, tx database.DBTX) error {
    _, err := tx.ExecContext(ctx, "INSERT INTO events(name) VALUES($1)", "x")
    return err
})
```

### After

```go
// Typed result:
u := uow.New[MyResult](mgr)
result, err := u.Do(ctx, func(ctx context.Context, tx database.DBTX) (MyResult, error) {
    // ...
    return MyResult{...}, nil
})

// No result (void):
u := uow.NewVoid(mgr)
_, err := u.Do(ctx, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
    _, err := tx.ExecContext(ctx, "INSERT INTO events(name) VALUES($1)", "x")
    return struct{}{}, err
})
```

**Key differences:**
- `Do` now returns `(T, error)` instead of just `error`.
- Use `uow.NewVoid(mgr)` as a convenience alias for `uow.New[struct{}](mgr)`.
- Isolation level and read-only mode are configured at construction time via `WithIsolation` and `WithReadOnly`.

---

## Summary of Removed Packages

| Removed | Replacement |
|---------|-------------|
| `pkg/database/pgxpool_manager` | `pkg/database/manager` + `pkg/database/postgres` |
| `pkg/database/postgres_otelsql` | `pkg/database/manager` + `manager.WithObservability(obs)` |
| `pkg/migration` | `pkg/database/migration` |
