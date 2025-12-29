# PostgreSQL Connection Example

This example demonstrates how to use the `pkg/database/postgres` package to connect to a PostgreSQL database.

## Prerequisites

- Go 1.25 or higher
- PostgreSQL server running locally or remotely

## Setup

1. Make sure you have PostgreSQL running:

```bash
# Using Docker
docker run --name postgres-test \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=testdb \
  -p 5432:5432 \
  -d postgres:16
```

2. Run the example:

```bash
go run examples/postgres/main.go
```

## Examples Included

### 1. Basic Connection
Demonstrates a simple connection to PostgreSQL with minimal configuration.

### 2. Production Configuration
Shows a production-ready configuration with:
- Connection pooling
- SSL mode
- Timeouts
- Retry logic
- Custom pool sizes

### 3. Health Check
Illustrates how to perform health checks on the database connection with periodic monitoring.

## Configuration Options

The example covers various configuration options:

- **Connection Settings**: host, port, user, password, database
- **SSL Mode**: SSL connection security
- **Pool Settings**: max open/idle connections, connection lifetime
- **Timeouts**: connection, ping, and query timeouts
- **Retry Logic**: automatic retry on connection failures

## Error Handling

The example demonstrates proper error handling:
- Connection errors
- Query errors
- Health check failures
- Graceful shutdown

## Best Practices

The example follows these best practices:
1. Always use context for operations
2. Defer database closure
3. Handle errors appropriately
4. Check return values
5. Use health checks in production
6. Configure connection pools based on workload
