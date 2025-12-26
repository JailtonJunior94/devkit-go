package responses

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJSON(t *testing.T) {
	t.Run("writes valid JSON response", func(t *testing.T) {
		w := httptest.NewRecorder()
		data := map[string]string{"message": "success"}

		JSON(w, http.StatusOK, data)

		if w.Code != http.StatusOK {
			t.Errorf("JSON() status = %v, want %v", w.Code, http.StatusOK)
		}

		contentType := w.Header().Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("JSON() Content-Type = %v, want application/json", contentType)
		}

		var response map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("JSON() produced invalid JSON: %v", err)
		}

		if response["message"] != "success" {
			t.Errorf("JSON() body = %v, want %v", response["message"], "success")
		}
	})

	t.Run("handles different status codes", func(t *testing.T) {
		testCases := []int{
			http.StatusOK,
			http.StatusCreated,
			http.StatusBadRequest,
			http.StatusNotFound,
			http.StatusInternalServerError,
		}

		for _, statusCode := range testCases {
			w := httptest.NewRecorder()
			JSON(w, statusCode, map[string]string{"status": "test"})

			if w.Code != statusCode {
				t.Errorf("JSON() status = %v, want %v", w.Code, statusCode)
			}
		}
	})

	t.Run("handles nil data without panic", func(t *testing.T) {
		w := httptest.NewRecorder()

		// Isso não deve causar panic
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("JSON() panicked with nil data: %v", r)
			}
		}()

		JSON(w, http.StatusOK, nil)

		if w.Code != http.StatusOK {
			t.Errorf("JSON() status = %v, want %v", w.Code, http.StatusOK)
		}
	})

	t.Run("handles complex nested structures", func(t *testing.T) {
		w := httptest.NewRecorder()
		data := map[string]interface{}{
			"user": map[string]interface{}{
				"id":   123,
				"name": "John",
				"tags": []string{"admin", "user"},
			},
			"metadata": map[string]int{
				"count": 10,
			},
		}

		JSON(w, http.StatusOK, data)

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("JSON() failed to encode complex structure: %v", err)
		}
	})

	t.Run("does not panic with unserializable data", func(t *testing.T) {
		w := httptest.NewRecorder()

		// chan não pode ser serializado para JSON
		data := struct {
			Chan chan int `json:"chan"`
		}{
			Chan: make(chan int),
		}

		// Isso não deve causar panic, mesmo com erro de serialização
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("JSON() panicked with unserializable data: %v", r)
			}
		}()

		JSON(w, http.StatusOK, data)

		// Deve ter escrito o status code antes de falhar
		if w.Code != http.StatusOK {
			t.Errorf("JSON() status = %v, want %v", w.Code, http.StatusOK)
		}
	})
}

func TestError(t *testing.T) {
	t.Run("writes error response", func(t *testing.T) {
		w := httptest.NewRecorder()
		message := "something went wrong"

		Error(w, http.StatusBadRequest, message)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Error() status = %v, want %v", w.Code, http.StatusBadRequest)
		}

		var response struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Error() produced invalid JSON: %v", err)
		}

		if response.Message != message {
			t.Errorf("Error() message = %v, want %v", response.Message, message)
		}
	})

	t.Run("handles empty message", func(t *testing.T) {
		w := httptest.NewRecorder()

		Error(w, http.StatusBadRequest, "")

		var response struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Error() produced invalid JSON: %v", err)
		}

		if response.Message != "" {
			t.Errorf("Error() message = %v, want empty string", response.Message)
		}
	})
}

func TestErrorWithDetails(t *testing.T) {
	t.Run("writes error with details", func(t *testing.T) {
		w := httptest.NewRecorder()
		message := "validation failed"
		details := map[string]string{
			"field": "email",
			"error": "invalid format",
		}

		ErrorWithDetails(w, http.StatusUnprocessableEntity, message, details)

		if w.Code != http.StatusUnprocessableEntity {
			t.Errorf("ErrorWithDetails() status = %v, want %v", w.Code, http.StatusUnprocessableEntity)
		}

		var response struct {
			Message string                 `json:"message"`
			Details map[string]interface{} `json:"details"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("ErrorWithDetails() produced invalid JSON: %v", err)
		}

		if response.Message != message {
			t.Errorf("ErrorWithDetails() message = %v, want %v", response.Message, message)
		}

		if response.Details == nil {
			t.Error("ErrorWithDetails() details = nil, want non-nil")
		}
	})

	t.Run("handles nil details", func(t *testing.T) {
		w := httptest.NewRecorder()
		message := "error occurred"

		ErrorWithDetails(w, http.StatusInternalServerError, message, nil)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("ErrorWithDetails() status = %v, want %v", w.Code, http.StatusInternalServerError)
		}

		body, _ := io.ReadAll(w.Body)
		var response map[string]interface{}
		if err := json.Unmarshal(body, &response); err != nil {
			t.Fatalf("ErrorWithDetails() produced invalid JSON: %v", err)
		}

		// Details pode estar presente como null ou ausente
		if msg, ok := response["message"].(string); !ok || msg != message {
			t.Errorf("ErrorWithDetails() message = %v, want %v", response["message"], message)
		}
	})

	t.Run("handles complex details", func(t *testing.T) {
		w := httptest.NewRecorder()
		message := "multiple errors"
		details := []map[string]string{
			{"field": "email", "error": "required"},
			{"field": "password", "error": "too short"},
		}

		ErrorWithDetails(w, http.StatusBadRequest, message, details)

		var response struct {
			Message string                   `json:"message"`
			Details []map[string]interface{} `json:"details"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("ErrorWithDetails() produced invalid JSON: %v", err)
		}

		if len(response.Details) != 2 {
			t.Errorf("ErrorWithDetails() details count = %v, want 2", len(response.Details))
		}
	})
}

// Test concurrent usage
func TestConcurrentUsage(t *testing.T) {
	t.Run("handles concurrent JSON writes", func(t *testing.T) {
		const goroutines = 100

		for i := 0; i < goroutines; i++ {
			go func(id int) {
				w := httptest.NewRecorder()
				data := map[string]int{"id": id}
				JSON(w, http.StatusOK, data)
			}(i)
		}
	})

	t.Run("handles concurrent Error writes", func(t *testing.T) {
		const goroutines = 100

		for i := 0; i < goroutines; i++ {
			go func(id int) {
				w := httptest.NewRecorder()
				Error(w, http.StatusBadRequest, "error")
			}(i)
		}
	})
}

// Benchmarks
func BenchmarkJSON(b *testing.B) {
	data := map[string]string{"message": "success", "status": "ok"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		JSON(w, http.StatusOK, data)
	}
}

func BenchmarkError(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		Error(w, http.StatusBadRequest, "error message")
	}
}

func BenchmarkErrorWithDetails(b *testing.B) {
	details := map[string]string{"field": "email", "error": "invalid"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		ErrorWithDetails(w, http.StatusBadRequest, "validation failed", details)
	}
}
