package fiberserver

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/gofiber/fiber/v2"
)

type (
	// Server defines the HTTP server interface.
	Server interface {
		// Run starts the server and returns a shutdown function.
		// The shutdown function should be called for graceful shutdown.
		Run() Shutdown
		// RegisterRoute adds a route to the server.
		// Routes can be registered before or after Run() is called.
		RegisterRoute(route Route)
		// Group creates a new route group with the given prefix.
		// Useful for organizing routes by version or resource.
		Group(prefix string, middlewares ...Middleware) *RouteGroup
		// ShutdownListener returns a channel that receives the server's
		// termination error (or nil if shutdown was clean).
		ShutdownListener() chan error
		// App returns the underlying Fiber app for testing purposes.
		App() *fiber.App
	}

	server struct {
		app              *fiber.App
		port             string
		shutdownListener chan error
		errorHandler     ErrorHandler
		mu               sync.Mutex
	}

	// Shutdown is a function that gracefully shuts down the server.
	Shutdown func(ctx context.Context) error
	// Middleware is a Fiber middleware handler.
	Middleware func(c *fiber.Ctx) error
	// Handler is a function that handles HTTP requests and may return an error.
	// Errors returned are passed to the ErrorHandler.
	Handler func(c *fiber.Ctx) error
	// ErrorHandler handles errors returned by route Handlers.
	ErrorHandler func(c *fiber.Ctx, err error) error

	// Route defines an HTTP route with its handler and middlewares.
	Route struct {
		Path        string
		Method      string
		Handler     Handler
		Middlewares []Middleware
	}
)

// New creates a new HTTP server with the given options.
// Default configuration:
//   - Port: 8080
//   - ReadTimeout: 15s
//   - WriteTimeout: 15s
//   - IdleTimeout: 60s
//   - BodyLimit: 4MB
func New(options ...Option) Server {
	settings := defaultSettings
	for _, option := range options {
		settings = option(settings)
	}

	app := fiber.New(fiber.Config{
		ReadTimeout:             settings.readTimeout,
		WriteTimeout:            settings.writeTimeout,
		IdleTimeout:             settings.idleTimeout,
		BodyLimit:               settings.bodyLimit,
		ReadBufferSize:          settings.readBufferSize,
		WriteBufferSize:         settings.writeBufferSize,
		Prefork:                 settings.prefork,
		StrictRouting:           settings.strictRouting,
		CaseSensitive:           settings.caseSensitive,
		DisableStartupMessage:   true,
		EnablePrintRoutes:       false,
		DisableDefaultDate:      true,
		DisableHeaderNormalizing: false,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			// Handle Fiber errors (404, etc.)
			var e *fiber.Error
			if errors.As(err, &e) {
				return c.Status(e.Code).JSON(fiber.Map{
					"error": e.Message,
				})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Internal Server Error",
			})
		},
	})

	srv := &server{
		app:              app,
		port:             settings.port,
		shutdownListener: make(chan error, 1),
		errorHandler:     settings.errorHandler,
	}

	// Apply global middlewares
	for _, middleware := range settings.globalMiddlewares {
		app.Use(fiber.Handler(middleware))
	}

	// Register initial routes (no lock needed during initialization)
	for _, route := range settings.routes {
		srv.registerRoute(route)
	}

	return srv
}

// ShutdownListener returns a channel that receives server termination errors.
func (s *server) ShutdownListener() chan error {
	return s.shutdownListener
}

// App returns the underlying Fiber app for testing purposes.
func (s *server) App() *fiber.App {
	return s.app
}

// Run starts the HTTP server in a goroutine and returns a shutdown function.
// The server listens on the configured port and handles incoming requests.
// Use the returned Shutdown function to gracefully stop the server.
func (s *server) Run() Shutdown {
	go func() {
		addr := fmt.Sprintf(":%s", s.port)
		err := s.app.Listen(addr)
		if err == nil {
			s.shutdownListener <- nil
			return
		}
		s.shutdownListener <- err
	}()

	return func(ctx context.Context) error {
		return s.app.ShutdownWithContext(ctx)
	}
}

// NewRoute creates a new Route with the given parameters.
func NewRoute(method, path string, handler Handler, middlewares ...Middleware) Route {
	return Route{
		Path:        path,
		Method:      method,
		Handler:     handler,
		Middlewares: middlewares,
	}
}

// RegisterRoute adds a route to the server.
// This method is thread-safe and can be called after Run().
func (s *server) RegisterRoute(route Route) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.registerRoute(route)
}

// registerRoute registers a route to the Fiber app.
func (s *server) registerRoute(route Route) {
	handlers := make([]fiber.Handler, 0, len(route.Middlewares)+1)

	// Add route-specific middlewares
	for _, middleware := range route.Middlewares {
		handlers = append(handlers, fiber.Handler(middleware))
	}

	// Add the main handler with error handling
	handlers = append(handlers, s.wrapHandler(route.Handler))

	// Register the route with the appropriate method
	switch route.Method {
	case fiber.MethodGet:
		s.app.Get(route.Path, handlers...)
	case fiber.MethodPost:
		s.app.Post(route.Path, handlers...)
	case fiber.MethodPut:
		s.app.Put(route.Path, handlers...)
	case fiber.MethodDelete:
		s.app.Delete(route.Path, handlers...)
	case fiber.MethodPatch:
		s.app.Patch(route.Path, handlers...)
	case fiber.MethodHead:
		s.app.Head(route.Path, handlers...)
	case fiber.MethodOptions:
		s.app.Options(route.Path, handlers...)
	default:
		s.app.Add(route.Method, route.Path, handlers...)
	}
}

// wrapHandler wraps a Handler to handle errors using the ErrorHandler.
func (s *server) wrapHandler(handler Handler) fiber.Handler {
	return func(c *fiber.Ctx) error {
		err := handler(c)
		if err == nil {
			return nil
		}
		return s.errorHandler(c, err)
	}
}

// defaultHandleError is the default error handler.
// It logs the error and returns a 500 Internal Server Error.
func defaultHandleError(c *fiber.Ctx, err error) error {
	requestID := GetRequestID(c)
	if requestID == "" {
		log.Printf("HTTP handler error: %v", err)
	} else {
		log.Printf("[%s] HTTP handler error: %v", requestID, err)
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
		"error": "Internal Server Error",
	})
}

// GetShutdownTimeout returns a context with the default shutdown timeout.
// Useful for graceful shutdown handling.
func GetShutdownTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), defaultShutdownTimeout)
}
