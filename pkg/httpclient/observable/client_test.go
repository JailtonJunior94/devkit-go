package observable

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/httpclient"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/suite"
)

type ClientSuite struct {
	suite.Suite
	ctx context.Context
	obs *fake.Provider
}

func TestClientSuite(t *testing.T) {
	suite.Run(t, new(ClientSuite))
}

func (s *ClientSuite) SetupTest() {
	s.ctx = context.Background()
	s.obs = fake.NewProvider()
}

func (s *ClientSuite) mustNewClient(opts ...ClientOption) *Client {
	s.T().Helper()
	client, err := NewClient(s.obs, opts...)
	s.Require().NoError(err)
	return client
}

func (s *ClientSuite) closeBody(c io.Closer) {
	s.T().Helper()
	s.NoError(c.Close())
}

func (s *ClientSuite) TestNewClient_Defaults() {
	client := s.mustNewClient()

	s.Equal(httpclient.DefaultTimeout, client.timeout)
	s.Equal(int64(httpclient.DefaultMaxRequestBodySize), client.maxBodySize)
	s.NotNil(client.instrumentation)
	s.NotNil(client.httpClient)
}

func (s *ClientSuite) TestNewClient_WithOptions() {
	client := s.mustNewClient(
		WithTimeout(5*time.Second),
		WithBodySize(1024),
	)

	s.Equal(5*time.Second, client.timeout)
	s.Equal(int64(1024), client.maxBodySize)
}

func (s *ClientSuite) TestNewClient_NilObservabilityReturnsError() {
	client, err := NewClient(nil)

	s.Nil(client)
	s.Require().Error(err)
	s.Contains(err.Error(), "observability provider cannot be nil")
}

func (s *ClientSuite) TestClient_Do_ImplementsHTTPClientInterface() {
	var _ httpclient.HTTPClient = (*Client)(nil)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := s.mustNewClient()
	req, err := http.NewRequestWithContext(s.ctx, http.MethodGet, server.URL, nil)
	s.Require().NoError(err)

	resp, err := client.Do(req)
	s.Require().NoError(err)
	defer s.closeBody(resp.Body)
	s.Equal(http.StatusOK, resp.StatusCode)
}

func (s *ClientSuite) TestClient_Get() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Equal(http.MethodGet, r.Method)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message": "success"}`))
	}))
	defer server.Close()

	client := s.mustNewClient()
	resp, err := client.Get(s.ctx, server.URL)
	s.Require().NoError(err)
	defer s.closeBody(resp.Body)

	s.Equal(http.StatusOK, resp.StatusCode)

	body, readErr := io.ReadAll(resp.Body)
	s.Require().NoError(readErr)
	s.Equal(`{"message": "success"}`, string(body))
}

func (s *ClientSuite) TestClient_Post() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Equal(http.MethodPost, r.Method)
		body, readErr := io.ReadAll(r.Body)
		s.Require().NoError(readErr)
		s.Equal("test data", string(body))
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := s.mustNewClient()
	resp, err := client.Post(s.ctx, server.URL, strings.NewReader("test data"))
	s.Require().NoError(err)
	defer s.closeBody(resp.Body)
	s.Equal(http.StatusCreated, resp.StatusCode)
}

func (s *ClientSuite) TestClient_WithHeaders() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Equal("Bearer token123", r.Header.Get("Authorization"))
		s.Equal("application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := s.mustNewClient()
	resp, err := client.Get(s.ctx, server.URL,
		httpclient.WithHeader("Authorization", "Bearer token123"),
		httpclient.WithHeader("Content-Type", "application/json"),
	)
	s.Require().NoError(err)
	defer s.closeBody(resp.Body)
	s.Equal(http.StatusOK, resp.StatusCode)
}

func (s *ClientSuite) TestClient_Retry() {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	}))
	defer server.Close()

	client := s.mustNewClient()
	resp, err := client.Get(s.ctx, server.URL,
		httpclient.WithRetry(3, 10*time.Millisecond, httpclient.DefaultRetryPolicy),
	)
	s.Require().NoError(err)
	defer s.closeBody(resp.Body)

	s.Equal(http.StatusOK, resp.StatusCode)
	s.Equal(3, attempts)

	fakeTracer := s.obs.Tracer().(*fake.FakeTracer)
	spans := fakeTracer.GetSpans()
	s.NotEmpty(spans)

	found := false
	for _, span := range spans {
		if span.Name == "http.client.request" {
			found = true
		}
	}
	s.True(found, "expected http.client.request span")
}

func (s *ClientSuite) TestClient_NoRetry() {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := s.mustNewClient()
	resp, err := client.Get(s.ctx, server.URL)
	s.Require().NoError(err)
	defer s.closeBody(resp.Body)

	s.Equal(http.StatusServiceUnavailable, resp.StatusCode)
	s.Equal(1, attempts)
}

