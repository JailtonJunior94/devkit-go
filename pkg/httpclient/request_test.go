package httpclient

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type MakeRequestSuite struct {
	suite.Suite
	ctx context.Context
}

func TestMakeRequestSuite(t *testing.T) {
	suite.Run(t, new(MakeRequestSuite))
}

func (s *MakeRequestSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *MakeRequestSuite) TestMakeRequest_Success() {
	type successBody struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(successBody{ID: "1", Name: "test"})
	}))
	defer server.Close()

	client := NewHTTPClient()
	result, errBody, err := MakeRequest[successBody, any](s.ctx, client, http.MethodGet, server.URL, nil, nil)

	s.Require().NoError(err)
	s.Nil(errBody)
	s.Require().NotNil(result)
	s.Equal("1", result.ID)
	s.Equal("test", result.Name)
}

func (s *MakeRequestSuite) TestMakeRequest_ErrorResponse() {
	type errorBody struct {
		Message string `json:"message"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(errorBody{Message: "invalid input"})
	}))
	defer server.Close()

	client := NewHTTPClient()
	result, errBody, err := MakeRequest[any, errorBody](s.ctx, client, http.MethodGet, server.URL, nil, nil)

	s.Require().NoError(err)
	s.Nil(result)
	s.Require().NotNil(errBody)
	s.Equal("invalid input", errBody.Message)
}

func (s *MakeRequestSuite) TestMakeRequestWithLimit_ResponseTooLarge() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":"` + strings.Repeat("x", 100) + `"}`))
	}))
	defer server.Close()

	client := NewHTTPClient()
	result, errBody, err := MakeRequestWithLimit[any, any](s.ctx, client, http.MethodGet, server.URL, nil, nil, 10)

	s.Nil(result)
	s.Nil(errBody)
	s.Require().Error(err)
	s.True(errors.Is(err, ErrResponseTooLarge))
}

func (s *MakeRequestSuite) TestMakeRequest_WithHeaders() {
	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := NewHTTPClient()
	headers := map[string]string{"Authorization": "Bearer token"}
	_, _, err := MakeRequest[any, any](s.ctx, client, http.MethodGet, server.URL, headers, nil)

	s.Require().NoError(err)
	s.Equal("Bearer token", receivedAuth)
}

type BaseClientSuite struct {
	suite.Suite
	ctx context.Context
}

func TestBaseClientSuite(t *testing.T) {
	suite.Run(t, new(BaseClientSuite))
}

func (s *BaseClientSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *BaseClientSuite) closeBody(c io.Closer) {
	s.T().Helper()
	s.NoError(c.Close())
}

func (s *BaseClientSuite) TestNewBaseClient_Defaults() {
	client := NewBaseClient()

	s.NotNil(client)
	s.Equal(DefaultTimeout, client.timeout)
	s.Equal(int64(DefaultMaxRequestBodySize), client.maxBodySize)
	s.NotNil(client.httpClient)
}

func (s *BaseClientSuite) TestNewBaseClient_WithOptions() {
	client := NewBaseClient(
		WithTimeout(5*time.Second),
		WithBodySize(1024),
	)

	s.Equal(5*time.Second, client.timeout)
	s.Equal(int64(1024), client.maxBodySize)
}

func (s *BaseClientSuite) TestBaseClient_Do_ImplementsHTTPClient() {
	var _ HTTPClient = (*BaseClient)(nil)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := NewBaseClient()
	req, err := http.NewRequestWithContext(s.ctx, http.MethodGet, server.URL, nil)
	s.Require().NoError(err)

	resp, err := client.Do(req)
	s.Require().NoError(err)
	defer s.closeBody(resp.Body)
	s.Equal(http.StatusOK, resp.StatusCode)
}

func (s *BaseClientSuite) TestBaseClient_Get() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Equal(http.MethodGet, r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewBaseClient()
	resp, err := client.Get(s.ctx, server.URL)
	s.Require().NoError(err)
	defer s.closeBody(resp.Body)
	s.Equal(http.StatusOK, resp.StatusCode)
}

func (s *BaseClientSuite) TestBaseClient_Retry() {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewBaseClient()
	resp, err := client.Get(s.ctx, server.URL,
		WithRetry(3, 10*time.Millisecond, DefaultRetryPolicy),
	)
	s.Require().NoError(err)
	defer s.closeBody(resp.Body)
	s.Equal(http.StatusOK, resp.StatusCode)
	s.Equal(3, attempts)
}

