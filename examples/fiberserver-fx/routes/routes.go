package routes

import (
	"github.com/JailtonJunior94/devkit-go/examples/fiberserver-fx/handlers"
	"github.com/JailtonJunior94/devkit-go/pkg/fiberserver"
	"go.uber.org/fx"
)

// Module provides all routes as a group.
var Module = fx.Module("routes",
	// Health routes
	fx.Provide(fx.Annotate(
		provideHealthRoute,
		fx.ResultTags(`group:"routes"`),
	)),
	fx.Provide(fx.Annotate(
		provideReadyRoute,
		fx.ResultTags(`group:"routes"`),
	)),

	// User routes
	fx.Provide(fx.Annotate(
		provideGetUsersRoute,
		fx.ResultTags(`group:"routes"`),
	)),
	fx.Provide(fx.Annotate(
		provideGetUserByIDRoute,
		fx.ResultTags(`group:"routes"`),
	)),
	fx.Provide(fx.Annotate(
		provideCreateUserRoute,
		fx.ResultTags(`group:"routes"`),
	)),

	// Order routes
	fx.Provide(fx.Annotate(
		provideGetOrdersRoute,
		fx.ResultTags(`group:"routes"`),
	)),
	fx.Provide(fx.Annotate(
		provideGetOrderByIDRoute,
		fx.ResultTags(`group:"routes"`),
	)),
	fx.Provide(fx.Annotate(
		provideCreateOrderRoute,
		fx.ResultTags(`group:"routes"`),
	)),
)

// Health routes
func provideHealthRoute(h *handlers.HealthHandler) fiberserver.Route {
	return fiberserver.NewRoute("GET", "/health", h.Health)
}

func provideReadyRoute(h *handlers.HealthHandler) fiberserver.Route {
	return fiberserver.NewRoute("GET", "/ready", h.Ready)
}

// User routes
func provideGetUsersRoute(h *handlers.UserHandler) fiberserver.Route {
	return fiberserver.NewRoute("GET", "/api/v1/users", h.GetAll)
}

func provideGetUserByIDRoute(h *handlers.UserHandler) fiberserver.Route {
	return fiberserver.NewRoute("GET", "/api/v1/users/:id", h.GetByID)
}

func provideCreateUserRoute(h *handlers.UserHandler) fiberserver.Route {
	return fiberserver.NewRoute("POST", "/api/v1/users", h.Create)
}

// Order routes
func provideGetOrdersRoute(h *handlers.OrderHandler) fiberserver.Route {
	return fiberserver.NewRoute("GET", "/api/v1/orders", h.GetAll)
}

func provideGetOrderByIDRoute(h *handlers.OrderHandler) fiberserver.Route {
	return fiberserver.NewRoute("GET", "/api/v1/orders/:id", h.GetByID)
}

func provideCreateOrderRoute(h *handlers.OrderHandler) fiberserver.Route {
	return fiberserver.NewRoute("POST", "/api/v1/orders", h.Create)
}
