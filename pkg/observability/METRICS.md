# Metrics Catalog

Comprehensive catalog of all available metrics in the observability package and associated components.

## Overview

This document lists all metrics emitted by the devkit-go observability system, including:

- Application metrics (custom business metrics)
- HTTP server metrics (automatic instrumentation)
- Database metrics (via otelsql)
- System runtime metrics (Go runtime)

## Metric Naming Conventions

Metrics follow OpenTelemetry Semantic Conventions:

- **Format**: `<domain>.<component>.<name>`
- **Separator**: `.` (converted to `_` in Prometheus)
- **Units**: Explicitly specified (e.g., `s`, `ms`, `bytes`, `{request}`)
- **Suffixes**:
  - `.total` for counters
  - `.duration` for histograms measuring time
  - `.count` for count metrics

## Label Guidelines

### ✅ Recommended Labels (Low Cardinality)

Use these types of labels in your metrics:

- **Status/State**: `status=success|error|timeout`
- **Type/Category**: `operation_type=read|write`, `user_type=premium|free`
- **HTTP**: `http.method=GET|POST`, `http.status_code=200|404|500`
- **Environment**: `environment=production|staging|dev`
- **Service**: `service_name=user-api|order-api`
- **Component**: `component=database|cache|api`

### ❌ Blocked Labels (High Cardinality)

These labels are automatically blocked when `EnableCardinalityCheck` is enabled:

- `user_id` - Use `user_type` or `user_tier` instead
- `session_id` - Use `session_type` instead
- `trace_id` - Only for traces/logs, not metrics
- `span_id` - Only for traces, not metrics
- `request_id` - Only for logs, not metrics
- `transaction_id` - Use `transaction_type` instead
- `correlation_id` - Use `correlation_type` instead
- `ip_address` - Use `region` or `country` instead
- `email` - Use `email_domain` or `user_type` instead
- `phone` - Never use in metrics
- `uuid` / `guid` - Use categorical alternatives

### Custom Blocked Labels

You can add your own high-cardinality labels to the blocklist:

```go
config := &otel.Config{
    EnableCardinalityCheck: true,
    CustomBlockedLabels: []string{
        "customer_id",
        "order_id",
        "api_key",
    },
}
```

## HTTP Server Metrics

Automatically emitted by HTTP server middleware when `EnableOTelMetrics` is enabled.

### `http.server.duration`

**Type**: Histogram
**Unit**: `s` (seconds)
**Description**: Duration of HTTP server requests

**Labels**:
- `http.method` - HTTP method (GET, POST, PUT, DELETE, etc.)
- `http.route` - Route template (e.g., `/users/:id`, NOT `/users/123`)
- `http.status_code` - HTTP status code (200, 404, 500, etc.)

**Buckets**: Default OpenTelemetry exponential buckets

**Example**:
```
http.server.duration{http.method="GET",http.route="/users/:id",http.status_code="200"} = 0.045
```

**Usage**:
```promql
# P95 latency by route
histogram_quantile(0.95, rate(http_server_duration_bucket[5m]))

# Average request duration
rate(http_server_duration_sum[5m]) / rate(http_server_duration_count[5m])
```

---

### `http.server.request.count`

**Type**: Counter
**Unit**: `{request}`
**Description**: Total number of HTTP requests processed

**Labels**:
- `http.method` - HTTP method (GET, POST, PUT, DELETE, etc.)
- `http.route` - Route template (e.g., `/users/:id`)
- `http.status_code` - HTTP status code (200, 404, 500, etc.)

**Example**:
```
http.server.request.count{http.method="GET",http.route="/users/:id",http.status_code="200"} = 1523
```

**Usage**:
```promql
# Request rate by route
rate(http_server_request_count[5m])

# Error rate (4xx + 5xx)
sum(rate(http_server_request_count{http.status_code=~"4..|5.."}[5m]))
```

---

### `http.server.active_requests`

**Type**: UpDownCounter
**Unit**: `{request}`
**Description**: Number of HTTP requests currently being processed

**Labels**: None (to avoid cardinality issues)

**Example**:
```
http.server.active_requests = 12
```

**Usage**:
```promql
# Current active requests
http_server_active_requests

# Alert on high load
http_server_active_requests > 100
```

