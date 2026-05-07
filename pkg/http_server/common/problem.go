package common

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

// fallbackInternalErrorDetail is the fixed Detail string returned for any
// error that is not explicitly mapped. The original err.Error() is never
// propagated to the response body to avoid leaking internal information.
const fallbackInternalErrorDetail = "internal server error"

// ProblemFromError maps err to an RFC 7807 ProblemDetail without leaking
// internal details to the client. Only *fiber.Error is preserved (its
// Code/Message are framework-defined and safe). Any other error — including
// nil — falls back to HTTP 500 with a fixed Detail.
//
// Callers are responsible for logging the original err separately via
// pkg/observability with the request id, so it is observable server-side
// without being exposed to clients.
//
// instance is the URI of the occurrence (typically r.URL.Path or c.Path()).
// requestID is optional and used for client-side correlation.
func ProblemFromError(err error, instance, requestID string) ProblemDetail {
	code := http.StatusInternalServerError
	detail := fallbackInternalErrorDetail

	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		code = fiberErr.Code
		detail = fiberErr.Message
	}

	return ProblemDetail{
		Type:      problemTypeURI(code),
		Title:     GetStatusText(code),
		Status:    code,
		Detail:    detail,
		Instance:  instance,
		Timestamp: time.Now().UTC(),
		RequestID: requestID,
	}
}

// problemTypeURI returns the canonical URI used as the ProblemDetail.Type
// for a given status code.
func problemTypeURI(code int) string {
	return "https://httpstatuses.com/" + strconv.Itoa(code)
}
