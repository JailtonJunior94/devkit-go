package httpserverfx

import (
	"context"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/httpserver"
	"go.uber.org/fx"
)

// Module provides HTTP server with lifecycle management.
// Usage:
//
//	fx.New(
//	    httpserverfx.Module,
//	    fx.Supply(httpserverfx.Config{Port: "8080"}),
//	    fx.Provide(fx.Annotate(
//	        func(h *Handler) httpserver.Route {
//	            return httpserver.NewRoute("GET", "/api", h.Get)
//	        },
//	        fx.ResultTags(`group:"routes"`),
//	    )),
//	)
var Module = fx.Module("httpserver",
	fx.Provide(ProvideServer),
	fx.Invoke(RegisterLifecycle),
)

// ServerParams contains dependencies for creating a server.
type ServerParams struct {
	fx.In

	Config       Config                  `optional:"true"`
	Routes       []httpserver.Route      `group:"routes"`
	Middlewares  []httpserver.Middleware `group:"middlewares"`
	ErrorHandler httpserver.ErrorHandler `optional:"true"`
}

// ServerResult contains the server output.
type ServerResult struct {
	fx.Out

	Server httpserver.Server
}

// ProvideServer creates an HTTP server with injected dependencies.
func ProvideServer(p ServerParams) ServerResult {
	cfg := p.Config
	if cfg.Port == "" {
		cfg = DefaultConfig()
	}

	opts := []httpserver.Option{
		httpserver.WithPort(cfg.Port),
		httpserver.WithReadTimeout(cfg.ReadTimeout),
		httpserver.WithWriteTimeout(cfg.WriteTimeout),
		httpserver.WithIdleTimeout(cfg.IdleTimeout),
		httpserver.WithReadHeaderTimeout(cfg.ReadHeaderTimeout),
		httpserver.WithMaxHeaderBytes(cfg.MaxHeaderBytes),
		httpserver.WithShutdownTimeout(cfg.ShutdownTimeout),
	}

	if len(p.Routes) > 0 {
		opts = append(opts, httpserver.WithRoutes(p.Routes...))
	}

	if len(p.Middlewares) > 0 {
		opts = append(opts, httpserver.WithMiddlewares(p.Middlewares...))
	}

	if p.ErrorHandler != nil {
		opts = append(opts, httpserver.WithErrorHandler(p.ErrorHandler))
	}

	return ServerResult{Server: httpserver.New(opts...)}
}

// LifecycleParams contains dependencies for server lifecycle management.
type LifecycleParams struct {
	fx.In

	Server httpserver.Server
	LC     fx.Lifecycle
	Config Config `optional:"true"`
}

// RegisterLifecycle registers server start/stop with FX lifecycle.
func RegisterLifecycle(p LifecycleParams) {
	var shutdown httpserver.Shutdown

	shutdownTimeout := p.Config.ShutdownTimeout
	if shutdownTimeout == 0 {
		shutdownTimeout = 30 * time.Second
	}

	p.LC.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			shutdown = p.Server.Run()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
			defer cancel()
			return shutdown(shutdownCtx)
		},
	})
}

// ProvideRoute is a helper to provide routes to the server.
// Usage:
//
//	fx.Provide(fx.Annotate(
//	    httpserverfx.ProvideRoute("GET", "/users", handler.GetUsers),
//	    fx.ResultTags(`group:"routes"`),
//	))
func ProvideRoute(method, path string, handler httpserver.Handler, middlewares ...httpserver.Middleware) func() httpserver.Route {
	return func() httpserver.Route {
		return httpserver.NewRoute(method, path, handler, middlewares...)
	}
}

// ProvideMiddleware is a helper to provide global middlewares.
// Usage:
//
//	fx.Provide(fx.Annotate(
//	    httpserverfx.ProvideMiddleware(httpserver.RequestID),
//	    fx.ResultTags(`group:"middlewares"`),
//	))
func ProvideMiddleware(m httpserver.Middleware) func() httpserver.Middleware {
	return func() httpserver.Middleware {
		return m
	}
}

// ProvideErrorHandler is a helper to provide a custom error handler.
// Usage:
//
//	fx.Provide(httpserverfx.ProvideErrorHandler(myErrorHandler))
func ProvideErrorHandler(h httpserver.ErrorHandler) func() httpserver.ErrorHandler {
	return func() httpserver.ErrorHandler {
		return h
	}
}
