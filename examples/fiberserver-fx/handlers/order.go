package handlers

import (
	"context"
	"net/http"

	"github.com/JailtonJunior94/devkit-go/pkg/httpclient"
	"github.com/JailtonJunior94/devkit-go/pkg/messaging"
	o11y "github.com/JailtonJunior94/devkit-go/pkg/telemetry"
	"github.com/gofiber/fiber/v2"
)

type OrderHandler struct {
	telemetry  o11y.Telemetry
	httpClient httpclient.HTTPClient
	publisher  messaging.Publisher
}

func NewOrderHandler(
	telemetry o11y.Telemetry,
	httpClient httpclient.HTTPClient,
	publisher messaging.Publisher,
) *OrderHandler {
	return &OrderHandler{
		telemetry:  telemetry,
		httpClient: httpClient,
		publisher:  publisher,
	}
}

func (h *OrderHandler) Create(c *fiber.Ctx) error {
	ctx := context.Background()
	ctx, span := h.telemetry.Tracer().Start(ctx, "OrderHandler.Create")
	defer span.End()

	h.telemetry.Logger().Info(ctx, "Creating new order")

	// Simulate creating an order
	order := map[string]any{
		"id":     "order-123",
		"status": "created",
		"items":  []string{"item-1", "item-2"},
	}

	// Publish event to message broker
	if h.publisher != nil {
		msg := &messaging.Message{
			Body: []byte(`{"order_id": "order-123", "event": "order.created"}`),
		}
		headers := map[string]string{
			"content_type": "application/json",
			"event_type":   "order.created",
		}

		if err := h.publisher.Publish(ctx, "orders", "order.created", headers, msg); err != nil {
			h.telemetry.Logger().Error(ctx, err, "Failed to publish order event")
			span.RecordError(err)
		} else {
			h.telemetry.Logger().Info(ctx, "Order event published successfully")
		}
	}

	h.telemetry.Metrics().AddCounter(ctx, "orders_created_total", 1)

	return c.Status(http.StatusCreated).JSON(order)
}

func (h *OrderHandler) GetByID(c *fiber.Ctx) error {
	ctx := context.Background()
	ctx, span := h.telemetry.Tracer().Start(ctx, "OrderHandler.GetByID")
	defer span.End()

	id := c.Params("id")
	h.telemetry.Logger().Info(ctx, "Fetching order by ID", o11y.LogField("order_id", id))

	order := map[string]any{
		"id":     id,
		"status": "completed",
		"items":  []string{"item-1", "item-2"},
	}

	return c.Status(http.StatusOK).JSON(order)
}

func (h *OrderHandler) GetAll(c *fiber.Ctx) error {
	ctx := context.Background()
	ctx, span := h.telemetry.Tracer().Start(ctx, "OrderHandler.GetAll")
	defer span.End()

	h.telemetry.Logger().Info(ctx, "Fetching all orders")

	orders := []map[string]any{
		{"id": "order-1", "status": "completed"},
		{"id": "order-2", "status": "pending"},
	}

	return c.Status(http.StatusOK).JSON(orders)
}
