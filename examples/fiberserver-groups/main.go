package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/JailtonJunior94/devkit-go/pkg/fiberserver"
	"github.com/gofiber/fiber/v2"
)

func main() {
	server := fiberserver.New(
		fiberserver.WithPort("8080"),
		fiberserver.WithMiddlewares(
			fiberserver.RequestID,
			fiberserver.Recovery,
			fiberserver.SecurityHeaders,
			fiberserver.Logger,
		),
	)

	// Health check routes (no versioning)
	server.RegisterRoute(fiberserver.NewRoute(fiber.MethodGet, "/health", fiberserver.HealthCheck))
	server.RegisterRoute(fiberserver.NewRoute(fiber.MethodGet, "/ready", fiberserver.ReadinessCheck))

	// API v1
	v1 := server.Group("/api/v1")
	registerV1Routes(v1)

	// API v2 (with additional features)
	v2 := server.Group("/api/v2")
	registerV2Routes(v2)

	// Start server
	shutdown := server.Run()
	log.Println("Server started on :8080")
	log.Println("API v1: http://localhost:8080/api/v1")
	log.Println("API v2: http://localhost:8080/api/v2")

	// Graceful shutdown
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := <-server.ShutdownListener(); err != nil {
			log.Printf("Server error: %v", err)
			interrupt <- syscall.SIGTERM
		}
	}()

	<-interrupt
	log.Println("Shutting down server...")

	ctx, cancel := fiberserver.GetShutdownTimeout()
	defer cancel()

	if err := shutdown(ctx); err != nil {
		log.Fatal(err)
	}

	log.Println("Server stopped gracefully")
}

// ============================================================================
// API v1 Routes
// ============================================================================

func registerV1Routes(v1 *fiberserver.RouteGroup) {
	// Accounts resource
	accounts := v1.Group("/accounts")
	accounts.Get("", listAccountsV1)
	accounts.Get("/:id", getAccountV1)
	accounts.Post("", createAccountV1)
	accounts.Put("/:id", updateAccountV1)
	accounts.Delete("/:id", deleteAccountV1)

	// Transactions resource
	transactions := v1.Group("/transactions")
	transactions.Get("", listTransactionsV1)
	transactions.Get("/:id", getTransactionV1)
	transactions.Post("", createTransactionV1)

	// Users resource
	users := v1.Group("/users")
	users.Get("", listUsersV1)
	users.Get("/:id", getUserV1)
	users.Post("", createUserV1)
	users.Put("/:id", updateUserV1)
	users.Delete("/:id", deleteUserV1)

	// Nested: User's accounts
	users.Get("/:userId/accounts", getUserAccountsV1)

	// Nested: Account's transactions
	accounts.Get("/:accountId/transactions", getAccountTransactionsV1)
}

// ============================================================================
// API v2 Routes (with enhanced features)
// ============================================================================

func registerV2Routes(v2 *fiberserver.RouteGroup) {
	// Accounts resource (v2 with pagination metadata)
	accounts := v2.Group("/accounts")
	accounts.Get("", listAccountsV2)
	accounts.Get("/:id", getAccountV2)
	accounts.Post("", createAccountV2)
	accounts.Put("/:id", updateAccountV2)
	accounts.Patch("/:id", patchAccountV2) // New in v2
	accounts.Delete("/:id", deleteAccountV2)

	// Transactions resource (v2 with filtering)
	transactions := v2.Group("/transactions")
	transactions.Get("", listTransactionsV2)
	transactions.Get("/:id", getTransactionV2)
	transactions.Post("", createTransactionV2)
	transactions.Post("/bulk", bulkCreateTransactionsV2) // New in v2

	// Users resource
	users := v2.Group("/users")
	users.Get("", listUsersV2)
	users.Get("/:id", getUserV2)
	users.Post("", createUserV2)
	users.Put("/:id", updateUserV2)
	users.Patch("/:id", patchUserV2) // New in v2
	users.Delete("/:id", deleteUserV2)

	// Reports resource (new in v2)
	reports := v2.Group("/reports")
	reports.Get("/summary", getReportSummaryV2)
	reports.Get("/transactions", getTransactionsReportV2)
	reports.Get("/accounts/:id/statement", getAccountStatementV2)
}

// ============================================================================
// V1 Handlers - Accounts
// ============================================================================

func listAccountsV1(c *fiber.Ctx) error {
	accounts := []fiber.Map{
		{"id": "1", "name": "Main Account", "balance": 1000.00, "currency": "USD"},
		{"id": "2", "name": "Savings", "balance": 5000.00, "currency": "USD"},
	}
	return c.JSON(fiber.Map{"accounts": accounts})
}

func getAccountV1(c *fiber.Ctx) error {
	id := c.Params("id")
	return c.JSON(fiber.Map{
		"id":       id,
		"name":     "Main Account",
		"balance":  1000.00,
		"currency": "USD",
	})
}

