package fiberserver

import "github.com/gofiber/fiber/v2"

type (
	// Router defines the interface for route registration.
	Router interface {
		// Group creates a new route group with the given prefix.
		Group(prefix string, middlewares ...Middleware) *RouteGroup
		// Get registers a GET route.
		Get(path string, handler Handler, middlewares ...Middleware)
		// Post registers a POST route.
		Post(path string, handler Handler, middlewares ...Middleware)
		// Put registers a PUT route.
		Put(path string, handler Handler, middlewares ...Middleware)
		// Delete registers a DELETE route.
		Delete(path string, handler Handler, middlewares ...Middleware)
		// Patch registers a PATCH route.
		Patch(path string, handler Handler, middlewares ...Middleware)
		// Head registers a HEAD route.
		Head(path string, handler Handler, middlewares ...Middleware)
		// Options registers an OPTIONS route.
		Options(path string, handler Handler, middlewares ...Middleware)
	}

	// RouteGroup represents a group of routes with a common prefix.
	RouteGroup struct {
		prefix       string
		app          *fiber.App
		group        fiber.Router
		errorHandler ErrorHandler
		middlewares  []Middleware
	}

	// RouteRegistrar is a function that registers routes on a router.
	RouteRegistrar func(router Router)
)

// NewRouteGroup creates a new route group with the given prefix.
func (s *server) Group(prefix string, middlewares ...Middleware) *RouteGroup {
	fiberMiddlewares := make([]fiber.Handler, len(middlewares))
	for i, m := range middlewares {
		fiberMiddlewares[i] = fiber.Handler(m)
	}

	return &RouteGroup{
		prefix:       prefix,
		app:          s.app,
		group:        s.app.Group(prefix, fiberMiddlewares...),
		errorHandler: s.errorHandler,
		middlewares:  middlewares,
	}
}

// Group creates a nested route group.
func (rg *RouteGroup) Group(prefix string, middlewares ...Middleware) *RouteGroup {
	fiberMiddlewares := make([]fiber.Handler, len(middlewares))
	for i, m := range middlewares {
		fiberMiddlewares[i] = fiber.Handler(m)
	}

	return &RouteGroup{
		prefix:       rg.prefix + prefix,
		app:          rg.app,
		group:        rg.group.Group(prefix, fiberMiddlewares...),
		errorHandler: rg.errorHandler,
		middlewares:  append(rg.middlewares, middlewares...),
	}
}

// Get registers a GET route in the group.
func (rg *RouteGroup) Get(path string, handler Handler, middlewares ...Middleware) {
	rg.registerRoute(fiber.MethodGet, path, handler, middlewares...)
}

// Post registers a POST route in the group.
func (rg *RouteGroup) Post(path string, handler Handler, middlewares ...Middleware) {
	rg.registerRoute(fiber.MethodPost, path, handler, middlewares...)
}

// Put registers a PUT route in the group.
func (rg *RouteGroup) Put(path string, handler Handler, middlewares ...Middleware) {
	rg.registerRoute(fiber.MethodPut, path, handler, middlewares...)
}

// Delete registers a DELETE route in the group.
func (rg *RouteGroup) Delete(path string, handler Handler, middlewares ...Middleware) {
	rg.registerRoute(fiber.MethodDelete, path, handler, middlewares...)
}

// Patch registers a PATCH route in the group.
func (rg *RouteGroup) Patch(path string, handler Handler, middlewares ...Middleware) {
	rg.registerRoute(fiber.MethodPatch, path, handler, middlewares...)
}

// Head registers a HEAD route in the group.
func (rg *RouteGroup) Head(path string, handler Handler, middlewares ...Middleware) {
	rg.registerRoute(fiber.MethodHead, path, handler, middlewares...)
}

// Options registers an OPTIONS route in the group.
func (rg *RouteGroup) Options(path string, handler Handler, middlewares ...Middleware) {
	rg.registerRoute(fiber.MethodOptions, path, handler, middlewares...)
}

// Use adds middlewares to the group.
func (rg *RouteGroup) Use(middlewares ...Middleware) *RouteGroup {
	for _, m := range middlewares {
		rg.group.Use(fiber.Handler(m))
	}
	rg.middlewares = append(rg.middlewares, middlewares...)
	return rg
}

// registerRoute registers a route in the group.
func (rg *RouteGroup) registerRoute(method, path string, handler Handler, middlewares ...Middleware) {
	handlers := make([]fiber.Handler, 0, len(middlewares)+1)

	for _, m := range middlewares {
		handlers = append(handlers, fiber.Handler(m))
	}

	handlers = append(handlers, rg.wrapHandler(handler))

	rg.group.Add(method, path, handlers...)
}

// wrapHandler wraps a Handler to handle errors using the ErrorHandler.
func (rg *RouteGroup) wrapHandler(handler Handler) fiber.Handler {
	return func(c *fiber.Ctx) error {
		err := handler(c)
		if err == nil {
			return nil
		}
		return rg.errorHandler(c, err)
	}
}

// RegisterRoutes is a helper function to register routes using a RouteRegistrar.
func (rg *RouteGroup) RegisterRoutes(registrar RouteRegistrar) {
	registrar(rg)
}

// Static serves static files from the given root directory.
func (rg *RouteGroup) Static(prefix, root string) {
	rg.group.Static(prefix, root)
}

// Mount mounts a sub-application to the group.
func (rg *RouteGroup) Mount(prefix string, app *fiber.App) {
	rg.app.Mount(rg.prefix+prefix, app)
}