func (s *ClientSuite) TestClient_RetryMaxAttemptsBodyReadable() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"unavailable"}`))
	}))
	defer server.Close()

	client := s.mustNewClient()
	resp, err := client.Get(s.ctx, server.URL, httpclient.WithRetry(2, 0, httpclient.DefaultRetryPolicy))
	s.Require().NoError(err)
	s.Require().NotNil(resp)
	defer s.closeBody(resp.Body)

	s.Equal(http.StatusServiceUnavailable, resp.StatusCode)

	body, readErr := io.ReadAll(resp.Body)
	s.Require().NoError(readErr)
	s.Equal(`{"error":"unavailable"}`, string(body))
}

func (s *ClientSuite) TestClient_RetryBodyTooLarge() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := s.mustNewClient(WithBodySize(10))
	resp, err := client.Post(s.ctx, server.URL,
		strings.NewReader("this body is larger than 10 bytes"),
		httpclient.WithRetry(3, 10*time.Millisecond, httpclient.DefaultRetryPolicy),
	)
	if resp != nil {
		defer s.closeBody(resp.Body)
	}

	s.Require().Error(err)
	s.True(strings.Contains(err.Error(), "request body exceeds maximum allowed size"))
}

func (s *ClientSuite) TestClient_Metrics() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := s.mustNewClient()
	resp, err := client.Get(s.ctx, server.URL)
	s.Require().NoError(err)
	defer s.closeBody(resp.Body)

	fakeMetrics := s.obs.Metrics().(*fake.FakeMetrics)

	requestCounter := fakeMetrics.GetCounter("http.client.request.count")
	s.Require().NotNil(requestCounter)
	s.NotEmpty(requestCounter.GetValues())

	latencyHistogram := fakeMetrics.GetHistogram("http.client.request.duration")
	s.Require().NotNil(latencyHistogram)
	s.NotEmpty(latencyHistogram.GetValues())
}

func (s *ClientSuite) TestClient_ErrorMetrics() {
	client := s.mustNewClient()
	resp, err := client.Get(s.ctx, "http://invalid-host-that-does-not-exist.local")
	if resp != nil {
		defer s.closeBody(resp.Body)
	}

	s.Require().Error(err)

	fakeMetrics := s.obs.Metrics().(*fake.FakeMetrics)
	errorCounter := fakeMetrics.GetCounter("http.client.request.errors")
	s.Require().NotNil(errorCounter)
	s.NotEmpty(errorCounter.GetValues())
}

func (s *ClientSuite) TestClient_Timeout() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := s.mustNewClient(WithTimeout(50 * time.Millisecond))
	resp, err := client.Get(s.ctx, server.URL)
	if resp != nil {
		defer s.closeBody(resp.Body)
	}

	s.Require().Error(err)
}

func (s *ClientSuite) TestClient_RetryDoesNotRetryContextErrors() {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(s.ctx, 50*time.Millisecond)
	defer cancel()

	startTime := time.Now()
	client := s.mustNewClient()
	resp, err := client.Get(ctx, server.URL,
		httpclient.WithRetry(3, 10*time.Millisecond, httpclient.DefaultRetryPolicy),
	)
	elapsed := time.Since(startTime)
	if resp != nil {
		defer s.closeBody(resp.Body)
	}

	s.Require().Error(err)
	s.True(errors.Is(err, context.DeadlineExceeded))
	s.Equal(int32(1), attempts.Load())
	s.Less(elapsed, 100*time.Millisecond)
}

func (s *ClientSuite) TestClient_RetryValidation() {
	scenarios := []struct {
		name        string
		maxAttempts int
		backoff     time.Duration
		policy      httpclient.RetryPolicy
		shouldError bool
		errorMsg    string
	}{
		{name: "valid", maxAttempts: 3, backoff: time.Second, policy: httpclient.DefaultRetryPolicy, shouldError: false},
		{name: "exceeds max attempts", maxAttempts: httpclient.MaxRetryAttempts + 1, backoff: time.Second, policy: httpclient.DefaultRetryPolicy, shouldError: true, errorMsg: "maxAttempts"},
		{name: "negative backoff", maxAttempts: 3, backoff: -time.Second, policy: httpclient.DefaultRetryPolicy, shouldError: true, errorMsg: "negative"},
		{name: "backoff exceeds limit", maxAttempts: 3, backoff: httpclient.MaxRetryBackoff + time.Second, policy: httpclient.DefaultRetryPolicy, shouldError: true, errorMsg: "backoff"},
		{name: "nil policy", maxAttempts: 3, backoff: time.Second, policy: nil, shouldError: true, errorMsg: "policy cannot be nil"},
		{name: "zero attempts disabled", maxAttempts: 0, backoff: time.Second, policy: httpclient.DefaultRetryPolicy, shouldError: false},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			client := s.mustNewClient()
			resp, err := client.Get(s.ctx, server.URL,
				httpclient.WithRetry(sc.maxAttempts, sc.backoff, sc.policy),
			)
			if resp != nil {
				defer s.closeBody(resp.Body)
			}

			if sc.shouldError {
				s.Require().Error(err)
				s.Contains(err.Error(), sc.errorMsg)
			} else {
				s.Require().NoError(err)
			}
		})
	}
}

func (s *ClientSuite) TestMakeRequest_WithObservableClient() {
	type successBody struct {
		Status string `json:"status"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(successBody{Status: "ok"})
	}))
	defer server.Close()

	client := s.mustNewClient()
	result, errBody, err := httpclient.MakeRequest[successBody, any](
		s.ctx, client, http.MethodGet, server.URL, nil, nil,
	)

	s.Require().NoError(err)
	s.Nil(errBody)
	s.Require().NotNil(result)
	s.Equal("ok", result.Status)
}

type ClassifyErrorSuite struct {
	suite.Suite
}

func TestClassifyErrorSuite(t *testing.T) {
	suite.Run(t, new(ClassifyErrorSuite))
}

func (s *ClassifyErrorSuite) TestClassifyError() {
	scenarios := []struct {
		name     string
		err      error
		expected string
	}{
		{name: "nil", err: nil, expected: "none"},
		{name: "deadline exceeded", err: context.DeadlineExceeded, expected: "timeout"},
		{name: "canceled", err: context.Canceled, expected: "canceled"},
		{name: "body too large", err: httpclient.ErrRequestBodyTooLarge, expected: "body_too_large"},
		{name: "unknown", err: fmt.Errorf("some error"), expected: "unknown"},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			s.Equal(sc.expected, classifyError(sc.err))
		})
	}
}
