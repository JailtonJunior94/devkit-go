package handlers

import "github.com/gofiber/fiber/v2"

// TransactionHandler handles transaction-related requests.
type TransactionHandler struct {
	// In real app, inject repository/service here
}

// NewTransactionHandler creates a new TransactionHandler.
func NewTransactionHandler() *TransactionHandler {
	return &TransactionHandler{}
}

// List returns all transactions with optional filtering.
func (h *TransactionHandler) List(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)
	txType := c.Query("type")
	accountID := c.Query("account_id")

	_ = txType
	_ = accountID

	transactions := []fiber.Map{
		{
			"id":          "txn_1",
			"amount":      -50.00,
			"type":        "debit",
			"description": "Coffee Shop",
			"category":    "food",
			"created_at":  "2024-01-15T10:30:00Z",
		},
		{
			"id":          "txn_2",
			"amount":      3000.00,
			"type":        "credit",
			"description": "Salary",
			"category":    "income",
			"created_at":  "2024-01-01T09:00:00Z",
		},
	}

	return c.JSON(fiber.Map{
		"data": transactions,
		"meta": fiber.Map{
			"page":  page,
			"limit": limit,
			"total": len(transactions),
		},
	})
}

// Get returns a single transaction by ID.
func (h *TransactionHandler) Get(c *fiber.Ctx) error {
	id := c.Params("id")

	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"id":          id,
			"amount":      -50.00,
			"type":        "debit",
			"description": "Coffee Shop",
			"category":    "food",
			"account_id":  "acc_1",
			"created_at":  "2024-01-15T10:30:00Z",
		},
	})
}

// Create creates a new transaction.
func (h *TransactionHandler) Create(c *fiber.Ctx) error {
	type Request struct {
		AccountID   string  `json:"account_id"`
		Amount      float64 `json:"amount"`
		Type        string  `json:"type"`
		Description string  `json:"description"`
		Category    string  `json:"category"`
	}

	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.AccountID == "" || req.Amount == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "account_id and amount are required",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"data": fiber.Map{
			"id":          "txn_new",
			"account_id":  req.AccountID,
			"amount":      req.Amount,
			"type":        req.Type,
			"description": req.Description,
			"category":    req.Category,
		},
	})
}

// BulkCreate creates multiple transactions at once (v2 only).
func (h *TransactionHandler) BulkCreate(c *fiber.Ctx) error {
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
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if len(req.Transactions) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "At least one transaction is required",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Transactions created successfully",
		"count":   len(req.Transactions),
	})
}

// GetReport returns a transaction report (v2 only).
func (h *TransactionHandler) GetReport(c *fiber.Ctx) error {
	startDate := c.Query("start", "2024-01-01")
	endDate := c.Query("end", "2024-12-31")

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
				{"category": "income", "total": 5000.00},
				{"category": "food", "total": -500.00},
				{"category": "transport", "total": -200.00},
			},
		},
	})
}
