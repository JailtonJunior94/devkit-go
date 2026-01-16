package common

import (
	"net/http"
	"time"
)

// ProblemDetail represents an RFC 7807 Problem Details for HTTP APIs response.
// This structure provides a standardized way to carry machine-readable details
// of errors in HTTP response bodies.
//
// Reference: https://datatracker.ietf.org/doc/html/rfc7807
type ProblemDetail struct {
	Type      string    `json:"type"`               // URI reference that identifies the problem type
	Title     string    `json:"title"`              // Short, human-readable summary
	Status    int       `json:"status"`             // HTTP status code
	Detail    string    `json:"detail,omitempty"`   // Human-readable explanation specific to this occurrence
	Instance  string    `json:"instance"`           // URI reference for the specific occurrence
	Timestamp time.Time `json:"timestamp"`          // When the error occurred
	RequestID string    `json:"request_id,omitempty"` // Request ID for tracing
}

// GetStatusText returns the human-readable status text for the given HTTP status code.
// This function provides a centralized mapping from status codes to standard HTTP status text.
func GetStatusText(code int) string {
	switch code {
	case http.StatusBadRequest:
		return "Bad Request"
	case http.StatusUnauthorized:
		return "Unauthorized"
	case http.StatusForbidden:
		return "Forbidden"
	case http.StatusNotFound:
		return "Not Found"
	case http.StatusMethodNotAllowed:
		return "Method Not Allowed"
	case http.StatusRequestTimeout:
		return "Request Timeout"
	case http.StatusConflict:
		return "Conflict"
	case http.StatusUnprocessableEntity:
		return "Unprocessable Entity"
	case http.StatusRequestEntityTooLarge:
		return "Request Entity Too Large"
	case http.StatusTooManyRequests:
		return "Too Many Requests"
	case http.StatusInternalServerError:
		return "Internal Server Error"
	case http.StatusNotImplemented:
		return "Not Implemented"
	case http.StatusBadGateway:
		return "Bad Gateway"
	case http.StatusServiceUnavailable:
		return "Service Unavailable"
	case http.StatusGatewayTimeout:
		return "Gateway Timeout"
	default:
		return "Error"
	}
}
