package httpserver

import "time"

const (
	defaultHTTPPort        = "8080"
	defaultReadTimeout     = 15 * time.Second
	defaultWriteTimeout    = 15 * time.Second
	defaultIdleTimeout     = 60 * time.Second
	defaultReadHeaderTime  = 5 * time.Second
	defaultMaxHeaderBytes  = 1 << 20 // 1MB
	defaultShutdownTimeout = 30 * time.Second
)

var (
	defaultSettings = settings{
		port:              defaultHTTPPort,
		readTimeout:       defaultReadTimeout,
		writeTimeout:      defaultWriteTimeout,
		idleTimeout:       defaultIdleTimeout,
		readHeaderTimeout: defaultReadHeaderTime,
		maxHeaderBytes:    defaultMaxHeaderBytes,
		shutdownTimeout:   defaultShutdownTimeout,
		errorHandler:      defaultHandleError,
	}
)

type (
	Option   func(s settings) settings
	settings struct {
		port              string
		readTimeout       time.Duration
		writeTimeout      time.Duration
		idleTimeout       time.Duration
		readHeaderTimeout time.Duration
		maxHeaderBytes    int
		shutdownTimeout   time.Duration
		routes            []Route
		globalMiddlewares []Middleware
		errorHandler      ErrorHandler
	}
)

// WithPort sets the server port.
// Default: "8080"
func WithPort(port string) Option {
	return func(s settings) settings {
		s.port = port
		return s
	}
}

// WithReadTimeout sets the maximum duration for reading the entire request.
// This includes the body. A zero or negative value means no timeout.
// Default: 15 seconds
func WithReadTimeout(timeout time.Duration) Option {
	return func(s settings) settings {
		s.readTimeout = timeout
		return s
	}
}

// WithWriteTimeout sets the maximum duration before timing out writes of the response.
// A zero or negative value means no timeout.
// Default: 15 seconds
func WithWriteTimeout(timeout time.Duration) Option {
	return func(s settings) settings {
		s.writeTimeout = timeout
		return s
	}
}

// WithIdleTimeout sets the maximum amount of time to wait for the next request
// when keep-alives are enabled.
// Default: 60 seconds
func WithIdleTimeout(timeout time.Duration) Option {
	return func(s settings) settings {
		s.idleTimeout = timeout
		return s
	}
}

// WithReadHeaderTimeout sets the amount of time allowed to read request headers.
// Default: 5 seconds
func WithReadHeaderTimeout(timeout time.Duration) Option {
	return func(s settings) settings {
		s.readHeaderTimeout = timeout
		return s
	}
}

// WithMaxHeaderBytes sets the maximum size of request headers.
// Default: 1MB (1 << 20)
func WithMaxHeaderBytes(size int) Option {
	return func(s settings) settings {
		s.maxHeaderBytes = size
		return s
	}
}

// WithShutdownTimeout sets the timeout for graceful shutdown.
// Default: 30 seconds
func WithShutdownTimeout(timeout time.Duration) Option {
	return func(s settings) settings {
		s.shutdownTimeout = timeout
		return s
	}
}

// WithRoutes adds routes to the server.
// Routes must be added before calling Run().
func WithRoutes(routes ...Route) Option {
	return func(s settings) settings {
		s.routes = append(s.routes, routes...)
		return s
	}
}

// WithMiddlewares adds global middlewares that apply to all routes.
// Middlewares are executed in the order they are added.
func WithMiddlewares(middlewares ...Middleware) Option {
	return func(s settings) settings {
		s.globalMiddlewares = append(s.globalMiddlewares, middlewares...)
		return s
	}
}

// WithErrorHandler sets a custom error handler for route errors.
// The handler is called when a route Handler returns a non-nil error.
func WithErrorHandler(handler ErrorHandler) Option {
	return func(s settings) settings {
		s.errorHandler = handler
		return s
	}
}
