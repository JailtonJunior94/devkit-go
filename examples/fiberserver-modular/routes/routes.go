package routes

import (
	"github.com/JailtonJunior94/devkit-go/examples/fiberserver-modular/handlers"
	"github.com/JailtonJunior94/devkit-go/pkg/fiberserver"
	"github.com/gofiber/fiber/v2"
)

// RegisterHealthRoutes registers health check endpoints.
func RegisterHealthRoutes(server fiberserver.Server) {
	server.RegisterRoute(fiberserver.NewRoute(
		fiber.MethodGet,
		"/health",
		fiberserver.HealthCheck,
	))
	server.RegisterRoute(fiberserver.NewRoute(
		fiber.MethodGet,
		"/ready",
		fiberserver.ReadinessCheck,
	))
}

// RegisterV1Routes registers all v1 API routes.
func RegisterV1Routes(
	server fiberserver.Server,
	accountHandler *handlers.AccountHandler,
	transactionHandler *handlers.TransactionHandler,
	userHandler *handlers.UserHandler,
) {
	v1 := server.Group("/api/v1")

	// Account routes
	registerAccountRoutesV1(v1, accountHandler)

	// Transaction routes
	registerTransactionRoutesV1(v1, transactionHandler)

	// User routes
	registerUserRoutesV1(v1, userHandler)
}

// RegisterV2Routes registers all v2 API routes.
func RegisterV2Routes(
	server fiberserver.Server,
	accountHandler *handlers.AccountHandler,
	transactionHandler *handlers.TransactionHandler,
	userHandler *handlers.UserHandler,
) {
	v2 := server.Group("/api/v2")

	// Account routes
	registerAccountRoutesV2(v2, accountHandler)

	// Transaction routes
	registerTransactionRoutesV2(v2, transactionHandler)

	// User routes
	registerUserRoutesV2(v2, userHandler)

	// Reports (new in v2)
	registerReportRoutes(v2, accountHandler, transactionHandler)
}

// ============================================================================
// V1 Route Registration
// ============================================================================

func registerAccountRoutesV1(group *fiberserver.RouteGroup, h *handlers.AccountHandler) {
	accounts := group.Group("/accounts")

	accounts.Get("", h.List)
	accounts.Get("/:id", h.Get)
	accounts.Post("", h.Create)
	accounts.Put("/:id", h.Update)
	accounts.Delete("/:id", h.Delete)

	// Nested resource
	accounts.Get("/:id/transactions", h.GetTransactions)
}

func registerTransactionRoutesV1(group *fiberserver.RouteGroup, h *handlers.TransactionHandler) {
	transactions := group.Group("/transactions")

	transactions.Get("", h.List)
	transactions.Get("/:id", h.Get)
	transactions.Post("", h.Create)
}

func registerUserRoutesV1(group *fiberserver.RouteGroup, h *handlers.UserHandler) {
	users := group.Group("/users")

	users.Get("", h.List)
	users.Get("/:id", h.Get)
	users.Post("", h.Create)
	users.Put("/:id", h.Update)
	users.Delete("/:id", h.Delete)

	// Nested resource
	users.Get("/:id/accounts", h.GetAccounts)
}

// ============================================================================
// V2 Route Registration (with additional features)
// ============================================================================

func registerAccountRoutesV2(group *fiberserver.RouteGroup, h *handlers.AccountHandler) {
	accounts := group.Group("/accounts")

	accounts.Get("", h.List)
	accounts.Get("/:id", h.Get)
	accounts.Post("", h.Create)
	accounts.Put("/:id", h.Update)
	accounts.Delete("/:id", h.Delete)

	// Nested resources
	accounts.Get("/:id/transactions", h.GetTransactions)
	accounts.Get("/:id/statement", h.GetStatement) // New in v2
}

func registerTransactionRoutesV2(group *fiberserver.RouteGroup, h *handlers.TransactionHandler) {
	transactions := group.Group("/transactions")

	transactions.Get("", h.List)
	transactions.Get("/:id", h.Get)
	transactions.Post("", h.Create)
	transactions.Post("/bulk", h.BulkCreate) // New in v2
}

func registerUserRoutesV2(group *fiberserver.RouteGroup, h *handlers.UserHandler) {
	users := group.Group("/users")

	users.Get("", h.List)
	users.Get("/:id", h.Get)
	users.Post("", h.Create)
	users.Put("/:id", h.Update)
	users.Patch("/:id", h.Patch) // New in v2
	users.Delete("/:id", h.Delete)

	// Nested resource
	users.Get("/:id/accounts", h.GetAccounts)
}

func registerReportRoutes(
	group *fiberserver.RouteGroup,
	accountHandler *handlers.AccountHandler,
	transactionHandler *handlers.TransactionHandler,
) {
	reports := group.Group("/reports")

	reports.Get("/transactions", transactionHandler.GetReport)
}
