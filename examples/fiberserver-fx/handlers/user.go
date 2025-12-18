package handlers

import (
	"context"
	"net/http"

	o11y "github.com/JailtonJunior94/devkit-go/pkg/telemetry"
	"github.com/gofiber/fiber/v2"
)

type UserHandler struct {
	telemetry o11y.Telemetry
}

func NewUserHandler(telemetry o11y.Telemetry) *UserHandler {
	return &UserHandler{
		telemetry: telemetry,
	}
}

func (h *UserHandler) GetAll(c *fiber.Ctx) error {
	ctx := context.Background()
	ctx, span := h.telemetry.Tracer().Start(ctx, "UserHandler.GetAll")
	defer span.End()

	h.telemetry.Logger().Info(ctx, "Fetching all users")
	h.telemetry.Metrics().AddCounter(ctx, "users_list_total", 1)

	users := []map[string]any{
		{"id": "1", "name": "John Doe", "email": "john@example.com"},
		{"id": "2", "name": "Jane Doe", "email": "jane@example.com"},
	}

	return c.Status(http.StatusOK).JSON(users)
}

func (h *UserHandler) GetByID(c *fiber.Ctx) error {
	ctx := context.Background()
	ctx, span := h.telemetry.Tracer().Start(ctx, "UserHandler.GetByID")
	defer span.End()

	id := c.Params("id")
	h.telemetry.Logger().Info(ctx, "Fetching user by ID", o11y.LogField("user_id", id))

	user := map[string]any{
		"id":    id,
		"name":  "John Doe",
		"email": "john@example.com",
	}

	return c.Status(http.StatusOK).JSON(user)
}

func (h *UserHandler) Create(c *fiber.Ctx) error {
	ctx := context.Background()
	ctx, span := h.telemetry.Tracer().Start(ctx, "UserHandler.Create")
	defer span.End()

	h.telemetry.Logger().Info(ctx, "Creating new user")
	h.telemetry.Metrics().AddCounter(ctx, "users_created_total", 1)

	user := map[string]any{
		"id":    "3",
		"name":  "New User",
		"email": "new@example.com",
	}

	return c.Status(http.StatusCreated).JSON(user)
}
