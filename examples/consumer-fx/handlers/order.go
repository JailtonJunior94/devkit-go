package handlers

import (
	"context"
	"encoding/json"
	"log"

	o11y "github.com/JailtonJunior94/devkit-go/pkg/telemetry"
)

type OrderEvent struct {
	OrderID string `json:"order_id"`
	Event   string `json:"event"`
}

type OrderHandler struct {
	telemetry o11y.Telemetry
}

func NewOrderHandler(telemetry o11y.Telemetry) *OrderHandler {
	return &OrderHandler{
		telemetry: telemetry,
	}
}

func (h *OrderHandler) HandleOrderCreated(ctx context.Context, params map[string]string, body []byte) error {
	ctx, span := h.telemetry.Tracer().Start(ctx, "OrderHandler.HandleOrderCreated")
	defer span.End()

	var event OrderEvent
	if err := json.Unmarshal(body, &event); err != nil {
		h.telemetry.Logger().Error(ctx, err, "Failed to unmarshal order event")
		span.RecordError(err)
		return err
	}

	h.telemetry.Logger().Info(ctx, "Processing order created event",
		o11y.LogField("order_id", event.OrderID),
		o11y.LogField("event", event.Event),
	)

	// Simulate processing
	log.Printf("[OrderHandler] Order created: %s", event.OrderID)

	h.telemetry.Metrics().AddCounter(ctx, "orders_processed_total", 1)

	return nil
}

func (h *OrderHandler) HandleOrderUpdated(ctx context.Context, params map[string]string, body []byte) error {
	ctx, span := h.telemetry.Tracer().Start(ctx, "OrderHandler.HandleOrderUpdated")
	defer span.End()

	var event OrderEvent
	if err := json.Unmarshal(body, &event); err != nil {
		h.telemetry.Logger().Error(ctx, err, "Failed to unmarshal order event")
		span.RecordError(err)
		return err
	}

	h.telemetry.Logger().Info(ctx, "Processing order updated event",
		o11y.LogField("order_id", event.OrderID),
		o11y.LogField("event", event.Event),
	)

	log.Printf("[OrderHandler] Order updated: %s", event.OrderID)

	return nil
}

func (h *OrderHandler) HandleOrderCancelled(ctx context.Context, params map[string]string, body []byte) error {
	ctx, span := h.telemetry.Tracer().Start(ctx, "OrderHandler.HandleOrderCancelled")
	defer span.End()

	var event OrderEvent
	if err := json.Unmarshal(body, &event); err != nil {
		h.telemetry.Logger().Error(ctx, err, "Failed to unmarshal order event")
		span.RecordError(err)
		return err
	}

	h.telemetry.Logger().Info(ctx, "Processing order cancelled event",
		o11y.LogField("order_id", event.OrderID),
		o11y.LogField("event", event.Event),
	)

	log.Printf("[OrderHandler] Order cancelled: %s", event.OrderID)

	h.telemetry.Metrics().AddCounter(ctx, "orders_cancelled_total", 1)

	return nil
}
