package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRequestID_GeneratesID(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := GetRequestID(r.Context())
		if requestID == "" {
			t.Error("expected request ID, got empty string")
		}
		w.WriteHeader(http.StatusOK)
	})

	wrapped := RequestID(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	// Check X-Request-ID header is set
	headerID := rec.Header().Get("X-Request-ID")
	if headerID == "" {
		t.Error("expected X-Request-ID header, got empty")
	}
}

func TestRequestID_UniqueIDs(t *testing.T) {
	ids := make(map[string]bool)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := GetRequestID(r.Context())
		if ids[requestID] {
			t.Errorf("duplicate request ID: %s", requestID)
		}
		ids[requestID] = true
		w.WriteHeader(http.StatusOK)
	})

	wrapped := RequestID(handler)

	// Generate multiple request IDs
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
	}

	if len(ids) != 100 {
		t.Errorf("expected 100 unique IDs, got %d", len(ids))
	}
}

func TestGetRequestID_NilContext(t *testing.T) {
	id := GetRequestID(nil)
	if id != "" {
		t.Errorf("expected empty string for nil context, got %s", id)
	}
}

func TestGetRequestID_NoID(t *testing.T) {
	ctx := context.Background()
	id := GetRequestID(ctx)
	if id != "" {
		t.Errorf("expected empty string for context without ID, got %s", id)
	}
}

func TestGetRequestID_WithID(t *testing.T) {
	ctx := context.WithValue(context.Background(), ContextKeyRequestID, "test-id-123")
	id := GetRequestID(ctx)
	if id != "test-id-123" {
		t.Errorf("expected test-id-123, got %s", id)
	}
}

func TestGenerateFallbackID(t *testing.T) {
	ids := make(map[string]bool)

	// Generate multiple IDs
	for i := 0; i < 1000; i++ {
		id := generateFallbackID()
		if ids[id] {
			t.Errorf("duplicate fallback ID: %s", id)
		}
		ids[id] = true

		// Check format (should be hex characters)
		if len(id) != 24 { // 8 bytes timestamp + 4 bytes random = 12 bytes = 24 hex chars
			t.Errorf("expected 24 char fallback ID, got %d chars: %s", len(id), id)
		}
	}
}

func TestRecovery_NoPanic(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	wrapped := Recovery(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if rec.Body.String() != "OK" {
		t.Errorf("expected body OK, got %s", rec.Body.String())
	}
}

func TestRecovery_WithPanic(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	wrapped := Recovery(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	// Should not panic
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

func TestRecovery_WithRequestID(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	wrapped := RequestID(Recovery(handler))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	// Should not panic
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

func TestCORS_BasicHeaders(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := CORS("*", "GET,POST", "Content-Type")(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("expected origin *, got %s", rec.Header().Get("Access-Control-Allow-Origin"))
	}
	if rec.Header().Get("Access-Control-Allow-Methods") != "GET,POST" {
		t.Errorf("expected methods GET,POST, got %s", rec.Header().Get("Access-Control-Allow-Methods"))
	}
	if rec.Header().Get("Access-Control-Allow-Headers") != "Content-Type" {
		t.Errorf("expected headers Content-Type, got %s", rec.Header().Get("Access-Control-Allow-Headers"))
	}
}

func TestCORS_Preflight(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for preflight")
	})

	wrapped := CORS("*", "GET,POST", "Content-Type")(handler)

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", rec.Code)
	}
}

func TestContentType(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := ContentType("text/plain")(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Type") != "text/plain" {
		t.Errorf("expected Content-Type text/plain, got %s", rec.Header().Get("Content-Type"))
	}
}

func TestJSONContentType(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := JSONContentType(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", rec.Header().Get("Content-Type"))
	}
}

func TestSecurityHeaders(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := SecurityHeaders(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	tests := []struct {
		header   string
		expected string
	}{
		{"X-Content-Type-Options", "nosniff"},
		{"X-Frame-Options", "DENY"},
		{"X-XSS-Protection", "1; mode=block"},
		{"Referrer-Policy", "strict-origin-when-cross-origin"},
	}

	for _, tt := range tests {
		if rec.Header().Get(tt.header) != tt.expected {
			t.Errorf("expected %s: %s, got %s", tt.header, tt.expected, rec.Header().Get(tt.header))
		}
	}
}

func TestTimeout_NoTimeout(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	wrapped := Timeout(1 * time.Second)(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestTimeout_Exceeded(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(1 * time.Second):
			w.WriteHeader(http.StatusOK)
		}
	})

	wrapped := Timeout(50 * time.Millisecond)(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusGatewayTimeout {
		t.Errorf("expected status 504, got %d", rec.Code)
	}
}

func TestContextKey_String(t *testing.T) {
	key := ContextKeyRequestID
	if string(key) != "request-id" {
		t.Errorf("expected request-id, got %s", string(key))
	}
}

func TestMiddlewareChain(t *testing.T) {
	var sequence []string

	m1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sequence = append(sequence, "m1-before")
			next.ServeHTTP(w, r)
			sequence = append(sequence, "m1-after")
		})
	}

	m2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sequence = append(sequence, "m2-before")
			next.ServeHTTP(w, r)
			sequence = append(sequence, "m2-after")
		})
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sequence = append(sequence, "handler")
		w.WriteHeader(http.StatusOK)
	})

	wrapped := Middlewares(handler, m1, m2)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	expected := []string{"m1-before", "m2-before", "handler", "m2-after", "m1-after"}
	if len(sequence) != len(expected) {
		t.Fatalf("expected %d items, got %d: %v", len(expected), len(sequence), sequence)
	}
	for i, v := range expected {
		if sequence[i] != v {
			t.Errorf("expected sequence[%d] = %s, got %s", i, v, sequence[i])
		}
	}
}

func TestCORS_SpecificOrigin(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := CORS("https://example.com", "GET,POST,PUT,DELETE", "Content-Type,Authorization")(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	origin := rec.Header().Get("Access-Control-Allow-Origin")
	if origin != "https://example.com" {
		t.Errorf("expected origin https://example.com, got %s", origin)
	}

	methods := rec.Header().Get("Access-Control-Allow-Methods")
	if !strings.Contains(methods, "DELETE") {
		t.Errorf("expected methods to contain DELETE, got %s", methods)
	}

	headers := rec.Header().Get("Access-Control-Allow-Headers")
	if !strings.Contains(headers, "Authorization") {
		t.Errorf("expected headers to contain Authorization, got %s", headers)
	}
}
