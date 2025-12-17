package fiberserver

import "time"

const (
	defaultHTTPPort        = "8080"
	defaultReadTimeout     = 15 * time.Second
	defaultWriteTimeout    = 15 * time.Second
	defaultIdleTimeout     = 60 * time.Second
	defaultShutdownTimeout = 30 * time.Second
	defaultBodyLimit       = 4 * 1024 * 1024 // 4MB
	defaultReadBufferSize  = 4096
	defaultWriteBufferSize = 4096
)

var (
	defaultSettings = settings{
		port:            defaultHTTPPort,
		readTimeout:     defaultReadTimeout,
		writeTimeout:    defaultWriteTimeout,
		idleTimeout:     defaultIdleTimeout,
		shutdownTimeout: defaultShutdownTimeout,
		bodyLimit:       defaultBodyLimit,
		readBufferSize:  defaultReadBufferSize,
		writeBufferSize: defaultWriteBufferSize,
		errorHandler:    defaultHandleError,
		prefork:         false,
		strictRouting:   false,
		caseSensitive:   true,
	}
)

type (
	Option   func(s settings) settings
	settings struct {
		port              string
		readTimeout       time.Duration
		writeTimeout      time.Duration
		idleTimeout       time.Duration
		shutdownTimeout   time.Duration
		bodyLimit         int
		readBufferSize    int
		writeBufferSize   int
		routes            []Route
		globalMiddlewares []Middleware
		errorHandler      ErrorHandler
		prefork           bool
		strictRouting     bool
		caseSensitive     bool
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
// Default: 15 seconds
func WithReadTimeout(timeout time.Duration) Option {
	return func(s settings) settings {
		s.readTimeout = timeout
		return s
	}
}

// WithWriteTimeout sets the maximum duration before timing out writes of the response.
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

// WithShutdownTimeout sets the timeout for graceful shutdown.
// Default: 30 seconds
func WithShutdownTimeout(timeout time.Duration) Option {
	return func(s settings) settings {
		s.shutdownTimeout = timeout
		return s
	}
}

// WithBodyLimit sets the maximum allowed size for a request body.
// Default: 4MB
func WithBodyLimit(size int) Option {
	return func(s settings) settings {
		s.bodyLimit = size
		return s
	}
}

// WithReadBufferSize sets the per-connection buffer size for requests.
// Default: 4096
func WithReadBufferSize(size int) Option {
	return func(s settings) settings {
		s.readBufferSize = size
		return s
	}
}

// WithWriteBufferSize sets the per-connection buffer size for responses.
// Default: 4096
func WithWriteBufferSize(size int) Option {
	return func(s settings) settings {
		s.writeBufferSize = size
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

// WithPrefork enables prefork mode for multi-process handling.
// WARNING: Use with care. When enabled, the application will spawn
// multiple Go processes listening on the same port.
// Default: false
func WithPrefork(enabled bool) Option {
	return func(s settings) settings {
		s.prefork = enabled
		return s
	}
}

// WithStrictRouting enables strict routing.
// When enabled, /foo and /foo/ are treated as different routes.
// Default: false
func WithStrictRouting(enabled bool) Option {
	return func(s settings) settings {
		s.strictRouting = enabled
		return s
	}
}

// WithCaseSensitive enables case-sensitive routing.
// When enabled, /Foo and /foo are treated as different routes.
// Default: true
func WithCaseSensitive(enabled bool) Option {
	return func(s settings) settings {
		s.caseSensitive = enabled
		return s
	}
}