func (s *BaseClientSuite) TestBaseClient_RetryMaxAttemptsBodyReadable() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"unavailable"}`))
	}))
	defer server.Close()

	client := NewBaseClient()
	resp, err := client.Get(s.ctx, server.URL, WithRetry(2, 0, DefaultRetryPolicy))
	s.Require().NoError(err)
	s.Require().NotNil(resp)
	defer s.closeBody(resp.Body)

	s.Equal(http.StatusServiceUnavailable, resp.StatusCode)

	body, readErr := io.ReadAll(resp.Body)
	s.Require().NoError(readErr)
	s.Equal(`{"error":"unavailable"}`, string(body))
}

func (s *BaseClientSuite) TestBaseClient_RetryBodyBufferingLimit() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewBaseClient(WithBodySize(10))
	resp, err := client.Post(s.ctx, server.URL,
		strings.NewReader("this body is larger than 10 bytes"),
		WithRetry(3, 10*time.Millisecond, DefaultRetryPolicy),
	)
	if resp != nil {
		defer s.closeBody(resp.Body)
	}

	s.Require().Error(err)
	s.True(errors.Is(err, ErrRequestBodyTooLarge))
}

func (s *BaseClientSuite) TestBaseClient_Timeout() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewBaseClient(WithTimeout(50 * time.Millisecond))
	resp, err := client.Get(s.ctx, server.URL)
	if resp != nil {
		defer s.closeBody(resp.Body)
	}

	s.Require().Error(err)
}

func (s *BaseClientSuite) TestBaseClient_RetryContextCancellation() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(s.ctx, 50*time.Millisecond)
	defer cancel()

	client := NewBaseClient()
	resp, err := client.Get(ctx, server.URL,
		WithRetry(3, 10*time.Millisecond, DefaultRetryPolicy),
	)
	if resp != nil {
		defer s.closeBody(resp.Body)
	}

	s.Require().Error(err)
	s.True(errors.Is(err, context.DeadlineExceeded))
}

func (s *BaseClientSuite) TestMakeRequest_WithBaseClient() {
	type successBody struct {
		Status string `json:"status"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(successBody{Status: "ok"})
	}))
	defer server.Close()

	client := NewBaseClient()
	result, errBody, err := MakeRequest[successBody, any](s.ctx, client, http.MethodGet, server.URL, nil, nil)

	s.Require().NoError(err)
	s.Nil(errBody)
	s.Require().NotNil(result)
	s.Equal("ok", result.Status)
}

type RetryPolicySuite struct {
	suite.Suite
}

func TestRetryPolicySuite(t *testing.T) {
	suite.Run(t, new(RetryPolicySuite))
}

func (s *RetryPolicySuite) TestDefaultRetryPolicy() {
	scenarios := []struct {
		name     string
		err      error
		resp     *http.Response
		expected bool
	}{
		{name: "network error retries", err: errors.New("network error"), resp: nil, expected: true},
		{name: "5xx retries", err: nil, resp: &http.Response{StatusCode: http.StatusInternalServerError}, expected: true},
		{name: "4xx does not retry", err: nil, resp: &http.Response{StatusCode: http.StatusBadRequest}, expected: false},
		{name: "context canceled does not retry", err: context.Canceled, resp: nil, expected: false},
		{name: "context deadline does not retry", err: context.DeadlineExceeded, resp: nil, expected: false},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			s.Equal(sc.expected, DefaultRetryPolicy(sc.err, sc.resp))
		})
	}
}

func (s *RetryPolicySuite) TestIdempotentRetryPolicy() {
	scenarios := []struct {
		name     string
		err      error
		resp     *http.Response
		expected bool
	}{
		{name: "429 retries", err: nil, resp: &http.Response{StatusCode: http.StatusTooManyRequests}, expected: true},
		{name: "5xx retries", err: nil, resp: &http.Response{StatusCode: http.StatusServiceUnavailable}, expected: true},
		{name: "context canceled does not retry", err: context.Canceled, resp: nil, expected: false},
		{name: "context deadline does not retry", err: context.DeadlineExceeded, resp: nil, expected: false},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			s.Equal(sc.expected, IdempotentRetryPolicy(sc.err, sc.resp))
		})
	}
}

func (s *RetryPolicySuite) TestNoRetryPolicy() {
	s.False(NoRetryPolicy(errors.New("any"), nil))
	s.False(NoRetryPolicy(nil, &http.Response{StatusCode: http.StatusInternalServerError}))
}
