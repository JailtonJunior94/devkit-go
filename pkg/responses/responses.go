package responses

import (
	"encoding/json"
	"net/http"
)

func JSON(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		panic(err)
	}
}

func Error(w http.ResponseWriter, statusCode int, message string) {
	JSON(w, statusCode, struct {
		Message string `json:"message"`
	}{
		Message: message,
	})
}

func ErrorWithDetails(w http.ResponseWriter, statusCode int, message string, details any) {
	JSON(w, statusCode, struct {
		Message string `json:"message"`
		Details any    `json:"details,omitempty"`
	}{
		Message: message,
		Details: details,
	})
}
