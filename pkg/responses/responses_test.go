package responses

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

type problemResponse struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail"`
	Errors any    `json:"errors"`
}

func TestJSON(t *testing.T) {
	t.Run("writes valid JSON response", func(t *testing.T) {
		w := httptest.NewRecorder()
		JSON(w, http.StatusOK, map[string]string{"message": "success"})

		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, contentTypeJSON, w.Header().Get("Content-Type"))

		var response map[string]string
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		require.Equal(t, "success", response["message"])
	})

	t.Run("handles different status codes", func(t *testing.T) {
		codes := []int{
			http.StatusOK,
			http.StatusCreated,
			http.StatusBadRequest,
			http.StatusNotFound,
			http.StatusInternalServerError,
		}
		for _, code := range codes {
			w := httptest.NewRecorder()
			JSON(w, code, map[string]string{"status": "test"})
			require.Equal(t, code, w.Code)
		}
	})

	t.Run("handles nil data", func(t *testing.T) {
		w := httptest.NewRecorder()
		JSON(w, http.StatusOK, nil)

		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, "null", w.Body.String())
	})

	t.Run("handles complex nested structures", func(t *testing.T) {
		w := httptest.NewRecorder()
		data := map[string]any{
			"user": map[string]any{
				"id":   123,
				"name": "John",
				"tags": []string{"admin", "user"},
			},
			"metadata": map[string]int{"count": 10},
		}

		JSON(w, http.StatusOK, data)

		var response map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	})

	t.Run("returns RFC 7807 500 on unserializable data", func(t *testing.T) {
		w := httptest.NewRecorder()
		data := struct {
			Ch chan int `json:"ch"`
		}{Ch: make(chan int)}

		JSON(w, http.StatusOK, data)

		require.Equal(t, http.StatusInternalServerError, w.Code)
		require.Equal(t, contentTypeProblem, w.Header().Get("Content-Type"))

		var response problemResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		require.Equal(t, problemTypeBlank, response.Type)
		require.Equal(t, "Internal Server Error", response.Title)
		require.Equal(t, http.StatusInternalServerError, response.Status)
		require.Equal(t, "internal server error", response.Detail)
	})
}

func TestError(t *testing.T) {
	t.Run("writes RFC 7807 error response", func(t *testing.T) {
		w := httptest.NewRecorder()
		Error(w, http.StatusBadRequest, "something went wrong")

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Equal(t, contentTypeProblem, w.Header().Get("Content-Type"))

		var response problemResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		require.Equal(t, problemTypeBlank, response.Type)
		require.Equal(t, "Bad Request", response.Title)
		require.Equal(t, http.StatusBadRequest, response.Status)
		require.Equal(t, "something went wrong", response.Detail)
	})

	t.Run("omits detail when message is empty", func(t *testing.T) {
		w := httptest.NewRecorder()
		Error(w, http.StatusBadRequest, "")

		var response map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		require.NotContains(t, response, "detail")
	})
}

func TestErrorWithDetails(t *testing.T) {
	t.Run("writes RFC 7807 error with errors extension", func(t *testing.T) {
		w := httptest.NewRecorder()
		details := map[string]string{"field": "email", "error": "invalid format"}

		ErrorWithDetails(w, http.StatusUnprocessableEntity, "validation failed", details)

		require.Equal(t, http.StatusUnprocessableEntity, w.Code)
		require.Equal(t, contentTypeProblem, w.Header().Get("Content-Type"))

		var response struct {
			Type   string         `json:"type"`
			Title  string         `json:"title"`
			Status int            `json:"status"`
			Detail string         `json:"detail"`
			Errors map[string]any `json:"errors"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		require.Equal(t, problemTypeBlank, response.Type)
		require.Equal(t, "Unprocessable Entity", response.Title)
		require.Equal(t, http.StatusUnprocessableEntity, response.Status)
		require.Equal(t, "validation failed", response.Detail)
		require.NotNil(t, response.Errors)
	})

	t.Run("omits errors field when nil", func(t *testing.T) {
		w := httptest.NewRecorder()
		ErrorWithDetails(w, http.StatusInternalServerError, "error occurred", nil)

		require.Equal(t, http.StatusInternalServerError, w.Code)

		var response map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		require.NotContains(t, response, "errors")
	})

	t.Run("handles complex errors extension", func(t *testing.T) {
		w := httptest.NewRecorder()
		details := []map[string]string{
			{"field": "email", "error": "required"},
			{"field": "password", "error": "too short"},
		}

		ErrorWithDetails(w, http.StatusBadRequest, "multiple errors", details)

		var response struct {
			Errors []map[string]any `json:"errors"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		require.Len(t, response.Errors, 2)
	})

	t.Run("returns RFC 7807 500 on unserializable errors extension", func(t *testing.T) {
		w := httptest.NewRecorder()
		ErrorWithDetails(w, http.StatusBadRequest, "error", struct {
			Ch chan int `json:"ch"`
		}{Ch: make(chan int)})

		require.Equal(t, http.StatusInternalServerError, w.Code)
		require.Equal(t, contentTypeProblem, w.Header().Get("Content-Type"))

		var response problemResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		require.Equal(t, problemTypeBlank, response.Type)
		require.Equal(t, http.StatusInternalServerError, response.Status)
	})
}

func TestConcurrentUsage(t *testing.T) {
	t.Run("handles concurrent JSON writes", func(t *testing.T) {
		const goroutines = 100
		var wg sync.WaitGroup
		wg.Add(goroutines)

		for i := range goroutines {
			go func(id int) {
				defer wg.Done()
				w := httptest.NewRecorder()
				JSON(w, http.StatusOK, map[string]int{"id": id})
				require.Equal(t, http.StatusOK, w.Code)
			}(i)
		}
		wg.Wait()
	})

	t.Run("handles concurrent Error writes", func(t *testing.T) {
		const goroutines = 100
		var wg sync.WaitGroup
		wg.Add(goroutines)

		for range goroutines {
			go func() {
				defer wg.Done()
				w := httptest.NewRecorder()
				Error(w, http.StatusBadRequest, "error")
				require.Equal(t, http.StatusBadRequest, w.Code)
			}()
		}
		wg.Wait()
	})
}

func BenchmarkJSON(b *testing.B) {
	data := map[string]string{"message": "success", "status": "ok"}
	for b.Loop() {
		w := httptest.NewRecorder()
		JSON(w, http.StatusOK, data)
	}
}

func BenchmarkError(b *testing.B) {
	for b.Loop() {
		w := httptest.NewRecorder()
		Error(w, http.StatusBadRequest, "error message")
	}
}

func BenchmarkErrorWithDetails(b *testing.B) {
	details := map[string]string{"field": "email", "error": "invalid"}
	for b.Loop() {
		w := httptest.NewRecorder()
		ErrorWithDetails(w, http.StatusBadRequest, "validation failed", details)
	}
}
