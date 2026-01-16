# database

PostgreSQL database management with connection pooling, Unit of Work pattern, and OpenTelemetry instrumentation.

## Introduction

### Problem It Solves

- Manages database connections with pooling and health checks
- Provides Unit of Work pattern for transactional consistency
- Abstracts *sql.DB and *sql.Tx with single DBTX interface
- Enables repository pattern with transaction support
- Integrates OpenTelemetry for database observability

### When to Use

✅ **Use when:** PostgreSQL backend, need transactions, repository pattern, observability
❌ **Don't use when:** NoSQL databases, simple single-query operations

---

## Architecture

```
database/
├── db.go              # DBTX interface (works with DB and Tx)
├── uow/               # Unit of Work pattern for transactions
├── postgres/          # Basic PostgreSQL connection
├── postgres_otelsql/  # PostgreSQL with OpenTelemetry
└── pgxpool_manager/   # Advanced connection pooling (pgx)
```

### Key Concepts

1. **DBTX Interface**: Unifies *sql.DB and *sql.Tx - repositories work with both
2. **Unit of Work**: Manages transactions - automatic commit/rollback
3. **Connection Pooling**: Managed by postgres managers

---

## API Reference

### DBTX Interface

```go
type DBTX interface {
    PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
    QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
    QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
    ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}
```

### Unit of Work

```go
type UnitOfWork interface {
    Do(ctx context.Context, fn func(ctx context.Context, db DBTX) error) error
}

// Constructor
NewUnitOfWork(db *sql.DB, opts ...UnitOfWorkOption) UnitOfWork

// Options
WithIsolationLevel(level sql.IsolationLevel) UnitOfWorkOption
WithReadOnly(readOnly bool) UnitOfWorkOption
```

### PostgreSQL Manager

```go
type Config struct {
    DSN                 string
    MaxOpenConns        int
    MaxIdleConns        int
    ConnMaxLifetime     time.Duration
    ConnMaxIdleTime     time.Duration
}

// Basic manager
postgres.NewDatabaseManager(ctx context.Context, config *Config) (Manager, error)

// With OpenTelemetry
postgres_otelsql.NewDatabaseManager(ctx context.Context, config *Config, serviceName string) (Manager, error)

type Manager interface {
    DB() *sql.DB
    Close() error
    Ping(ctx context.Context) error
}
```

---

## Examples

### Basic Setup

```go
config := &postgres.Config{
    DSN:             "postgres://user:pass@localhost:5432/mydb?sslmode=disable",
    MaxOpenConns:    25,
    MaxIdleConns:    5,
    ConnMaxLifetime: 5 * time.Minute,
}

manager, err := postgres.NewDatabaseManager(ctx, config)
if err != nil {
    panic(err)
}
defer manager.Close()

db := manager.DB()
```

### Repository Pattern with DBTX

```go
type UserRepository struct {
    db database.DBTX  // Works with *sql.DB or *sql.Tx
}

func NewUserRepository(db database.DBTX) *UserRepository {
    return &UserRepository{db: db}
}

func (r *UserRepository) FindByID(ctx context.Context, id string) (*User, error) {
    var user User
    err := r.db.QueryRowContext(ctx, 
        "SELECT id, name, email FROM users WHERE id = $1", id,
    ).Scan(&user.ID, &user.Name, &user.Email)
    return &user, err
}

func (r *UserRepository) Save(ctx context.Context, user *User) error {
    _, err := r.db.ExecContext(ctx,
        "INSERT INTO users (id, name, email) VALUES ($1, $2, $3)",
        user.ID, user.Name, user.Email,
    )
    return err
}

// Usage without transaction
repo := NewUserRepository(manager.DB())
user, _ := repo.FindByID(ctx, "123")

// Usage with transaction (same repository code!)
uow := uow.NewUnitOfWork(manager.DB())
uow.Do(ctx, func(ctx context.Context, tx database.DBTX) error {
    repo := NewUserRepository(tx)  // Pass transaction
    return repo.Save(ctx, user)
})
```

### Unit of Work: Automatic Transactions

```go
uow := uow.NewUnitOfWork(manager.DB())

err := uow.Do(ctx, func(ctx context.Context, tx database.DBTX) error {
    userRepo := NewUserRepository(tx)
    orderRepo := NewOrderRepository(tx)

    // All operations in same transaction
    if err := userRepo.Save(ctx, user); err != nil {
        return err  // Automatic rollback
    }

    if err := orderRepo.Create(ctx, order); err != nil {
        return err  // Automatic rollback
    }

    return nil  // Automatic commit
})
```

### Custom Isolation Level

```go
uow := uow.NewUnitOfWork(
    manager.DB(),
    uow.WithIsolationLevel(sql.LevelSerializable),
)

err := uow.Do(ctx, func(ctx context.Context, tx database.DBTX) error {
    // Operations run with Serializable isolation
    return nil
})
```

### Read-Only Transaction

```go
uow := uow.NewUnitOfWork(
    manager.DB(),
    uow.WithReadOnly(true),
)

err := uow.Do(ctx, func(ctx context.Context, tx database.DBTX) error {
    // Read operations only
    users, err := userRepo.FindAll(ctx, tx)
    return err
})
```

### With OpenTelemetry

```go
// Automatically instruments all queries
manager, err := postgres_otelsql.NewDatabaseManager(
    ctx,
    config,
    "my-service",  // Service name for traces
)

// All queries will have traces with timing, SQL, and parameters
db := manager.DB()
db.QueryContext(ctx, "SELECT * FROM users WHERE id = $1", id)
```

---

## Best Practices

### 1. Always Use Unit of Work for Multi-Operation Transactions

```go
// ✅ Good: Atomic
uow.Do(ctx, func(ctx context.Context, tx database.DBTX) error {
    userRepo.Save(ctx, tx, user)
    orderRepo.Create(ctx, tx, order)
    return nil  // Both succeed or both fail
})

// ❌ Bad: Not atomic (race conditions possible)
userRepo.Save(ctx, db, user)
orderRepo.Create(ctx, db, order)
```

### 2. Pass DBTX to Repositories

```go
// ✅ Good: Flexible (works with DB or Tx)
func NewUserRepository(db database.DBTX) *UserRepository {
    return &UserRepository{db: db}
}

// ❌ Bad: Forces non-transactional use
func NewUserRepository(db *sql.DB) *UserRepository {
    return &UserRepository{db: db}
}
```

### 3. Check Context Before Long Operations

```go
func (r *UserRepository) FindAll(ctx context.Context) ([]*User, error) {
    if err := ctx.Err(); err != nil {
        return nil, err
    }

    rows, err := r.db.QueryContext(ctx, "SELECT * FROM users")
    // ...
}
```

---

## Caveats

**UoW Context Cancellation**: sql.Commit() doesn't accept context, so commit cannot be cancelled.

**Connection Limits**: Set MaxOpenConns based on database capacity (default: 25).

**Panic on Nil DB**: NewUnitOfWork panics if db is nil (programming error).

---

## Related Packages

- `pkg/migration` - Database migrations
- `pkg/observability` - Add to repositories for logging
- `pkg/vos` - Use for entity IDs and nullable types