func createAccountV1(c *fiber.Ctx) error {
	type Request struct {
		Name     string `json:"name"`
		Currency string `json:"currency"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":       "3",
		"name":     req.Name,
		"balance":  0,
		"currency": req.Currency,
	})
}

func updateAccountV1(c *fiber.Ctx) error {
	id := c.Params("id")
	return c.JSON(fiber.Map{"id": id, "message": "Account updated"})
}

func deleteAccountV1(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNoContent).Send(nil)
}

func getAccountTransactionsV1(c *fiber.Ctx) error {
	accountId := c.Params("accountId")
	transactions := []fiber.Map{
		{"id": "1", "account_id": accountId, "amount": -50.00, "type": "debit"},
		{"id": "2", "account_id": accountId, "amount": 200.00, "type": "credit"},
	}
	return c.JSON(fiber.Map{"transactions": transactions})
}

// ============================================================================
// V1 Handlers - Transactions
// ============================================================================

func listTransactionsV1(c *fiber.Ctx) error {
	transactions := []fiber.Map{
		{"id": "1", "amount": -50.00, "type": "debit", "description": "Coffee"},
		{"id": "2", "amount": 200.00, "type": "credit", "description": "Salary"},
	}
	return c.JSON(fiber.Map{"transactions": transactions})
}

func getTransactionV1(c *fiber.Ctx) error {
	id := c.Params("id")
	return c.JSON(fiber.Map{
		"id":          id,
		"amount":      -50.00,
		"type":        "debit",
		"description": "Coffee",
		"created_at":  "2024-01-15T10:30:00Z",
	})
}

func createTransactionV1(c *fiber.Ctx) error {
	type Request struct {
		AccountID   string  `json:"account_id"`
		Amount      float64 `json:"amount"`
		Type        string  `json:"type"`
		Description string  `json:"description"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":          "3",
		"account_id":  req.AccountID,
		"amount":      req.Amount,
		"type":        req.Type,
		"description": req.Description,
	})
}

// ============================================================================
// V1 Handlers - Users
// ============================================================================

func listUsersV1(c *fiber.Ctx) error {
	users := []fiber.Map{
		{"id": "1", "name": "John Doe", "email": "john@example.com"},
		{"id": "2", "name": "Jane Smith", "email": "jane@example.com"},
	}
	return c.JSON(fiber.Map{"users": users})
}

func getUserV1(c *fiber.Ctx) error {
	id := c.Params("id")
	return c.JSON(fiber.Map{
		"id":    id,
		"name":  "John Doe",
		"email": "john@example.com",
	})
}

func createUserV1(c *fiber.Ctx) error {
	type Request struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":    "3",
		"name":  req.Name,
		"email": req.Email,
	})
}

func updateUserV1(c *fiber.Ctx) error {
	id := c.Params("id")
	return c.JSON(fiber.Map{"id": id, "message": "User updated"})
}

func deleteUserV1(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNoContent).Send(nil)
}

func getUserAccountsV1(c *fiber.Ctx) error {
	userId := c.Params("userId")
	accounts := []fiber.Map{
		{"id": "1", "user_id": userId, "name": "Main Account", "balance": 1000.00},
		{"id": "2", "user_id": userId, "name": "Savings", "balance": 5000.00},
	}
	return c.JSON(fiber.Map{"accounts": accounts})
}

// ============================================================================
// V2 Handlers - Accounts (with pagination)
// ============================================================================

func listAccountsV2(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)

	accounts := []fiber.Map{
		{"id": "1", "name": "Main Account", "balance": 1000.00, "currency": "USD", "type": "checking"},
		{"id": "2", "name": "Savings", "balance": 5000.00, "currency": "USD", "type": "savings"},
	}
	return c.JSON(fiber.Map{
		"data": accounts,
		"meta": fiber.Map{
			"page":        page,
			"limit":       limit,
			"total":       2,
			"total_pages": 1,
		},
	})
}

func getAccountV2(c *fiber.Ctx) error {
	id := c.Params("id")
	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"id":         id,
			"name":       "Main Account",
			"balance":    1000.00,
			"currency":   "USD",
			"type":       "checking",
			"created_at": "2024-01-01T00:00:00Z",
			"updated_at": "2024-01-15T10:30:00Z",
		},
	})
}

