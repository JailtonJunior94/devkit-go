# migration

Safe database migrations for Postgres, MySQL, and CockroachDB.

## Quick Start

```go
migrator, err := migration.New(
    migration.WithDriver(migration.DriverPostgres),
    migration.WithDSN("postgres://user:pass@localhost:5432/mydb?sslmode=disable"),
    migration.WithSource("file://migrations"),
)
if err != nil {
    panic(err)
}
defer migrator.Close()

// Run migrations
if err := migrator.Up(context.Background()); err != nil {
    panic(err)
}
```

## API

```go
// Constructor
New(opts ...Option) (*Migrator, error)

// Options
WithDriver(driver Driver)  // DriverPostgres, DriverMySQL, DriverCockroachDB
WithDSN(dsn string)
WithSource(source string)  // file://path or other sources
WithLogger(logger Logger)

// Methods
Up(ctx context.Context) error
Down(ctx context.Context) error
Steps(ctx context.Context, n int) error
Version() (uint, bool, error)
Close() error
```

## Example: Full Setup

```go
logger := &MyLogger{}  // Implement migration.Logger interface

migrator, err := migration.New(
    migration.WithDriver(migration.DriverPostgres),
    migration.WithDSN(os.Getenv("DATABASE_URL")),
    migration.WithSource("file://./migrations"),
    migration.WithLogger(logger),
    migration.WithLockTimeout(10*time.Second),
)
if err != nil {
    log.Fatal(err)
}
defer migrator.Close()

// Apply all pending migrations
if err := migrator.Up(context.Background()); err != nil {
    log.Fatal(err)
}

// Get current version
version, dirty, err := migrator.Version()
fmt.Printf("Current version: %d, dirty: %v\n", version, dirty)
```

## Migration Files

```
migrations/
├── 000001_create_users_table.up.sql
├── 000001_create_users_table.down.sql
├── 000002_add_email_index.up.sql
└── 000002_add_email_index.down.sql
```

## Best Practices

- Always provide down migrations
- Test migrations on non-production first
- Use transactions where possible
- Version control migration files
