package serverfiber

import (
	"encoding/json"

	"github.com/JailtonJunior94/devkit-go/pkg/http_server/common"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/gofiber/fiber/v2"
)

const problemContentType = "application/problem+json"

// defaultErrorHandler builds the framework error handler that maps every
// error to an RFC 7807 Problem Details response via common.ProblemFromError.
// The original err is logged via observability so it stays observable
// server-side without leaking internal details to clients.
func defaultErrorHandler(o11y observability.Observability) fiber.ErrorHandler {
	logger := o11y.Logger()
	return func(c *fiber.Ctx, err error) error {
		requestID, _ := c.Locals("requestID").(string)

		problem := common.ProblemFromError(err, c.Path(), requestID)

		logger.Error(c.UserContext(), "request failed",
			observability.String("request_id", requestID),
			observability.String("path", c.Path()),
			observability.String("method", c.Method()),
			observability.Int("status", problem.Status),
			observability.Error(err),
		)

		// Marshal manually so the content-type header is application/problem+json;
		// c.JSON would overwrite it with application/json.
		body, marshalErr := json.Marshal(problem)
		if marshalErr != nil {
			return marshalErr
		}
		c.Status(problem.Status).Set(fiber.HeaderContentType, problemContentType)
		return c.Send(body)
	}
}