func createAccountV2(c *fiber.Ctx) error {
	type Request struct {
		Name     string `json:"name"`
		Currency string `json:"currency"`
		Type     string `json:"type"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"data": fiber.Map{
			"id":         "3",
			"name":       req.Name,
			"balance":    0,
			"currency":   req.Currency,
			"type":       req.Type,
			"created_at": "2024-01-15T12:00:00Z",
		},
	})
}

func updateAccountV2(c *fiber.Ctx) error {
	id := c.Params("id")
	return c.JSON(fiber.Map{
		"data":    fiber.Map{"id": id, "name": "Updated Account"},
		"message": "Account updated successfully",
	})
}

func patchAccountV2(c *fiber.Ctx) error {
	id := c.Params("id")
	return c.JSON(fiber.Map{
		"data":    fiber.Map{"id": id},
		"message": "Account partially updated",
	})
}

func deleteAccountV2(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNoContent).Send(nil)
}

// ============================================================================
// V2 Handlers - Transactions (with filtering)
// ============================================================================

func listTransactionsV2(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)
	txType := c.Query("type")     // Filter by type
	startDate := c.Query("start") // Filter by date range
	endDate := c.Query("end")

	_ = txType
	_ = startDate
	_ = endDate

	transactions := []fiber.Map{
		{"id": "1", "amount": -50.00, "type": "debit", "description": "Coffee", "category": "food"},
		{"id": "2", "amount": 200.00, "type": "credit", "description": "Salary", "category": "income"},
	}
	return c.JSON(fiber.Map{
		"data": transactions,
		"meta": fiber.Map{
			"page":        page,
			"limit":       limit,
			"total":       2,
			"total_pages": 1,
		},
	})
}

func getTransactionV2(c *fiber.Ctx) error {
	id := c.Params("id")
	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"id":          id,
			"amount":      -50.00,
			"type":        "debit",
			"description": "Coffee",
			"category":    "food",
			"created_at":  "2024-01-15T10:30:00Z",
			"account": fiber.Map{
				"id":   "1",
				"name": "Main Account",
			},
		},
	})
}

func createTransactionV2(c *fiber.Ctx) error {
	type Request struct {
		AccountID   string  `json:"account_id"`
		Amount      float64 `json:"amount"`
		Type        string  `json:"type"`
		Description string  `json:"description"`
		Category    string  `json:"category"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"data": fiber.Map{
			"id":          "3",
			"account_id":  req.AccountID,
			"amount":      req.Amount,
			"type":        req.Type,
			"description": req.Description,
			"category":    req.Category,
		},
	})
}

func bulkCreateTransactionsV2(c *fiber.Ctx) error {
	type Transaction struct {
		AccountID   string  `json:"account_id"`
		Amount      float64 `json:"amount"`
		Type        string  `json:"type"`
		Description string  `json:"description"`
	}
	type Request struct {
		Transactions []Transaction `json:"transactions"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Bulk transactions created",
		"count":   len(req.Transactions),
	})
}

// ============================================================================
// V2 Handlers - Users
// ============================================================================

func listUsersV2(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)

	users := []fiber.Map{
		{"id": "1", "name": "John Doe", "email": "john@example.com", "role": "admin"},
		{"id": "2", "name": "Jane Smith", "email": "jane@example.com", "role": "user"},
	}
	return c.JSON(fiber.Map{
		"data": users,
		"meta": fiber.Map{
			"page":        page,
			"limit":       limit,
			"total":       2,
			"total_pages": 1,
		},
	})
}

func getUserV2(c *fiber.Ctx) error {
	id := c.Params("id")
	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"id":         id,
			"name":       "John Doe",
			"email":      "john@example.com",
			"role":       "admin",
			"created_at": "2024-01-01T00:00:00Z",
		},
	})
}

func createUserV2(c *fiber.Ctx) error {
	type Request struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"data": fiber.Map{
			"id":    "3",
			"name":  req.Name,
			"email": req.Email,
			"role":  req.Role,
		},
	})
}

func updateUserV2(c *fiber.Ctx) error {
	id := c.Params("id")
	return c.JSON(fiber.Map{
		"data":    fiber.Map{"id": id},
		"message": "User updated successfully",
	})
}

func patchUserV2(c *fiber.Ctx) error {
	id := c.Params("id")
	return c.JSON(fiber.Map{
		"data":    fiber.Map{"id": id},
		"message": "User partially updated",
	})
}

func deleteUserV2(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNoContent).Send(nil)
}

// ============================================================================
// V2 Handlers - Reports (new in v2)
// ============================================================================

func getReportSummaryV2(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"total_accounts":     5,
			"total_users":        10,
			"total_transactions": 150,
			"total_balance":      25000.00,
			"currency":           "USD",
		},
	})
}

func getTransactionsReportV2(c *fiber.Ctx) error {
	startDate := c.Query("start", "2024-01-01")
	endDate := c.Query("end", "2024-01-31")

	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"period": fiber.Map{
				"start": startDate,
				"end":   endDate,
			},
			"summary": fiber.Map{
				"total_income":   5000.00,
				"total_expenses": 2500.00,
				"net":            2500.00,
			},
			"by_category": []fiber.Map{
				{"category": "income", "amount": 5000.00},
				{"category": "food", "amount": -500.00},
				{"category": "transport", "amount": -200.00},
				{"category": "utilities", "amount": -300.00},
				{"category": "other", "amount": -1500.00},
			},
		},
	})
}

func getAccountStatementV2(c *fiber.Ctx) error {
	accountId := c.Params("id")
	startDate := c.Query("start", "2024-01-01")
	endDate := c.Query("end", "2024-01-31")

	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"account": fiber.Map{
				"id":   accountId,
				"name": "Main Account",
			},
			"period": fiber.Map{
				"start": startDate,
				"end":   endDate,
			},
			"opening_balance": 1000.00,
			"closing_balance": 1500.00,
			"transactions": []fiber.Map{
				{"date": "2024-01-05", "description": "Deposit", "amount": 1000.00, "balance": 2000.00},
				{"date": "2024-01-10", "description": "Withdrawal", "amount": -500.00, "balance": 1500.00},
			},
		},
	})
}
