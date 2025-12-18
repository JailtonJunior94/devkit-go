package fiberserverfx

import (
	"context"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/fiberserver"
	"go.uber.org/fx"
)

// Module provides Fiber server with lifecycle management.
// Usage:
//
//	fx.New(
//	    fiberserverfx.Module,
//	    fx.Supply(fiberserverfx.Config{Port: "8080"}),
//	    fx.Provide(fx.Annotate(
//	        func(h *Handler) fiberserver.Route {
//	            return fiberserver.NewRoute("GET", "/api", h.Get)
//	        },
//	        fx.ResultTags(`group:"routes"`),
//	    )),
//	)
var Module = fx.Module("fiberserver",
	fx.Provide(ProvideServer),
	fx.Invoke(RegisterLifecycle),
)

// ServerParams contains dependencies for creating a server.
type ServerParams struct {
	fx.In

	Config       Config                   `optional:"true"`
	Routes       []fiberserver.Route      `group:"routes"`
	Middlewares  []fiberserver.Middleware `group:"middlewares"`
	ErrorHandler fiberserver.ErrorHandler `optional:"true"`
}

// ServerResult contains the server output.
type ServerResult struct {
	fx.Out

	Server fiberserver.Server
}

// ProvideServer creates a Fiber server with injected dependencies.
func ProvideServer(p ServerParams) ServerResult {
	cfg := p.Config
	if cfg.Port == "" {
		cfg = DefaultConfig()
	}

	opts := []fiberserver.Option{
		fiberserver.WithPort(cfg.Port),
		fiberserver.WithReadTimeout(cfg.ReadTimeout),
		fiberserver.WithWriteTimeout(cfg.WriteTimeout),
		fiberserver.WithIdleTimeout(cfg.IdleTimeout),
		fiberserver.WithShutdownTimeout(cfg.ShutdownTimeout),
		fiberserver.WithBodyLimit(cfg.BodyLimit),
		fiberserver.WithReadBufferSize(cfg.ReadBufferSize),
		fiberserver.WithWriteBufferSize(cfg.WriteBufferSize),
		fiberserver.WithPrefork(cfg.Prefork),
		fiberserver.WithStrictRouting(cfg.StrictRouting),
		fiberserver.WithCaseSensitive(cfg.CaseSensitive),
	}

	if len(p.Routes) > 0 {
		opts = append(opts, fiberserver.WithRoutes(p.Routes...))
	}

	if len(p.Middlewares) > 0 {
		opts = append(opts, fiberserver.WithMiddlewares(p.Middlewares...))
	}

	if p.ErrorHandler != nil {
		opts = append(opts, fiberserver.WithErrorHandler(p.ErrorHandler))
	}

	return ServerResult{Server: fiberserver.New(opts...)}
}

// LifecycleParams contains dependencies for server lifecycle management.
type LifecycleParams struct {
	fx.In

	Server fiberserver.Server
	LC     fx.Lifecycle
	Config Config `optional:"true"`
}

// RegisterLifecycle registers server start/stop with FX lifecycle.
func RegisterLifecycle(p LifecycleParams) {
	var shutdown fiberserver.Shutdown

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
//	    fiberserverfx.ProvideRoute("GET", "/users", handler.GetUsers),
//	    fx.ResultTags(`group:"routes"`),
//	))
func ProvideRoute(method, path string, handler fiberserver.Handler, middlewares ...fiberserver.Middleware) func() fiberserver.Route {
	return func() fiberserver.Route {
		return fiberserver.NewRoute(method, path, handler, middlewares...)
	}
}

// ProvideMiddleware is a helper to provide global middlewares.
// Usage:
//
//	fx.Provide(fx.Annotate(
//	    fiberserverfx.ProvideMiddleware(fiberserver.RequestID),
//	    fx.ResultTags(`group:"middlewares"`),
//	))
func ProvideMiddleware(m fiberserver.Middleware) func() fiberserver.Middleware {
	return func() fiberserver.Middleware {
		return m
	}
}

// ProvideErrorHandler is a helper to provide a custom error handler.
// Usage:
//
//	fx.Provide(fiberserverfx.ProvideErrorHandler(myErrorHandler))
func ProvideErrorHandler(h fiberserver.ErrorHandler) func() fiberserver.ErrorHandler {
	return func() fiberserver.ErrorHandler {
		return h
	}
}

// RouteGroupParams contains dependencies for creating a route group.
type RouteGroupParams struct {
	fx.In

	Server fiberserver.Server
}

// ProvideRouteGroup creates a route group provider for the given prefix.
// Usage:
//
//	fx.Provide(fiberserverfx.ProvideRouteGroup("/api/v1"))
func ProvideRouteGroup(prefix string, middlewares ...fiberserver.Middleware) func(RouteGroupParams) *fiberserver.RouteGroup {
	return func(p RouteGroupParams) *fiberserver.RouteGroup {
		return p.Server.Group(prefix, middlewares...)
	}
}

// RouteRegistrarParams contains dependencies for route registration.
type RouteRegistrarParams struct {
	fx.In

	Registrars []fiberserver.RouteRegistrar `group:"route_registrars"`
	Server     fiberserver.Server
}

// ProvideRouteRegistrar is a helper to provide route registrars.
// Usage:
//
//	fx.Provide(fx.Annotate(
//	    func(h *UserHandler) fiberserver.RouteRegistrar {
//	        return func(r fiberserver.Router) {
//	            r.Get("/users", h.GetAll)
//	            r.Post("/users", h.Create)
//	        }
//	    },
//	    fx.ResultTags(`group:"route_registrars"`),
//	))
func ProvideRouteRegistrar(registrar fiberserver.RouteRegistrar) func() fiberserver.RouteRegistrar {
	return func() fiberserver.RouteRegistrar {
		return registrar
	}
}
