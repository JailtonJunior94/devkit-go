package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
)

type (
	// Server defines the HTTP server interface.
	Server interface {
		// Run starts the server and returns a shutdown function.
		// The shutdown function should be called for graceful shutdown.
		Run() Shutdown
		// RegisterRoute adds a route to the server.
		// Routes can be registered before or after Run() is called.
		// Note: Routes registered after Run() will be available immediately.
		RegisterRoute(route Route)
		// ShutdownListener returns a channel that receives the server's
		// termination error (or nil if shutdown was clean).
		ShutdownListener() chan error
		// ServeHTTP implements http.Handler for testing purposes.
		ServeHTTP(http.ResponseWriter, *http.Request)
	}

	server struct {
		http.Server
		router           *chi.Mux
		shutdownListener chan error
		errorHandler     ErrorHandler
		mu               sync.Mutex
	}

	// Shutdown is a function that gracefully shuts down the server.
	Shutdown func(ctx context.Context) error
	// Middleware is a function that wraps an http.Handler.
	Middleware func(handler http.Handler) http.Handler
	// Handler is a function that handles HTTP requests and may return an error.
	// Errors returned are passed to the ErrorHandler.
	Handler func(w http.ResponseWriter, req *http.Request) error
	// ErrorHandler handles errors returned by route Handlers.
	ErrorHandler func(ctx context.Context, w http.ResponseWriter, err error)

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
//   - ReadHeaderTimeout: 5s
//   - MaxHeaderBytes: 1MB
func New(options ...Option) Server {
	settings := defaultSettings
	for _, option := range options {
		settings = option(settings)
	}

	router := chi.NewRouter()

	srv := &server{
		Server: http.Server{
			Addr:              fmt.Sprintf(":%s", settings.port),
			Handler:           Middlewares(router, settings.globalMiddlewares...),
			ReadTimeout:       settings.readTimeout,
			WriteTimeout:      settings.writeTimeout,
			IdleTimeout:       settings.idleTimeout,
			ReadHeaderTimeout: settings.readHeaderTimeout,
			MaxHeaderBytes:    settings.maxHeaderBytes,
		},
		router:           router,
		shutdownListener: make(chan error, 1),
		errorHandler:     settings.errorHandler,
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

// Run starts the HTTP server in a goroutine and returns a shutdown function.
// The server listens on the configured port and handles incoming requests.
// Use the returned Shutdown function to gracefully stop the server.
func (s *server) Run() Shutdown {
	go func() {
		err := s.Server.ListenAndServe()
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			s.shutdownListener <- nil
			return
		}
		s.shutdownListener <- err
	}()
	return s.Server.Shutdown
}

// ServeHTTP implements http.Handler for testing purposes.
func (s *server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	s.Server.Handler.ServeHTTP(w, req)
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

// Middlewares wraps a handler with the given middlewares.
// Middlewares are applied in reverse order so the first middleware
// in the list is the outermost wrapper.
func Middlewares(main http.Handler, middlewares ...Middleware) http.Handler {
	handler := main
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

// RegisterRoute adds a route to the server.
// This method is thread-safe and can be called after Run().
func (s *server) RegisterRoute(route Route) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.registerRoute(route)
}

// registerRoute registers a route to the chi router.
func (s *server) registerRoute(route Route) {
	s.router.Method(
		route.Method,
		route.Path,
		Middlewares(
			newErrorHandler(s.errorHandler, route.Handler),
			route.Middlewares...,
		),
	)
}

// defaultHandleError is the default error handler.
// It logs the error and returns a 500 Internal Server Error.
func defaultHandleError(ctx context.Context, w http.ResponseWriter, err error) {
	requestID, _ := ctx.Value(ContextKeyRequestID).(string)
	if requestID == "" {
		log.Printf("HTTP handler error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	log.Printf("[%s] HTTP handler error: %v", requestID, err)
	w.WriteHeader(http.StatusInternalServerError)
}

// newErrorHandler wraps a Handler to handle errors using the ErrorHandler.
func newErrorHandler(errorHandler ErrorHandler, handler Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		err := handler(w, req)
		if err == nil {
			return
		}
		errorHandler(req.Context(), w, err)
	})
}

// GetShutdownTimeout returns a context with the default shutdown timeout.
// Useful for graceful shutdown handling.
func GetShutdownTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), defaultShutdownTimeout)
}
