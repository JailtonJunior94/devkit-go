package serverfiber

import (
	"errors"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
)

type ProblemDetail struct {
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Status    int       `json:"status"`
	Detail    string    `json:"detail,omitempty"`
	Instance  string    `json:"instance"`
	Timestamp time.Time `json:"timestamp"`
	RequestID string    `json:"request_id,omitempty"`
}

func defaultErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	detail := err.Error()

	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		code = fiberErr.Code
		detail = fiberErr.Message
	}

	title := getErrorTitle(code)

	requestID := ""
	if rid, ok := c.Locals("requestID").(string); ok {
		requestID = rid
	}

	problem := ProblemDetail{
		Type:      fmt.Sprintf("https://httpstatuses.com/%d", code),
		Title:     title,
		Status:    code,
		Detail:    detail,
		Instance:  c.Path(),
		Timestamp: time.Now(),
		RequestID: requestID,
	}

	return c.Status(code).JSON(problem)
}

func getErrorTitle(code int) string {
	switch code {
	case fiber.StatusBadRequest:
		return "Bad Request"
	case fiber.StatusUnauthorized:
		return "Unauthorized"
	case fiber.StatusForbidden:
		return "Forbidden"
	case fiber.StatusNotFound:
		return "Not Found"
	case fiber.StatusMethodNotAllowed:
		return "Method Not Allowed"
	case fiber.StatusRequestTimeout:
		return "Request Timeout"
	case fiber.StatusConflict:
		return "Conflict"
	case fiber.StatusUnprocessableEntity:
		return "Unprocessable Entity"
	case fiber.StatusTooManyRequests:
		return "Too Many Requests"
	case fiber.StatusInternalServerError:
		return "Internal Server Error"
	case fiber.StatusNotImplemented:
		return "Not Implemented"
	case fiber.StatusBadGateway:
		return "Bad Gateway"
	case fiber.StatusServiceUnavailable:
		return "Service Unavailable"
	case fiber.StatusGatewayTimeout:
		return "Gateway Timeout"
	default:
		return "Error"
	}
}
