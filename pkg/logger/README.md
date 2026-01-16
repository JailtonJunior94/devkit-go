# logger

Logger interface and Zap implementation for structured logging.

## Introduction

Provides a simple logging interface with structured field support and a production-ready Zap implementation.

**Note:** For most applications, use `pkg/observability` instead, which includes logging with automatic trace context correlation.

### When to Use

✅ **Use when:** Need standalone logger without observability, legacy code migration
❌ **Consider instead:** `pkg/observability` for full observability stack

---

## API Reference

```go
type Logger interface {
    Info(msg string, fields ...Field)
    Error(msg string, fields ...Field)
    Debug(msg string, fields ...Field)
    Warn(msg string, fields ...Field)
    Fatal(msg string, fields ...Field)
    WithFields(fields ...Field) Logger
}

type Field struct {
    Key   string
    Value any
}

type Level uint

const (
    InfoLevel Level = iota
    ErrorLevel
    WarnLevel
    DebugLevel
)
```

---

## Quick Start

```go
import (
    "github.com/JailtonJunior94/devkit-go/pkg/logger"
    "go.uber.org/zap"
)

func main() {
    // Create Zap logger
    zapLogger, _ := zap.NewProduction()
    logger := logger.NewZapLogger(zapLogger)

    // Simple logging
    logger.Info("Application started")

    // With structured fields
    logger.Info("User created",
        logger.Field{Key: "user_id", Value: "123"},
        logger.Field{Key: "email", Value: "user@example.com"},
    )

    // Create child logger with permanent fields
    requestLogger := logger.WithFields(
        logger.Field{Key: "request_id", Value: "abc-123"},
        logger.Field{Key: "method", Value: "POST"},
    )

    requestLogger.Info("Processing request")  // Includes request_id and method
}
```

---

## Examples

### All Log Levels

```go
logger.Debug("Debugging information")
logger.Info("Informational message")
logger.Warn("Warning message")
logger.Error("Error occurred")
logger.Fatal("Critical error")  // Calls os.Exit(1)
```

### Structured Logging

```go
logger.Info("Order processed",
    logger.Field{Key: "order_id", Value: "ORD-123"},
    logger.Field{Key: "amount", Value: 99.99},
    logger.Field{Key: "currency", Value: "USD"},
    logger.Field{Key: "items", Value: 3},
)
```

### Child Logger

```go
// Parent logger
logger := logger.NewZapLogger(zapLogger)

// Child with additional context
userLogger := logger.WithFields(
    logger.Field{Key: "user_id", Value: "123"},
    logger.Field{Key: "session_id", Value: "sess-456"},
)

// All logs from userLogger include user_id and session_id
userLogger.Info("User logged in")
userLogger.Info("User viewed dashboard")
```

### Development vs Production

```go
// Development: Human-readable console output
zapLogger, _ := zap.NewDevelopment()
logger := logger.NewZapLogger(zapLogger)

// Production: JSON output for log aggregation
zapLogger, _ := zap.NewProduction()
logger := logger.NewZapLogger(zapLogger)
```

### Custom Zap Configuration

```go
config := zap.Config{
    Level:            zap.NewAtomicLevelAt(zap.InfoLevel),
    Encoding:         "json",
    OutputPaths:      []string{"stdout", "/var/log/app.log"},
    ErrorOutputPaths: []string{"stderr"},
    EncoderConfig: zapcore.EncoderConfig{
        TimeKey:        "timestamp",
        LevelKey:       "level",
        MessageKey:     "message",
        EncodeTime:     zapcore.ISO8601TimeEncoder,
        EncodeLevel:    zapcore.LowercaseLevelEncoder,
    },
}

zapLogger, _ := config.Build()
logger := logger.NewZapLogger(zapLogger)
```

---

## Best Practices

### 1. Use Structured Fields

```go
// ✅ Good: Structured, searchable
logger.Info("User login",
    logger.Field{Key: "user_id", Value: userID},
    logger.Field{Key: "ip", Value: ip},
)

// ❌ Bad: Unstructured, hard to query
logger.Info(fmt.Sprintf("User %s logged in from %s", userID, ip))
```

### 2. Create Child Loggers for Context

```go
// ✅ Good: Context preserved
func HandleRequest(logger logger.Logger, requestID string) {
    requestLogger := logger.WithFields(
        logger.Field{Key: "request_id", Value: requestID},
    )
    requestLogger.Info("Processing")
    requestLogger.Info("Completed")
}

// ❌ Bad: Repeating fields
func HandleRequest(logger logger.Logger, requestID string) {
    logger.Info("Processing", logger.Field{Key: "request_id", Value: requestID})
    logger.Info("Completed", logger.Field{Key: "request_id", Value: requestID})
}
```

### 3. Use Appropriate Log Levels

```go
// Debug: Detailed diagnostic info (disabled in production)
logger.Debug("Cache hit", logger.Field{Key: "key", Value: cacheKey})

// Info: General informational messages
logger.Info("Server started", logger.Field{Key: "port", Value: 8080})

// Warn: Warning conditions, not errors
logger.Warn("Rate limit approaching", logger.Field{Key: "current", Value: 950})

// Error: Error conditions
logger.Error("Database connection failed", logger.Field{Key: "error", Value: err})

// Fatal: Unrecoverable errors (exits process!)
logger.Fatal("Configuration missing", logger.Field{Key: "file", Value: configPath})
```

---

## Migration to pkg/observability

If you're using this logger, consider migrating to `pkg/observability` for:
- Automatic trace context in logs
- Distributed tracing correlation
- Metrics integration
- Vendor-neutral observability

```go
// Old: pkg/logger
logger := logger.NewZapLogger(zapLogger)
logger.Info("User created", logger.Field{Key: "user_id", Value: id})

// New: pkg/observability (includes trace context automatically)
obs := otel.NewProvider(ctx, config)
logger := obs.Logger()
logger.Info(ctx, "User created", observability.String("user_id", id))
// Automatically includes trace_id and span_id!
```

---

## Related Packages

- `pkg/observability` - Full observability with tracing (recommended)
- Zap documentation: https://github.com/uber-go/zap
