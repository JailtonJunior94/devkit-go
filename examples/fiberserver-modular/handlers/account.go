package handlers

import "github.com/gofiber/fiber/v2"

// AccountHandler handles account-related requests.
type AccountHandler struct {
	// In real app, inject repository/service here
}

// NewAccountHandler creates a new AccountHandler.
func NewAccountHandler() *AccountHandler {
	return &AccountHandler{}
}

// List returns all accounts.
func (h *AccountHandler) List(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)

	accounts := []fiber.Map{
		{"id": "acc_1", "name": "Main Account", "balance": 1000.00, "currency": "USD"},
		{"id": "acc_2", "name": "Savings", "balance": 5000.00, "currency": "USD"},
	}

	return c.JSON(fiber.Map{
		"data": accounts,
		"meta": fiber.Map{
			"page":  page,
			"limit": limit,
			"total": len(accounts),
		},
	})
}

// Get returns a single account by ID.
func (h *AccountHandler) Get(c *fiber.Ctx) error {
	id := c.Params("id")

	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"id":       id,
			"name":     "Main Account",
			"balance":  1000.00,
			"currency": "USD",
		},
	})
}

// Create creates a new account.
func (h *AccountHandler) Create(c *fiber.Ctx) error {
	type Request struct {
		Name     string `json:"name"`
		Currency string `json:"currency"`
	}

	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Name is required",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"data": fiber.Map{
			"id":       "acc_new",
			"name":     req.Name,
			"balance":  0,
			"currency": req.Currency,
		},
	})
}

// Update updates an existing account.
func (h *AccountHandler) Update(c *fiber.Ctx) error {
	id := c.Params("id")

	type Request struct {
		Name string `json:"name"`
	}

	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"id":   id,
			"name": req.Name,
		},
		"message": "Account updated",
	})
}

// Delete deletes an account.
func (h *AccountHandler) Delete(c *fiber.Ctx) error {
	// id := c.Params("id")
	return c.SendStatus(fiber.StatusNoContent)
}

// GetTransactions returns transactions for an account.
func (h *AccountHandler) GetTransactions(c *fiber.Ctx) error {
	accountID := c.Params("id")

	transactions := []fiber.Map{
		{"id": "txn_1", "account_id": accountID, "amount": -50.00, "type": "debit"},
		{"id": "txn_2", "account_id": accountID, "amount": 200.00, "type": "credit"},
	}

	return c.JSON(fiber.Map{
		"data": transactions,
	})
}

// GetStatement returns an account statement.
func (h *AccountHandler) GetStatement(c *fiber.Ctx) error {
	accountID := c.Params("id")
	startDate := c.Query("start", "2024-01-01")
	endDate := c.Query("end", "2024-12-31")

	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"account_id":      accountID,
			"period_start":    startDate,
			"period_end":      endDate,
			"opening_balance": 1000.00,
			"closing_balance": 1500.00,
			"transactions": []fiber.Map{
				{"date": "2024-01-05", "description": "Deposit", "amount": 500.00},
			},
		},
	})
}
