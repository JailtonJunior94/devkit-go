package httpserver

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/otel"
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

func TestRequestID_PreservesExistingContextID(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := GetRequestID(r.Context()); got != "existing-request-id" {
			t.Errorf("expected existing-request-id, got %s", got)
		}
		w.WriteHeader(http.StatusOK)
	})
	wrapped := RequestID(handler)

	ctx := context.WithValue(context.Background(), ContextKeyRequestID, "existing-request-id")
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if got := rec.Header().Get(headerRequestID); got != "existing-request-id" {
		t.Errorf("expected response request ID existing-request-id, got %s", got)
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
		_, _ = w.Write([]byte("OK"))
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
		_, _ = w.Write([]byte("OK"))
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

func TestObservabilityMiddleware(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		headers           map[string]string
		handler           http.HandlerFunc
		wantRequestID     string
		wantCorrelationID string
		wantStatus        int
		wantBytes         int64
	}{
		{
			name: "request with correlation headers delegates request and response to hook",
			headers: map[string]string{
				headerRequestID:     "req-123",
				headerCorrelationID: "corr-456",
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte("created"))
			},
			wantRequestID:     "req-123",
			wantCorrelationID: "corr-456",
			wantStatus:        http.StatusCreated,
			wantBytes:         int64(len("created")),
		},
		{
			name: "request without correlation headers still creates root scope",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name: "server error response is finished as HTTP error status",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "failed", http.StatusServiceUnavailable)
			},
			wantStatus: http.StatusServiceUnavailable,
			wantBytes:  int64(len("failed\n")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			hook := &recordingHTTPInstrumentation{}
			var handlerRequestID string
			wrapped := ObservabilityMiddleware(hook)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerRequestID = GetRequestID(r.Context())
				tt.handler(w, r)
			}))

			req := httptest.NewRequest(http.MethodPost, "/orders/123", nil)
			req.RemoteAddr = "10.0.0.1:1234"
			req.Header.Set("User-Agent", "devkit-test")
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}
			rec := httptest.NewRecorder()

			wrapped.ServeHTTP(rec, req)

			if len(hook.requests) != 1 {
				t.Fatalf("expected 1 root HTTP scope, got %d", len(hook.requests))
			}
			gotReq := hook.requests[0]
			if gotReq.Method != http.MethodPost {
				t.Errorf("expected method POST, got %s", gotReq.Method)
			}
			if gotReq.Target != "/orders/123" {
				t.Errorf("expected target /orders/123, got %s", gotReq.Target)
			}
			if gotReq.RemoteAddr != "10.0.0.1:1234" {
				t.Errorf("expected remote addr 10.0.0.1:1234, got %s", gotReq.RemoteAddr)
			}
			if gotReq.UserAgent != "devkit-test" {
				t.Errorf("expected user agent devkit-test, got %s", gotReq.UserAgent)
			}
			if tt.wantRequestID != "" && gotReq.RequestID != tt.wantRequestID {
				t.Errorf("expected request ID %s, got %s", tt.wantRequestID, gotReq.RequestID)
			}
			if gotReq.RequestID == "" {
				t.Error("expected request ID to be delegated to hook")
			}
			if handlerRequestID != gotReq.RequestID {
				t.Errorf("expected handler context request ID %s, got %s", gotReq.RequestID, handlerRequestID)
			}
			if gotReq.CorrelationID != tt.wantCorrelationID {
				t.Errorf("expected correlation ID %s, got %s", tt.wantCorrelationID, gotReq.CorrelationID)
			}
			if rec.Header().Get(headerRequestID) != gotReq.RequestID {
				t.Errorf("expected response request ID %s, got %s", gotReq.RequestID, rec.Header().Get(headerRequestID))
			}

			scope := hook.scopes[0]
			if len(scope.responses) != 1 {
				t.Fatalf("expected 1 finished response, got %d", len(scope.responses))
			}
			if scope.responses[0].StatusCode != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, scope.responses[0].StatusCode)
			}
			if scope.responses[0].Bytes != tt.wantBytes {
				t.Errorf("expected bytes %d, got %d", tt.wantBytes, scope.responses[0].Bytes)
			}
		})
	}
}

func TestObservabilityMiddleware_NilHookIsNoop(t *testing.T) {
	t.Parallel()

	called := false
	wrapped := ObservabilityMiddleware(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusAccepted)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	wrapped.ServeHTTP(rec, req)

	if !called {
		t.Error("expected handler to be called")
	}
	if rec.Code != http.StatusAccepted {
		t.Errorf("expected status 202, got %d", rec.Code)
	}
}

func TestRecordHTTPHandlerError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("handler failed")
	scope := &recordingHTTPRequestScope{}
	ctx := context.WithValue(context.Background(), httpRequestScopeContextKey{}, scope)

	recordHTTPHandlerError(ctx, expectedErr)
	recordHTTPHandlerError(context.Background(), expectedErr)
	recordHTTPHandlerError(ctx, nil)

	if len(scope.errors) != 1 {
		t.Fatalf("expected 1 recorded error, got %d", len(scope.errors))
	}
	if !errors.Is(scope.errors[0], expectedErr) {
		t.Errorf("expected recorded error %v, got %v", expectedErr, scope.errors[0])
	}
}

type recordingHTTPInstrumentation struct {
	requests []otel.HTTPRequest
	scopes   []*recordingHTTPRequestScope
}

func (r *recordingHTTPInstrumentation) StartRequest(
	ctx context.Context,
	req otel.HTTPRequest,
) (context.Context, otel.HTTPRequestScope) {
	scope := &recordingHTTPRequestScope{}
	r.requests = append(r.requests, req)
	r.scopes = append(r.scopes, scope)
	return ctx, scope
}

type recordingHTTPRequestScope struct {
	errors    []error
	responses []otel.HTTPResponse
}

func (r *recordingHTTPRequestScope) OnError(err error) {
	r.errors = append(r.errors, err)
}

func (r *recordingHTTPRequestScope) Finish(resp otel.HTTPResponse) {
	r.responses = append(r.responses, resp)
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
