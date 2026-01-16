package common

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestResponseWriter_WriteHeader_Once(t *testing.T) {
	w := httptest.NewRecorder()
	rw := NewResponseWriter(w)

	rw.WriteHeader(http.StatusOK)
	rw.WriteHeader(http.StatusInternalServerError) // Should be ignored

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestResponseWriter_Write_ImplicitHeader(t *testing.T) {
	w := httptest.NewRecorder()
	rw := NewResponseWriter(w)

	_, err := rw.Write([]byte("test"))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Write should trigger implicit WriteHeader(200)
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	if w.Body.String() != "test" {
		t.Errorf("expected body 'test', got '%s'", w.Body.String())
	}
}

func TestResponseWriter_HeaderWritten_False(t *testing.T) {
	w := httptest.NewRecorder()
	rw := NewResponseWriter(w)

	if rw.HeaderWritten() {
		t.Error("expected HeaderWritten to be false initially")
	}
}

func TestResponseWriter_HeaderWritten_AfterWriteHeader(t *testing.T) {
	w := httptest.NewRecorder()
	rw := NewResponseWriter(w)

	rw.WriteHeader(http.StatusOK)

	if !rw.HeaderWritten() {
		t.Error("expected HeaderWritten to be true after WriteHeader")
	}
}

func TestResponseWriter_HeaderWritten_AfterWrite(t *testing.T) {
	w := httptest.NewRecorder()
	rw := NewResponseWriter(w)

	_, _ = rw.Write([]byte("test"))

	if !rw.HeaderWritten() {
		t.Error("expected HeaderWritten to be true after Write")
	}
}

func TestResponseWriter_ConcurrentWriteHeader(t *testing.T) {
	// Test thread-safety: multiple goroutines calling WriteHeader
	w := httptest.NewRecorder()
	rw := NewResponseWriter(w)

	var wg sync.WaitGroup
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rw.WriteHeader(http.StatusOK)
		}()
	}

	wg.Wait()

	// Should only write header once despite concurrent calls
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	if !rw.HeaderWritten() {
		t.Error("expected HeaderWritten to be true")
	}
}

func TestResponseWriter_ConcurrentWrite(t *testing.T) {
	// Test thread-safety: multiple goroutines calling Write
	w := httptest.NewRecorder()
	rw := NewResponseWriter(w)

	var wg sync.WaitGroup
	numGoroutines := 10
	message := "test"

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = rw.Write([]byte(message))
		}()
	}

	wg.Wait()

	// Should have written all messages
	body := w.Body.String()
	expectedLength := len(message) * numGoroutines

	if len(body) != expectedLength {
		t.Errorf("expected body length %d, got %d", expectedLength, len(body))
	}

	if !rw.HeaderWritten() {
		t.Error("expected HeaderWritten to be true")
	}
}

func TestResponseWriter_ConcurrentHeaderWritten(t *testing.T) {
	// Test thread-safety: concurrent reads of HeaderWritten
	w := httptest.NewRecorder()
	rw := NewResponseWriter(w)

	var wg sync.WaitGroup
	numGoroutines := 100

	// Start readers
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = rw.HeaderWritten() // Just read, should not panic
		}()
	}

	// Start one writer
	wg.Add(1)
	go func() {
		defer wg.Done()
		rw.WriteHeader(http.StatusOK)
	}()

	wg.Wait()

	// Should not panic and HeaderWritten should be true
	if !rw.HeaderWritten() {
		t.Error("expected HeaderWritten to be true")
	}
}

func TestResponseWriter_ConcurrentMixedOperations(t *testing.T) {
	// Test thread-safety: mix of WriteHeader, Write, and HeaderWritten
	w := httptest.NewRecorder()
	rw := NewResponseWriter(w)

	var wg sync.WaitGroup
	numGoroutines := 50

	// WriteHeader goroutines
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rw.WriteHeader(http.StatusOK)
		}()
	}

	// Write goroutines
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = rw.Write([]byte("x"))
		}()
	}

	// HeaderWritten goroutines
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = rw.HeaderWritten()
		}()
	}

	wg.Wait()

	// Should not panic and HeaderWritten should be true
	if !rw.HeaderWritten() {
		t.Error("expected HeaderWritten to be true")
	}
}

func TestResponseWriter_PreventDoubleWrite_Scenario(t *testing.T) {
	// Simulate panic recovery scenario where we check if headers were written
	w := httptest.NewRecorder()
	rw := NewResponseWriter(w)

	// Handler writes response
	rw.WriteHeader(http.StatusOK)
	_, _ = rw.Write([]byte("success"))

	// Panic recovery checks if it can write error
	if !rw.HeaderWritten() {
		t.Error("this code should not execute - headers already written")
		rw.WriteHeader(http.StatusInternalServerError)
	}

	// Verify original response is intact
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	if w.Body.String() != "success" {
		t.Errorf("expected body 'success', got '%s'", w.Body.String())
	}
}

// Benchmark for checking performance overhead
func BenchmarkResponseWriter_WriteHeader(b *testing.B) {
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		rw := NewResponseWriter(w)
		rw.WriteHeader(http.StatusOK)
	}
}

func BenchmarkResponseWriter_Write(b *testing.B) {
	data := []byte("test data")
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		rw := NewResponseWriter(w)
		_, _ = rw.Write(data)
	}
}

func BenchmarkResponseWriter_HeaderWritten(b *testing.B) {
	w := httptest.NewRecorder()
	rw := NewResponseWriter(w)
	rw.WriteHeader(http.StatusOK)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = rw.HeaderWritten()
	}
}