---

## Application Metrics

Examples of custom business metrics you can create.

### Counter Example: `orders.created.total`

```go
counter := metrics.Counter(
    "orders.created.total",
    "Total number of orders created",
    "1",
)
counter.Increment(ctx,
    observability.String("status", "success"),
    observability.String("payment_method", "credit_card"),
)
```

**Recommended Labels**:
- `status` - success, failed, cancelled
- `payment_method` - credit_card, paypal, bank_transfer
- `order_type` - standard, express, bulk

**Usage**:
```promql
# Order creation rate
rate(orders_created_total[5m])

# Success rate
rate(orders_created_total{status="success"}[5m])
  / rate(orders_created_total[5m])
```

---

### Histogram Example: `order.processing.duration`

```go
histogram := metrics.Histogram(
    "order.processing.duration",
    "Order processing time",
    "s",
)
histogram.Record(ctx, duration,
    observability.String("complexity", "simple"),
)
```

**Recommended Labels**:
- `complexity` - simple, medium, complex
- `priority` - low, medium, high

**Custom Buckets** (for microsecond precision):

```go
histogram := metrics.HistogramWithBuckets(
    "order.processing.duration",
    "Order processing time",
    "ms",
    []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 5000},
)
```

---

### UpDownCounter Example: `database.connections.active`

```go
connectionCounter := metrics.UpDownCounter(
    "database.connections.active",
    "Active database connections",
    "1",
)

// Connection acquired
connectionCounter.Add(ctx, 1,
    observability.String("database", "postgres"),
)

// Connection released
connectionCounter.Add(ctx, -1,
    observability.String("database", "postgres"),
)
```

---

### Gauge Example: `memory.heap.usage`

```go
err := metrics.Gauge(
    "memory.heap.usage",
    "Current heap memory usage",
    "bytes",
    func(ctx context.Context) float64 {
        var m runtime.MemStats
        runtime.ReadMemStats(&m)
        return float64(m.HeapAlloc)
    },
)
```

**Note**: Gauges are asynchronous and polled by the MeterProvider.

---

## Database Metrics

When using `postgres_otelsql`, the following metrics are automatically emitted:

### `db.client.connections.usage`

**Type**: UpDownCounter
**Unit**: `{connection}`
**Description**: Number of connections in use

**Labels**:
- `db.system` - postgresql, mysql, etc.
- `db.name` - Database name
- `state` - idle, used

---

### `db.client.connections.wait_time`

**Type**: Histogram
**Unit**: `ms`
**Description**: Time spent waiting for a connection

---

## Metric Namespacing

To prevent name collisions in multi-service environments, use the `MetricNamespace` configuration:

```go
config := &otel.Config{
    ServiceName:     "user-api",
    MetricNamespace: "userapi", // All metrics prefixed with "userapi."
}
```

**Result**:
```
userapi.orders.created.total
userapi.http.server.duration
```

---

## Export Configuration

### Export Interval

Control how frequently metrics are pushed to the collector:

```go
config := &otel.Config{
    MetricExportInterval: 30, // Export every 30 seconds (default: 60)
}
```

**Considerations**:
- **Lower interval** (10-30s): More real-time data, higher network overhead
- **Higher interval** (60-120s): Less overhead, delayed visibility

---

## Cardinality Best Practices

### Cardinality Explosion Example (❌ Bad)

```go
counter.Increment(ctx,
    observability.String("user_id", "123456"),      // ❌ Millions of unique values
    observability.String("order_id", "ORD-789"),    // ❌ Unbounded
    observability.String("ip_address", "1.2.3.4"),  // ❌ Thousands of IPs
)
```

**Impact**: Creates millions of time series → Out of memory in Prometheus

### Fixed Version (✅ Good)

```go
counter.Increment(ctx,
    observability.String("user_type", "premium"),    // ✅ Limited values
    observability.String("order_type", "standard"),  // ✅ Categorical
    observability.String("region", "us-east-1"),     // ✅ Bounded
)
```

---

## Cardinality Validation

Enable automatic validation to prevent high-cardinality labels:

```go
config := &otel.Config{
    Environment:            "production",
    EnableCardinalityCheck: true, // Auto-enabled in production
}
```

