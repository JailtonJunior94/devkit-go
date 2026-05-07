// Package http_server is the unified HTTP toolkit component for devkit-go.
// It provides two concrete adapters — [chi_server] (net/http + go-chi/chi) and
// [server_fiber] (gofiber/fiber/v2) — that share configuration, security and
// observability helpers from the [common] sub-package.
//
// # Option Parity
//
// Every Option below is available in both adapters with identical semantics
// unless noted otherwise. Consumers switching between adapters only need to
// change the import path.
//
//	Option                  chi_server   server_fiber   Notes
//	──────────────────────────────────────────────────────────────────────────
//	WithConfig              ✓            ✓
//	WithPort                ✓            ✓
//	WithReadTimeout         ✓            ✓
//	WithWriteTimeout        ✓            ✓
//	WithIdleTimeout         ✓            ✓
//	WithShutdownTimeout     ✓            ✓              RF-8.2
//	WithBodyLimit           ✓            ✓
//	WithCORS                ✓            ✓
//	WithMetrics             ✓            ✓
//	WithHealthChecks        ✓            ✓
//	WithMiddleware          ✓            ✓
//	WithRouteTimeout        ✓            ✓
//	WithServiceName         ✓            ✓
//	WithServiceVersion      ✓            ✓
//	WithEnvironment         ✓            ✓
//	WithTracing             ✓            ✓
//	WithOTelMetrics         ✓            ✓
//	WithErrorHandler        ✓            ✓              Signature differs (see below)
//	WithTimeoutCleanup      ✓            —              chi_server only (RF-4.5)
//
// # WithErrorHandler Signature Differences
//
// In chi_server, the error handler receives a stdlib context and ResponseWriter:
//
//	WithErrorHandler(fn func(ctx context.Context, w http.ResponseWriter, err error))
//
// In server_fiber, the error handler follows the native Fiber convention:
//
//	WithErrorHandler(fn fiber.ErrorHandler)  // func(*fiber.Ctx, error) error
//
// # Timeout Behaviour in chi_server (RF-5.4)
//
// The Chi adapter installs timeout enforcement at two levels:
//
//  1. A top-level middleware applies globalTimeout to every route registered
//     directly on chi.Router (including those wired through RegisterRouters).
//     This middleware does not consult per-route timeout maps because the route
//     pattern is not resolved at pre-match time.
//
//  2. Routes registered via [Server.RegisterHandler] receive a per-route
//     timeout envelope wrapping the individual handler. The envelope knows the
//     pattern at registration time, so routeTimeouts[pattern] is applied
//     correctly even for parametric routes like "/users/{id}".
//
// Consumers that need per-route timeouts must use RegisterHandler. Using
// RegisterRouters alone only grants the global timeout.
//
// # Timeout Behaviour in server_fiber (RF-4.1 trade-off)
//
// The Fiber adapter delegates timeout enforcement to the official
// github.com/gofiber/fiber/v2/middleware/timeout package via
// timeout.NewWithContext. This eliminates the goroutine-reentrancy race of the
// previous custom implementation. Trade-off accepted: the middleware applies
// context.WithTimeout to UserContext but does not interrupt a hung handler —
// handlers that ignore ctx.Done() will block indefinitely without returning
// a 408. Consumers must design handlers to respect context cancellation.
package http_server
