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

func Error(w http.ResponseWriter, statusCode int, errorMessage string) {
	JSON(w, statusCode, struct {
		ErrorMessage string `json:"errorMessage"`
	}{
		ErrorMessage: errorMessage,
	})
}
