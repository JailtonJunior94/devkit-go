package responses

import (
	"encoding/json"
	"net/http"
)

type problemDetail struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail,omitempty"`
	Errors any    `json:"errors,omitempty"`
}

const (
	contentTypeJSON    = "application/json"
	contentTypeProblem = "application/problem+json"
	problemTypeBlank   = "about:blank"
	fallbackProblem    = `{"type":"about:blank","title":"Internal Server Error","status":500,"detail":"internal server error"}`
)

func JSON(w http.ResponseWriter, statusCode int, data any) {
	b, err := json.Marshal(data)
	if err != nil {
		w.Header().Set("Content-Type", contentTypeProblem)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(fallbackProblem))
		return
	}
	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(statusCode)
	_, _ = w.Write(b)
}

func Error(w http.ResponseWriter, statusCode int, message string) {
	writeProblem(w, statusCode, problemDetail{
		Type:   problemTypeBlank,
		Title:  http.StatusText(statusCode),
		Status: statusCode,
		Detail: message,
	})
}

func ErrorWithDetails(w http.ResponseWriter, statusCode int, message string, details any) {
	writeProblem(w, statusCode, problemDetail{
		Type:   problemTypeBlank,
		Title:  http.StatusText(statusCode),
		Status: statusCode,
		Detail: message,
		Errors: details,
	})
}

func writeProblem(w http.ResponseWriter, statusCode int, p problemDetail) {
	b, err := json.Marshal(p)
	if err != nil {
		w.Header().Set("Content-Type", contentTypeProblem)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(fallbackProblem))
		return
	}
	w.Header().Set("Content-Type", contentTypeProblem)
	w.WriteHeader(statusCode)
	_, _ = w.Write(b)
}