**Behavior**:
- Blocks metrics with high-cardinality labels
- Silently drops the metric (doesn't crash your app)
- Logs a warning (if logging is enabled)

**Testing**: In development, cardinality check is disabled by default to avoid interrupting workflow.

---

## Prometheus Queries

### Request Rate (QPS)

```promql
# Overall request rate
sum(rate(http_server_request_count[5m]))

# Per route
sum by (http_route) (rate(http_server_request_count[5m]))
```

### Error Rate

```promql
# 5xx error rate
sum(rate(http_server_request_count{http_status_code=~"5.."}[5m]))

# Error percentage
sum(rate(http_server_request_count{http_status_code=~"4..|5.."}[5m]))
  / sum(rate(http_server_request_count[5m])) * 100
```

### Latency Percentiles

```promql
# P50, P95, P99 latency
histogram_quantile(0.50, rate(http_server_duration_bucket[5m]))
histogram_quantile(0.95, rate(http_server_duration_bucket[5m]))
histogram_quantile(0.99, rate(http_server_duration_bucket[5m]))
```

### Active Requests

```promql
# Current load
http_server_active_requests

# Alert if > 100 active requests for 5 minutes
http_server_active_requests > 100
```

---

## Architecture: Metric Flow

```
┌─────────────────────────────────────────────────────────────────┐
│ Application Code                                                │
│   counter.Increment(ctx, labels...)                             │
└─────────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────────┐
│ Cardinality Validation (if enabled)                             │
│   ✓ Check for blocked labels (user_id, session_id, etc.)       │
│   ✗ Drop metric if blocked label found                          │
└─────────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────────┐
│ Namespace Prefix (if configured)                                │
│   "orders.total" → "myapp.orders.total"                         │
└─────────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────────┐
│ OpenTelemetry MeterProvider                                     │
│   Accumulates metrics in memory                                 │
└─────────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────────┐
│ PeriodicReader (every 60s or configured interval)               │
│   Batches and exports metrics via OTLP                          │
└─────────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────────┐
│ OpenTelemetry Collector                                         │
│   Receives: OTLP gRPC/HTTP (port 4317/4318)                     │
│   Processes: Batching, filtering, transformations               │
│   Exports: Prometheus format                                    │
└─────────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────────┐
│ Prometheus Server                                               │
│   Scrapes: http://collector:8889/metrics                        │
│   Stores: Time series data                                      │
└─────────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────────┐
│ Grafana / Visualization                                         │
│   Queries Prometheus                                            │
│   Displays dashboards and alerts                                │
└─────────────────────────────────────────────────────────────────┘
```

---

## Additional Resources

- **OpenTelemetry Semantic Conventions**: https://opentelemetry.io/docs/specs/semconv/
- **Prometheus Best Practices**: https://prometheus.io/docs/practices/naming/
- **Cardinality in Prometheus**: https://prometheus.io/docs/practices/instrumentation/#cardinality

---

## Summary

### Key Takeaways

1. **Always use low-cardinality labels** - Avoid user IDs, session IDs, UUIDs
2. **Enable cardinality validation in production** - Automatic protection
3. **Use metric namespacing** - Prevent collisions in multi-service setups
4. **Monitor cardinality** - Track unique label combinations in Prometheus
5. **Choose appropriate metric types**:
   - **Counter**: Total requests, errors, events
   - **Histogram**: Latencies, sizes, durations
   - **UpDownCounter**: Active connections, queue size
   - **Gauge**: Memory usage, CPU, temperature

### Quick Reference

| Metric Type | When to Use | Example |
|-------------|-------------|---------|
| Counter | Cumulative totals | `requests.total`, `errors.total` |
| Histogram | Distributions | `http.duration`, `request.size` |
| UpDownCounter | Values that go up/down | `connections.active`, `queue.size` |
| Gauge | Current snapshot | `memory.usage`, `cpu.percent` |

### Configuration Checklist

- [x] Set `ServiceName` and `ServiceVersion`
- [x] Configure `MetricNamespace` for multi-service environments
- [x] Enable `EnableCardinalityCheck` in production
- [x] Set appropriate `MetricExportInterval` (default: 60s)
- [x] Configure OTLP endpoint and protocol
- [x] Use low-cardinality labels only
- [x] Test metrics in development before deploying
