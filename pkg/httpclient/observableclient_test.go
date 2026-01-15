package httpclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
)

// Helper function to create client and fail test on error
func mustNewObservableClient(t *testing.T, o11y observability.Observability, opts ...ClientOption) *ObservableClient {
	t.Helper()
	client, err := NewObservableClient(o11y, opts...)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	return client
}

func TestNewObservableClient(t *testing.T) {
	obs := fake.NewProvider()

	client := mustNewObservableClient(t, obs)

	if client == nil {
		t.Fatal("expected client to be created")
	}

	if client.timeout != DefaultTimeout {
		t.Errorf("expected timeout %v, got %v", DefaultTimeout, client.timeout)
	}

	if client.maxBodySize != DefaultMaxRequestBodySize {
		t.Errorf("expected maxBodySize %d, got %d", DefaultMaxRequestBodySize, client.maxBodySize)
	}

	if client.instrumentation == nil {
		t.Error("expected instrumentation to be initialized")
	}
}

func TestNewObservableClientWithOptions(t *testing.T) {
	obs := fake.NewProvider()

	customTimeout := 5 * time.Second
	customBodySize := int64(1024 * 1024) // 1MB

	client := mustNewObservableClient(t, obs,
		WithClientTimeout(customTimeout),
		WithMaxBodySize(customBodySize),
	)

	if client.timeout != customTimeout {
		t.Errorf("expected timeout %v, got %v", customTimeout, client.timeout)
	}

	if client.maxBodySize != customBodySize {
		t.Errorf("expected maxBodySize %d, got %d", customBodySize, client.maxBodySize)
	}
}

func TestNewObservableClientErrorOnNilObservability(t *testing.T) {
	client, err := NewObservableClient(nil)

	if err == nil {
		t.Fatal("expected error when observability provider is nil")
	}

	if client != nil {
		t.Error("expected nil client when error occurs")
	}

	expectedMsg := "httpclient: observability provider cannot be nil"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestObservableClientGet(t *testing.T) {
	obs := fake.NewProvider()
	client := mustNewObservableClient(t, obs)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "success"}`))
	}))
	defer server.Close()

	ctx := context.Background()
	resp, err := client.Get(ctx, server.URL)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	expected := `{"message": "success"}`
	if string(body) != expected {
		t.Errorf("expected body %s, got %s", expected, string(body))
	}
}

func TestObservableClientPost(t *testing.T) {
	obs := fake.NewProvider()
	client := mustNewObservableClient(t, obs)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		body, _ := io.ReadAll(r.Body)
		if string(body) != "test data" {
			t.Errorf("expected body 'test data', got %s", string(body))
		}

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id": "123"}`))
	}))
	defer server.Close()

	ctx := context.Background()
	resp, err := client.Post(ctx, server.URL, strings.NewReader("test data"))

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status 201, got %d", resp.StatusCode)
	}
}

func TestObservableClientWithHeaders(t *testing.T) {
	obs := fake.NewProvider()
	client := mustNewObservableClient(t, obs)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer token123" {
			t.Errorf("expected Authorization 'Bearer token123', got %s", auth)
		}

		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("expected Content-Type 'application/json', got %s", contentType)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx := context.Background()
	resp, err := client.Get(ctx, server.URL,
		WithHeader("Authorization", "Bearer token123"),
		WithHeader("Content-Type", "application/json"),
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestObservableClientRetry(t *testing.T) {
	obs := fake.NewProvider()
	client := mustNewObservableClient(t, obs)

	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer server.Close()

	ctx := context.Background()
	resp, err := client.Get(ctx, server.URL,
		WithRetry(3, 10*time.Millisecond, DefaultNewRetryPolicy),
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}

	fakeTracer := obs.Tracer().(*fake.FakeTracer)
	spans := fakeTracer.GetSpans()

	if len(spans) == 0 {
		t.Fatal("expected spans to be created")
	}

	// Note: FakeTracer.SpanFromContext() returns a new empty span instead of the actual span.
	// This is a known limitation of the fake provider.
	// In production with the real OTEL provider, SpanFromContext() works correctly
	// and retry attributes (retry.enabled, retry.attempt) are properly set.
	// For this test, we verify that the span was created and the retry logic executed.

	foundHTTPSpan := false
	for _, span := range spans {
		if span.Name == "http.client.request" {
			foundHTTPSpan = true
		}
	}

	if !foundHTTPSpan {
		t.Error("expected http.client.request span to be created")
	}
}

func TestObservableClientNoRetry(t *testing.T) {
	obs := fake.NewProvider()
	client := mustNewObservableClient(t, obs)

	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	ctx := context.Background()
	resp, err := client.Get(ctx, server.URL)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", resp.StatusCode)
	}

	if attempts != 1 {
		t.Errorf("expected 1 attempt (no retry), got %d", attempts)
	}
}

