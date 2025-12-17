package handlers

import "github.com/gofiber/fiber/v2"

// UserHandler handles user-related requests.
type UserHandler struct {
	// In real app, inject repository/service here
}

// NewUserHandler creates a new UserHandler.
func NewUserHandler() *UserHandler {
	return &UserHandler{}
}

// List returns all users.
func (h *UserHandler) List(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)

	users := []fiber.Map{
		{
			"id":         "usr_1",
			"name":       "John Doe",
			"email":      "john@example.com",
			"role":       "admin",
			"created_at": "2024-01-01T00:00:00Z",
		},
		{
			"id":         "usr_2",
			"name":       "Jane Smith",
			"email":      "jane@example.com",
			"role":       "user",
			"created_at": "2024-01-05T00:00:00Z",
		},
	}

	return c.JSON(fiber.Map{
		"data": users,
		"meta": fiber.Map{
			"page":  page,
			"limit": limit,
			"total": len(users),
		},
	})
}

// Get returns a single user by ID.
func (h *UserHandler) Get(c *fiber.Ctx) error {
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

// Create creates a new user.
func (h *UserHandler) Create(c *fiber.Ctx) error {
	type Request struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}

	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Name == "" || req.Email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Name and email are required",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"data": fiber.Map{
			"id":    "usr_new",
			"name":  req.Name,
			"email": req.Email,
			"role":  req.Role,
		},
	})
}

// Update updates an existing user.
func (h *UserHandler) Update(c *fiber.Ctx) error {
	id := c.Params("id")

	type Request struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Role  string `json:"role"`
	}

	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"id":    id,
			"name":  req.Name,
			"email": req.Email,
			"role":  req.Role,
		},
		"message": "User updated",
	})
}

// Patch partially updates a user (v2 only).
func (h *UserHandler) Patch(c *fiber.Ctx) error {
	id := c.Params("id")

	return c.JSON(fiber.Map{
		"data":    fiber.Map{"id": id},
		"message": "User partially updated",
	})
}

// Delete deletes a user.
func (h *UserHandler) Delete(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusNoContent)
}

// GetAccounts returns all accounts for a user.
func (h *UserHandler) GetAccounts(c *fiber.Ctx) error {
	userID := c.Params("id")

	accounts := []fiber.Map{
		{"id": "acc_1", "user_id": userID, "name": "Main Account", "balance": 1000.00},
		{"id": "acc_2", "user_id": userID, "name": "Savings", "balance": 5000.00},
	}

	return c.JSON(fiber.Map{
		"data": accounts,
	})
}
