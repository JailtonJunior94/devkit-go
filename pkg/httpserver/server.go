package httpserver

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type (
	Server interface {
		Run() Shutdown
		RegisterRoute(route Route)
		ShutdownListener() chan error
		ServeHTTP(http.ResponseWriter, *http.Request)
	}

	server struct {
		http.Server
		routes           []Route
		router           *chi.Mux
		shutdownListener chan error
		errorHandler     ErrorHandler
	}

	Shutdown     func(ctx context.Context) error
	Middleware   func(handler http.Handler) http.Handler
	Handler      func(w http.ResponseWriter, req *http.Request) error
	ErrorHandler func(ctx context.Context, w http.ResponseWriter, err error)

	Route struct {
		Path        string
		Method      string
		Handler     Handler
		Middlewares []Middleware
	}
)

func New(options ...Option) Server {
	settings := defaultSettings
	for _, option := range options {
		settings = option(settings)
	}

	srv := &server{
		Server: http.Server{
			Addr: fmt.Sprintf(":%s", settings.port),
		},
		router:           chi.NewRouter(),
		routes:           settings.routes,
		shutdownListener: make(chan error, 1),
		errorHandler:     settings.errorHandler,
	}

	srv.Server.Handler = Middlewares(srv.router, settings.globalMiddlewares...)
	srv.buildRoutes()
	return srv
}

func (s *server) ShutdownListener() chan error {
	return s.shutdownListener
}

func (s *server) Run() Shutdown {
	go func() {
		s.shutdownListener <- s.Server.ListenAndServe()
	}()
	return s.Server.Shutdown
}

func (s *server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	s.Server.Handler.ServeHTTP(w, req)
}

func NewRoute(method, path string, handler Handler, middlewares ...Middleware) Route {
	return Route{
		Path:        path,
		Method:      method,
		Handler:     handler,
		Middlewares: middlewares,
	}
}

func Middlewares(main http.Handler, middlewares ...Middleware) http.Handler {
	handler := main
	for i := range middlewares {
		handler = middlewares[len(middlewares)-1-i](handler)
	}
	return handler
}

func (s *server) RegisterRoute(route Route) {
	s.routes = append(s.routes, route)
}

func (s *server) buildRoutes() {
	for _, r := range s.routes {
		s.router.Method(
			r.Method,
			r.Path,
			Middlewares(
				newErrorHandler(s.errorHandler, r.Handler),
				r.Middlewares...,
			),
		)
	}
}

func defaultHandleError(ctx context.Context, w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusInternalServerError)
}

func newErrorHandler(errorHandler ErrorHandler, handler Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if err := handler(w, req); err != nil {
			errorHandler(req.Context(), w, err)
		}
	})
}
