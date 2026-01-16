# httpserver

Production-ready HTTP server with Chi router, graceful shutdown, middleware support, and error handling.

## Quick Start

```go
server := httpserver.New(
    httpserver.WithPort("8080"),
    httpserver.WithReadTimeout(15*time.Second),
)

server.RegisterRoute(httpserver.Route{
    Path:   "/users",
    Method: http.MethodGet,
    Handler: func(w http.ResponseWriter, r *http.Request) error {
        return nil  // Return error, server handles it
    },
})

shutdown := server.Run()
defer shutdown(context.Background())
```

## Features

- **Graceful Shutdown**: Handles SIGINT/SIGTERM
- **Error Handling**: Handlers return errors, processed centrally
- **Middleware Support**: Global and per-route
- **Chi Router**: Full Chi router features
- **Production Defaults**: Timeouts, limits pre-configured

## API

```go
// Constructor
New(options ...Option) Server

// Options
WithPort(port string)
WithReadTimeout(timeout time.Duration)
WithWriteTimeout(timeout time.Duration)
WithMiddlewares(middlewares ...Middleware)
WithErrorHandler(handler ErrorHandler)
WithRoutes(routes ...Route)

// Route
type Route struct {
    Path        string
    Method      string
    Handler     Handler  // Returns error
    Middlewares []Middleware
}

// Handler returns error for centralized error handling
type Handler func(w http.ResponseWriter, req *http.Request) error
```

## Example: Full Setup

```go
server := httpserver.New(
    httpserver.WithPort("8080"),
    httpserver.WithMiddlewares(
        middleware.Logger,
        middleware.Recoverer,
    ),
    httpserver.WithErrorHandler(func(ctx context.Context, w http.ResponseWriter, err error) {
        // Custom error handling
        http.Error(w, err.Error(), http.StatusInternalServerError)
    }),
)

server.RegisterRoute(httpserver.Route{
    Path:   "/health",
    Method: http.MethodGet,
    Handler: func(w http.ResponseWriter, r *http.Request) error {
        w.WriteHeader(http.StatusOK)
        return nil
    },
})

shutdown := server.Run()

// Graceful shutdown
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
<-sigChan

ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
shutdown(ctx)
```

## Related

- `pkg/responses` - HTTP response helpers
- `pkg/observability` - Add tracing middleware