func TestObservableClientRetryBodyBuffering(t *testing.T) {
	obs := fake.NewProvider()
	client := mustNewObservableClient(t, obs,
		WithMaxBodySize(10), // Small limit: 10 bytes
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	ctx := context.Background()

	largeBody := strings.NewReader("this body is larger than 10 bytes")
	resp, err := client.Post(ctx, server.URL, largeBody,
		WithRetry(3, 10*time.Millisecond, DefaultNewRetryPolicy),
	)

	if err == nil {
		t.Fatal("expected error for body too large")
	}

	if resp != nil {
		t.Error("expected nil response when body is too large")
	}

	if !strings.Contains(err.Error(), "request body exceeds maximum allowed size") {
		t.Errorf("expected error about body size, got %v", err)
	}
}

func TestObservableClientMetrics(t *testing.T) {
	obs := fake.NewProvider()
	client := mustNewObservableClient(t, obs)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx := context.Background()
	resp, err := client.Get(ctx, server.URL)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	fakeMetrics := obs.Metrics().(*fake.FakeMetrics)

	requestCounter := fakeMetrics.GetCounter("http.client.request.count")
	if requestCounter == nil {
		t.Fatal("expected request counter to be created")
	}

	values := requestCounter.GetValues()
	if len(values) == 0 {
		t.Error("expected request counter to be incremented")
	}

	latencyHistogram := fakeMetrics.GetHistogram("http.client.request.duration")
	if latencyHistogram == nil {
		t.Fatal("expected latency histogram to be created")
	}

	histValues := latencyHistogram.GetValues()
	if len(histValues) == 0 {
		t.Error("expected latency histogram to have values")
	}
}

func TestObservableClientError(t *testing.T) {
	obs := fake.NewProvider()
	client := mustNewObservableClient(t, obs)

	ctx := context.Background()
	resp, err := client.Get(ctx, "http://invalid-host-that-does-not-exist.local")

	if err == nil {
		t.Fatal("expected error for invalid host")
	}

	if resp != nil {
		t.Error("expected nil response on error")
	}

	fakeMetrics := obs.Metrics().(*fake.FakeMetrics)
	errorCounter := fakeMetrics.GetCounter("http.client.request.errors")

	if errorCounter == nil {
		t.Fatal("expected error counter to be created")
	}

	errorValues := errorCounter.GetValues()
	if len(errorValues) == 0 {
		t.Error("expected error counter to be incremented")
	}
}

func TestRetryPolicies(t *testing.T) {
	tests := []struct {
		name     string
		policy   NewRetryPolicy
		err      error
		resp     *http.Response
		expected bool
	}{
		{
			name:     "DefaultPolicy - network error",
			policy:   DefaultNewRetryPolicy,
			err:      fmt.Errorf("network error"),
			resp:     nil,
			expected: true,
		},
		{
			name:   "DefaultPolicy - 5xx error",
			policy: DefaultNewRetryPolicy,
			err:    nil,
			resp: &http.Response{
				StatusCode: http.StatusInternalServerError,
			},
			expected: true,
		},
		{
			name:   "DefaultPolicy - 4xx error (no retry)",
			policy: DefaultNewRetryPolicy,
			err:    nil,
			resp: &http.Response{
				StatusCode: http.StatusBadRequest,
			},
			expected: false,
		},
		{
			name:   "IdempotentPolicy - 429 rate limit",
			policy: IdempotentNewRetryPolicy,
			err:    nil,
			resp: &http.Response{
				StatusCode: http.StatusTooManyRequests,
			},
			expected: true,
		},
		{
			name:     "NoRetryPolicy - always false",
			policy:   NoNewRetryPolicy,
			err:      fmt.Errorf("error"),
			resp:     nil,
			expected: false,
		},
		{
			name:     "DefaultPolicy - context canceled (no retry)",
			policy:   DefaultNewRetryPolicy,
			err:      context.Canceled,
			resp:     nil,
			expected: false,
		},
		{
			name:     "DefaultPolicy - context deadline exceeded (no retry)",
			policy:   DefaultNewRetryPolicy,
			err:      context.DeadlineExceeded,
			resp:     nil,
			expected: false,
		},
		{
			name:     "IdempotentPolicy - context canceled (no retry)",
			policy:   IdempotentNewRetryPolicy,
			err:      context.Canceled,
			resp:     nil,
			expected: false,
		},
		{
			name:     "IdempotentPolicy - context deadline exceeded (no retry)",
			policy:   IdempotentNewRetryPolicy,
			err:      context.DeadlineExceeded,
			resp:     nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.policy(tt.err, tt.resp)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestObservableClientRetryDoesNotRetryContextErrors(t *testing.T) {
	o11y := fake.NewProvider()
	client := mustNewObservableClient(t, o11y)

	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		// Simulate slow response that exceeds timeout
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Context with very short timeout (will exceed on first attempt)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	startTime := time.Now()
	resp, err := client.Get(ctx, server.URL,
		WithRetry(3, 10*time.Millisecond, DefaultNewRetryPolicy),
	)
	elapsed := time.Since(startTime)

	// Should fail with deadline exceeded
	if err == nil {
		t.Fatal("expected error for timeout")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}

	if resp != nil {
		t.Error("expected nil response on timeout")
	}

	// Should only attempt once (context error should not be retried)
	actualAttempts := attempts.Load()
	if actualAttempts != 1 {
		t.Errorf("expected 1 attempt (no retry on context error), got %d", actualAttempts)
	}

	// Should return quickly (not wait for backoff)
	// Elapsed should be close to timeout (50ms) + small overhead, NOT 50ms + backoff * retries
	if elapsed > 100*time.Millisecond {
		t.Errorf("expected fast return on context error, took %v", elapsed)
	}
}

func TestWithRetryValidation(t *testing.T) {
	tests := []struct {
		name        string
		maxAttempts int
		backoff     time.Duration
		policy      NewRetryPolicy
		shouldPanic bool
		panicMsg    string
	}{
		{
			name:        "valid configuration",
			maxAttempts: 3,
			backoff:     time.Second,
			policy:      DefaultNewRetryPolicy,
			shouldPanic: false,
		},
		{
			name:        "max attempts at limit",
			maxAttempts: MaxRetryAttempts,
			backoff:     time.Second,
			policy:      DefaultNewRetryPolicy,
			shouldPanic: false,
		},
		{
			name:        "max attempts exceeds limit",
			maxAttempts: MaxRetryAttempts + 1,
			backoff:     time.Second,
			policy:      DefaultNewRetryPolicy,
			shouldPanic: true,
			panicMsg:    "maxAttempts",
		},
		{
			name:        "max attempts way too high",
			maxAttempts: 1000,
			backoff:     time.Second,
			policy:      DefaultNewRetryPolicy,
			shouldPanic: true,
			panicMsg:    "maxAttempts",
		},
		{
			name:        "negative backoff",
			maxAttempts: 3,
			backoff:     -1 * time.Second,
			policy:      DefaultNewRetryPolicy,
			shouldPanic: true,
			panicMsg:    "negative",
		},
		{
			name:        "backoff exceeds limit",
			maxAttempts: 3,
			backoff:     MaxRetryBackoff + time.Second,
			policy:      DefaultNewRetryPolicy,
			shouldPanic: true,
			panicMsg:    "backoff",
		},
		{
			name:        "nil policy",
			maxAttempts: 3,
			backoff:     time.Second,
			policy:      nil,
			shouldPanic: true,
			panicMsg:    "policy cannot be nil",
		},
		{
			name:        "zero attempts (disabled)",
			maxAttempts: 0,
			backoff:     time.Second,
			policy:      DefaultNewRetryPolicy,
			shouldPanic: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if tt.shouldPanic {
					if r == nil {
						t.Errorf("expected panic but didn't panic")
					} else {
						panicMsg := fmt.Sprint(r)
						if !strings.Contains(panicMsg, tt.panicMsg) {
							t.Errorf("expected panic message to contain %q, got %q", tt.panicMsg, panicMsg)
						}
					}
				} else {
					if r != nil {
						t.Errorf("unexpected panic: %v", r)
					}
				}
			}()

			// Create option
			opt := WithRetry(tt.maxAttempts, tt.backoff, tt.policy)

			// Apply to config
			cfg := &requestConfig{}
			opt(cfg)

			// Validate if not panicking
			if !tt.shouldPanic && tt.maxAttempts > 0 {
				if !cfg.retryEnabled {
					t.Error("expected retry to be enabled")
				}
				if cfg.retryMaxAttempts != tt.maxAttempts {
					t.Errorf("expected maxAttempts %d, got %d", tt.maxAttempts, cfg.retryMaxAttempts)
				}
			}
		})
	}
}

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: "none",
		},
		{
			name:     "context deadline exceeded",
			err:      context.DeadlineExceeded,
			expected: "timeout",
		},
		{
			name:     "context canceled",
			err:      context.Canceled,
			expected: "canceled",
		},
		{
			name:     "body too large",
			err:      ErrRequestBodyTooLarge,
			expected: "body_too_large",
		},
		{
			name:     "unknown error",
			err:      fmt.Errorf("some error"),
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyError(tt.err)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}
