package chiserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ProblemDetail represents an RFC 7807 Problem Details for HTTP APIs response.
type ProblemDetail struct {
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Status    int       `json:"status"`
	Detail    string    `json:"detail"`
	Instance  string    `json:"instance"`
	Timestamp time.Time `json:"timestamp"`
	RequestID string    `json:"request_id,omitempty"`
}

// HTTPError represents an HTTP error with a status code and message.
type HTTPError struct {
	Code    int
	Message string
}

// Error implements the error interface.
func (e *HTTPError) Error() string {
	return fmt.Sprintf("http error: %d - %s", e.Code, e.Message)
}

// NewHTTPError creates a new HTTPError.
func NewHTTPError(code int, message string) *HTTPError {
	return &HTTPError{
		Code:    code,
		Message: message,
	}
}

// getStatusText returns the human-readable status text for the given status code.
func getStatusText(code int) string {
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
	}

	return "Error"
}

// writeErrorResponse writes an error response following RFC 7807.
func writeErrorResponse(w http.ResponseWriter, r *http.Request, code int, detail string) {
	requestID, _ := r.Context().Value(requestIDKey).(string)

	problem := ProblemDetail{
		Type:      fmt.Sprintf("https://httpstatuses.com/%d", code),
		Title:     getStatusText(code),
		Status:    code,
		Detail:    detail,
		Instance:  r.URL.Path,
		Timestamp: time.Now(),
		RequestID: requestID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	_ = json.NewEncoder(w).Encode(problem)
}
